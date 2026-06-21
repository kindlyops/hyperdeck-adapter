//! OS- and player-specific driven adapters for the HyperDeck adapter.
//!
//! This crate hosts the implementations that need external crates or
//! platform-specific APIs and therefore live outside the pure `hyperdeck-core`:
//! the VLC HTTP controller (here), and — added behind `cfg(target_os = ...)` —
//! keystroke injection, window enumeration, and Windows UI Automation.

pub mod injector;
pub mod vlchttp;
