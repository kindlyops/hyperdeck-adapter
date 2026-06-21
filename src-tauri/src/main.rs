// Tray-only application: no console window on Windows release builds.
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use tauri::menu::{Menu, MenuItem};
use tauri::tray::TrayIconBuilder;

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_updater::Builder::new().build())
        .plugin(tauri_plugin_process::init())
        .plugin(tauri_plugin_dialog::init())
        .setup(|app| {
            // Run as a menu-bar agent on macOS: no Dock icon, no main window.
            #[cfg(target_os = "macos")]
            app.set_activation_policy(tauri::ActivationPolicy::Accessory);

            build_tray(app)?;
            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running HyperDeck Adapter");
}

/// Builds the system-tray menu. The core wiring (HyperDeck server, profile
/// lock/reconcile, status, profile submenu, re-home) is layered in next.
fn build_tray(app: &tauri::App) -> tauri::Result<()> {
    let check = MenuItem::with_id(
        app,
        "check_update",
        "Check for Updates…",
        true,
        None::<&str>,
    )?;
    let quit = MenuItem::with_id(app, "quit", "Quit", true, None::<&str>)?;
    let menu = Menu::with_items(app, &[&check, &quit])?;

    let icon = app.default_window_icon().expect("bundle icon").clone();
    TrayIconBuilder::with_id("main")
        .icon(icon)
        .tooltip("HyperDeck Adapter")
        .menu(&menu)
        .on_menu_event(|app, event| match event.id.as_ref() {
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
