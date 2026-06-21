//! Title-regex + none probes
//! (port of `titleregex.go` and `none.go`).

use regex::Regex;

use crate::domain::{TransportState, Window};
use crate::port::StateProbe;

/// Infers playing state from the window title.
pub struct TitleRegex {
    re: Option<Regex>,
}

impl TitleRegex {
    /// Compiles `pattern`; an invalid pattern yields a probe that never detects.
    pub fn new(pattern: &str) -> Self {
        TitleRegex {
            re: Regex::new(pattern).ok(),
        }
    }
}

impl StateProbe for TitleRegex {
    fn detect(&self, w: &Window) -> Option<TransportState> {
        let re = self.re.as_ref()?;
        if re.is_match(&w.title) {
            Some(TransportState::Playing)
        } else {
            Some(TransportState::Stopped)
        }
    }
}

/// Performs no detection; the modeled state is authoritative.
pub struct NoneProbe;

impl StateProbe for NoneProbe {
    fn detect(&self, _w: &Window) -> Option<TransportState> {
        None
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn win(title: &str) -> Window {
        Window {
            title: title.into(),
            ..Default::default()
        }
    }

    #[test]
    fn title_regex_detects_playing_and_stopped() {
        let p = TitleRegex::new(".+ - VLC");
        assert_eq!(p.detect(&win("Movie - VLC")), Some(TransportState::Playing));
        assert_eq!(p.detect(&win("idle")), Some(TransportState::Stopped));
    }

    #[test]
    fn none_never_detects() {
        assert!(NoneProbe.detect(&win("x")).is_none());
    }
}
