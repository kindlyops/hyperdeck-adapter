//! Profile loading + validation (port of `internal/adapter/driven/config/store.go`).

use std::collections::HashMap;
use std::path::PathBuf;

use regex::Regex;
use serde::Deserialize;

use crate::domain::{
    parse_chord, ApiConfig, ClipSourceConfig, ControlMode, InjectionMode, KeyName, Keymap, Match,
    Profile, StateConfig,
};
use crate::error::{DeckError, DeckResult};
use crate::port::ProfileStore;

/// Loads profiles from a YAML file path.
pub struct Store {
    path: PathBuf,
}

impl Store {
    /// Returns a profile store reading from `path`.
    pub fn new(path: impl Into<PathBuf>) -> Self {
        Store { path: path.into() }
    }
}

impl ProfileStore for Store {
    fn load(&self) -> DeckResult<Vec<Profile>> {
        let data = std::fs::read(&self.path)
            .map_err(|e| DeckError::Other(format!("read config {:?}: {e}", self.path)))?;
        load_bytes(&data).map_err(DeckError::Other)
    }
}

#[derive(Deserialize, Default)]
struct FileSchema {
    #[serde(default)]
    profiles: Vec<ProfileSchema>,
}

#[derive(Deserialize, Default)]
struct ProfileSchema {
    #[serde(default)]
    id: String,
    #[serde(default, rename = "match")]
    match_: MatchSchema,
    #[serde(default)]
    control: String,
    #[serde(default)]
    injection: String,
    #[serde(default)]
    api: ApiSchema,
    #[serde(default)]
    uia: HashMap<String, String>,
    #[serde(default)]
    keymap: HashMap<String, String>,
    #[serde(default)]
    play_toggle: bool,
    #[serde(default)]
    cue_on_navigate: bool,
    #[serde(default)]
    clip_source: ClipSchema,
    #[serde(default)]
    state: StateSchema,
    #[serde(default)]
    homing: Vec<String>,
}

#[derive(Deserialize, Default)]
struct MatchSchema {
    #[serde(default)]
    process: Vec<String>,
    #[serde(default)]
    title_regex: String,
}

#[derive(Deserialize, Default)]
struct ApiSchema {
    #[serde(default, rename = "type")]
    kind: String,
    #[serde(default)]
    base_url: String,
    #[serde(default)]
    password: String,
}

#[derive(Deserialize, Default)]
struct ClipSchema {
    #[serde(default, rename = "type")]
    kind: String,
    #[serde(default)]
    path: String,
    #[serde(default)]
    count: i32,
}

#[derive(Deserialize, Default)]
struct StateSchema {
    #[serde(default, rename = "type")]
    kind: String,
    #[serde(default)]
    playing: String,
    #[serde(default)]
    automation_id: String,
}

/// Parses and validates the profile file bytes.
pub fn load_bytes(data: &[u8]) -> Result<Vec<Profile>, String> {
    let f: FileSchema = serde_norway::from_slice(data).map_err(|e| format!("parse config: {e}"))?;
    f.profiles.into_iter().map(convert).collect()
}

