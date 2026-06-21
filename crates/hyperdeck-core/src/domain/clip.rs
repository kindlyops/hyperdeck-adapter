//! Clip list (port of `internal/core/domain/clip.go`).

/// One entry in the deck's clip list.
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct Clip {
    pub id: i32,
    pub name: String,
    pub timecode: String,
    pub duration: String,
}

/// The ordered set of clips the controller can navigate.
pub type ClipList = Vec<Clip>;

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn clip_list_indexing() {
        let cl: ClipList = vec![
            Clip {
                id: 1,
                name: "a".into(),
                ..Default::default()
            },
            Clip {
                id: 2,
                name: "b".into(),
                ..Default::default()
            },
        ];
        assert_eq!(cl.len(), 2);
    }
}
