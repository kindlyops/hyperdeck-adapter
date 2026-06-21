//! Shared deck session state (port of `internal/core/app/session.go`).

use std::sync::{Arc, Mutex};

use crate::domain::{ClipList, LockState, Profile, TransportState, Window};
use crate::port::{ClipSource, StateProbe};

/// A clip source shared with the reconciler (cloned out under the lock so the
/// potentially-blocking `list()` runs without holding the mutex).
pub type SharedClipSource = Arc<dyn ClipSource + Send + Sync>;
/// A state probe shared with the reconciler.
pub type SharedStateProbe = Arc<dyn StateProbe + Send + Sync>;

struct Inner {
    lock: LockState,
    state: TransportState,
    clips: ClipList,
    current_clip: i32,
    clip_source: Option<SharedClipSource>,
    probe: Option<SharedStateProbe>,
}

/// The mutex-guarded shared state of the running deck.
pub struct Session {
    inner: Mutex<Inner>,
}

impl Session {
    /// Returns an unlocked session in the stopped state.
    pub fn new() -> Self {
        Session {
            inner: Mutex::new(Inner {
                lock: LockState::default(),
                state: TransportState::Stopped,
                clips: ClipList::new(),
                current_clip: 1,
                clip_source: None,
                probe: None,
            }),
        }
    }

    /// Binds an active profile, window, clip source, and state probe.
    pub fn lock(
        &self,
        p: Profile,
        w: Window,
        cs: Option<SharedClipSource>,
        sp: Option<SharedStateProbe>,
    ) {
        let mut g = self.inner.lock().unwrap();
        g.lock = LockState {
            locked: true,
            profile: Some(p),
            window: w,
        };
        g.clip_source = cs;
        g.probe = sp;
        g.state = TransportState::Stopped;
        g.current_clip = 1;
    }

    /// Clears the active binding.
    pub fn unlock(&self) {
        let mut g = self.inner.lock().unwrap();
        g.lock = LockState::default();
        g.clip_source = None;
        g.probe = None;
        g.clips = ClipList::new();
    }

    /// Returns the locked profile and window, or `None` when unlocked.
    pub fn active(&self) -> Option<(Profile, Window)> {
        let g = self.inner.lock().unwrap();
        match (g.lock.locked, &g.lock.profile) {
            (true, Some(p)) => Some((p.clone(), g.lock.window.clone())),
            _ => None,
        }
    }

    /// Returns a copy of the current lock status.
    pub fn lock_state(&self) -> LockState {
        self.inner.lock().unwrap().lock.clone()
    }

    pub fn state(&self) -> TransportState {
        self.inner.lock().unwrap().state
    }

    pub fn set_state(&self, st: TransportState) {
        self.inner.lock().unwrap().state = st;
    }

    /// Atomically sets the modeled state to `want` and reports whether it
    /// differed from the previous state. Folding read-modify-write into one lock
    /// acquisition prevents concurrent transport commands from making a wrong
    /// toggle decision.
    pub fn set_state_if_changed(&self, want: TransportState) -> bool {
        let mut g = self.inner.lock().unwrap();
        if g.state == want {
            return false;
        }
        g.state = want;
        true
    }

    pub fn clips(&self) -> ClipList {
        self.inner.lock().unwrap().clips.clone()
    }

    pub fn set_clips(&self, c: ClipList) {
        self.inner.lock().unwrap().clips = c;
    }

    pub fn current_clip(&self) -> i32 {
        self.inner.lock().unwrap().current_clip
    }

    pub fn set_current_clip(&self, n: i32) {
        self.inner.lock().unwrap().current_clip = n;
    }

    /// Returns the active clip source (`None` when unlocked).
    pub fn clip_source(&self) -> Option<SharedClipSource> {
        self.inner.lock().unwrap().clip_source.clone()
    }

    /// Returns the active state probe (`None` when unlocked).
    pub fn probe(&self) -> Option<SharedStateProbe> {
        self.inner.lock().unwrap().probe.clone()
    }
}

impl Default for Session {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn lock_unlock() {
        let s = Session::new();
        assert!(s.active().is_none());
        s.lock(
            Profile {
                id: "vlc".into(),
                ..Default::default()
            },
            Window {
                process: "vlc.exe".into(),
                ..Default::default()
            },
            None,
            None,
        );
        let (p, w) = s.active().expect("locked");
        assert_eq!(p.id, "vlc");
        assert_eq!(w.process, "vlc.exe");
        s.unlock();
        assert!(s.active().is_none());
    }

    #[test]
    fn state_and_clip() {
        let s = Session::new();
        s.set_state(TransportState::Playing);
        assert_eq!(s.state(), TransportState::Playing);
        s.set_clips(vec![Default::default(), Default::default()]);
        assert_eq!(s.clips().len(), 2);
        s.set_current_clip(2);
        assert_eq!(s.current_clip(), 2);
    }

    #[test]
    fn set_state_if_changed_reports_transition() {
        let s = Session::new();
        assert!(s.set_state_if_changed(TransportState::Playing));
        assert!(!s.set_state_if_changed(TransportState::Playing));
        assert!(s.set_state_if_changed(TransportState::Stopped));
    }
}
