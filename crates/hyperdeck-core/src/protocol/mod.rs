//! HyperDeck Ethernet Protocol driving adapter
//! (port of `internal/adapter/driving/hyperdeck`).
//!
//! The TCP server itself lives with the async runtime wiring; this module holds
//! the pure, testable parser, response codes, and command responder.

mod codes;
mod parser;
mod responder;

pub use codes::*;
pub use parser::{parse_command, Command};
pub use responder::Responder;
