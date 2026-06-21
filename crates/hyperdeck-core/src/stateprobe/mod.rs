//! State probe strategies (port of `internal/adapter/driven/stateprobe`).

mod titleregex;
mod uia;

pub use titleregex::{NoneProbe, TitleRegex};
pub use uia::{ElementNamer, SharedElementNamer, Uia};

use std::sync::Arc;

use crate::app::SharedStateProbe;
use crate::domain::Profile;

/// Builds the state probe named by the profile's `state.type`. `namer` supplies
/// UI Automation reads for the `uia` probe and may be `None` when no uia profile
/// is in use.
pub fn new(p: &Profile, namer: Option<SharedElementNamer>) -> SharedStateProbe {
    match p.state.kind.as_str() {
        "title_regex" => Arc::new(TitleRegex::new(&p.state.playing)),
        "uia" => Arc::new(Uia::new(namer, &p.state.automation_id, &p.state.playing)),
        _ => Arc::new(NoneProbe),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::domain::{Profile, StateConfig};

    #[test]
    fn factory_none_does_not_detect() {
        let p = Profile {
            state: StateConfig {
                kind: "none".into(),
                ..Default::default()
            },
            ..Default::default()
        };
        assert!(new(&p, None).detect(&Default::default()).is_none());
    }

    #[test]
    fn factory_builds_uia() {
        let p = Profile {
            state: StateConfig {
                kind: "uia".into(),
                automation_id: "TogglePlaybackButton".into(),
                playing: "Pause".into(),
            },
            ..Default::default()
        };
        let namer: SharedElementNamer = Arc::new(uia::tests::FakeNamer::ok("Pause"));
        assert_eq!(
            new(&p, Some(namer)).detect(&Default::default()),
            Some(crate::domain::TransportState::Playing)
        );
    }
}
