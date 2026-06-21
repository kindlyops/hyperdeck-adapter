//! VLC HTTP controller (port of `internal/adapter/driven/vlchttp`).
//!
//! Drives VLC through its HTTP "requests" interface (Preferences > Interface >
//! Main interfaces > Web, or `vlc --extraintf http --http-password <pw>`). It is
//! the control backend for API-control profiles whose `api.type` is `vlc_http`.

use std::time::Duration;

use base64::engine::general_purpose::STANDARD;
use base64::Engine as _;

use hyperdeck_core::domain::{KeyName, Profile, Window};
use hyperdeck_core::error::{DeckError, DeckResult};
use hyperdeck_core::port::PlayerController;

/// Used when a profile's `api.base_url` is empty.
pub const DEFAULT_BASE_URL: &str = "http://127.0.0.1:8080";

/// Maps a logical transport action to a VLC `requests/status.json` command.
/// Actions with no VLC equivalent (e.g. record) return `None` and are an acked
/// no-op, mirroring the injector's handling of unmapped keys.
fn command_for(key: KeyName) -> Option<&'static str> {
    match key {
        KeyName::Play => Some("pl_play"),
        KeyName::Stop => Some("pl_stop"),
        KeyName::Next => Some("pl_next"),
        KeyName::Prev => Some("pl_previous"),
        KeyName::Record => None,
    }
}

/// Drives VLC over HTTP with a bounded per-request timeout.
pub struct VlcController {
    agent: ureq::Agent,
}

impl VlcController {
    /// Returns a VLC HTTP controller.
    pub fn new() -> Self {
        let agent = ureq::AgentBuilder::new()
            .timeout(Duration::from_secs(4))
            .build();
        VlcController { agent }
    }
}

impl Default for VlcController {
    fn default() -> Self {
        Self::new()
    }
}

impl PlayerController for VlcController {
    /// Issues the VLC command for `key` against the player described by the
    /// profile's api config. The window is unused (VLC is addressed by URL).
    fn control(&self, p: &Profile, _w: &Window, key: KeyName) -> DeckResult<()> {
        let Some(cmd) = command_for(key) else {
            return Ok(()); // no VLC equivalent: acked no-op
        };

        let base = if p.api.base_url.is_empty() {
            DEFAULT_BASE_URL
        } else {
            p.api.base_url.as_str()
        };
        let url = format!(
            "{}/requests/status.json?command={cmd}",
            base.trim_end_matches('/')
        );
        // VLC's HTTP interface uses Basic auth with an empty username.
        let auth = format!("Basic {}", STANDARD.encode(format!(":{}", p.api.password)));

        match self.agent.get(&url).set("Authorization", &auth).call() {
            Ok(_) => Ok(()),
            Err(ureq::Error::Status(401, _)) => Err(DeckError::Other(format!(
                "vlc control {key:?}: unauthorized — check api.password matches VLC's HTTP password"
            ))),
            Err(ureq::Error::Status(code, _)) => Err(DeckError::Other(format!(
                "vlc control {key:?} ({cmd}): HTTP {code}"
            ))),
            Err(e) => Err(DeckError::Other(format!(
                "vlc control {key:?} ({cmd}): {e}"
            ))),
        }
    }
}

#[cfg(test)]
mod tests {
    use std::io::{BufRead, BufReader, Write};
    use std::net::TcpListener;
    use std::sync::mpsc;
    use std::thread;

    use super::*;
    use hyperdeck_core::domain::{ApiConfig, ControlMode};

    /// What the stub server observed about one request.
    #[derive(Default, Debug)]
    struct Seen {
        request_line: String,
        authorization: String,
    }

    /// Starts a one-shot HTTP stub that replies with `status`, returning the bound
    /// base URL and a receiver delivering what it saw (or nothing if never hit).
    fn stub_server(status: u16) -> (String, mpsc::Receiver<Seen>) {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let base = format!("http://{}", listener.local_addr().unwrap());
        let (tx, rx) = mpsc::channel();
        thread::spawn(move || {
            if let Ok((mut stream, _)) = listener.accept() {
                let mut reader = BufReader::new(stream.try_clone().unwrap());
                let mut seen = Seen::default();
                let mut line = String::new();
                reader.read_line(&mut line).ok();
                seen.request_line = line.trim_end().to_string();
                loop {
                    let mut header = String::new();
                    if reader.read_line(&mut header).unwrap_or(0) == 0 {
                        break;
                    }
                    let trimmed = header.trim_end();
                    if trimmed.is_empty() {
                        break;
                    }
                    if let Some(v) = trimmed.strip_prefix("Authorization:") {
                        seen.authorization = v.trim().to_string();
                    }
                }
                let body = "{}";
                let _ = write!(
                    stream,
                    "HTTP/1.1 {status} X\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                    body.len()
                );
                let _ = stream.flush();
                let _ = tx.send(seen);
            }
        });
        (base, rx)
    }

    fn api_profile(base_url: &str) -> Profile {
        Profile {
            id: "vlc".into(),
            control: ControlMode::Api,
            api: ApiConfig {
                kind: "vlc_http".into(),
                base_url: base_url.into(),
                password: "pw".into(),
            },
            ..Default::default()
        }
    }

    fn decode_basic(header: &str) -> String {
        let b64 = header.strip_prefix("Basic ").unwrap_or("");
        String::from_utf8(STANDARD.decode(b64).unwrap_or_default()).unwrap_or_default()
    }

    #[test]
    fn issues_mapped_commands_with_basic_auth() {
        for (key, want_cmd) in [
            (KeyName::Play, "pl_play"),
            (KeyName::Stop, "pl_stop"),
            (KeyName::Next, "pl_next"),
            (KeyName::Prev, "pl_previous"),
        ] {
            let (base, rx) = stub_server(200);
            VlcController::new()
                .control(&api_profile(&base), &Window::default(), key)
                .expect("ok");
            let seen = rx.recv().expect("server saw a request");
            assert!(
                seen.request_line
                    .contains(&format!("/requests/status.json?command={want_cmd}")),
                "{key:?}: request line {:?}",
                seen.request_line
            );
            // Empty username + "pw" password.
            assert_eq!(decode_basic(&seen.authorization), ":pw");
        }
    }

    #[test]
    fn unmapped_key_is_noop() {
        // Record has no VLC equivalent: returns Ok without contacting the server.
        let (base, rx) = stub_server(200);
        VlcController::new()
            .control(&api_profile(&base), &Window::default(), KeyName::Record)
            .expect("ok");
        assert!(
            rx.recv_timeout(Duration::from_millis(300)).is_err(),
            "server must not be hit"
        );
    }

    #[test]
    fn unauthorized_is_error() {
        let (base, _rx) = stub_server(401);
        let err =
            VlcController::new().control(&api_profile(&base), &Window::default(), KeyName::Play);
        assert!(err.is_err());
    }

    #[test]
    fn server_error_is_error() {
        let (base, _rx) = stub_server(500);
        let err =
            VlcController::new().control(&api_profile(&base), &Window::default(), KeyName::Stop);
        assert!(err.is_err());
    }

    #[test]
    fn empty_base_url_defaults_without_panic() {
        let p = api_profile("");
        assert!(p.api.base_url.is_empty());
        // Nothing listening on the default port; must error, not panic.
        let _ = VlcController::new().control(&p, &Window::default(), KeyName::Play);
    }
}
