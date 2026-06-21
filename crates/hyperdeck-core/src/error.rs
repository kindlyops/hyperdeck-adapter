//! Core error type shared across ports and services.

use std::fmt;

/// Errors surfaced by the deck's command surface and driven adapters.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum DeckError {
    /// A transport command arrived with no locked player
    /// (port of Go `app.ErrNotLocked`).
    NotLocked,
    /// An injector / controller failed to deliver an action.
    Injector(String),
    /// Any other failure carrying a human-readable message.
    Other(String),
}

impl fmt::Display for DeckError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            DeckError::NotLocked => write!(f, "no player locked"),
            DeckError::Injector(m) => write!(f, "injector: {m}"),
            DeckError::Other(m) => write!(f, "{m}"),
        }
    }
}

impl std::error::Error for DeckError {}

/// Convenience result alias for fallible deck operations.
pub type DeckResult<T> = std::result::Result<T, DeckError>;
