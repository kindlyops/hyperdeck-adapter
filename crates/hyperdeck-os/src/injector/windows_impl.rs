//! Windows keystroke injection + window enumeration (port of `injector_windows.go`).

use std::collections::HashMap;
use std::ffi::c_void;
use std::thread::sleep;
use std::time::Duration;

use windows::Win32::Foundation::{CloseHandle, BOOL, HWND, LPARAM, WPARAM};
use windows::Win32::System::Diagnostics::ToolHelp::{
    CreateToolhelp32Snapshot, Process32FirstW, Process32NextW, PROCESSENTRY32W, TH32CS_SNAPPROCESS,
};
use windows::Win32::System::Threading::{AttachThreadInput, GetCurrentThreadId};
use windows::Win32::UI::Input::KeyboardAndMouse::{
    MapVirtualKeyW, SendInput, SetFocus, INPUT, INPUT_0, INPUT_KEYBOARD, KEYBDINPUT,
    KEYBD_EVENT_FLAGS, KEYEVENTF_EXTENDEDKEY, KEYEVENTF_KEYUP, MAPVK_VK_TO_VSC, VIRTUAL_KEY,
};
use windows::Win32::UI::WindowsAndMessaging::{
    BringWindowToTop, EnumWindows, GetForegroundWindow, GetWindowTextLengthW, GetWindowTextW,
    GetWindowThreadProcessId, IsIconic, IsWindowVisible, PostMessageW, SetForegroundWindow,
    ShowWindow, SystemParametersInfoW, SPIF_SENDCHANGE, SPI_GETFOREGROUNDLOCKTIMEOUT,
    SPI_SETFOREGROUNDLOCKTIMEOUT, SW_RESTORE, SYSTEM_PARAMETERS_INFO_UPDATE_FLAGS, WM_KEYDOWN,
    WM_KEYUP,
};

use hyperdeck_core::domain::{Chord, Window};
use hyperdeck_core::error::{DeckError, DeckResult};
use hyperdeck_core::port::{KeyInjector, WindowEnumerator};

use super::keymap_windows::{is_extended, key_code, modifier_vk};

// Delays that make synthesized input reliable (mirrors the Go injector).
const FOCUS_SETTLE: Duration = Duration::from_millis(120);
const AFTER_KEY: Duration = Duration::from_millis(25);
const KEY_HOLD: Duration = Duration::from_millis(15);

const LPARAM_KEYUP_FLAGS: isize = 0xC000_0000; // bits 30 (previous-down) + 31 (transition/up)
const LPARAM_EXTENDED_FLAG: isize = 0x0100_0000; // bit 24

/// Stateless Windows injector + window enumerator.
pub struct WinInput;

fn to_hwnd(handle: usize) -> HWND {
    HWND(handle as *mut c_void)
}

