//! Keystroke injection + window enumeration (port of `internal/adapter/driven/injector`).
//!
//! The real per-OS implementations live behind `cfg(target_os = ...)`:
//! - Windows: [`windows_impl`] (SendInput / PostMessage / EnumWindows).
//! - Other platforms: a no-op that lets the protocol server still run for
//!   development against an out-of-band controller. (macOS Accessibility
//!   injection is added in a follow-up.)

use std::sync::Arc;

use hyperdeck_core::port::{KeyInjector, WindowEnumerator};

#[cfg(windows)]
mod keymap_windows;
#[cfg(windows)]
mod windows_impl;

/// Returns the platform key injector.
pub fn injector() -> Arc<dyn KeyInjector + Send + Sync> {
    #[cfg(windows)]
    {
        Arc::new(windows_impl::WinInput)
    }
    #[cfg(not(windows))]
    {
        Arc::new(Noop)
    }
}

/// Returns the platform window enumerator.
pub fn enumerator() -> Arc<dyn WindowEnumerator + Send + Sync> {
    #[cfg(windows)]
    {
        Arc::new(windows_impl::WinInput)
    }
    #[cfg(not(windows))]
    {
        Arc::new(Noop)
    }
}

/// Reports whether this process may synthesize input, prompting the user when it
/// cannot. Only macOS gates this (Accessibility permission); elsewhere input is
/// always permitted, so this returns `true`.
pub fn request_accessibility() -> bool {
    true
}

/// No-op injector/enumerator for platforms without an implementation yet.
#[cfg(not(windows))]
struct Noop;

#[cfg(not(windows))]
impl KeyInjector for Noop {
    fn focus(&self, _w: &hyperdeck_core::domain::Window) -> hyperdeck_core::error::DeckResult<()> {
        Ok(())
    }
    fn send_keys(
        &self,
        _w: &hyperdeck_core::domain::Window,
        _chords: &[hyperdeck_core::domain::Chord],
    ) -> hyperdeck_core::error::DeckResult<()> {
        Ok(())
    }
}

#[cfg(not(windows))]
impl WindowEnumerator for Noop {
    fn open_windows(
        &self,
    ) -> hyperdeck_core::error::DeckResult<Vec<hyperdeck_core::domain::Window>> {
        Ok(Vec::new())
    }
}
