# HyperDeck Adapter — Design

**Date:** 2026-06-11
**Status:** Approved (brainstorming complete; ready for implementation planning)

## Summary

A single cross-platform Go binary, `hyperdeck-adapter`, that runs as a tray /
menu-bar application and emulates **one Blackmagic HyperDeck deck**. A HyperDeck
controller (Bitfocus Companion, an ATEM switcher, or a hardware HyperDeck remote)
connects over TCP port 9993 and issues standard HyperDeck Ethernet Protocol
commands. The adapter translates those commands into **keystrokes** delivered to a
single locked-on local player application that has no network control of its own
(VLC, Example Player, or Mitti). The tray icon indicates whether the adapter has
**locked onto** a running player or not.

The adapter does not read the player's internal state directly. It maintains a
**modeled (open-loop)** transport state and corrects it with **best-effort
detection** (e.g. window-title parsing) where a profile defines how.

## Goals

- Let a standard HyperDeck controller drive a network-less local player via keys.
- Cross-platform: real injection backends for **Windows** and **macOS**; a **mock**
  backend so nearly all logic is developed and tested on macOS/Linux/CI.
- Adding a new player app is **configuration, not code** (YAML profiles).
- Clear status feedback: tray icon shows locked / not-locked.

## Non-goals (MVP)

- Jog / shuttle / variable-speed playback. (Protocol may ack but no-op or map to
  nearest discrete action; not a first-class feature.)
- Recording to disk on the host (the `record` command maps to a player key only if
  the profile defines one; otherwise it is acked as a no-op).
- mDNS/Bonjour auto-advertisement. Controllers add the deck by IP for MVP.
- Multiple simultaneous virtual decks. Exactly one app, one deck.
- True closed-loop state sync. State is modeled + best-effort, never authoritative.
- **Asynchronous `5xx` push notifications.** The `notify:` subscription command is
  acknowledged, but the deck does not yet emit unsolicited `5xx` transport/slot/
  configuration-change messages — controllers must poll `transport info` /
  `slot info`. Async push is the **top follow-up** to implement before live ATEM /
  hardware-remote fidelity testing, since those controllers rely on it to keep
  their UI current. (Deferred consciously; the responder no longer claims to emit
  it.)

## Requirements (from brainstorming)

| Decision | Choice |
|---|---|
| Structure | Hexagonal (ports & adapters): pure domain core, swappable driving/driven adapters |
| Platform scope | Cross-platform Windows + macOS; mock backend for Linux/CI |
| Target apps | VLC, Example Player, Mitti — each a config-driven profile |
| Controllers | Bitfocus Companion / Stream Deck **and** ATEM / hardware switcher |
| Command scope | Basic transport (play/stop/record), clip navigation (goto/next/prev), clip-list reporting |
| Deck count | One running app → one virtual deck on port 9993; profile auto-selected |
| Injection model | Configurable per app: `focus` (foreground + real keys) or `background` (PostMessage-style) |
| Clip model | Read from a playlist file where possible; positional slots otherwise |
| State feedback | Modeled (open-loop) primary + best-effort detection, reconciled |
| Homing | Explicit only — tray "Re-home" or a designated command |
| Config format | YAML |

## Architecture

The design follows **hexagonal architecture (ports & adapters)**. A pure domain
core — the **virtual deck** — is surrounded by adapters that translate between the
core and the outside world (the network, the OS, the filesystem, the tray). All
dependencies point **inward**: adapters depend on ports defined by the core; the
core depends on nothing external and contains no platform code or build tags, so
it compiles and is tested on any OS.

### The hexagon (domain core)

Pure Go, no I/O, no build tags. Holds the domain model and logic:

- **Value objects / entities:** `TransportState` (stopped/playing/paused), `Clip`,
  `ClipList`, `ClipIndex`, `Chord`, `Profile`, `Window`, `LockState`, `DeviceInfo`.
- **`VirtualDeck`** (application service): the open-loop transport state model,
  **toggle resolution**, **goto-delta** computation, and command → chord
  translation against the active profile. Implements the driving ports. This
  absorbs the old `deck` + `translator` split — it is all pure domain logic.
- **`LockManager`** (application service): matches enumerated windows against
  profile rules to pick the active profile and lock state; notifies the status
  presenter. (Window *matching* is domain logic; window *enumeration* is a port.)