impl KeyInjector for WinInput {
    fn focus(&self, win: &Window) -> DeckResult<()> {
        if win.handle == 0 {
            return Err(DeckError::Injector("focus: nil window handle".into()));
        }
        let hwnd = to_hwnd(win.handle);
        unsafe {
            if IsIconic(hwnd).as_bool() {
                let _ = ShowWindow(hwnd, SW_RESTORE);
            }

            // Drop the foreground-lock timeout to 0 around the call (restoring it
            // after) so SetForegroundWindow is honored — the standard workaround.
            let mut prev_timeout: u32 = 0;
            let _ = SystemParametersInfoW(
                SPI_GETFOREGROUNDLOCKTIMEOUT,
                0,
                Some(&mut prev_timeout as *mut u32 as *mut c_void),
                SYSTEM_PARAMETERS_INFO_UPDATE_FLAGS(0),
            );
            let _ = SystemParametersInfoW(SPI_SETFOREGROUNDLOCKTIMEOUT, 0, None, SPIF_SENDCHANGE);

            let cur_thread = GetCurrentThreadId();
            let fg = GetForegroundWindow();
            let fg_thread = if fg.0.is_null() {
                0
            } else {
                GetWindowThreadProcessId(fg, None)
            };

            // Attaching our input queue to the foreground thread lets
            // SetForegroundWindow/SetFocus act as if the same thread owned focus.
            let attached = fg_thread != 0 && fg_thread != cur_thread;
            if attached {
                let _ = AttachThreadInput(cur_thread, fg_thread, BOOL(1));
            }
            let _ = BringWindowToTop(hwnd);
            let _ = SetForegroundWindow(hwnd);
            let _ = SetFocus(hwnd);
            if attached {
                let _ = AttachThreadInput(cur_thread, fg_thread, BOOL(0));
            }

            let _ = SystemParametersInfoW(
                SPI_SETFOREGROUNDLOCKTIMEOUT,
                0,
                Some(&mut prev_timeout as *mut u32 as *mut c_void),
                SPIF_SENDCHANGE,
            );

            sleep(FOCUS_SETTLE);

            let got = GetForegroundWindow();
            if got.0 != hwnd.0 {
                return Err(DeckError::Injector(format!(
                    "focus: window {:p} did not become foreground (got {:p}); \
                     check it is not elevated relative to this process",
                    hwnd.0, got.0
                )));
            }
        }
        Ok(())
    }

    fn send_keys(&self, win: &Window, chords: &[Chord]) -> DeckResult<()> {
        if win.handle == 0 {
            return Err(DeckError::Injector("sendkeys: nil window handle".into()));
        }
        let hwnd = to_hwnd(win.handle);
        let foreground = unsafe { GetForegroundWindow().0 == hwnd.0 };

        for c in chords {
            let Some(vk) = key_code(&c.key) else {
                return Err(DeckError::Injector(format!(
                    "sendkeys: no Windows key code for {:?}",
                    c.key
                )));
            };
            if foreground {
                send_input_chord(&c.mods, vk)?;
            } else {
                post_key(hwnd, vk)?;
            }
            sleep(AFTER_KEY);
        }
        Ok(())
    }
}

/// Posts a WM_KEYDOWN/WM_KEYUP pair to the HWND (background-capable).
fn post_key(hwnd: HWND, vk: u16) -> DeckResult<()> {
    unsafe {
        let scan = MapVirtualKeyW(vk as u32, MAPVK_VK_TO_VSC) as isize;
        let ext = if is_extended(vk) {
            LPARAM_EXTENDED_FLAG
        } else {
            0
        };
        let down = 1isize | ((scan & 0xFF) << 16) | ext;
        let up = down | LPARAM_KEYUP_FLAGS;
        PostMessageW(hwnd, WM_KEYDOWN, WPARAM(vk as usize), LPARAM(down)).map_err(|e| {
            DeckError::Injector(format!("sendkeys: PostMessage WM_KEYDOWN vk={vk:#x}: {e}"))
        })?;
        sleep(KEY_HOLD);
        PostMessageW(hwnd, WM_KEYUP, WPARAM(vk as usize), LPARAM(up)).map_err(|e| {
            DeckError::Injector(format!("sendkeys: PostMessage WM_KEYUP vk={vk:#x}: {e}"))
        })?;
    }
    Ok(())
}

/// Drives the system input queue: modifier-down(s) → key-down → key-up →
/// modifier-up(s) in reverse. Goes to the foreground window.
fn send_input_chord(mods: &[hyperdeck_core::domain::Modifier], vk: u16) -> DeckResult<()> {
    let mut seq: Vec<INPUT> = Vec::new();
    let mod_vks: Vec<u16> = mods.iter().map(|m| modifier_vk(*m)).collect();
    for &mv in &mod_vks {
        seq.push(key_event(mv, false));
    }
    seq.push(key_event(vk, false));
    seq.push(key_event(vk, true));
    for &mv in mod_vks.iter().rev() {
        seq.push(key_event(mv, true));
    }

    let sent = unsafe { SendInput(&seq, std::mem::size_of::<INPUT>() as i32) };
    if sent as usize != seq.len() {
        return Err(DeckError::Injector(format!(
            "sendkeys: SendInput inserted {sent}/{} events (UIPI/elevation?)",
            seq.len()
        )));
    }
    Ok(())
}

