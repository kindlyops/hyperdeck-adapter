//! Diagnostic for the injector adapter: list on-screen windows, focus an app by
//! pid/handle, and send key chords. It verifies the real OS injector against a
//! running player during on-device testing (port of the Go `cmd/injcheck`).
//!
//! Usage:
//!   injcheck trust                     # check / prompt for input permission (macOS)
//!   injcheck list [filter]             # list windows, optionally filtered
//!   injcheck focus <pid>               # bring an app to the foreground
//!   injcheck keys   <pid> <chord...>   # focus, then send chords (foreground)
//!   injcheck bgkeys <pid> <chord...>   # send chords without stealing focus

use std::process::exit;

use hyperdeck_core::domain::{parse_chord, Chord, Window};
use hyperdeck_os::injector;

fn main() {
    let args: Vec<String> = std::env::args().collect();
    if args.len() < 2 {
        usage();
    }
    match args[1].as_str() {
        "trust" => trust(),
        "list" => list(&args[2..].join(" ")),
        "focus" => {
            if args.len() < 3 {
                usage();
            }
            focus(must_pid(&args[2]));
        }
        "keys" => {
            if args.len() < 4 {
                usage();
            }
            keys(must_pid(&args[2]), &args[3..], true);
        }
        "bgkeys" => {
            if args.len() < 4 {
                usage();
            }
            keys(must_pid(&args[2]), &args[3..], false);
        }
        _ => usage(),
    }
}

fn trust() {
    if injector::request_accessibility() {
        println!("Accessibility: granted");
    } else {
        eprintln!("Accessibility: NOT granted — enable this binary in");
        eprintln!("System Settings > Privacy & Security > Accessibility, then re-run.");
        exit(1);
    }
}

fn list(filter: &str) {
    let windows = injector::enumerator()
        .open_windows()
        .unwrap_or_else(|e| fail(&format!("open_windows: {e}")));
    let filter = filter.to_lowercase();
    println!("{:<8}  {:<28}  TITLE", "HANDLE", "PROCESS");
    for w in windows {
        if !filter.is_empty()
            && !w.process.to_lowercase().contains(&filter)
            && !w.title.to_lowercase().contains(&filter)
        {
            continue;
        }
        println!("{:<8}  {:<28}  {}", w.handle, w.process, w.title);
    }
}

fn focus(pid: usize) {
    injector::injector()
        .focus(&window(pid))
        .unwrap_or_else(|e| fail(&format!("focus: {e}")));
    println!("focused pid {pid}");
}

fn keys(pid: usize, specs: &[String], focus: bool) {
    let mut chords: Vec<Chord> = Vec::with_capacity(specs.len());
    for s in specs {
        match parse_chord(s) {
            Ok(c) => chords.push(c),
            Err(e) => fail(&format!("parse chord {s:?}: {e}")),
        }
    }
    let w = window(pid);
    let inj = injector::injector();
    if focus {
        inj.focus(&w)
            .unwrap_or_else(|e| fail(&format!("focus: {e}")));
    }
    inj.send_keys(&w, &chords)
        .unwrap_or_else(|e| fail(&format!("send_keys: {e}")));
    let mode = if focus { "foreground" } else { "background" };
    println!(
        "sent {} chord(s) to pid {pid} ({mode}): {specs:?}",
        chords.len()
    );
}

/// The injector targets a window by its native handle, which is the pid here.
fn window(pid: usize) -> Window {
    Window {
        handle: pid,
        ..Default::default()
    }
}

fn must_pid(s: &str) -> usize {
    s.parse()
        .unwrap_or_else(|_| fail(&format!("invalid pid {s:?}")))
}

fn usage() -> ! {
    eprintln!(
        "usage: injcheck trust | list [filter] | focus <pid> | keys <pid> <chord...> | bgkeys <pid> <chord...>"
    );
    exit(2);
}

fn fail(msg: &str) -> ! {
    eprintln!("injcheck: {msg}");
    exit(1);
}
