//! Virtual deck: transport + query over a session
//! (port of `internal/core/app/virtualdeck.go`).

use std::sync::Arc;

use super::session::Session;
use crate::domain::{
    Chord, ClipList, DeviceInfo, KeyName, Profile, SlotInfo, TransportInfo, TransportState, Window,
};
use crate::error::{DeckError, DeckResult};
use crate::port::{KeyInjector, PlayerController, Query, Transport};

/// A key injector shared across the deck.
pub type SharedInjector = Arc<dyn KeyInjector + Send + Sync>;
/// An out-of-band player controller shared across the deck.
pub type SharedController = Arc<dyn PlayerController + Send + Sync>;

/// Implements [`Transport`] and [`Query`] over a [`Session`].
pub struct VirtualDeck {
    session: Arc<Session>,
    injector: SharedInjector,
    controller: Option<SharedController>,
    device: DeviceInfo,
}

impl VirtualDeck {
    /// Wires the deck to its shared session and key injector.
    pub fn new(session: Arc<Session>, injector: SharedInjector) -> Self {
        VirtualDeck {
            session,
            injector,
            controller: None,
            device: DeviceInfo {
                protocol_version: "1.11".into(),
                model: "HyperDeck Studio Mini".into(),
                unique_id: "hyperdeck-adapter".into(),
            },
        }
    }

    /// Supplies the player-control backend used by API/UIA control profiles.
    /// Without it, such a profile errors when a transport command is issued.
    pub fn with_controller(mut self, c: SharedController) -> Self {
        self.controller = Some(c);
        self
    }

    fn cue_if_navigate_pauses(&self, p: &Profile) {
        if p.cue_on_navigate {
            self.session.set_state(TransportState::Stopped);
        }
    }

    fn send(&self, p: &Profile, w: &Window, key: KeyName) -> DeckResult<()> {
        if !p.has_action(key) {
            return Ok(()); // unmapped action -> acked no-op
        }
        if p.uses_controller() {
            return match &self.controller {
                Some(c) => c.control(p, w, key),
                None => Err(DeckError::Other(format!(
                    "profile {:?} uses {:?} control but no controller is configured",
                    p.id, p.control
                ))),
            };
        }
        if p.injection == crate::domain::InjectionMode::Focus {
            self.injector.focus(w)?;
        }
        let chord = p.keymap.get(&key).cloned().unwrap_or_else(Chord::default);
        self.injector.send_keys(w, &[chord])
    }

    fn step(&self, key: KeyName, delta: i32) -> DeckResult<()> {
        let Some((p, w)) = self.session.active() else {
            return Err(DeckError::NotLocked);
        };
        let n = self.session.clips().len() as i32;
        if n == 0 {
            return Ok(());
        }
        let next = clamp(self.session.current_clip() + delta, 1, n);
        self.send(&p, &w, key)?;
        self.session.set_current_clip(next);
        self.cue_if_navigate_pauses(&p);
        Ok(())
    }
}

impl Transport for VirtualDeck {
    fn play(&self) -> DeckResult<()> {
        let Some((p, w)) = self.session.active() else {
            return Err(DeckError::NotLocked);
        };
        if p.play_toggle {
            // The play key toggles play/pause; emit it only when not already playing.
            if !self.session.set_state_if_changed(TransportState::Playing) {
                return Ok(());
            }
            return self.send(&p, &w, KeyName::Play);
        }
        self.session.set_state(TransportState::Playing);
        self.send(&p, &w, KeyName::Play)
    }

    fn stop(&self) -> DeckResult<()> {
        let Some((p, w)) = self.session.active() else {
            return Err(DeckError::NotLocked);
        };
        if p.has_action(KeyName::Stop) {
            // Discrete stop action (e.g. VLC 's', Mitti panic): always fire it.
            self.session.set_state(TransportState::Stopped);
            return self.send(&p, &w, KeyName::Stop);
        }
        if p.play_toggle {
            // No discrete stop key: pause via the toggle play key, only when playing.
            if !self.session.set_state_if_changed(TransportState::Stopped) {
                return Ok(());
            }
            return self.send(&p, &w, KeyName::Play);
        }
        self.session.set_state(TransportState::Stopped);
        Ok(())
    }