fn convert(ps: ProfileSchema) -> Result<Profile, String> {
    if ps.id.is_empty() {
        return Err("profile missing id".to_string());
    }
    let id = &ps.id;

    let control = match ps.control.as_str() {
        "" | "keys" => ControlMode::Keys,
        "api" => ControlMode::Api,
        "uia" => ControlMode::Uia,
        other => {
            return Err(format!(
                "profile {id:?}: invalid control {other:?} (want keys|api|uia)"
            ))
        }
    };

    // Injection only governs keystroke control; api/uia profiles may omit it.
    let injection = match ps.injection.as_str() {
        "focus" => InjectionMode::Focus,
        "background" => InjectionMode::Background,
        other => {
            if control == ControlMode::Keys {
                return Err(format!(
                    "profile {id:?}: invalid injection {other:?} (want focus|background)"
                ));
            }
            InjectionMode::Focus // unused for controller profiles
        }
    };

    let mut api = ApiConfig::default();
    if control == ControlMode::Api {
        if ps.api.kind != "vlc_http" {
            return Err(format!(
                "profile {id:?}: invalid api.type {:?} (want vlc_http)",
                ps.api.kind
            ));
        }
        api = ApiConfig {
            kind: ps.api.kind,
            base_url: ps.api.base_url,
            password: ps.api.password,
        };
    }

    let mut uia: HashMap<KeyName, String> = HashMap::new();
    if control == ControlMode::Uia {
        if ps.uia.is_empty() {
            return Err(format!(
                "profile {id:?}: uia control requires a uia: map of action -> AutomationId"
            ));
        }
        for (name, aid) in &ps.uia {
            if aid.is_empty() {
                return Err(format!(
                    "profile {id:?}: uia.{name} has an empty AutomationId"
                ));
            }
            if let Some(k) = KeyName::parse(name) {
                uia.insert(k, aid.clone());
            }
        }
    }

    if ps.match_.process.is_empty() {
        return Err(format!(
            "profile {id:?}: match.process must list at least one process name"
        ));
    }
    if ps.match_.process.iter().any(String::is_empty) {
        return Err(format!(
            "profile {id:?}: match.process contains an empty entry"
        ));
    }
    if !ps.match_.title_regex.is_empty() {
        Regex::new(&ps.match_.title_regex)
            .map_err(|e| format!("profile {id:?}: invalid title_regex: {e}"))?;
    }

    let mut keymap = Keymap::new();
    for (name, spec) in &ps.keymap {
        // Every chord is validated (matching Go), even for actions the deck does
        // not model (e.g. "pause"); unmodeled actions are then dropped.
        let chord = parse_chord(spec).map_err(|e| format!("profile {id:?} key {name:?}: {e}"))?;
        if let Some(k) = KeyName::parse(name) {
            keymap.insert(k, chord);
        }
    }

    if control == ControlMode::Uia {
        if !uia.contains_key(&KeyName::Play) {
            return Err(format!(
                "profile {id:?}: missing required uia.play AutomationId"
            ));
        }
    } else if !keymap.contains_key(&KeyName::Play) {
        return Err(format!("profile {id:?}: missing required 'play' key"));
    }

    let mut homing = Vec::new();
    for spec in &ps.homing {
        let chord =
            parse_chord(spec).map_err(|e| format!("profile {id:?} homing {spec:?}: {e}"))?;
        homing.push(chord);
    }

    Ok(Profile {
        id: ps.id,
        r#match: Match {
            process: ps.match_.process,
            title_regex: ps.match_.title_regex,
        },
        control,
        injection,
        api,
        uia,
        keymap,
        play_toggle: ps.play_toggle,
        cue_on_navigate: ps.cue_on_navigate,
        clip_source: ClipSourceConfig {
            kind: ps.clip_source.kind,
            path: ps.clip_source.path,
            count: ps.clip_source.count,
        },
        state: StateConfig {
            kind: ps.state.kind,
            playing: ps.state.playing,
            automation_id: ps.state.automation_id,
        },
        homing,
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::domain::KeyName;

    const TESTDATA: &str = concat!(env!("CARGO_MANIFEST_DIR"), "/../../testdata/profiles.yaml");

    #[test]
    fn load_profiles_from_file() {
        let profiles = Store::new(TESTDATA).load().expect("load");
        assert_eq!(profiles.len(), 2);
        let vlc = &profiles[0];
        assert_eq!(vlc.id, "vlc");
        assert_eq!(vlc.injection, InjectionMode::Background);
        assert_eq!(vlc.keymap.get(&KeyName::Next).unwrap().key, "n");
        let example = &profiles[1];
        assert!(example.play_toggle);
        let next = example.keymap.get(&KeyName::Next).unwrap();
        assert_eq!(next.key, "right");
        assert_eq!(next.mods.len(), 1);
    }

    #[test]
    fn loads_api_profile() {
        let profiles = load_bytes(
            br#"profiles:
  - id: vlc_api
    match: { process: ["vlc.exe"] }
    control: api
    api: { type: vlc_http, base_url: "http://127.0.0.1:8080", password: "pw" }
    keymap: { play: "space", stop: "s", next: "n", prev: "p" }
    play_toggle: true
"#,
        )
        .expect("load");
        let p = &profiles[0];
        assert_eq!(p.control, ControlMode::Api);
        assert_eq!(p.api.kind, "vlc_http");
        assert_eq!(p.api.base_url, "http://127.0.0.1:8080");
        assert_eq!(p.api.password, "pw");
    }

    #[test]
    fn loads_uia_profile() {
        let profiles = load_bytes(
            br#"profiles:
  - id: example_win
    match: { process: ["ApplicationFrameHost.exe"], title_regex: "Example Player" }
    control: uia
    uia: { play: "TogglePlaybackButton", next: "PlayNextButton", prev: "PlayPreviousButton" }
    play_toggle: true
    cue_on_navigate: true
"#,
        )
        .expect("load");
        let p = &profiles[0];
        assert_eq!(p.control, ControlMode::Uia);
        assert_eq!(
            p.uia.get(&KeyName::Play).map(String::as_str),
            Some("TogglePlaybackButton")
        );
        assert_eq!(
            p.uia.get(&KeyName::Next).map(String::as_str),
            Some("PlayNextButton")
        );
        assert!(p.has_action(KeyName::Prev));
    }

    fn rejects(yaml: &[u8]) {
        assert!(load_bytes(yaml).is_err());
    }

    #[test]
    fn rejects_uia_without_play() {
        rejects(
            br#"profiles:
  - id: bad
    match: { process: ["x"], title_regex: "y" }
    control: uia
    uia: { next: "PlayNextButton" }
"#,
        );
    }

    #[test]
    fn rejects_bad_control() {
        rejects(
            br#"profiles:
  - id: bad
    match: { process: ["x"] }
    control: telepathy
    keymap: { play: "space" }
"#,
        );
    }

    #[test]
    fn rejects_api_without_type() {
        rejects(
            br#"profiles:
  - id: bad
    match: { process: ["x"] }
    control: api
    keymap: { play: "space" }
"#,
        );
    }

    #[test]
    fn rejects_missing_play_key() {
        rejects(
            br#"profiles:
  - id: bad
    match: { process: ["x"] }
    injection: focus
    keymap: { next: "n" }
"#,
        );
    }

    #[test]
    fn rejects_bad_injection() {
        rejects(
            br#"profiles:
  - id: bad
    match: { process: ["x"] }
    injection: telepathy
    keymap: { play: "space" }
"#,
        );
    }

    #[test]
    fn rejects_bad_title_regex() {
        rejects(
            br#"profiles:
  - id: bad
    match: { process: ["x"], title_regex: "[unterminated" }
    injection: focus
    keymap: { play: "space" }
"#,
        );
    }

    #[test]
    fn rejects_empty_process() {
        rejects(
            br#"profiles:
  - id: bad
    match: { process: [] }
    injection: focus
    keymap: { play: "space" }
"#,
        );
    }
}