/// Builds one keyboard INPUT for a virtual key (down, or up when `up`).
fn key_event(vk: u16, up: bool) -> INPUT {
    let scan = unsafe { MapVirtualKeyW(vk as u32, MAPVK_VK_TO_VSC) } as u16;
    let mut flags = KEYBD_EVENT_FLAGS(0);
    if is_extended(vk) {
        flags |= KEYEVENTF_EXTENDEDKEY;
    }
    if up {
        flags |= KEYEVENTF_KEYUP;
    }
    INPUT {
        r#type: INPUT_KEYBOARD,
        Anonymous: INPUT_0 {
            ki: KEYBDINPUT {
                wVk: VIRTUAL_KEY(vk),
                wScan: scan,
                dwFlags: flags,
                time: 0,
                dwExtraInfo: 0,
            },
        },
    }
}

struct EnumCtx {
    names: HashMap<u32, String>,
    out: Vec<Window>,
}

impl WindowEnumerator for WinInput {
    fn open_windows(&self) -> DeckResult<Vec<Window>> {
        let mut ctx = EnumCtx {
            names: process_names(),
            out: Vec::new(),
        };
        unsafe {
            EnumWindows(Some(enum_proc), LPARAM(&mut ctx as *mut EnumCtx as isize))
                .map_err(|e| DeckError::Other(format!("openwindows: EnumWindows: {e}")))?;
        }
        Ok(ctx.out)
    }
}

unsafe extern "system" fn enum_proc(hwnd: HWND, lparam: LPARAM) -> BOOL {
    let ctx = &mut *(lparam.0 as *mut EnumCtx);
    if !IsWindowVisible(hwnd).as_bool() {
        return BOOL(1); // continue
    }
    let title = window_text(hwnd);
    if title.is_empty() {
        return BOOL(1);
    }
    let mut pid: u32 = 0;
    GetWindowThreadProcessId(hwnd, Some(&mut pid));
    ctx.out.push(Window {
        handle: hwnd.0 as usize,
        title,
        process: ctx.names.get(&pid).cloned().unwrap_or_default(),
    });
    BOOL(1)
}

/// Reads a window's title via GetWindowTextLengthW + GetWindowTextW.
fn window_text(hwnd: HWND) -> String {
    unsafe {
        let n = GetWindowTextLengthW(hwnd);
        if n == 0 {
            return String::new();
        }
        let mut buf = vec![0u16; (n + 1) as usize];
        let got = GetWindowTextW(hwnd, &mut buf);
        String::from_utf16_lossy(&buf[..got as usize])
    }
}

/// Builds a pid → executable base name map via a Toolhelp snapshot.
fn process_names() -> HashMap<u32, String> {
    let mut names = HashMap::new();
    unsafe {
        let Ok(snap) = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0) else {
            return names;
        };
        let mut e = PROCESSENTRY32W {
            dwSize: std::mem::size_of::<PROCESSENTRY32W>() as u32,
            ..Default::default()
        };
        if Process32FirstW(snap, &mut e).is_ok() {
            loop {
                let end = e
                    .szExeFile
                    .iter()
                    .position(|&c| c == 0)
                    .unwrap_or(e.szExeFile.len());
                names.insert(
                    e.th32ProcessID,
                    String::from_utf16_lossy(&e.szExeFile[..end]),
                );
                if Process32NextW(snap, &mut e).is_err() {
                    break;
                }
            }
        }
        let _ = CloseHandle(snap);
    }
    names
}
