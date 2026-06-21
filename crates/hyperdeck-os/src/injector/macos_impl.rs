//! macOS keystroke injection + window enumeration (port of `injector_darwin.go`
//! and `cinject_darwin.m`).

use std::thread::sleep;
use std::time::Duration;

use core_foundation::base::{CFType, TCFType};
use core_foundation::boolean::CFBoolean;
use core_foundation::dictionary::{CFDictionary, CFDictionaryRef};
use core_foundation::number::CFNumber;
use core_foundation::string::{CFString, CFStringRef};
use core_graphics::display::CGDisplay;
use core_graphics::event::{CGEvent, CGEventFlags};
use core_graphics::event_source::{CGEventSource, CGEventSourceStateID};
use core_graphics::window::{kCGWindowListExcludeDesktopElements, kCGWindowListOptionOnScreenOnly};
use objc2_app_kit::{NSApplicationActivationOptions, NSRunningApplication};

use hyperdeck_core::domain::{Chord, Window};
use hyperdeck_core::error::{DeckError, DeckResult};
use hyperdeck_core::port::{KeyInjector, WindowEnumerator};

use super::keymap_darwin::{event_flags, key_code};

// Delays that make synthesized input reliable (mirrors cinject_darwin.m).
const FOCUS_SETTLE: Duration = Duration::from_millis(120);
const AFTER_KEY: Duration = Duration::from_millis(25);
const KEY_HOLD: Duration = Duration::from_millis(12);

/// macOS injector + window enumerator.
pub struct MacInjector;

impl KeyInjector for MacInjector {
    // NSApplicationActivateIgnoringOtherApps is deprecated in macOS 14 but is the
    // documented way to foreground another app for input; the Go code suppresses
    // the same deprecation. activate()'s newer form does not raise reliably.
    #[allow(deprecated)]
    fn focus(&self, win: &Window) -> DeckResult<()> {
        let pid = win.handle as i32;
        // SAFETY: NSRunningApplication lookup + activation are standard AppKit
        // calls; a missing pid yields None and an error.
        let app = unsafe { NSRunningApplication::runningApplicationWithProcessIdentifier(pid) };
        let Some(app) = app else {
            return Err(DeckError::Injector(format!(
                "focus: could not activate pid {pid} ({})",
                win.process
            )));
        };
        unsafe {
            app.activateWithOptions(
                NSApplicationActivationOptions::NSApplicationActivateIgnoringOtherApps,
            );
        }
        sleep(FOCUS_SETTLE); // let the app become first responder before keys arrive
        Ok(())
    }

    fn send_keys(&self, win: &Window, chords: &[Chord]) -> DeckResult<()> {
        let pid = win.handle as i32;
        for c in chords {
            let Some(code) = key_code(&c.key) else {
                return Err(DeckError::Injector(format!(
                    "sendkeys: no macOS key code for {:?}",
                    c.key
                )));
            };
            let flags = CGEventFlags::from_bits_retain(event_flags(&c.mods));
            post_key_to_pid(pid, code, flags)?;
            sleep(AFTER_KEY);
        }
        Ok(())
    }
}

fn post_key_to_pid(pid: i32, keycode: u16, flags: CGEventFlags) -> DeckResult<()> {
    let make_event = |down: bool| -> DeckResult<CGEvent> {
        let source = CGEventSource::new(CGEventSourceStateID::HIDSystemState)
            .map_err(|_| DeckError::Injector("sendkeys: CGEventSource create failed".into()))?;
        let event = CGEvent::new_keyboard_event(source, keycode, down)
            .map_err(|_| DeckError::Injector("sendkeys: CGEvent create failed".into()))?;
        if !flags.is_empty() {
            event.set_flags(flags);
        }
        Ok(event)
    };
    let down = make_event(true)?;
    let up = make_event(false)?;
    down.post_to_pid(pid);
    sleep(KEY_HOLD); // key-hold so the target registers the press
    up.post_to_pid(pid);
    Ok(())
}

impl WindowEnumerator for MacInjector {
    fn open_windows(&self) -> DeckResult<Vec<Window>> {
        let option = kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements;
        let Some(array) = CGDisplay::window_list_info(option, None) else {
            return Ok(Vec::new());
        };
        let mut out = Vec::new();
        for i in 0..array.len() {
            let Some(item) = array.get(i) else { continue };
            // Each element is a CGWindow info dictionary.
            let dict: CFDictionary<CFString, CFType> =
                unsafe { CFDictionary::wrap_under_get_rule(*item as CFDictionaryRef) };
            let pid = dict_i64(&dict, "kCGWindowOwnerPID").unwrap_or(0);
            let owner = dict_string(&dict, "kCGWindowOwnerName").unwrap_or_default();
            let title = dict_string(&dict, "kCGWindowName").unwrap_or_default();
            out.push(Window {
                handle: pid as usize,
                process: owner,
                title,
            });
        }
        Ok(out)
    }
}

fn dict_value(dict: &CFDictionary<CFString, CFType>, key: &str) -> Option<CFType> {
    dict.find(CFString::new(key)).map(|v| v.clone())
}

fn dict_string(dict: &CFDictionary<CFString, CFType>, key: &str) -> Option<String> {
    dict_value(dict, key)?
        .downcast::<CFString>()
        .map(|s| s.to_string())
}

fn dict_i64(dict: &CFDictionary<CFString, CFType>, key: &str) -> Option<i64> {
    dict_value(dict, key)?.downcast::<CFNumber>()?.to_i64()
}

/// Reports whether this process is trusted for Accessibility, prompting the user
/// to grant it (via System Settings) when it is not.
pub fn request_accessibility() -> bool {
    let key = unsafe { CFString::wrap_under_get_rule(kAXTrustedCheckOptionPrompt) };
    let opts =
        CFDictionary::from_CFType_pairs(&[(key.as_CFType(), CFBoolean::true_value().as_CFType())]);
    unsafe { AXIsProcessTrustedWithOptions(opts.as_concrete_TypeRef()) != 0 }
}

#[link(name = "ApplicationServices", kind = "framework")]
extern "C" {
    fn AXIsProcessTrustedWithOptions(options: CFDictionaryRef) -> u8;
    static kAXTrustedCheckOptionPrompt: CFStringRef;
}
