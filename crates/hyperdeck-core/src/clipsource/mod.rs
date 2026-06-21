//! Clip source strategies (port of `internal/adapter/driven/clipsource`).

mod playlist;
mod positional;

pub use playlist::Playlist;
pub use positional::{Mitti, Positional};

use std::sync::Arc;

use crate::app::SharedClipSource;
use crate::domain::Profile;

/// Builds the clip source named by the profile's `clip_source.type`.
pub fn new(p: &Profile) -> SharedClipSource {
    let cfg = &p.clip_source;
    match cfg.kind.as_str() {
        "playlist_file" => Arc::new(Playlist::new(cfg.path.clone())),
        "mitti" => Arc::new(Mitti::new(default_count(cfg.count))),
        // "positional" and unknown -> positional
        _ => Arc::new(Positional::new(default_count(cfg.count))),
    }
}

fn default_count(n: i32) -> i32 {
    if n <= 0 {
        1
    } else {
        n
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::domain::ClipSourceConfig;

    #[test]
    fn factory_builds_positional() {
        let p = Profile {
            clip_source: ClipSourceConfig {
                kind: "positional".into(),
                count: 2,
                ..Default::default()
            },
            ..Default::default()
        };
        let cs = new(&p);
        assert_eq!(cs.list().unwrap().len(), 2);
    }
}
