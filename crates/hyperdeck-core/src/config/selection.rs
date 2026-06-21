//! Pinned-profile persistence (port of `internal/adapter/driven/config/selection.go`).

use std::io::ErrorKind;
use std::path::PathBuf;

use serde::{Deserialize, Serialize};

use crate::error::{DeckError, DeckResult};

/// Persists the pinned profile id (empty = Auto / match-any) to a JSON file kept
/// separate from `profiles.yaml`, so the user's commented config is never rewritten.
pub struct SelectionStore {
    path: PathBuf,
}

#[derive(Serialize, Deserialize, Default)]
struct SelectionSchema {
    #[serde(default)]
    active_profile: String,
}

impl SelectionStore {
    /// Returns a selection store backed by `path`.
    pub fn new(path: impl Into<PathBuf>) -> Self {
        SelectionStore { path: path.into() }
    }

    /// Returns the pinned profile id. A missing file is not an error and yields
    /// `""` (Auto); a malformed file returns an error.
    pub fn load(&self) -> DeckResult<String> {
        let data = match std::fs::read(&self.path) {
            Ok(d) => d,
            Err(e) if e.kind() == ErrorKind::NotFound => return Ok(String::new()),
            Err(e) => {
                return Err(DeckError::Other(format!(
                    "read selection {:?}: {e}",
                    self.path
                )))
            }
        };
        let sel: SelectionSchema = serde_json::from_slice(&data)
            .map_err(|e| DeckError::Other(format!("parse selection {:?}: {e}", self.path)))?;
        Ok(sel.active_profile)
    }

    /// Writes the pinned profile id (empty clears to Auto), creating parent
    /// directories as needed.
    pub fn save(&self, id: &str) -> DeckResult<()> {
        if let Some(dir) = self.path.parent() {
            std::fs::create_dir_all(dir).map_err(|e| {
                DeckError::Other(format!("create selection dir for {:?}: {e}", self.path))
            })?;
        }
        let data = serde_json::to_vec(&SelectionSchema {
            active_profile: id.to_string(),
        })
        .map_err(|e| DeckError::Other(format!("encode selection: {e}")))?;
        std::fs::write(&self.path, data)
            .map_err(|e| DeckError::Other(format!("write selection {:?}: {e}", self.path)))?;
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn temp_path(name: &str) -> PathBuf {
        let mut p = std::env::temp_dir();
        p.push(format!("hyperdeck-sel-{}-{name}", std::process::id()));
        let _ = std::fs::create_dir_all(&p);
        p.push("selection.json");
        p
    }

    #[test]
    fn round_trip() {
        let path = temp_path("rt");
        let s = SelectionStore::new(&path);
        s.save("vlc").expect("save");
        assert_eq!(s.load().expect("load"), "vlc");
        let _ = std::fs::remove_file(&path);
    }

    #[test]
    fn missing_file_is_auto() {
        let path = temp_path("missing").with_file_name("does-not-exist.json");
        assert_eq!(SelectionStore::new(&path).load().expect("load"), "");
    }

    #[test]
    fn malformed_file_errors() {
        let path = temp_path("bad");
        std::fs::write(&path, b"{not json").unwrap();
        assert!(SelectionStore::new(&path).load().is_err());
        let _ = std::fs::remove_file(&path);
    }
}
