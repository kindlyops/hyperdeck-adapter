// Tray-only application: no console window on Windows release builds.
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

mod backend;

use std::sync::Arc;
use std::time::Duration;

use tauri::menu::{Menu, MenuItem};
use tauri::tray::TrayIconBuilder;
use tauri::Manager;

use hyperdeck_core::domain::LockState;
use hyperdeck_core::port::StatusPresenter;

const BIND: &str = "0.0.0.0:9993";
const POLL: Duration = Duration::from_secs(1);

fn main() {
    let args: Vec<String> = std::env::args().collect();

    // Verify / prompt for input permission, then exit (parity with the Go flag).
    if args.iter().any(|a| a == "--check-accessibility") {
        if hyperdeck_os::injector::request_accessibility() {
            println!("input permission granted");
            std::process::exit(0);
        }
        eprintln!("input permission not granted; enable this app under the OS input/accessibility settings");
        std::process::exit(1);
    }

    // Headless mode: run the core with a logging presenter, no tray.
    if args.iter().any(|a| a == "--no-tray" || a == "--headless") {
        run_headless();
        return;
    }

    run_tray();
}

fn run_headless() {
    let presenter: Arc<dyn StatusPresenter + Send + Sync> = Arc::new(LogPresenter);
    match backend::start(presenter, BIND, POLL) {
        Ok(_backend) => {
            log::info!("hyperdeck-adapter started (headless) on {BIND}");
            loop {
                std::thread::sleep(Duration::from_secs(3600));
            }
        }
        Err(e) => {
            eprintln!("failed to start: {e}");
            std::process::exit(1);
        }
    }
}

fn run_tray() {
    tauri::Builder::default()
        .plugin(tauri_plugin_updater::Builder::new().build())
        .plugin(tauri_plugin_process::init())
        .plugin(tauri_plugin_dialog::init())
        .setup(|app| {
            // Run as a menu-bar agent on macOS: no Dock icon, no main window.
            #[cfg(target_os = "macos")]
            app.set_activation_policy(tauri::ActivationPolicy::Accessory);

            let presenter: Arc<dyn StatusPresenter + Send + Sync> = Arc::new(TrayPresenter {
                app: app.handle().clone(),
            });
            let backend = backend::start(presenter, BIND, POLL)
                .map_err(|e| -> Box<dyn std::error::Error> { e.into() })?;

            build_tray(app, backend.deck)?;
            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running HyperDeck Adapter");
}

/// Builds the system-tray menu (Re-home, Check for Updates…, Quit).
fn build_tray(app: &tauri::App, deck: Arc<hyperdeck_core::app::VirtualDeck>) -> tauri::Result<()> {
    use hyperdeck_core::port::Transport;

    let rehome = MenuItem::with_id(app, "rehome", "Re-home", true, None::<&str>)?;
    let check = MenuItem::with_id(
        app,
        "check_update",
        "Check for Updates…",
        true,
        None::<&str>,
    )?;
    let quit = MenuItem::with_id(app, "quit", "Quit", true, None::<&str>)?;
    let menu = Menu::with_items(app, &[&rehome, &check, &quit])?;

    let icon = app.default_window_icon().expect("bundle icon").clone();
    TrayIconBuilder::with_id("main")
        .icon(icon)
        .tooltip("HyperDeck Adapter")
        .menu(&menu)
        .on_menu_event(move |app, event| match event.id.as_ref() {
            "rehome" => {
                if let Err(e) = deck.rehome() {
                    log::warn!("re-home failed: {e}");
                }
            }
            "check_update" => {
                let handle = app.clone();
                tauri::async_runtime::spawn(async move {
                    check_for_updates(handle).await;
                });
            }
            "quit" => app.exit(0),
            _ => {}
        })
        .build(app)?;
    Ok(())
}

/// Checks for, downloads, verifies, and installs an update, then relaunches.
async fn check_for_updates(app: tauri::AppHandle) {
    use tauri_plugin_updater::UpdaterExt;
    let updater = match app.updater() {
        Ok(u) => u,
        Err(e) => {
            log::warn!("updater unavailable: {e}");
            return;
        }
    };
    match updater.check().await {
        Ok(Some(update)) => {
            log::info!("installing update {}", update.version);
            if let Err(e) = update.download_and_install(|_, _| {}, || {}).await {
                log::error!("update install failed: {e}");
                return;
            }
            app.restart();
        }
        Ok(None) => log::info!("already up to date"),
        Err(e) => log::error!("update check failed: {e}"),
    }
}

/// Reflects lock status in the tray tooltip.
struct TrayPresenter {
    app: tauri::AppHandle,
}

impl StatusPresenter for TrayPresenter {
    fn present(&self, lock: &LockState) {
        let text = status_text(lock);
        if let Some(tray) = self.app.tray_by_id("main") {
            let _ = tray.set_tooltip(Some(&text));
        }
    }
}

/// Logs lock status (headless mode).
struct LogPresenter;

impl StatusPresenter for LogPresenter {
    fn present(&self, lock: &LockState) {
        log::info!("{}", status_text(lock));
    }
}

fn status_text(lock: &LockState) -> String {
    match (lock.locked, &lock.profile) {
        (true, Some(p)) => format!("Locked: {} ({})", p.id, lock.window.title),
        _ => "Disconnected — no player".to_string(),
    }
}
