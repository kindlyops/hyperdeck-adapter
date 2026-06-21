//! Player lock manager (port of `internal/core/app/lockmanager.go`).

use std::sync::{Arc, Mutex};

use super::session::{Session, SharedClipSource, SharedStateProbe};
use crate::domain::{Profile, Window};
use crate::port::{StatusPresenter, WindowEnumerator};

/// Builds a clip source for a profile (supplied by the composition root).
pub type ClipSourceFactory = Box<dyn Fn(&Profile) -> SharedClipSource + Send + Sync>;
/// Builds a state probe for a profile.
pub type StateProbeFactory = Box<dyn Fn(&Profile) -> SharedStateProbe + Send + Sync>;

/// A window enumerator shared with the lock manager.
pub type SharedEnumerator = Arc<dyn WindowEnumerator + Send + Sync>;
/// A status presenter shared with the lock manager.
pub type SharedPresenter = Arc<dyn StatusPresenter + Send + Sync>;

/// Binds a running player to the session by matching profiles.
pub struct LockManager {
    session: Arc<Session>,
    windows: SharedEnumerator,
    profiles: Vec<Profile>,
    presenter: SharedPresenter,
    clips_for: ClipSourceFactory,
    probe_for: StateProbeFactory,
    /// Pinned profile id; empty means Auto / match any.
    active: Mutex<String>,
}

impl LockManager {
    /// Wires a lock manager.
    pub fn new(
        session: Arc<Session>,
        windows: SharedEnumerator,
        profiles: Vec<Profile>,
        presenter: SharedPresenter,
        clips_for: ClipSourceFactory,
        probe_for: StateProbeFactory,
    ) -> Self {
        LockManager {
            session,
            windows,
            profiles,
            presenter,
            clips_for,
            probe_for,
            active: Mutex::new(String::new()),
        }
    }

    /// Runs one match cycle: lock on the first match, unlock when the locked
    /// window is gone.
    pub fn poll(&self) {
        let windows = self.windows.open_windows().unwrap_or_default();
        if let Some((profile, win)) = self.first_match(&windows) {
            let already = matches!(self.session.active(), Some((cur, _)) if cur.id == profile.id);
            if !already {
                let cs = (self.clips_for)(&profile);
                let sp = (self.probe_for)(&profile);
                self.session.lock(profile, win, Some(cs), Some(sp));
                self.presenter.present(&self.session.lock_state());
            }
            return;
        }
        if self.session.active().is_some() {
            self.session.unlock();
            self.presenter.present(&self.session.lock_state());
        }
    }

    /// Pins the matcher to one profile id; empty restores Auto (match any). It
    /// re-polls immediately so the lock state reflects the new selection.
    pub fn set_active(&self, id: &str) {
        *self.active.lock().unwrap() = id.to_string();
        self.poll();
    }

    fn first_match(&self, windows: &[Window]) -> Option<(Profile, Window)> {
        let active = self.active.lock().unwrap().clone();
        for p in &self.profiles {
            if !active.is_empty() && p.id != active {
                continue;
            }
            for w in windows {
                if p.matches_window(w) {
                    return Some((p.clone(), w.clone()));
                }
            }
        }
        None
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::domain::{KeyName, Keymap, Match};
    use crate::testsupport::{FakeClipSource, FakePresenter, MockInjector, NoProbe};

    fn vlc_profile() -> Profile {
        let mut km = Keymap::new();
        km.insert(
            KeyName::Play,
            crate::domain::Chord {
                mods: vec![],
                key: "space".into(),
            },
        );
        Profile {
            id: "vlc".into(),
            r#match: Match {
                process: vec!["vlc.exe".into()],
                title_regex: "VLC media player".into(),
            },
            keymap: km,
            ..Default::default()
        }
    }

    fn mitti_profile() -> Profile {
        Profile {
            id: "mitti".into(),
            r#match: Match {
                process: vec!["Mitti".into()],
                title_regex: String::new(),
            },
            ..Default::default()
        }
    }

    fn example_profile() -> Profile {
        Profile {
            id: "example".into(),
            r#match: Match {
                process: vec!["Example Player".into()],
                title_regex: String::new(),
            },
            ..Default::default()
        }
    }

    fn factories() -> (ClipSourceFactory, StateProbeFactory) {
        (
            Box::new(|_p| Arc::new(FakeClipSource { clips: vec![] }) as SharedClipSource),
            Box::new(|_p| Arc::new(NoProbe) as SharedStateProbe),
        )
    }

    fn win(process: &str, title: &str) -> Window {
        Window {
            process: process.into(),
            title: title.into(),
            ..Default::default()
        }
    }

    struct Harness {
        session: Arc<Session>,
        injector: Arc<MockInjector>,
        presenter: Arc<FakePresenter>,
        lm: LockManager,
    }

    fn harness(profiles: Vec<Profile>) -> Harness {
        let session = Arc::new(Session::new());
        let injector = Arc::new(MockInjector::new());
        let presenter = Arc::new(FakePresenter::new());
        let (clips_for, probe_for) = factories();
        let lm = LockManager::new(
            session.clone(),
            injector.clone(),
            profiles,
            presenter.clone(),
            clips_for,
            probe_for,
        );
        Harness {
            session,
            injector,
            presenter,
            lm,
        }
    }

    #[test]
    fn locks_on_match() {
        let h = harness(vec![vlc_profile()]);
        h.injector
            .set_windows(vec![win("vlc.exe", "x - VLC media player")]);
        h.lm.poll();
        assert!(h.session.active().is_some());
        assert!(h.presenter.last().expect("presented").locked);
    }

    #[test]
    fn unlocks_when_gone() {
        let h = harness(vec![vlc_profile()]);
        h.injector
            .set_windows(vec![win("vlc.exe", "x - VLC media player")]);
        h.lm.poll();
        h.injector.set_windows(vec![]); // player closed
        h.lm.poll();
        assert!(h.session.active().is_none());
        assert!(!h.presenter.last().expect("presented").locked);
    }

    #[test]
    fn relocks_on_profile_change() {
        let h = harness(vec![vlc_profile(), example_profile()]);
        h.injector
            .set_windows(vec![win("vlc.exe", "x - VLC media player")]);
        h.lm.poll();
        assert_eq!(h.session.active().unwrap().0.id, "vlc");

        h.injector
            .set_windows(vec![win("Example Player", "Example Player")]);
        h.lm.poll();
        assert_eq!(h.session.active().unwrap().0.id, "example");
        let last = h.presenter.last().expect("presented");
        assert_eq!(last.profile.expect("profile").id, "example");
    }

    #[test]
    fn pinned_matches_only_active() {
        let h = harness(vec![vlc_profile(), mitti_profile()]);
        h.injector.set_windows(vec![
            win("vlc.exe", "x - VLC media player"),
            win("Mitti", "Mitti"),
        ]);
        h.lm.set_active("mitti"); // pins mitti even though vlc also matches
        assert_eq!(h.session.active().expect("locked").0.id, "mitti");
    }

    #[test]
    fn pinned_not_running_stays_unlocked() {
        let h = harness(vec![vlc_profile(), mitti_profile()]);
        h.injector
            .set_windows(vec![win("vlc.exe", "x - VLC media player")]);
        h.lm.set_active("mitti"); // mitti not in the window list
        assert!(h.session.active().is_none());
    }
}
