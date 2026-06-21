//! Keyboard chords (port of `internal/core/domain/chord.go`).

/// A keyboard modifier key.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum Modifier {
    Ctrl,
    Alt,
    Shift,
    Cmd,
}

impl Modifier {
    /// Parses a lowercase modifier token, mirroring Go's `knownModifiers`.
    fn parse(token: &str) -> Option<Modifier> {
        match token {
            "ctrl" => Some(Modifier::Ctrl),
            "alt" => Some(Modifier::Alt),
            "shift" => Some(Modifier::Shift),
            "cmd" => Some(Modifier::Cmd),
            _ => None,
        }
    }

    /// The canonical lowercase token for this modifier.
    pub fn as_str(self) -> &'static str {
        match self {
            Modifier::Ctrl => "ctrl",
            Modifier::Alt => "alt",
            Modifier::Shift => "shift",
            Modifier::Cmd => "cmd",
        }
    }
}

/// A single keystroke: zero or more modifiers plus one base key.
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct Chord {
    pub mods: Vec<Modifier>,
    pub key: String,
}

/// Parses strings like `"ctrl+right"` or `"space"` into a [`Chord`].
///
/// Matching is case-insensitive and the base key must be the last token. This is
/// a faithful port of Go's `domain.ParseChord`, including its error cases.
pub fn parse_chord(s: &str) -> Result<Chord, String> {
    let lowered = s.trim().to_lowercase();
    let parts: Vec<&str> = lowered.split('+').collect();
    if parts.last().is_none_or(|p| p.is_empty()) {
        return Err(format!("parse chord {s:?}: empty key"));
    }
    let mut chord = Chord::default();
    let last = parts.len() - 1;
    for (i, part) in parts.iter().enumerate() {
        if part.is_empty() {
            return Err(format!("parse chord {s:?}: empty token"));
        }
        if i == last {
            chord.key = (*part).to_string();
            continue;
        }
        match Modifier::parse(part) {
            Some(m) => chord.mods.push(m),
            None => return Err(format!("parse chord {s:?}: unknown modifier {part:?}")),
        }
    }
    Ok(chord)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_valid_chords() {
        let cases = [
            (
                "space",
                Chord {
                    mods: vec![],
                    key: "space".into(),
                },
            ),
            (
                "s",
                Chord {
                    mods: vec![],
                    key: "s".into(),
                },
            ),
            (
                "ctrl+right",
                Chord {
                    mods: vec![Modifier::Ctrl],
                    key: "right".into(),
                },
            ),
            (
                "cmd+esc",
                Chord {
                    mods: vec![Modifier::Cmd],
                    key: "esc".into(),
                },
            ),
            (
                "CTRL+Shift+Up",
                Chord {
                    mods: vec![Modifier::Ctrl, Modifier::Shift],
                    key: "up".into(),
                },
            ),
        ];
        for (input, want) in cases {
            let got = parse_chord(input).expect("should parse");
            assert_eq!(got, want, "parse_chord({input:?})");
        }
    }

    #[test]
    fn rejects_invalid_chords() {
        for input in ["", "ctrl+", "+s", "bogusmod+x"] {
            assert!(
                parse_chord(input).is_err(),
                "parse_chord({input:?}) should error"
            );
        }
    }
}
