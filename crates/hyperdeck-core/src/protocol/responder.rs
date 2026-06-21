//! Command responder (port of `internal/adapter/driving/hyperdeck/responder.go`).

use std::fmt::Write as _;

use super::codes::*;
use super::parser::Command;
use crate::error::DeckResult;
use crate::port::{Query, Transport};

/// Turns parsed commands into port calls and formatted responses.
///
/// The deck type `D` implements both inbound ports, mirroring Go's
/// `NewResponder(deck, deck)`.
pub struct Responder<D> {
    deck: D,
}

impl<D: Transport + Query> Responder<D> {
    /// Wires a responder to a deck implementing both inbound ports.
    pub fn new(deck: D) -> Self {
        Self { deck }
    }

    /// Executes one command and returns the full response text.
    pub fn handle(&self, cmd: &Command) -> String {
        match cmd.name.as_str() {
            "ping" => ack(),
            "play" => self.ack_err(self.deck.play()),
            "stop" => self.ack_err(self.deck.stop()),
            "record" => self.ack_err(self.deck.record()),
            "goto" => self.handle_goto(cmd),
            "transport info" => self.transport_info(),
            "clips get" => self.clips(),
            "slot info" => self.slot_info(),
            "device info" => self.device_info(),
            // Subscriptions are acked; async 5xx push notifications are a
            // deferred MVP item (see the design spec).
            "notify" | "remote" | "configuration" => ack(),
            "quit" => ack(),
            _ => syntax_error(),
        }
    }

    fn handle_goto(&self, cmd: &Command) -> String {
        let Some(id_str) = cmd.params.get("clip id") else {
            return syntax_error();
        };
        // i32::from_str accepts a leading '+' or '-' (like Go's strconv.Atoi).
        let Ok(n) = id_str.parse::<i32>() else {
            return syntax_error();
        };
        // A signed value is a relative offset from the current clip; an unsigned
        // value is an absolute 1-based id.
        let target = if id_str.starts_with('+') || id_str.starts_with('-') {
            self.deck.transport_info().clip_id + n
        } else {
            n
        };
        self.ack_err(self.deck.goto(target))
    }

    fn transport_info(&self) -> String {
        let ti = self.deck.transport_info();
        let mut b = String::new();
        let _ = write!(b, "{CODE_TRANSPORT_INFO} transport info:\r\n");
        let _ = write!(b, "status: {}\r\n", ti.status);
        let _ = write!(b, "speed: {}\r\n", ti.speed);
        let _ = write!(b, "clip id: {}\r\n", ti.clip_id);
        let _ = write!(b, "slot id: {}\r\n", ti.slot_id);
        b.push_str("\r\n");
        b
    }

    fn clips(&self) -> String {
        let clips = self.deck.clips();
        let mut b = String::new();
        let _ = write!(b, "{CODE_CLIPS_INFO} clips info:\r\n");
        let _ = write!(b, "clip count: {}\r\n", clips.len());
        for c in &clips {
            let _ = write!(b, "{}: {} {} {}\r\n", c.id, c.name, c.timecode, c.duration);
        }
        b.push_str("\r\n");
        b
    }

    fn slot_info(&self) -> String {
        let si = self.deck.slot_info();
        let status = if si.present { "mounted" } else { "empty" };
        let mut b = String::new();
        let _ = write!(b, "{CODE_SLOT_INFO} slot info:\r\n");
        let _ = write!(b, "slot id: {}\r\n", si.slot_id);
        let _ = write!(b, "status: {status}\r\n");
        b.push_str("\r\n");
        b
    }

    fn device_info(&self) -> String {
        let di = self.deck.device_info();
        let mut b = String::new();
        let _ = write!(b, "{CODE_DEVICE_INFO} device info:\r\n");
        let _ = write!(b, "protocol version: {}\r\n", di.protocol_version);
        let _ = write!(b, "model: {}\r\n", di.model);
        let _ = write!(b, "unique id: {}\r\n", di.unique_id);
        b.push_str("\r\n");
        b
    }

    fn ack_err(&self, result: DeckResult<()>) -> String {
        match result {
            Ok(()) => ack(),
            Err(_) => format!("{CODE_INVALID_STATE} invalid state\r\n"),
        }
    }
}

fn ack() -> String {
    format!("{CODE_OK} ok\r\n")
}

fn syntax_error() -> String {
    format!("{CODE_SYNTAX_ERROR} syntax error\r\n")
}

#[cfg(test)]
mod tests {
    use std::cell::Cell;

    use super::*;
    use crate::domain::{Clip, ClipList, DeviceInfo, SlotInfo, TransportInfo};

