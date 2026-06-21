// Tray-only application: no console window on Windows release builds.
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

mod backend;

use std::sync::Arc;
use std::time::Duration;

use tauri::menu::{CheckMenuItem, IsMenuItem, Menu, MenuItem, PredefinedMenuItem, Submenu};
use tauri::tray::TrayIconBuilder;

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

            build_tray(app, backend)?;
            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running HyperDeck Adapter");
}

/// Builds the system-tray menu: a Profile submenu (Auto + one entry per profile),
/// Re-home, Check for Updates…, and Quit.
fn build_tray(app: &tauri::App, backend: backend::Backend) -> tauri::Result<()> {
    use hyperdeck_core::port::Transport;

    let backend::Backend {
        deck,
        lock_manager,
        selection,
        profile_ids,
        active,
    } = backend;

    // Profile submenu: "Auto (match any)" plus one checkbox per profile id, with
    // the checkmark on the persisted selection (Auto when none / unknown).
    let checked = checked_profile(&profile_ids, &active);
    let mut profile_items: Vec<CheckMenuItem<tauri::Wry>> =
        Vec::with_capacity(profile_ids.len() + 1);
    profile_items.push(CheckMenuItem::with_id(
        app,
        "profile:",
        "Auto (match any)",
        true,
        checked.is_empty(),
        None::<&str>,
    )?);
    for id in &profile_ids {
        profile_items.push(CheckMenuItem::with_id(
            app,
            format!("profile:{id}"),
            id,
            true,
            checked == *id,
            None::<&str>,
        )?);
    }
    let item_refs: Vec<&dyn IsMenuItem<tauri::Wry>> = profile_items
        .iter()
        .map(|i| i as &dyn IsMenuItem<tauri::Wry>)
        .collect();
    let profile_menu =
        Submenu::with_id_and_items(app, "profile_menu", "Profile", true, &item_refs)?;

    // (id, item) pairs for moving the checkmark on selection; "" is the Auto entry.
    let mut checks: Vec<(String, CheckMenuItem<tauri::Wry>)> =
        Vec::with_capacity(profile_items.len());
    checks.push((String::new(), profile_items[0].clone()));
    for (i, id) in profile_ids.iter().enumerate() {
        checks.push((id.clone(), profile_items[i + 1].clone()));
    }

    let sep = PredefinedMenuItem::separator(app)?;
    let rehome = MenuItem::with_id(app, "rehome", "Re-home", true, None::<&str>)?;
    let check = MenuItem::with_id(
        app,
        "check_update",
        "Check for Updates…",
        true,
        None::<&str>,
    )?;
    let quit = MenuItem::with_id(app, "quit", "Quit", true, None::<&str>)?;
    let menu = Menu::with_items(app, &[&profile_menu, &sep, &rehome, &check, &quit])?;

    let icon = app.default_window_icon().expect("bundle icon").clone();
    TrayIconBuilder::with_id("main")
        .icon(icon)
        .tooltip("HyperDeck Adapter")
        .menu(&menu)
        .on_menu_event(move |app, event| {
            let id = event.id.as_ref();
            if let Some(profile_id) = id.strip_prefix("profile:") {
                select_profile(&lock_manager, &selection, &checks, profile_id);
                return;
            }
            match id {
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
            }
        })
        .build(app)?;
    Ok(())
}

/// Pins `id` (empty = Auto): persists the selection, re-points the lock manager
/// (which re-polls immediately), and moves the checkmark to the chosen entry.
fn select_profile(
    lock_manager: &hyperdeck_core::app::LockManager,
    selection: &hyperdeck_core::config::SelectionStore,
    checks: &[(String, CheckMenuItem<tauri::Wry>)],
    id: &str,
) {
    if let Err(e) = selection.save(id) {
        log::warn!("persist profile selection failed: {e}");
    }
    lock_manager.set_active(id);
    for (key, item) in checks {
        let _ = item.set_checked(key == id);
    }
}

/// Port of the Go `checkedProfile`: the id whose entry shows a checkmark — the
/// active id when it names a known profile, otherwise "" (the Auto entry).
fn checked_profile(profiles: &[String], active: &str) -> String {
    if active.is_empty() || !profiles.iter().any(|p| p == active) {
        String::new()
    } else {
        active.to_string()
    }
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

#[cfg(test)]
mod tests {
    use super::checked_profile;

    fn ids() -> Vec<String> {
        vec!["vlc".to_string(), "mitti".to_string()]
    }

    #[test]
    fn empty_active_is_auto() {
        assert_eq!(checked_profile(&ids(), ""), "");
    }

    #[test]
    fn known_active_is_checked() {
        assert_eq!(checked_profile(&ids(), "mitti"), "mitti");
    }

    #[test]
    fn unknown_active_falls_back_to_auto() {
        assert_eq!(checked_profile(&ids(), "deleted"), "");
    }
}
