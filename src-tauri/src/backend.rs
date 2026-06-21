//! Composition root: builds and starts the HyperDeck core (protocol server +
//! profile lock + reconcile) over the OS adapters, independent of the UI.
//!
//! This is the Rust analog of the Go `cmd/hyperdeck-adapter/main.go` wiring.

use std::net::TcpListener;
use std::path::PathBuf;
use std::sync::Arc;
use std::thread;
use std::time::Duration;

use hyperdeck_core::app::{
    ClipSourceFactory, LockManager, Reconciler, Session, StateProbeFactory, VirtualDeck,
};
use hyperdeck_core::config::{self, SelectionStore, Store};
use hyperdeck_core::domain::{ControlMode, KeyName, Profile, Window};
use hyperdeck_core::error::DeckResult;
use hyperdeck_core::port::{PlayerController, ProfileStore, StatusPresenter};
use hyperdeck_core::protocol::Server;
use hyperdeck_core::{clipsource, stateprobe};
use hyperdeck_os::injector;
use hyperdeck_os::vlchttp::VlcController;

/// Handles the UI needs once the backend is running. `lock_manager`, `selection`,
/// `profile_ids`, and `active` are consumed by the tray profile submenu (added
/// next); `deck` powers Re-home today.
#[allow(dead_code)]
pub struct Backend {
    pub deck: Arc<VirtualDeck>,
    pub lock_manager: Arc<LockManager>,
    pub selection: SelectionStore,
    pub profile_ids: Vec<String>,
    pub active: String,
}

/// Routes a transport action to the controller backend for the profile's control
/// mode (api -> VLC HTTP, uia -> UI Automation). Keystroke profiles never reach a
/// controller — the injector handles them.
struct ControlRouter {
    vlc: VlcController,
    #[cfg(windows)]
    uia: Arc<hyperdeck_os::uia::Engine>,
}

impl PlayerController for ControlRouter {
    fn control(&self, p: &Profile, w: &Window, key: KeyName) -> DeckResult<()> {
        match p.control {
            ControlMode::Api => self.vlc.control(p, w, key),
            ControlMode::Uia => {
                #[cfg(windows)]
                {
                    self.uia.control(p, w, key)
                }
                #[cfg(not(windows))]
                {
                    let _ = (p, w, key);
                    Ok(())
                }
            }
            ControlMode::Keys => Ok(()),
        }
    }
}

/// The default profiles.yaml location (OS config dir / hyperdeck-adapter).
pub fn config_path() -> PathBuf {
    base_dir().join("profiles.yaml")
}

/// The default selection.json location.
pub fn selection_path() -> PathBuf {
    base_dir().join("selection.json")
}

fn base_dir() -> PathBuf {
    dirs::config_dir()
        .unwrap_or_else(|| PathBuf::from("."))
        .join("hyperdeck-adapter")
}

/// Builds the core, binds the protocol server, applies the persisted profile
/// selection, and spawns the server / lock / reconcile threads. Returns the
/// handles the UI needs (deck for re-home, lock manager for profile pinning).
pub fn start(
    presenter: Arc<dyn StatusPresenter + Send + Sync>,
    bind: &str,
    poll: Duration,
) -> Result<Backend, String> {
    let cfg = config_path();
    // First-run convenience: seed the default config when none exists.
    config::ensure_default(&cfg).map_err(|e| e.to_string())?;
    let profiles = Store::new(&cfg).load().map_err(|e| e.to_string())?;
    let profile_ids: Vec<String> = profiles.iter().map(|p| p.id.clone()).collect();

    let selection = SelectionStore::new(selection_path());
    let active = validate_active(selection.load().unwrap_or_default(), &profiles);

    let inj = injector::injector();
    let enumerator = injector::enumerator();

    #[cfg(windows)]
    let uia_engine = Arc::new(hyperdeck_os::uia::Engine::new());

    let router = ControlRouter {
        vlc: VlcController::new(),
        #[cfg(windows)]
        uia: uia_engine.clone(),
    };
    let controller = Arc::new(router);

    let namer: Option<stateprobe::SharedElementNamer> = {
        #[cfg(windows)]
        {
            Some(uia_engine.clone() as stateprobe::SharedElementNamer)
        }
        #[cfg(not(windows))]
        {
            None
        }
    };

    let session = Arc::new(Session::new());
    let deck = Arc::new(VirtualDeck::new(session.clone(), inj).with_controller(controller));

    let clips_for: ClipSourceFactory = Box::new(|p: &Profile| clipsource::new(p));
    let probe_for: StateProbeFactory =
        Box::new(move |p: &Profile| stateprobe::new(p, namer.clone()));

    let lock_manager = Arc::new(LockManager::new(
        session.clone(),
        enumerator,
        profiles,
        presenter,
        clips_for,
        probe_for,
    ));
    let reconciler = Arc::new(Reconciler::new(session));

    let listener = TcpListener::bind(bind).map_err(|e| format!("listen {bind}: {e}"))?;
    let server = Server::new(deck.clone());
    thread::spawn(move || {
        let _ = server.serve(listener);
    });

    // Apply the persisted selection and lock immediately if a match is running.
    lock_manager.set_active(&active);

    {
        let lm = lock_manager.clone();
        thread::spawn(move || loop {
            lm.poll();
            thread::sleep(poll);
        });
    }
    {
        let rec = reconciler.clone();
        thread::spawn(move || loop {
            rec.tick();
            thread::sleep(poll);
        });
    }

    Ok(Backend {
        deck,
        lock_manager,
        selection,
        profile_ids,
        active,
    })
}

/// Keeps a pinned id only when it still names a loaded profile; an unknown id
/// (renamed/removed profile) falls back to Auto.
fn validate_active(active: String, profiles: &[Profile]) -> String {
    if active.is_empty() || profiles.iter().any(|p| p.id == active) {
        active
    } else {
        String::new()
    }
}
