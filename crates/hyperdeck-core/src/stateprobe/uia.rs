//! UI Automation state probe (port of `stateprobe/uia.go`).
//!
//! Only the element *read* ([`ElementNamer`]) is OS-specific; the detection logic
//! is pure and lives here with its tests. The Windows namer is supplied by the
//! OS adapter crate.

use std::sync::Arc;

use regex::Regex;

use crate::domain::{TransportState, Window};
use crate::error::DeckResult;
use crate::port::StateProbe;

/// Reads the Name of a UI Automation element (by AutomationId) on a window (by
/// HWND). Implemented by the uia engine; error reads are treated as
/// "not detectable".
pub trait ElementNamer {
    fn name(&self, hwnd: usize, automation_id: &str) -> DeckResult<String>;
}

/// A shared element namer.
pub type SharedElementNamer = Arc<dyn ElementNamer + Send + Sync>;

/// Infers playing state from a UI Automation element's Name — e.g. Example
/// Player's TogglePlaybackButton is named "Pause" while playing and "Play" while
/// paused.
pub struct Uia {
    namer: Option<SharedElementNamer>,
    aid: String,
    re: Option<Regex>,
}

impl Uia {
    /// Builds a probe that reads `automation_id`'s Name and reports playing when
    /// it matches `playing_pattern`. An invalid pattern or absent namer yields a
    /// probe that never detects.
    pub fn new(
        namer: Option<SharedElementNamer>,
        automation_id: &str,
        playing_pattern: &str,
    ) -> Self {
        Uia {
            namer,
            aid: automation_id.to_string(),
            re: Regex::new(playing_pattern).ok(),
        }
    }
}

impl StateProbe for Uia {
    fn detect(&self, w: &Window) -> Option<TransportState> {
        let namer = self.namer.as_ref()?;
        let re = self.re.as_ref()?;
        // A read that fails or finds nothing (controls hidden, no clip open)
        // reports not-detected, so the modeled state is left untouched.
        let name = namer.name(w.handle, &self.aid).ok()?;
        if name.is_empty() {
            return None;
        }
        if re.is_match(&name) {
            Some(TransportState::Playing)
        } else {
            Some(TransportState::Stopped)
        }
    }
}

#[cfg(test)]
pub(crate) mod tests {
    use super::*;

    /// Returns a fixed element Name (port of the Go `fakeNamer`).
    pub struct FakeNamer {
        name: String,
        ok: bool,
    }

    impl FakeNamer {
        pub fn ok(name: &str) -> Self {
            FakeNamer {
                name: name.into(),
                ok: true,
            }
        }
        pub fn err() -> Self {
            FakeNamer {
                name: String::new(),
                ok: false,
            }
        }
    }

    impl ElementNamer for FakeNamer {
        fn name(&self, _hwnd: usize, _automation_id: &str) -> DeckResult<String> {
            if self.ok {
                Ok(self.name.clone())
            } else {
                Err(crate::error::DeckError::Other("uia read failed".into()))
            }
        }
    }

    fn probe(namer: FakeNamer) -> Uia {
        Uia::new(Some(Arc::new(namer)), "TogglePlaybackButton", "Pause")
    }

    #[test]
    fn detects_from_name() {
        assert_eq!(
            probe(FakeNamer::ok("Pause")).detect(&Window::default()),
            Some(TransportState::Playing)
        );
        assert_eq!(
            probe(FakeNamer::ok("Play")).detect(&Window::default()),
            Some(TransportState::Stopped)
        );
    }

    #[test]
    fn not_detected_when_absent() {
        assert!(probe(FakeNamer::ok(""))
            .detect(&Window::default())
            .is_none());
        assert!(probe(FakeNamer::err()).detect(&Window::default()).is_none());
    }
}