- **`Reconciler`** (application service): on each `Clock` tick, refreshes the clip
  list and runs best-effort state detection to correct the modeled state.

### Ports

**Driving (inbound) ports** — the deck's API, defined by the core, called by
driving adapters:

- `Transport`: `Play`, `Stop`, `Record`, `Goto(id)`, `Next`, `Prev`, `Rehome`.
- `Query`: `TransportInfo`, `Clips`, `SlotInfo`, `DeviceInfo`.

**Driven (outbound) ports** — what the core needs from the world, defined by the
core, implemented by driven adapters:

- `KeyInjector`: `Focus(Window)`, `SendKeys(Window, []Chord)`.
- `WindowEnumerator`: `OpenWindows() []Window`.
- `ClipSource`: `List() []Clip`.
- `StateProbe`: `Detect() (TransportState, bool)`.
- `StatusPresenter`: `Present(Status)` — drives the tray icon/state.
- `ProfileStore`: `Load() []Profile`.
- `Clock`: ticker/now, injected so the reconciler is deterministic in tests.

### Adapters

**Driving (primary) adapters:**

- **`hyperdeck`** — the TCP protocol server. Translates the HyperDeck wire protocol
  to `Transport`/`Query` calls and formats responses. Knows nothing of the OS or
  injectors. (A second driving adapter — an in-process test client — reuses the
  same inbound ports for end-to-end tests.)
- **`tray` (driving role)** — menu actions (Re-home, Quit) call inbound ports.

**Driven (secondary) adapters:**

- **`injector`** — implements `KeyInjector` + `WindowEnumerator`. Build-tagged:
  `windows`, `darwin`, `mock`.
- **`clipsource`** — implements `ClipSource`: `playlist_file`, `positional`, `mitti`.
- **`stateprobe`** — implements `StateProbe`: `title_regex`, `none`.
- **`tray` (driven role)** — implements `StatusPresenter`.
- **`config`** — implements `ProfileStore` (YAML).
- **`clock`** — implements `Clock` over the system clock.

**Composition root** — `cmd/hyperdeck-adapter/main.go` selects the OS injector via
build tags, wires adapters to ports, and starts the application services and
driving adapters. It is the only place that knows every concrete type.

### Package layout

```
internal/
  core/
    domain/        entities & value objects (pure)
    port/          inbound.go (Transport, Query) + outbound.go (KeyInjector, …)
    app/           virtualdeck.go, lockmanager.go, reconciler.go
  adapter/
    driving/
      hyperdeck/   TCP protocol server
    driven/
      injector/    windows.go //go:build windows; darwin.go; mock.go
      clipsource/  playlist.go, positional.go, mitti.go
      stateprobe/  titleregex.go, none.go
      config/      yaml profile store
      tray/        status presenter + menu (bidirectional)
      clock/       system clock
cmd/hyperdeck-adapter/  main.go (composition root)
```

### Data flow

```
[driving] HyperDeck controller --TCP 9993--> hyperdeck adapter
                                                 | Transport / Query (inbound port)
                                                 v
[core]                                       VirtualDeck --+--> KeyInjector --> [driven] injector --> player app
                                                 ^          +--> ClipSource --> [driven] clipsource
                                                 |          +--> StateProbe --> [driven] stateprobe
                                          Reconciler (Clock tick)
[core]   LockManager <-- WindowEnumerator [driven] injector
              | StatusPresenter (outbound)
              v
[bridge]  tray  (status out, menu in)
```

### Judicious limits (where we deliberately stop)

Hexagonal structure is applied where it earns its keep and no further:

- **Coarse ports, not one-per-method.** `Transport` and `Query` group related
  operations; `KeyInjector` keeps focus + send together.
- **Domain types are the port contracts.** No separate DTO/mapping layer between
  the protocol adapter and the inbound ports — the wire parser produces domain
  values directly. Revisit only if a real impedance mismatch appears.
- **`Clock` is a port solely for test determinism**, not speculative flexibility.
- **`tray` is one bidirectional adapter**, not split into two packages for purity.
- **Window matching stays in the core**; only enumeration crosses a port. Rule
  logic does not leak into adapters.

## Driven adapter: the injector (OS seam)

`KeyInjector` + `WindowEnumerator` are the single OS seam. The interfaces live in
`core/port`; the implementations are build-tagged adapters:

