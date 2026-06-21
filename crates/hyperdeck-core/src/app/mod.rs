//! Application services (port of `internal/core/app`).

mod lockmanager;
mod reconciler;
mod session;
mod virtualdeck;

pub use lockmanager::{
    ClipSourceFactory, LockManager, SharedEnumerator, SharedPresenter, StateProbeFactory,
};
pub use reconciler::Reconciler;
pub use session::{Session, SharedClipSource, SharedStateProbe};
pub use virtualdeck::{SharedController, SharedInjector, VirtualDeck};
