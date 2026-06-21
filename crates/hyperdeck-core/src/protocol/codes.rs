//! HyperDeck Ethernet Protocol response codes (v1.11).
//! Port of `internal/adapter/driving/hyperdeck/codes.go`.

pub const CODE_OK: u16 = 200;
pub const CODE_SLOT_INFO: u16 = 202;
pub const CODE_DEVICE_INFO: u16 = 204;
pub const CODE_CLIPS_INFO: u16 = 205;
pub const CODE_TRANSPORT_INFO: u16 = 208;
pub const CODE_CONNECTION_INFO: u16 = 500;
pub const CODE_SYNTAX_ERROR: u16 = 100;
pub const CODE_INVALID_STATE: u16 = 150;