```go
// core/port — defined by the core, implemented by driven adapters.
type Window struct { Handle uintptr; Title string; Process string }

type KeyInjector interface {
    Focus(w Window) error                  // bring to foreground (focus mode)
    SendKeys(w Window, keys []Chord) error // chords like ctrl+right, cmd+esc
}

type WindowEnumerator interface {
    OpenWindows() ([]Window, error)        // LockManager matches these to profiles
}
```

- **`windows`** adapter: `SendInput` for focus mode; `PostMessage`/`SendMessage`
  (`WM_KEYDOWN`/`WM_KEYUP`) for background mode; `EnumWindows` +
  `GetWindowThreadProcessId` for enumeration; `SetForegroundWindow` for focus.
  Built on `golang.org/x/sys/windows` syscalls; no cgo.
- **`darwin`** adapter: `CGEventCreateKeyboardEvent` + `CGEventPost` for synthetic
  keys; Accessibility APIs / AppleScript `activate` for focusing; window
  enumeration via CoreGraphics / Accessibility. Requires cgo and the Accessibility
  permission.
- **`mock`** adapter: records all calls; scriptable to simulate find/focus
  success/failure and a fake foreground window. Used by all core tests and CI.

Injection mode (`focus` vs `background`) is chosen **per profile**. Default to
`focus` where behavior is uncertain — background `PostMessage` is silently ignored
by DirectInput/fullscreen apps.

## Profiles (YAML)

Adding an app is config-only. Each profile declares its match rules, keymap,
injection mode, clip source, state-detection rules, and homing sequence.

```yaml
profiles:
  - id: vlc
    match: { process: ["vlc.exe", "VLC"], title_regex: "VLC media player" }
    injection: background
    keymap:
      play: "space"
      stop: "s"
      next: "n"
      prev: "p"
    toggle_keys: []          # keys that toggle rather than set a state
    clip_source:
      type: playlist_file
      path: "${VLC_PLAYLIST}" # explicit path or watched folder
    state:
      type: title_regex
      playing: ".+ - VLC"     # best-effort current/playing detection
    homing: ["s"]             # run on explicit Re-home only

  - id: example_player
    match: { process: ["Example Player"] }
    injection: focus
    keymap:
      play: "space"
      next: "ctrl+right"
      prev: "ctrl+left"
    toggle_keys: ["space"]    # Space toggles → modeled state decides emit
    clip_source: { type: positional, count: 50 }
    state: { type: none }
    homing: []

  - id: mitti
    match: { process: ["Mitti"] }
    injection: focus           # macOS CGEvent
    keymap:
      play: "enter"
      pause: "space"
      stop: "cmd+esc"          # Panic
      next: "cmd+down"
      prev: "cmd+up"
    toggle_keys: []
    clip_source: { type: mitti }
    state: { type: none }
    homing: []
```

Profile selection is automatic: `LockManager` matches enumerated windows (via the
`WindowEnumerator` port) and binds the first profile whose `match` succeeds. Config
also has top-level settings (TCP bind address/port, poll interval, log level,
log/config paths). The `ProfileStore` adapter loads and validates this YAML.

## Key behaviors

- **Toggle semantics.** HyperDeck exposes discrete `play` and `stop`. Example Player
  has only `Space` (toggle) and no stop. For a `toggle_key`, `VirtualDeck`
  consults its modeled state and emits the key **only when the requested state
  differs from the assumed state** (e.g. `play` while already modeled-playing
  emits nothing). This is why modeled state is load-bearing.
- **Clip list.** VLC → parse the playlist for real clip names/count. Mitti →
  best-effort (proprietary format; may yield generic names). Example Player →
  `positional`: a fixed count of generic slots; `goto N` / `next` / `prev` become
  repeated navigation keypresses from the modeled current index. The controller UI
  shows whatever the source provides.
- **`goto: clip id: N`.** Compute delta from modeled current index and emit that
  many `next`/`prev` chords; update modeled index. (HyperDeck clip ids are 1-based.)
- **Homing.** Explicit only. Tray "Re-home" or a designated command runs the
  profile's `homing` sequence to reach a known state, then resets modeled state.
- **Lock-on.** `LockManager` polls on each `Clock` tick. On match → lock; the
  `StatusPresenter` shows connected; the deck reports a present slot and the clip
  list. On loss → presenter shows disconnected; the deck reports no remote/slot so
  controllers reflect the loss.
