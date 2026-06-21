//! Player profile definition (port of `internal/core/domain/profile.go`).

use std::collections::HashMap;

use regex::Regex;

use super::{Chord, Window};

/// A logical transport action mapped to a chord by a profile.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum KeyName {
    Play,
    Stop,
    Record,
    Next,
    Prev,
}

impl KeyName {
    /// The lowercase config token for this action (e.g. `"play"`).
    pub fn as_str(self) -> &'static str {
        match self {
            KeyName::Play => "play",
            KeyName::Stop => "stop",
            KeyName::Record => "record",
            KeyName::Next => "next",
            KeyName::Prev => "prev",
        }
    }
}

/// Selects how keystrokes reach the target window.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum InjectionMode {
    #[default]
    Focus,
    Background,
}

/// Selects how transport commands reach the player.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum ControlMode {
    /// Synthesize keystrokes via the injector (the default; Go's `""`/`"keys"`).
    #[default]
    Keys,
    /// Drive the player through its control API.
    Api,
    /// Invoke the player's UI Automation controls.
    Uia,
}

/// Parameterizes a [`ControlMode::Api`] profile's control channel.
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct ApiConfig {
    /// Currently only `"vlc_http"`.
    pub kind: String,
    /// e.g. `"http://127.0.0.1:8080"`.
    pub base_url: String,
    /// Control-API password (VLC HTTP uses Basic auth, empty user).
    pub password: String,
}

/// Maps logical actions to concrete chords.
pub type Keymap = HashMap<KeyName, Chord>;

/// Describes how to recognize the player's window.
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct Match {
    pub process: Vec<String>,
    pub title_regex: String,
}

/// Selects and parameterizes the clip source strategy.
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct ClipSourceConfig {
    /// `"playlist_file"` | `"positional"` | `"mitti"`.
    pub kind: String,
    pub path: String,
    pub count: i32,
}

/// Selects and parameterizes best-effort state detection.
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct StateConfig {
    /// `"title_regex"` | `"uia"` | `"none"`.
    pub kind: String,
    /// Regex meaning "playing" vs the window title (title_regex) or UIA element Name (uia).
    pub playing: String,
    /// UIA element whose Name is read when `kind == "uia"`.
    pub automation_id: String,
}

/// One player application's complete mapping definition.
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct Profile {
    pub id: String,
    pub r#match: Match,
    /// How transport reaches the player; [`ControlMode::Keys`] is the default.
    pub control: ControlMode,
    pub injection: InjectionMode,
    /// Control channel for [`ControlMode::Api`] profiles.
    pub api: ApiConfig,
    /// Action -> UI Automation AutomationId, for [`ControlMode::Uia`] profiles.
    pub uia: HashMap<KeyName, String>,
    pub keymap: Keymap,
    /// When true, the play key toggles play/pause (e.g. Space in VLC).
    pub play_toggle: bool,
    /// When true, next/prev/goto cue the clip paused rather than playing it.
    pub cue_on_navigate: bool,
    pub clip_source: ClipSourceConfig,
    pub state: StateConfig,
    pub homing: Vec<Chord>,
}

impl Profile {
    /// Reports whether the profile is driven by an out-of-band player controller
    /// (API or UIA) rather than by synthesized keystrokes.
    pub fn uses_controller(&self) -> bool {
        matches!(self.control, ControlMode::Api | ControlMode::Uia)
    }

    /// Reports whether the profile defines the given transport action. For UIA
    /// profiles the source of truth is the UIA map; otherwise it is the keymap.
    pub fn has_action(&self, key: KeyName) -> bool {
        if self.control == ControlMode::Uia {
            self.uia.contains_key(&key)
        } else {
            self.keymap.contains_key(&key)
        }
    }

    /// Reports whether the window `w` belongs to this profile.
    pub fn matches_window(&self, w: &Window) -> bool {
        if !self.r#match.process.iter().any(|p| p == &w.process) {
            return false;
        }
        if self.r#match.title_regex.is_empty() {
            return true;
        }
        match Regex::new(&self.r#match.title_regex) {
            Ok(re) => re.is_match(&w.title),
            Err(_) => false,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::domain::Chord;

    fn vlc_profile() -> Profile {
        let mut keymap = Keymap::new();
        keymap.insert(
            KeyName::Play,
            Chord {
                mods: vec![],
                key: "space".into(),
            },
        );
        keymap.insert(
            KeyName::Stop,
            Chord {
                mods: vec![],
                key: "s".into(),
            },
        );
        keymap.insert(
            KeyName::Next,
            Chord {
                mods: vec![],
                key: "n".into(),
            },
        );
        keymap.insert(
            KeyName::Prev,
            Chord {
                mods: vec![],
                key: "p".into(),
            },
        );
        Profile {
            id: "vlc".into(),
            r#match: Match {
                process: vec!["vlc.exe".into(), "VLC".into()],
                title_regex: "VLC media player".into(),
            },
            injection: InjectionMode::Background,
            keymap,
            ..Default::default()
        }
    }

    fn win(process: &str, title: &str) -> Window {
        Window {
            process: process.into(),
            title: title.into(),
            ..Default::default()
        }
    }

    #[test]
    fn matches_window() {
        let p = vlc_profile();
        assert!(p.matches_window(&win("vlc.exe", "Big Buck Bunny - VLC media player")));
        assert!(!p.matches_window(&win("chrome.exe", "VLC media player")));
        assert!(!p.matches_window(&win("vlc.exe", "Something Else")));
    }

    #[test]
    fn empty_title_regex_matches_any_title() {
        let p = Profile {
            r#match: Match {
                process: vec!["Mitti".into()],
                title_regex: String::new(),
            },
            ..Default::default()
        };
        assert!(p.matches_window(&win("Mitti", "anything")));
    }

    #[test]
    fn uses_controller_and_has_action() {
        let mut p = vlc_profile();
        assert!(!p.uses_controller());
        assert!(p.has_action(KeyName::Play));
        assert!(!p.has_action(KeyName::Record));

        p.control = ControlMode::Uia;
        p.uia.insert(KeyName::Play, "TogglePlaybackButton".into());
        assert!(p.uses_controller());
        // UIA profiles read the UIA map, not the keymap.
        assert!(p.has_action(KeyName::Play));
        assert!(!p.has_action(KeyName::Stop));
    }
}
