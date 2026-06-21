//! Positional + Mitti clip sources
//! (port of `positional.go` and `mitti.go`).

use crate::domain::{Clip, ClipList};
use crate::error::DeckResult;
use crate::port::ClipSource;

/// Produces a fixed number of generic clip slots.
pub struct Positional {
    count: i32,
}

impl Positional {
    /// Returns a positional clip source with `n` slots.
    pub fn new(n: i32) -> Self {
        Positional { count: n }
    }
}

impl ClipSource for Positional {
    fn list(&self) -> DeckResult<ClipList> {
        Ok((1..=self.count)
            .map(|i| Clip {
                id: i,
                name: format!("Clip {i}"),
                ..Default::default()
            })
            .collect())
    }
}

/// A best-effort clip source for Mitti. Until the proprietary playlist format is
/// parsed, it falls back to a positional list of the configured size.
pub struct Mitti {
    fallback: Positional,
}

impl Mitti {
    /// Returns a Mitti clip source with a positional fallback of `n` slots.
    pub fn new(n: i32) -> Self {
        Mitti {
            fallback: Positional::new(n),
        }
    }
}

impl ClipSource for Mitti {
    fn list(&self) -> DeckResult<ClipList> {
        self.fallback.list()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn positional_generates_named_slots() {
        let clips = Positional::new(3).list().unwrap();
        assert_eq!(clips.len(), 3);
        assert_eq!(clips[2].id, 3);
        assert_eq!(clips[2].name, "Clip 3");
    }

    #[test]
    fn mitti_falls_back_to_positional() {
        assert_eq!(Mitti::new(4).list().unwrap().len(), 4);
    }
}
