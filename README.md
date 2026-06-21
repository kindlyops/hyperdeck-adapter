# hyperdeck-adapter

Emulate a Blackmagic HyperDeck so a HyperDeck controller (Bitfocus Companion, an
ATEM switcher, a hardware remote) can drive a **local media player that has no
network control** — by translating HyperDeck transport commands into keystrokes
(or out-of-band control where keystrokes don't work).

The adapter listens on TCP **9993** and speaks the HyperDeck Ethernet Protocol. It
locks onto one running player and turns `play` / `stop` / `goto` into the keys that
player expects:

```
HyperDeck controller ──TCP 9993──▶ hyperdeck-adapter ──keystrokes/API──▶ player (Example Player / VLC / Mitti)
```

It runs as a system-tray / menu-bar app with built-in **self-update**, and a tray
icon shows whether it is locked onto a player. Adding a new player is configuration,
not code.

**Website:** <https://kindlyops.github.io/hyperdeck-adapter/> (source in [`site/`](site/)) — psst, type `td`.

## Architecture

A [Tauri](https://v2.tauri.app) v2 desktop app over a pure-Rust hexagonal core:

- **`crates/hyperdeck-core`** — OS-independent domain, ports, application services
  (Session / VirtualDeck / LockManager / Reconciler), the HyperDeck protocol
  parser + responder + TCP server, profile/selection config, clip sources, and
  state probes. Fully unit-tested on Linux/macOS/Windows.
- **`crates/hyperdeck-os`** — the OS- and player-specific driven adapters behind the
  core's traits: keystroke injection + window enumeration (macOS CoreGraphics/AppKit,
  Windows WinAPI), Windows UI Automation, and the VLC HTTP controller.
- **`src-tauri`** — the tray app: wires the core to the OS adapters, owns the tray
  menu, and provides self-update via `tauri-plugin-updater`.

## Status

- **macOS / Windows** — injection, window enumeration, and (Windows) UI Automation
  are implemented in Rust and compile on each platform; end-to-end behavior is
  validated on hardware (see the testing handoff issues).
- The OS-independent core, protocol, config, clip sources, and the VLC HTTP
  controller are unit-tested in CI on all three platforms.

## Install

Prebuilt installers are attached to each [GitHub release](https://github.com/kindlyops/hyperdeck-adapter/releases):

- **macOS** — `…_universal.dmg` (arm64 + Intel). Drag **HyperDeck Adapter.app** to
  Applications; it runs as a menu-bar app.
- **Windows** — NSIS `-setup.exe` (or `.msi`). Adds a Start-menu shortcut.

Once installed, the app checks for and installs updates itself (tray → **Check for
Updates…**).

> Installers are unsigned until code-signing credentials are configured, so macOS
> Gatekeeper and Windows SmartScreen may warn on first launch.

## Development

Requirements:

- [Rust](https://rustup.rs) (stable).
- The [Tauri v2 prerequisites](https://v2.tauri.app/start/prerequisites/) for your
  OS, plus the Tauri CLI: `cargo install tauri-cli`.
- [`just`](https://github.com/casey/just) for the task runner.
- Python 3 for the demo client (`scripts/hyperdeck-demo.py`).

```sh
just test     # fmt check + clippy + tests for the library crates
just run      # run the tray app in dev mode (cargo tauri dev)
just serve    # run headless (no tray) in the foreground
just demo     # drive the locked player with a scripted HyperDeck sequence
just trust    # (macOS) prompt for / verify the Accessibility permission
```

On macOS the adapter needs the **Accessibility** (input) permission to deliver
keystrokes; it prompts on first run (enable it under **System Settings → Privacy &
Security → Accessibility**).

## Running it for real

Run the app (tray or `--headless`) and point a HyperDeck controller at the machine's
IP on port **9993** in Bitfocus Companion (or ATEM Software Control, etc.). Transport
buttons drive the locked-on player. The adapter binds `0.0.0.0:9993`.

## Configuration

Players are defined in a YAML profile file; see
[`examples/profiles.yaml`](examples/profiles.yaml) for working **Example Player**,
**VLC**, and **Mitti** profiles. The default file is seeded on first run at:

- macOS: `~/Library/Application Support/hyperdeck-adapter/profiles.yaml`
- Windows: `%AppData%\hyperdeck-adapter\profiles.yaml`

A minimal example:

```yaml
profiles:
  - id: vlc
    match: { process: ["VLC", "vlc", "vlc.exe"], title_regex: "VLC media player" }
    injection: background          # background = no focus steal; or "focus"
    keymap: { play: "space", stop: "cmd+.", next: "cmd+right", prev: "cmd+left" }
    play_toggle: true              # Space toggles play/pause; Stop uses the discrete key
    clip_source: { type: positional, count: 50 }
    state: { type: title_regex, playing: ".+ - VLC media player" }
```

Profiles also support out-of-band control: `control: api` (VLC's HTTP interface) and
`control: uia` (Windows UI Automation for UWP apps). The pinned-profile selection is
persisted to `selection.json` alongside the config.

## Releasing (maintainers)

Releases are built by [`.github/workflows/release.yml`](.github/workflows/release.yml)
via [`tauri-action`](https://github.com/tauri-apps/tauri-action) on a macOS + Windows
matrix. Bump `version` in `src-tauri/tauri.conf.json`, then push a `vX.Y.Z` tag (or
run **Actions → Release → Run workflow**). It creates a **draft** release with the
DMG / NSIS / MSI bundles and, once the updater signing key is set, `latest.json`.

**Signing keys and the updater key are maintainer-owned and must be added manually**
— see the "Maintainer setup: signing keys & secrets" issue. Until then, releases are
unsigned and the in-app updater won't verify (the release workflow disables updater
artifacts automatically when no signing key is present).
