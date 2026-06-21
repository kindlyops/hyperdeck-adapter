//! State reconciler (port of `internal/core/app/reconciler.go`).

use std::sync::Arc;

use super::session::Session;

/// Refreshes the clip list and corrects modeled state from the probe.
pub struct Reconciler {
    session: Arc<Session>,
}

impl Reconciler {
    /// Wires a reconciler to the session.
    pub fn new(session: Arc<Session>) -> Self {
        Reconciler { session }
    }

    /// Runs one reconciliation cycle. Safe to call when unlocked.
    pub fn tick(&self) {
        let Some((_, w)) = self.session.active() else {
            return;
        };
        if let Some(cs) = self.session.clip_source() {
            if let Ok(clips) = cs.list() {
                self.session.set_clips(clips);
            }
        }
        if let Some(probe) = self.session.probe() {
            if let Some(state) = probe.detect(&w) {
                self.session.set_state(state);
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::domain::{Clip, Profile, TransportState, Window};
    use crate::testsupport::{FakeClipSource, PlayingProbe};

    #[test]
    fn refreshes_clips_and_state() {
        let s = Arc::new(Session::new());
        s.lock(
            Profile {
                id: "vlc".into(),
                ..Default::default()
            },
            Window::default(),
            Some(Arc::new(FakeClipSource {
                clips: vec![Clip {
                    id: 1,
                    name: "a".into(),
                    ..Default::default()
                }],
            })),
            Some(Arc::new(PlayingProbe)),
        );
        let r = Reconciler::new(s.clone());
        r.tick();
        assert_eq!(s.clips().len(), 1);
        assert_eq!(s.state(), TransportState::Playing);
    }

    #[test]
    fn noop_when_unlocked() {
        let s = Arc::new(Session::new());
        let r = Reconciler::new(s);
        r.tick(); // must not panic with no clip source / probe
    }
}
