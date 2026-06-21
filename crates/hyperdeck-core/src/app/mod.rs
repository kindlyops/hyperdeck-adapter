//! Application services (port of `internal/core/app`).

mod session;
mod virtualdeck;

pub use session::{Session, SharedClipSource, SharedStateProbe};
pub use virtualdeck::{SharedController, SharedInjector, VirtualDeck};
