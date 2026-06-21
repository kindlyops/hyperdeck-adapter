//! Pure, OS-independent core of the HyperDeck adapter.
//!
//! This crate is a direct port of the Go `internal/core` hexagonal core: the
//! domain value objects, port traits, and application services. It compiles and
//! is fully testable without any OS-specific dependencies.

pub mod app;
pub mod clipsource;
pub mod config;
pub mod domain;
pub mod error;
pub mod port;
pub mod protocol;
pub mod stateprobe;

#[cfg(test)]
mod testsupport;