    fn record(&self) -> DeckResult<()> {
        let Some((p, w)) = self.session.active() else {
            return Err(DeckError::NotLocked);
        };
        self.send(&p, &w, KeyName::Record)
    }

    fn goto(&self, clip_id: i32) -> DeckResult<()> {
        let Some((p, w)) = self.session.active() else {
            return Err(DeckError::NotLocked);
        };
        let n = self.session.clips().len() as i32;
        if n == 0 {
            return Ok(());
        }
        let target = clamp(clip_id, 1, n);
        let mut delta = target - self.session.current_clip();
        let key = if delta < 0 {
            delta = -delta;
            KeyName::Prev
        } else {
            KeyName::Next
        };
        for _ in 0..delta {
            self.send(&p, &w, key)?;
        }
        self.session.set_current_clip(target);
        self.cue_if_navigate_pauses(&p);
        Ok(())
    }

    fn next(&self) -> DeckResult<()> {
        self.step(KeyName::Next, 1)
    }

    fn prev(&self) -> DeckResult<()> {
        self.step(KeyName::Prev, -1)
    }

    fn rehome(&self) -> DeckResult<()> {
        let Some((p, w)) = self.session.active() else {
            return Err(DeckError::NotLocked);
        };
        if p.uses_controller() {
            // No keystroke homing for out-of-band control; reset to a known
            // stopped state, issuing a discrete stop first if one is defined.
            if p.has_action(KeyName::Stop) {
                if let Some(c) = &self.controller {
                    c.control(&p, &w, KeyName::Stop)?;
                }
            }
            self.session.set_state(TransportState::Stopped);
            self.session.set_current_clip(1);
            return Ok(());
        }
        if p.injection == crate::domain::InjectionMode::Focus {
            self.injector.focus(&w)?;
        }
        if !p.homing.is_empty() {
            self.injector.send_keys(&w, &p.homing)?;
        }
        self.session.set_state(TransportState::Stopped);
        self.session.set_current_clip(1);
        Ok(())
    }
}

impl Query for VirtualDeck {
    fn transport_info(&self) -> TransportInfo {
        let playing = self.session.state() == TransportState::Playing;
        TransportInfo {
            status: self.session.state().hyperdeck_status().to_string(),
            speed: if playing { 100 } else { 0 },
            clip_id: self.session.current_clip(),
            slot_id: 1,
        }
    }

    fn clips(&self) -> ClipList {
        self.session.clips()
    }

    fn slot_info(&self) -> SlotInfo {
        SlotInfo {
            present: self.session.active().is_some(),
            slot_id: 1,
        }
    }

    fn device_info(&self) -> DeviceInfo {
        self.device.clone()
    }
}

fn clamp(v: i32, lo: i32, hi: i32) -> i32 {
    v.clamp(lo, hi)
}

#[cfg(test)]
mod tests {
    use std::sync::Arc;

    use super::*;
    use crate::domain::{Clip, ClipList, InjectionMode, Keymap, Modifier};
    use crate::testsupport::MockInjector;

    fn locked_session(p: Profile, clips: ClipList) -> Arc<Session> {
        let s = Arc::new(Session::new());
        s.lock(
            p,
            Window {
                process: "x".into(),
                ..Default::default()
            },
            None,
            None,
        );
        s.set_clips(clips);
        s
    }

    fn chord(key: &str) -> Chord {
        Chord {
            mods: vec![],
            key: key.into(),
        }
    }

