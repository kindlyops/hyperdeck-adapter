//! Configuration adapters (port of `internal/adapter/driven/config`).

mod default;
mod selection;
mod store;

pub use default::{ensure_default, DEFAULT_PROFILES};
pub use selection::SelectionStore;
pub use store::{load_bytes, Store};
