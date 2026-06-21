//! Playlist clip source for `.m3u` / `.xspf` files
//! (port of `internal/adapter/driven/clipsource/playlist.go`).

use std::path::{Path, PathBuf};

use serde::Deserialize;

use crate::domain::{Clip, ClipList};
use crate::error::{DeckError, DeckResult};
use crate::port::ClipSource;

/// Reads clip names from an `.m3u` or `.xspf` file.
pub struct Playlist {
    path: PathBuf,
}

#[derive(Deserialize)]
struct XspfFile {
    #[serde(rename = "trackList", default)]
    track_list: TrackList,
}

#[derive(Deserialize, Default)]
struct TrackList {
    #[serde(rename = "track", default)]
    tracks: Vec<XspfTrack>,
}

#[derive(Deserialize, Default)]
struct XspfTrack {
    #[serde(default)]
    title: String,
    #[serde(default)]
    location: String,
}

impl Playlist {
    /// Returns a playlist-backed clip source.
    pub fn new(path: impl Into<PathBuf>) -> Self {
        Playlist { path: path.into() }
    }

    fn list_m3u(&self) -> DeckResult<ClipList> {
        let text = std::fs::read_to_string(&self.path)
            .map_err(|e| DeckError::Other(format!("open playlist {:?}: {e}", self.path)))?;
        let mut clips = ClipList::new();
        let mut name = String::new();
        for raw in text.lines() {
            let line = raw.trim();
            if let Some(rest) = line.strip_prefix("#EXTINF:") {
                if let Some(comma) = rest.find(',') {
                    name = rest[comma + 1..].trim().to_string();
                }
            } else if line.is_empty() || line.starts_with('#') {
                // skip directives and blanks
            } else {
                let label = if name.is_empty() {
                    base_name(line)
                } else {
                    name.clone()
                };
                clips.push(Clip {
                    id: clips.len() as i32 + 1,
                    name: label,
                    ..Default::default()
                });
                name.clear();
            }
        }
        Ok(clips)
    }

    fn list_xspf(&self) -> DeckResult<ClipList> {
        let text = std::fs::read_to_string(&self.path)
            .map_err(|e| DeckError::Other(format!("read playlist {:?}: {e}", self.path)))?;
        // quick-xml's serde de matches on local element names and ignores the
        // default xspf namespace, so the derived structs map cleanly.
        let parsed: XspfFile = quick_xml::de::from_str(&text)
            .map_err(|e| DeckError::Other(format!("parse xspf {:?}: {e}", self.path)))?;
        let clips = parsed
            .track_list
            .tracks
            .into_iter()
            .enumerate()
            .map(|(i, tr)| {
                let name = if tr.title.is_empty() {
                    base_name(&tr.location)
                } else {
                    tr.title
                };
                Clip {
                    id: i as i32 + 1,
                    name,
                    ..Default::default()
                }
            })
            .collect();
        Ok(clips)
    }
}

impl ClipSource for Playlist {
    fn list(&self) -> DeckResult<ClipList> {
        let ext = self
            .path
            .extension()
            .and_then(|e| e.to_str())
            .map(str::to_ascii_lowercase)
            .unwrap_or_default();
        if ext == "xspf" {
            self.list_xspf()
        } else {
            self.list_m3u()
        }
    }
}

/// The trailing path component of a media reference (handles `file://` URLs too).
fn base_name(s: &str) -> String {
    Path::new(s)
        .file_name()
        .map(|n| n.to_string_lossy().into_owned())
        .unwrap_or_else(|| s.to_string())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn testdata(name: &str) -> String {
        format!("{}/../../testdata/{name}", env!("CARGO_MANIFEST_DIR"))
    }

    #[test]
    fn parses_m3u() {
        let clips = Playlist::new(testdata("sample.m3u")).list().unwrap();
        assert_eq!(clips.len(), 2);
        assert_eq!(clips[0].name, "Intro Clip");
        assert_eq!(clips[0].id, 1);
    }

    #[test]
    fn parses_xspf() {
        let clips = Playlist::new(testdata("sample.xspf")).list().unwrap();
        assert_eq!(clips.len(), 2);
        assert_eq!(clips[1].name, "Main Segment");
    }
}
