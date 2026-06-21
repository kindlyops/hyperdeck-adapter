//! Windows virtual-key mapping (port of `keymap_windows.go`).
//!
//! Pure — no Win32 calls — so the chord→VK mapping is unit-testable.

use hyperdeck_core::domain::Modifier;

/// Returns the Windows virtual-key code for a base key name.
pub fn key_code(key: &str) -> Option<u16> {
    let code = match key {
        "space" => 0x20,
        "enter" | "return" => 0x0D,
        "tab" => 0x09,
        "esc" | "escape" => 0x1B,
        "backspace" => 0x08,
        "delete" => 0x2E,
        "left" => 0x25,
        "up" => 0x26,
        "right" => 0x27,
        "down" => 0x28,
        "period" | "." => 0xBE,
        "comma" | "," => 0xBC,
        other => {
            let bytes = other.as_bytes();
            if bytes.len() == 1 {
                let ch = bytes[0];
                // Letters VK_A..VK_Z are 0x41..0x5A; digits VK_0..VK_9 are 0x30..0x39.
                if ch.is_ascii_lowercase() {
                    return Some((ch - b'a') as u16 + 0x41);
                }
                if ch.is_ascii_digit() {
                    return Some((ch - b'0') as u16 + 0x30);
                }
            }
            return None;
        }
    };
    Some(code)
}

/// Maps a chord modifier to its Windows virtual-key code.
pub fn modifier_vk(m: Modifier) -> u16 {
    match m {
        Modifier::Ctrl => 0x11,  // VK_CONTROL
        Modifier::Shift => 0x10, // VK_SHIFT
        Modifier::Alt => 0x12,   // VK_MENU
        Modifier::Cmd => 0x5B,   // VK_LWIN (rare in app shortcuts; for parity)
    }
}

/// Reports whether a virtual key lives in the extended block (arrows, ins/del),
/// whose scan codes need the extended bit set.
pub fn is_extended(vk: u16) -> bool {
    matches!(vk, 0x25 | 0x26 | 0x27 | 0x28 | 0x2D | 0x2E)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn named_and_computed_keys() {
        assert_eq!(key_code("space"), Some(0x20));
        assert_eq!(key_code("right"), Some(0x27));
        assert_eq!(key_code("a"), Some(0x41));
        assert_eq!(key_code("z"), Some(0x5A));
        assert_eq!(key_code("0"), Some(0x30));
        assert_eq!(key_code("9"), Some(0x39));
        assert_eq!(key_code("."), Some(0xBE));
        assert_eq!(key_code("nope"), None);
    }

    #[test]
    fn modifiers_and_extended() {
        assert_eq!(modifier_vk(Modifier::Ctrl), 0x11);
        assert_eq!(modifier_vk(Modifier::Cmd), 0x5B);
        assert!(is_extended(0x27)); // right arrow
        assert!(!is_extended(0x20)); // space
    }
}