- **Discovery.** IP-only for MVP. mDNS/Bonjour advertisement is a noted later
  enhancement.

## Protocol surface (server side)

Implements the HyperDeck Ethernet Protocol as the **device**. Framing: text over
TCP 9993; simple commands ack with `200 ok`; commands-with-response reply with a
`2xx` code and a colon-terminated multi-line body ending in a blank line; errors
use `1xx`; asynchronous notifications use `5xx`.

MVP command set (exact grammar to be mined from the HyperDeck product manual during
planning):

- **Connection/identity:** banner on connect, `ping`, `device info`, `commands`,
  `quit`. Identity responses must be faithful enough for ATEM / hardware remotes to
  accept the deck.
- **Transport:** `play` (incl. `play:` with params, treated as best-effort),
  `stop`, `record`, `transport info`.
- **Clips:** `clips get`, `goto: clip id: N`, `goto: clip id: +/-N`.
- **Slots:** `slot info` (report a single present slot when locked, empty when not).
- **Notifications:** `notify:` (slot / transport / configuration) and emission of
  `5xx` async responses on state changes.

Unsupported commands return a clean protocol error rather than dropping the
connection.

## Error handling

- **Config invalid** → fail fast at startup with a specific, actionable message
  (which profile, which field). No silent defaults for required fields.
- **No player locked** → protocol still serves; transport/slot responses indicate
  the disconnected state; tray shows not-locked.
- **Injection failure** (window vanished, focus denied, Accessibility permission
  missing on macOS) → log with context and surface in the tray; do not crash.
- **TCP client disconnect** → clean up per-connection state; keep serving new
  connections.
- Never swallow errors silently; every error includes operation + input + a
  suggested fix.

## Testing strategy

Hexagonal structure is what makes this testable: the entire core (`domain`, `app`)
has zero platform dependencies and is driven through its ports, so it tests on
macOS/Linux/CI against in-memory adapters.

- **Core (`app`)** — table-driven tests with the **mock injector** and fake clip
  source / state probe / clock, asserting exact emitted chords. Covers toggle edge
  cases (play-when-playing emits nothing; goto delta direction), `LockManager`
  match/lock transitions, and `Reconciler` correction on a stepped clock.
- **`hyperdeck` driving adapter** — unit + **property tests** for the
  parser/formatter (multi-line bodies, response codes, framing, partial reads),
  with a fake inbound port.
- **`clipsource`** — parser tests with sample `.m3u` / `.xspf` fixtures and the
  positional strategy.
- **`config` (ProfileStore)** — load/validate tests, including rejection of
  malformed configs.
- **Integration** — an in-process TCP test client (a second driving adapter,
  mirroring the SDK reference client's framing) drives the wired core end-to-end
  with the mock injector and asserts **both** protocol responses and recorded
  keystrokes.
- **Windows/macOS driven adapters** — thin manual smoke tests of the real injector
  + tray only; early test against ATEM Software Control for protocol fidelity.

## Dependencies (to confirm during planning)

- Tray UI: a maintained cross-platform systray library.
- Windows syscalls: `golang.org/x/sys/windows` (no cgo).
- macOS: cgo bridge to CoreGraphics / Accessibility (or a vetted wrapper).
- YAML: a maintained YAML library.
- Property testing: a Go property-testing library.

Each dependency is justified at planning time; prefer the standard library where
practical.

## Risks

- **ATEM / hardware-remote fidelity.** These poll `device info`, slot status, etc.
  and may reject a deck that responds incorrectly. Mitigation: implement the
  identity/status command set faithfully and test against a real controller early.
- **Background injection limits.** `PostMessage` is ignored by some apps
  (DirectInput/fullscreen). Mitigation: per-profile injection mode, default
  `focus`.
- **Mitti.** Already emulates HyperDeck natively and uses a proprietary playlist
  format; clip-name reporting is best-effort. Discrete keys work fine.
- **macOS permissions.** Synthetic input and window introspection require the
  Accessibility permission; the app must detect its absence and guide the user.

## Open items for the implementation plan

- Extract the exact HyperDeck command grammar and required identity/status fields
  from the HyperDeck product manual.
- Confirm VLC playlist path/format handling and whether a watched folder is needed.
- Confirm Mitti's best-effort clip-source approach (file parse vs. give up to
  positional).
- Choose and pin the systray, YAML, and property-testing libraries.
