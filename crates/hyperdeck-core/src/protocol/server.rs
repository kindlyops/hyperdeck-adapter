//! HyperDeck TCP server (port of `internal/adapter/driving/hyperdeck/server.go`).

use std::io::{BufRead, BufReader, Write};
use std::net::{TcpListener, TcpStream};
use std::sync::Arc;

use super::codes::CODE_CONNECTION_INFO;
use super::parser::parse_command;
use super::responder::Responder;
use crate::port::{Query, Transport};

/// Accepts controller connections and serves the protocol. The deck is shared
/// across connections via `Arc`.
pub struct Server<D> {
    deck: Arc<D>,
}

impl<D> Server<D>
where
    D: Transport + Query + Send + Sync + 'static,
{
    /// Wires a server to a shared deck implementing both inbound ports.
    pub fn new(deck: Arc<D>) -> Self {
        Server { deck }
    }

    /// Accepts connections on `listener` until it errors (e.g. is closed),
    /// handling each on its own thread.
    pub fn serve(&self, listener: TcpListener) -> std::io::Result<()> {
        for stream in listener.incoming() {
            let stream = stream?;
            let deck = self.deck.clone();
            std::thread::spawn(move || handle(stream, deck));
        }
        Ok(())
    }
}

fn handle<D: Transport + Query>(stream: TcpStream, deck: Arc<D>) {
    let responder = Responder::new(deck);
    let mut writer = match stream.try_clone() {
        Ok(w) => w,
        Err(_) => return,
    };
    // Greeting banner.
    if writer
        .write_all(
            format!(
                "{CODE_CONNECTION_INFO} connection info:\r\nprotocol version: 1.11\r\nmodel: HyperDeck Studio Mini\r\n\r\n"
            )
            .as_bytes(),
        )
        .is_err()
    {
        return;
    }

    let mut reader = BufReader::new(stream);
    loop {
        let block = match read_command_block(&mut reader) {
            Some(b) => b,
            None => return,
        };
        if block.trim().is_empty() {
            continue;
        }
        let response = match parse_command(&block) {
            Ok(cmd) => {
                let out = responder.handle(&cmd);
                if writer.write_all(out.as_bytes()).is_err() {
                    return;
                }
                if cmd.name == "quit" {
                    return;
                }
                continue;
            }
            Err(_) => format!("{} syntax error\r\n", super::codes::CODE_SYNTAX_ERROR),
        };
        if writer.write_all(response.as_bytes()).is_err() {
            return;
        }
    }
}

/// Reads one command: a single line, or — when the first line ends with `:` —
/// lines up to and including a terminating blank line. Returns `None` on EOF.
fn read_command_block<R: BufRead>(reader: &mut R) -> Option<String> {
    let mut first = String::new();
    if reader.read_line(&mut first).ok()? == 0 {
        return None;
    }
    if !first.trim_end_matches(['\r', '\n']).ends_with(':') {
        return Some(first);
    }
    let mut block = first;
    loop {
        let mut line = String::new();
        if reader.read_line(&mut line).ok()? == 0 {
            return None;
        }
        let blank = line.trim().is_empty();
        block.push_str(&line);
        if blank {
            return Some(block);
        }
    }
}

#[cfg(test)]
mod tests {
    use std::io::{BufRead, BufReader, Write};
    use std::net::{TcpListener, TcpStream};
    use std::time::Duration;

    use super::*;
    use crate::app::{Session, VirtualDeck};
    use crate::domain::{Chord, KeyName, Keymap, Profile, Window};
    use crate::testsupport::MockInjector;

    #[test]
    fn end_to_end_play() {
        let injector = Arc::new(MockInjector::new());
        let session = Arc::new(Session::new());
        let mut keymap = Keymap::new();
        keymap.insert(
            KeyName::Play,
            Chord {
                mods: vec![],
                key: "space".into(),
            },
        );
        session.lock(
            Profile {
                id: "vlc".into(),
                injection: crate::domain::InjectionMode::Background,
                keymap,
                ..Default::default()
            },
            Window {
                process: "vlc.exe".into(),
                ..Default::default()
            },
            None,
            None,
        );
        let deck = Arc::new(VirtualDeck::new(session, injector.clone()));

        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let addr = listener.local_addr().unwrap();
        std::thread::spawn(move || {
            let _ = Server::new(deck).serve(listener);
        });

        let mut conn = TcpStream::connect(addr).unwrap();
        conn.set_read_timeout(Some(Duration::from_secs(2))).unwrap();
        let mut reader = BufReader::new(conn.try_clone().unwrap());

        // Greeting banner: "500 connection info:" then a blank line.
        let mut banner = String::new();
        reader.read_line(&mut banner).unwrap();
        assert!(
            banner.starts_with("500 connection info:"),
            "banner: {banner:?}"
        );
        drain_blank(&mut reader);

        conn.write_all(b"play\r\n").unwrap();
        let mut resp = String::new();
        reader.read_line(&mut resp).unwrap();
        assert!(resp.starts_with("200 ok"), "play resp: {resp:?}");

        // Give the handler a moment, then confirm the keystroke was recorded.
        std::thread::sleep(Duration::from_millis(50));
        assert_eq!(injector.sent_keys(), vec!["space"]);
    }

    fn drain_blank<R: BufRead>(reader: &mut R) {
        loop {
            let mut line = String::new();
            if reader.read_line(&mut line).unwrap_or(0) == 0 || line.trim().is_empty() {
                return;
            }
        }
    }
}
