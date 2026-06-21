//! Domain value objects (port of `internal/core/domain`).

mod chord;
mod clip;
mod profile;
mod transport;
mod window;

pub use chord::{parse_chord, Chord, Modifier};
pub use clip::{Clip, ClipList};
pub use profile::{
    ApiConfig, ClipSourceConfig, ControlMode, InjectionMode, KeyName, Keymap, Match, Profile,
    StateConfig,
};
pub use transport::{DeviceInfo, SlotInfo, TransportInfo, TransportState};
pub use window::{LockState, Window};