    fn discrete_profile() -> Profile {
        let mut km = Keymap::new();
        km.insert(KeyName::Play, chord("space"));
        km.insert(KeyName::Stop, chord("s"));
        km.insert(KeyName::Next, chord("n"));
        km.insert(KeyName::Prev, chord("p"));
        Profile {
            id: "vlc".into(),
            injection: InjectionMode::Background,
            keymap: km,
            ..Default::default()
        }
    }

    fn toggle_profile() -> Profile {
        let mut km = Keymap::new();
        km.insert(KeyName::Play, chord("space"));
        km.insert(
            KeyName::Next,
            Chord {
                mods: vec![Modifier::Ctrl],
                key: "right".into(),
            },
        );
        km.insert(
            KeyName::Prev,
            Chord {
                mods: vec![Modifier::Ctrl],
                key: "left".into(),
            },
        );
        Profile {
            id: "example".into(),
            injection: InjectionMode::Focus,
            play_toggle: true,
            keymap: km,
            ..Default::default()
        }
    }

    /// VLC: play key (space) is a toggle, but there is ALSO a discrete stop (s).
    fn vlc_profile() -> Profile {
        let mut p = discrete_profile();
        p.play_toggle = true;
        p
    }

    fn clips(n: i32) -> ClipList {
        (1..=n)
            .map(|id| Clip {
                id,
                ..Default::default()
            })
            .collect()
    }

    fn deck(session: Arc<Session>, inj: &Arc<MockInjector>) -> VirtualDeck {
        VirtualDeck::new(session, inj.clone())
    }

    #[test]
    fn play_stop_discrete() {
        let m = Arc::new(MockInjector::new());
        let d = deck(locked_session(discrete_profile(), ClipList::new()), &m);
        d.play().unwrap();
        d.stop().unwrap();
        assert_eq!(m.sent_keys(), vec!["space", "s"]);
    }

    #[test]
    fn toggle_suppresses_redundant() {
        let m = Arc::new(MockInjector::new());
        let d = deck(locked_session(toggle_profile(), ClipList::new()), &m);
        let _ = d.play();
        let _ = d.play();
        let _ = d.stop();
        let _ = d.stop();
        assert_eq!(m.sent_keys().len(), 2);
    }

    #[test]
    fn toggle_focuses_first() {
        let m = Arc::new(MockInjector::new());
        let d = deck(locked_session(toggle_profile(), ClipList::new()), &m);
        let _ = d.play();
        assert_eq!(m.focused().len(), 1);
    }

    #[test]
    fn toggle_stop_emits_play_key() {
        let m = Arc::new(MockInjector::new());
        let d = deck(locked_session(toggle_profile(), ClipList::new()), &m);
        let _ = d.play();
        let _ = d.stop();
        let got = m.sent_keys();
        assert_eq!(got.len(), 2);
        assert_eq!(got[1], "space");
    }

    #[test]
    fn vlc_toggled_play_with_discrete_stop() {
        let m = Arc::new(MockInjector::new());
        let d = deck(locked_session(vlc_profile(), ClipList::new()), &m);
        let _ = d.play(); // space
        let _ = d.play(); // suppressed
        let _ = d.stop(); // discrete "s"
        assert_eq!(m.sent_keys(), vec!["space", "s"]);
    }

    #[test]
    fn vlc_stop_when_stopped_still_emits_discrete_stop() {
        let m = Arc::new(MockInjector::new());
        let d = deck(locked_session(vlc_profile(), ClipList::new()), &m);
        let _ = d.stop();
        assert_eq!(m.sent_keys(), vec!["s"]);
    }

    #[test]
    fn goto_computes_delta() {
        let m = Arc::new(MockInjector::new());
        let s = locked_session(discrete_profile(), clips(5));
        let d = deck(s.clone(), &m);
        d.goto(4).unwrap();
        let got = m.sent_keys();
        assert_eq!(got.len(), 3);
        assert_eq!(got[0], "n");
        assert_eq!(s.current_clip(), 4);
        m.clear_sent();
        d.goto(2).unwrap();
        let got = m.sent_keys();
        assert_eq!(got.len(), 2);
        assert_eq!(got[0], "p");
    }

