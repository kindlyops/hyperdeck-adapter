//! Window and lock state (port of `internal/core/domain/window.go`).

use super::Profile;

/// Identifies a target OS window the injector can act on.
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct Window {
    /// Native window handle (HWND on Windows, window id on macOS).
    pub handle: usize,
    pub title: String,
    pub process: String,
}

/// The current player-lock status.
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct LockState {
    pub locked: bool,
    pub profile: Option<Profile>,
    pub window: Window,
}
