//! Shared in-memory test doubles (port of `injector.Mock`), available only to
//! the crate's own unit tests.
//!
//! Some helpers here are consumed by sibling modules' tests (e.g. the lock
//! manager), so not every item is exercised from within this module.
#![allow(dead_code)]

use std::sync::Mutex;

use crate::domain::{Chord, Window};
use crate::error::DeckResult;
use crate::port::{KeyInjector, WindowEnumerator};

/// One recorded `send_keys` call.
#[derive(Debug, Clone)]
pub struct SentKeys {
    pub window: Window,
    pub chords: Vec<Chord>,
}

#[derive(Default)]
struct MockState {
    windows: Vec<Window>,
    focused: Vec<Window>,
    sent: Vec<SentKeys>,
    focus_err: Option<String>,
    send_err: Option<String>,
    enum_err: Option<String>,
}

/// An in-memory [`KeyInjector`] + [`WindowEnumerator`] for tests.
#[derive(Default)]
pub struct MockInjector {
    state: Mutex<MockState>,
}

impl MockInjector {
    pub fn new() -> Self {
        Self::default()
    }

    /// Sets the windows returned by `open_windows`.
    pub fn set_windows(&self, windows: Vec<Window>) {
        self.state.lock().unwrap().windows = windows;
    }

    /// Flat list of the base key of every chord sent so far (matches the Go
    /// `sentKeys` helper).
    pub fn sent_keys(&self) -> Vec<String> {
        self.state
            .lock()
            .unwrap()
            .sent
            .iter()
            .flat_map(|s| s.chords.iter().map(|c| c.key.clone()))
            .collect()
    }

    /// Windows that have been focused.
    pub fn focused(&self) -> Vec<Window> {
        self.state.lock().unwrap().focused.clone()
    }

    /// Clears recorded `send_keys` calls (matches `m.Sent = nil`).
    pub fn clear_sent(&self) {
        self.state.lock().unwrap().sent.clear();
    }
}

impl KeyInjector for MockInjector {
    fn focus(&self, w: &Window) -> DeckResult<()> {
        let mut g = self.state.lock().unwrap();
        if let Some(e) = &g.focus_err {
            return Err(crate::error::DeckError::Injector(e.clone()));
        }
        g.focused.push(w.clone());
        Ok(())
    }

    fn send_keys(&self, w: &Window, chords: &[Chord]) -> DeckResult<()> {
        let mut g = self.state.lock().unwrap();
        if let Some(e) = &g.send_err {
            return Err(crate::error::DeckError::Injector(e.clone()));
        }
        g.sent.push(SentKeys {
            window: w.clone(),
            chords: chords.to_vec(),
        });
        Ok(())
    }
}

impl WindowEnumerator for MockInjector {
    fn open_windows(&self) -> DeckResult<Vec<Window>> {
        let g = self.state.lock().unwrap();
        if let Some(e) = &g.enum_err {
            return Err(crate::error::DeckError::Other(e.clone()));
        }
        Ok(g.windows.clone())
    }
}
