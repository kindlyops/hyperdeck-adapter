//! macOS virtual key codes + event flags (port of `keymap_darwin.go`).
//!
//! Pure — kept free of CoreGraphics calls so the chord→keycode/flags mapping is
//! unit-testable.

use hyperdeck_core::domain::Modifier;

// CGEventFlags modifier masks (stable values from CGEventTypes.h).
const FLAG_SHIFT: u64 = 1 << 17;
const FLAG_CONTROL: u64 = 1 << 18;
const FLAG_ALT: u64 = 1 << 19;
const FLAG_COMMAND: u64 = 1 << 20;

/// Converts chord modifiers into a CGEventFlags bitmask.
pub fn event_flags(mods: &[Modifier]) -> u64 {
    let mut f = 0;
    for m in mods {
        f |= match m {
            Modifier::Shift => FLAG_SHIFT,
            Modifier::Ctrl => FLAG_CONTROL,
            Modifier::Alt => FLAG_ALT,
            Modifier::Cmd => FLAG_COMMAND,
        };
    }
    f
}

/// Returns the macOS ANSI virtual key code (kVK_* from Carbon HIToolbox) for a
/// base key name. Codes are physical positions on a US ANSI layout.
pub fn key_code(key: &str) -> Option<u16> {
    let code = match key {
        "space" => 49,
        "return" | "enter" => 36,
        "tab" => 48,
        "esc" | "escape" => 53,
        "delete" | "backspace" => 51,
        "period" | "." => 47,
        "comma" | "," => 43,
        "left" => 123,
        "right" => 124,
        "down" => 125,
        "up" => 126,
        "a" => 0,
        "s" => 1,
        "d" => 2,
        "f" => 3,
        "h" => 4,
        "g" => 5,
        "z" => 6,
        "x" => 7,
        "c" => 8,
        "v" => 9,
        "b" => 11,
        "q" => 12,
        "w" => 13,
        "e" => 14,
        "r" => 15,
        "y" => 16,
        "t" => 17,
        "o" => 31,
        "u" => 32,
        "i" => 34,
        "p" => 35,
        "l" => 37,
        "j" => 38,
        "k" => 40,
        "n" => 45,
        "m" => 46,
        "1" => 18,
        "2" => 19,
        "3" => 20,
        "4" => 21,
        "5" => 23,
        "6" => 22,
        "7" => 26,
        "8" => 28,
        "9" => 25,
        "0" => 29,
        _ => return None,
    };
    Some(code)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn key_codes() {
        assert_eq!(key_code("space"), Some(49));
        assert_eq!(key_code("right"), Some(124));
        assert_eq!(key_code("a"), Some(0));
        assert_eq!(key_code("."), Some(47));
        assert_eq!(key_code("nope"), None);
    }

    #[test]
    fn flags() {
        assert_eq!(event_flags(&[Modifier::Cmd]), 1 << 20);
        assert_eq!(
            event_flags(&[Modifier::Ctrl, Modifier::Shift]),
            (1 << 18) | (1 << 17)
        );
        assert_eq!(event_flags(&[]), 0);
    }
}
