//! Driving (inbound) port traits (port of `internal/core/port/inbound.go`).
//!
//! Driven (outbound) ports are added alongside the application services that
//! consume them.

use crate::domain::{ClipList, DeviceInfo, SlotInfo, TransportInfo};
use crate::error::DeckResult;

/// The deck's command surface (driving port).
pub trait Transport {
    fn play(&self) -> DeckResult<()>;
    fn stop(&self) -> DeckResult<()>;
    fn record(&self) -> DeckResult<()>;
    fn goto(&self, clip_id: i32) -> DeckResult<()>;
    fn next(&self) -> DeckResult<()>;
    fn prev(&self) -> DeckResult<()>;
    fn rehome(&self) -> DeckResult<()>;
}

/// The deck's read surface (driving port).
pub trait Query {
    fn transport_info(&self) -> TransportInfo;
    fn clips(&self) -> ClipList;
    fn slot_info(&self) -> SlotInfo;
    fn device_info(&self) -> DeviceInfo;
}
