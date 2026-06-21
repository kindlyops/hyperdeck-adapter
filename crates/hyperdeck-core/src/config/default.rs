//! First-run default config seeding (port of `internal/adapter/driven/config/default.go`).

use std::io::ErrorKind;
use std::path::Path;

use crate::error::{DeckError, DeckResult};

/// The seed config written on first run when no profiles file exists. Embedded
/// directly from the in-repo `examples/profiles.yaml`, so the example and the
/// seeded file can never drift.
pub const DEFAULT_PROFILES: &str = include_str!("../../../../examples/profiles.yaml");

/// Writes the embedded default profiles to `path` when nothing exists there yet,
/// creating parent directories as needed. Returns `true` only when it wrote the
/// seed file; a no-op (`false`) when a file already exists.
pub fn ensure_default(path: &Path) -> DeckResult<bool> {
    match std::fs::metadata(path) {
        Ok(_) => return Ok(false),
        Err(e) if e.kind() == ErrorKind::NotFound => {}
        Err(e) => return Err(DeckError::Other(format!("stat config {path:?}: {e}"))),
    }
    if let Some(dir) = path.parent() {
        std::fs::create_dir_all(dir)
            .map_err(|e| DeckError::Other(format!("create config dir for {path:?}: {e}")))?;
    }
    std::fs::write(path, DEFAULT_PROFILES)
        .map_err(|e| DeckError::Other(format!("write default config {path:?}: {e}")))?;
    Ok(true)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::load_bytes;

    #[test]
    fn default_profiles_parse_and_cover_all_players() {
        let profiles = load_bytes(DEFAULT_PROFILES.as_bytes()).expect("default seed must parse");
        let ids: Vec<&str> = profiles.iter().map(|p| p.id.as_str()).collect();
        for want in [
            "example_player",
            "example_player_windows",
            "vlc",
            "vlc_windows",
            "mitti",
        ] {
            assert!(
                ids.contains(&want),
                "default seed missing profile {want:?}; got {ids:?}"
            );
        }
    }

    #[test]
    fn ensure_default_writes_then_is_idempotent() {
        let mut path = std::env::temp_dir();
        path.push(format!("hyperdeck-default-{}", std::process::id()));
        path.push("profiles.yaml");
        let _ = std::fs::remove_file(&path);

        assert!(ensure_default(&path).expect("first write"));
        assert!(!ensure_default(&path).expect("second is no-op"));
        // The seeded file must itself be loadable.
        let data = std::fs::read(&path).unwrap();
        assert!(load_bytes(&data).is_ok());
        let _ = std::fs::remove_file(&path);
    }
}
