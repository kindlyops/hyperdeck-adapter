//! Port traits (port of `internal/core/port`).
//!
//! Driving (inbound) ports are the deck's command/read surfaces; driven
//! (outbound) ports are the OS- and player-specific adapters the core depends
//! on. The async tick source (`Clock`) and `ProfileStore` are introduced with
//! the components that consume them.

use crate::domain::{
    Chord, ClipList, DeviceInfo, KeyName, LockState, Profile, SlotInfo, TransportInfo,
    TransportState, Window,
};
use crate::error::DeckResult;

/// Driven port: load validated profiles.
pub trait ProfileStore {
    fn load(&self) -> DeckResult<Vec<Profile>>;
}

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

// Blanket impls so a shared `Arc<Deck>` satisfies the inbound ports — the server
// hands a cheap clone to each connection.
impl<T: Transport + ?Sized> Transport for std::sync::Arc<T> {
    fn play(&self) -> DeckResult<()> {
        (**self).play()
    }
    fn stop(&self) -> DeckResult<()> {
        (**self).stop()
    }
    fn record(&self) -> DeckResult<()> {
        (**self).record()
    }
    fn goto(&self, clip_id: i32) -> DeckResult<()> {
        (**self).goto(clip_id)
    }
    fn next(&self) -> DeckResult<()> {
        (**self).next()
    }
    fn prev(&self) -> DeckResult<()> {
        (**self).prev()
    }
    fn rehome(&self) -> DeckResult<()> {
        (**self).rehome()
    }
}

impl<T: Query + ?Sized> Query for std::sync::Arc<T> {
    fn transport_info(&self) -> TransportInfo {
        (**self).transport_info()
    }
    fn clips(&self) -> ClipList {
        (**self).clips()
    }
    fn slot_info(&self) -> SlotInfo {
        (**self).slot_info()
    }
    fn device_info(&self) -> DeviceInfo {
        (**self).device_info()
    }
}

/// Driven port: deliver keystrokes to a window.
pub trait KeyInjector {
    fn focus(&self, w: &Window) -> DeckResult<()>;
    fn send_keys(&self, w: &Window, chords: &[Chord]) -> DeckResult<()>;
}

/// Driven port: list currently-open OS windows.
pub trait WindowEnumerator {
    fn open_windows(&self) -> DeckResult<Vec<Window>>;
}

/// Driven port: perform a resolved transport action on the locked player through
/// an out-of-band control channel (e.g. an HTTP API or UI Automation) instead of
/// synthesizing keystrokes. Used by API/UIA control profiles.
pub trait PlayerController {
    fn control(&self, p: &Profile, w: &Window, key: KeyName) -> DeckResult<()>;
}

/// Driven port: produce the active clip list.
pub trait ClipSource {
    fn list(&self) -> DeckResult<ClipList>;
}

/// Driven port: best-effort real-state detection. Returns `None` when the probe
/// cannot determine the state (Go's `(state, detected=false)`).
pub trait StateProbe {
    fn detect(&self, w: &Window) -> Option<TransportState>;
}

/// Driven port: reflect lock status in the UI.
pub trait StatusPresenter {
    fn present(&self, lock: &LockState);
}