    #[test]
    fn goto_clamps_to_range() {
        let m = Arc::new(MockInjector::new());
        let s = locked_session(discrete_profile(), clips(2));
        let d = deck(s.clone(), &m);
        d.goto(99).unwrap();
        assert_eq!(s.current_clip(), 2);
    }

    #[test]
    fn next_prev_clamp_at_boundaries() {
        let m = Arc::new(MockInjector::new());
        let s = locked_session(discrete_profile(), clips(2));
        let d = deck(s.clone(), &m);
        d.prev().unwrap();
        assert_eq!(s.current_clip(), 1);
        s.set_current_clip(2);
        d.next().unwrap();
        assert_eq!(s.current_clip(), 2);
    }

    #[test]
    fn navigation_empty_clips_noop() {
        let m = Arc::new(MockInjector::new());
        let d = deck(locked_session(discrete_profile(), ClipList::new()), &m);
        d.goto(3).unwrap();
        d.next().unwrap();
        assert!(m.sent_keys().is_empty());
    }

    #[test]
    fn record_undefined_is_noop() {
        let m = Arc::new(MockInjector::new());
        let d = deck(locked_session(discrete_profile(), ClipList::new()), &m);
        d.record().unwrap();
        assert!(m.sent_keys().is_empty());
    }

    #[test]
    fn commands_without_lock_error() {
        let m = Arc::new(MockInjector::new());
        let d = VirtualDeck::new(Arc::new(Session::new()), m);
        assert_eq!(d.play(), Err(DeckError::NotLocked));
    }

    fn cue_mode_profile() -> Profile {
        let mut p = toggle_profile();
        p.cue_on_navigate = true;
        p
    }

    #[test]
    fn cue_on_navigate_leaves_deck_stopped_so_play_starts() {
        let m = Arc::new(MockInjector::new());
        let s = locked_session(cue_mode_profile(), clips(3));
        let d = deck(s.clone(), &m);
        let _ = d.play(); // stopped -> playing: space
        let _ = d.next(); // ctrl+right, then cued paused -> stopped
        assert_eq!(s.state(), TransportState::Stopped);
        let _ = d.play(); // must re-emit space
        assert_eq!(m.sent_keys(), vec!["space", "right", "space"]);
    }

    #[test]
    fn without_cue_on_navigate_play_stays_suppressed() {
        let m = Arc::new(MockInjector::new());
        let s = locked_session(toggle_profile(), clips(3));
        let d = deck(s, &m);
        let _ = d.play(); // space
        let _ = d.next(); // right; state stays playing
        let _ = d.play(); // suppressed
        assert_eq!(m.sent_keys(), vec!["space", "right"]);
    }

    #[test]
    fn cue_on_navigate_goto_also_cues() {
        let m = Arc::new(MockInjector::new());
        let s = locked_session(cue_mode_profile(), clips(4));
        let d = deck(s.clone(), &m);
        s.set_state(TransportState::Playing);
        d.goto(3).unwrap();
        assert_eq!(s.state(), TransportState::Stopped);
    }

    #[test]
    fn rehome_runs_homing_and_resets() {
        let m = Arc::new(MockInjector::new());
        let mut km = Keymap::new();
        km.insert(KeyName::Play, chord("space"));
        let p = Profile {
            id: "vlc".into(),
            injection: InjectionMode::Focus,
            keymap: km,
            homing: vec![chord("s")],
            ..Default::default()
        };
        let s = locked_session(p, clips(3));
        s.set_state(TransportState::Playing);
        s.set_current_clip(3);
        let d = deck(s.clone(), &m);
        d.rehome().unwrap();
        assert_eq!(m.focused().len(), 1);
        assert_eq!(m.sent_keys(), vec!["s"]);
        assert_eq!(s.state(), TransportState::Stopped);
        assert_eq!(s.current_clip(), 1);
    }
}