    /// Minimal deck mirroring the parts of `VirtualDeck` the responder relies on:
    /// it tracks play state and the current clip so relative `goto` and
    /// `transport info` behave like the Go integration tests.
    struct MockDeck {
        playing: Cell<bool>,
        current_clip: Cell<i32>,
        clips: ClipList,
    }

    impl MockDeck {
        fn new(current_clip: i32, clips: ClipList) -> Self {
            Self {
                playing: Cell::new(false),
                current_clip: Cell::new(current_clip),
                clips,
            }
        }
    }

    impl Transport for MockDeck {
        fn play(&self) -> DeckResult<()> {
            self.playing.set(true);
            Ok(())
        }
        fn stop(&self) -> DeckResult<()> {
            self.playing.set(false);
            Ok(())
        }
        fn record(&self) -> DeckResult<()> {
            Ok(())
        }
        fn goto(&self, clip_id: i32) -> DeckResult<()> {
            self.current_clip.set(clip_id);
            Ok(())
        }
        fn next(&self) -> DeckResult<()> {
            Ok(())
        }
        fn prev(&self) -> DeckResult<()> {
            Ok(())
        }
        fn rehome(&self) -> DeckResult<()> {
            Ok(())
        }
    }

    impl Query for MockDeck {
        fn transport_info(&self) -> TransportInfo {
            TransportInfo {
                status: if self.playing.get() {
                    "play"
                } else {
                    "stopped"
                }
                .to_string(),
                speed: if self.playing.get() { 100 } else { 0 },
                clip_id: self.current_clip.get(),
                slot_id: 1,
            }
        }
        fn clips(&self) -> ClipList {
            self.clips.clone()
        }
        fn slot_info(&self) -> SlotInfo {
            SlotInfo {
                present: true,
                slot_id: 1,
            }
        }
        fn device_info(&self) -> DeviceInfo {
            DeviceInfo {
                protocol_version: "1.11".into(),
                model: "HyperDeck Adapter".into(),
                unique_id: "test".into(),
            }
        }
    }

    fn deck_with_intro() -> MockDeck {
        MockDeck::new(
            1,
            vec![Clip {
                id: 1,
                name: "Intro".into(),
                timecode: "00:00:00:00".into(),
                duration: "00:00:10:00".into(),
            }],
        )
    }

    fn cmd(name: &str) -> Command {
        Command {
            name: name.into(),
            params: Default::default(),
        }
    }

    fn goto(id: &str) -> Command {
        let mut c = Command {
            name: "goto".into(),
            params: Default::default(),
        };
        c.params.insert("clip id".into(), id.into());
        c
    }

    #[test]
    fn play_acks() {
        let r = Responder::new(deck_with_intro());
        assert!(r.handle(&cmd("play")).starts_with("200 ok"));
    }

    #[test]
    fn transport_info_reflects_play() {
        let r = Responder::new(deck_with_intro());
        let _ = r.handle(&cmd("play"));
        let out = r.handle(&cmd("transport info"));
        assert!(out.starts_with("208 transport info:"), "head: {out:?}");
        assert!(out.contains("status: play"), "body: {out:?}");
        assert!(
            out.ends_with("\r\n\r\n"),
            "must end with blank line: {out:?}"
        );
    }

    #[test]
    fn clips_lists_entries() {
        let r = Responder::new(deck_with_intro());
        let out = r.handle(&cmd("clips get"));
        assert!(out.starts_with("205 clips info:"));
        assert!(out.contains("Intro"));
    }

    #[test]
    fn unknown_command_is_syntax_error() {
        let r = Responder::new(deck_with_intro());
        assert!(r.handle(&cmd("frobnicate")).starts_with("100 "));
    }

    #[test]
    fn goto_absolute_acks() {
        let r = Responder::new(deck_with_intro());
        assert!(r.handle(&goto("1")).starts_with("200 ok"));
    }

    #[test]
    fn goto_relative_uses_current_clip() {
        // 5 clips, current clip 3, so +2 -> 5 and then -2 -> 3.
        let deck = MockDeck::new(
            3,
            (1..=5)
                .map(|id| Clip {
                    id,
                    ..Default::default()
                })
                .collect(),
        );
        let r = Responder::new(deck);

        assert!(r.handle(&goto("+2")).starts_with("200 ok"));
        assert!(r.handle(&cmd("transport info")).contains("clip id: 5"));

        assert!(r.handle(&goto("-2")).starts_with("200 ok"));
        assert!(r.handle(&cmd("transport info")).contains("clip id: 3"));
    }
}
