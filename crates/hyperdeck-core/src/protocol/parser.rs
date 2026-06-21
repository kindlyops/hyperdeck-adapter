//! Command parser (port of `internal/adapter/driving/hyperdeck/parser.go`).

use std::collections::HashMap;

/// A parsed HyperDeck request.
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct Command {
    pub name: String,
    pub params: HashMap<String, String>,
}

/// Parses one full command block (single- or multi-line).
///
/// Faithful port of Go's `ParseCommand`: returns an error only when the command
/// is empty, and otherwise never fails (and never panics) on arbitrary input.
pub fn parse_command(raw: &str) -> Result<Command, String> {
    // Normalize CRLF then split on LF, matching Go's splitLines.
    let normalized = raw.replace("\r\n", "\n");
    let lines: Vec<&str> = normalized.split('\n').collect();
    if lines.first().map(|l| l.trim()).unwrap_or("").is_empty() {
        return Err("empty command".to_string());
    }

    let mut cmd = Command::default();
    let first = lines[0].trim();

    // Inline form: "goto: clip id: 3" -> name "goto", param "clip id"="3".
    if let Some((name, rest)) = first.split_once(':') {
        cmd.name = name.trim().to_string();
        let rest = rest.trim();
        if !rest.is_empty() {
            if let Some((k, v)) = rest.split_once(':') {
                cmd.params
                    .insert(k.trim().to_string(), v.trim().to_string());
            }
        }
    } else {
        cmd.name = first.to_string();
    }

    // Block form: subsequent "key: value" lines until a blank line.
    for line in &lines[1..] {
        let line = line.trim_end_matches('\r');
        if line.trim().is_empty() {
            break;
        }
        if let Some((k, v)) = line.split_once(':') {
            cmd.params
                .insert(k.trim().to_string(), v.trim().to_string());
        }
    }
    Ok(cmd)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn simple_command() {
        let cmd = parse_command("play\r\n").expect("parse");
        assert_eq!(cmd.name, "play");
        assert!(cmd.params.is_empty());
    }

    #[test]
    fn command_with_params() {
        let cmd = parse_command("play:\r\nsingle clip: true\r\nspeed: 100\r\n\r\n").expect("parse");
        assert_eq!(cmd.name, "play");
        let mut want = HashMap::new();
        want.insert("single clip".to_string(), "true".to_string());
        want.insert("speed".to_string(), "100".to_string());
        assert_eq!(cmd.params, want);
    }

    #[test]
    fn goto_inline() {
        let cmd = parse_command("goto: clip id: 3\r\n").expect("parse");
        assert_eq!(cmd.name, "goto");
        assert_eq!(cmd.params.get("clip id").map(String::as_str), Some("3"));
    }

    #[test]
    fn empty_is_error() {
        assert!(parse_command("").is_err());
        assert!(parse_command("   \r\n").is_err());
    }

    #[test]
    fn never_panics_on_arbitrary_input() {
        // Property-style smoke test mirroring the Go rapid check: a spread of
        // adversarial inputs must parse without panicking.
        let inputs = [
            "",
            ":",
            "::::",
            ":\r\n:\r\n",
            "a:b:c:d",
            "\r\r\r",
            "\n\n\n",
            "goto:",
            "goto: clip id:",
            "   :   :   ",
            "no colon here",
            "key: value\nno-blank-line",
            "🎬: clip id: ∞",
        ];
        for input in inputs {
            let _ = parse_command(input);
        }
    }
}
