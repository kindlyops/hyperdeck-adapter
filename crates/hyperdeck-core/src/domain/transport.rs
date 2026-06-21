//! Transport state and response payloads (port of `internal/core/domain/transport.go`).

/// The modeled (open-loop) play state of the deck.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum TransportState {
    #[default]
    Stopped,
    Playing,
}

impl TransportState {
    /// Maps the modeled state to a HyperDeck transport "status" value.
    pub fn hyperdeck_status(self) -> &'static str {
        match self {
            TransportState::Playing => "play",
            TransportState::Stopped => "stopped",
        }
    }
}

/// The payload of a `transport info` response.
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct TransportInfo {
    pub status: String,
    pub speed: i32,
    pub clip_id: i32,
    pub slot_id: i32,
}

/// The payload of a `slot info` response.
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct SlotInfo {
    pub present: bool,
    pub slot_id: i32,
}

/// The payload of a `device info` response.
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct DeviceInfo {
    pub protocol_version: String,
    pub model: String,
    pub unique_id: String,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn hyperdeck_status_strings() {
        assert_eq!(TransportState::Stopped.hyperdeck_status(), "stopped");
        assert_eq!(TransportState::Playing.hyperdeck_status(), "play");
    }
}
