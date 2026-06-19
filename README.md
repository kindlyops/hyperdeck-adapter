# hyperdeck-adapter

Emulate a Blackmagic HyperDeck so a HyperDeck controller (Bitfocus Companion, an
ATEM switcher, a hardware remote) can drive a **local media player that has no
network control** — by translating HyperDeck transport commands into keystrokes.

The adapter listens on TCP **9993** and speaks the HyperDeck Ethernet Protocol. It
locks onto one running player, and turns `play` / `stop` / `goto` into the keys that
player expects:

```
HyperDeck controller ──TCP 9993──▶ hyperdeck-adapter ──keystrokes──▶ player (Example Player / VLC / Mitti)
```

A system-tray / menu-bar icon shows whether it is locked onto a player. Adding a new
player is configuration, not code.

**Website:** <https://kindlyops.github.io/hyperdeck-adapter/> (source in [`site/`](site/)) — psst, type `td`.

## Status

- **macOS** — injector implemented and verified live against **Example Player** and
  **VLC** (foreground and background keystroke injection).
- **Windows** — injector not yet implemented; there is a handoff spec at
  [`docs/superpowers/specs/2026-06-12-windows-injector-handoff.md`](docs/superpowers/specs/2026-06-12-windows-injector-handoff.md).
  The transport logic is platform-agnostic and ready; only the OS keystroke backend
  is pending.
- **Linux / CI** — the whole core, protocol, and adapters build and test with a
  no-op injector, so almost everything is developed and tested off-Windows.

Design docs live in [`docs/superpowers/specs/`](docs/superpowers/specs/).

## Requirements

- [Go](https://go.dev) 1.23+ (developed on 1.26).
- [`just`](https://github.com/casey/just) for the task runner.
- macOS to drive a real player today (the injector uses CoreGraphics / AppKit).
- Python 3 for the demo client (`scripts/hyperdeck-demo.py`).

On macOS the adapter needs the **Accessibility** (input) permission to deliver
keystrokes — see [Permissions](#permissions-macos).

## Install

Prebuilt installers are attached to each [GitHub release](https://github.com/kindlyops/hyperdeck-adapter/releases):

- **macOS** — `…_macos_universal.dmg` (universal arm64 + Intel). Open it and drag
  **HyperDeck Adapter.app** to Applications. It runs as a menu-bar app.
- **Windows** — `…_windows_amd64-setup.exe` (NSIS installer). Adds a Start-menu
  shortcut.
- **Linux** — `…_linux_amd64.tar.gz` or the `.deb` (`sudo apt install ./hyperdeck-adapter_*_amd64.deb`).
  Headless only — the tray/injector are no-ops on Linux.

> Installers are unsigned until code-signing credentials are configured, so macOS
> Gatekeeper and Windows SmartScreen may warn on first launch. Verify downloads
> against `checksums.txt` on the release.

## Quick demo

1. Open a player and start a playlist — e.g. **Example Player** or **VLC** with a few
   items queued.
2. From the repo root:

   ```sh
   just demo
   ```

   This builds the binaries, starts the adapter (which locks onto the running
   player), sends a scripted HyperDeck sequence (`play` → idempotent `play` → `stop`
   → next / next / previous, with `transport info` readouts), then stops the adapter.

3. **Watch the player respond.** If macOS prompts for Accessibility, enable
   `bin/hyperdeck-adapter` in **System Settings → Privacy & Security → Accessibility**
   and re-run `just demo`.

## Running it for real

Run the adapter continuously and point a HyperDeck controller at the machine:

```sh
# headless (no tray) on all interfaces, port 9993
./bin/hyperdeck-adapter -no-tray -config examples/profiles.yaml -bind 0.0.0.0:9993

# or the tray application (macOS menu-bar icon)
just run
```

Then in Bitfocus Companion (or ATEM Software Control, etc.) add a HyperDeck at this
machine's IP on port 9993. Transport buttons will drive the locked-on player.

> The `just serve` / `just demo` recipes default to `127.0.0.1:9993` for local use.
> For a controller on another machine, bind `0.0.0.0:9993` as shown above.

## `just` commands

| Command | What it does |
|---|---|
| `just` | list all recipes |
| `just build` | build `hyperdeck-adapter` + `injcheck` into `./bin` |
| `just test` | run the full test suite with the race detector |
| `just check` | `go vet` + `gofmt` check (no changes) |
| `just fmt` | format the code in place |
| `just cross` | cross-compile sanity for Windows + Linux |
| `just trust` | (macOS) prompt for / verify the input permission |
| `just list [filter]` | list on-screen windows, e.g. `just list vlc` |
| `just serve` | run the adapter headless in the foreground (Ctrl-C to stop) |
| `just demo` | the end-to-end demo above |
| `just stop` | stop a running headless adapter |
| `just run` | run the tray application |

Override the address or config per invocation (assignments come **before** the
recipe name):

```sh
just bind=0.0.0.0:9993 serve
just bind=127.0.0.1:9993 demo
just profiles=/path/to/profiles.yaml serve
```

## Configuration

Players are defined in a YAML profile file. See
[`examples/profiles.yaml`](examples/profiles.yaml) for working **Example Player**,
**VLC**, and **Mitti** profiles. Copy it to the OS config directory the adapter reads
by default:

- macOS: `~/Library/Application Support/hyperdeck-adapter/profiles.yaml`
- Windows: `%AppData%\hyperdeck-adapter\profiles.yaml`

Each profile declares how to recognize the player's window, its keymap, the injection
mode, and how clips are reported. A minimal example:

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

To add an app: run it, find its window/process name with `just list <name>`, and add
a profile. Player keyboard shortcuts differ by OS (macOS apps often use ⌘ where
Windows uses Ctrl), so set keys for the platform you run on.

## Diagnostics (`injcheck`)

`injcheck` is a small tool for verifying the OS injector against a running app — it is
how the macOS injector was validated and how the Windows one will be:

```sh
just build
./bin/injcheck list vlc          # find a window (HANDLE / PROCESS / TITLE)
./bin/injcheck focus  <handle>   # bring it to the foreground
./bin/injcheck keys   <handle> space cmd+right   # focus then send keys
./bin/injcheck bgkeys <handle> space             # send without focusing (background)
./bin/injcheck trust             # (macOS) check/prompt the input permission
```

On macOS `HANDLE` is the process id; on Windows it is the window handle.

## Permissions (macOS)

Synthesized keystrokes require the **Accessibility** permission. The adapter prompts
on launch; enable the binary in **System Settings → Privacy & Security →
Accessibility**. `just trust` triggers the prompt and reports the current state.

Window enumeration and matching by process name need no special permission. Reading
window *titles* (used by `state: title_regex`) may require Screen Recording on recent
macOS; the bundled profiles avoid depending on it where possible.

## Project layout

```
cmd/hyperdeck-adapter/   the adapter (tray app / -no-tray headless)
cmd/injcheck/            injector diagnostics
internal/core/           the hexagon: domain, ports, application services (pure)
internal/adapter/driving/hyperdeck/   TCP protocol server (driving adapter)
internal/adapter/driven/              injector, clipsource, stateprobe, config, tray, clock
examples/profiles.yaml   sample player profiles
scripts/hyperdeck-demo.py  the demo's HyperDeck client
docs/superpowers/specs/  design + handoff specs
```

The architecture is hexagonal: a pure core surrounded by adapters. The only
OS-specific code is the injector, behind one interface with `windows` / `darwin` /
no-op implementations — which is what lets the rest build and test on any platform.

## Releasing (maintainers)

Releases are built by [`.github/workflows/release.yml`](.github/workflows/release.yml)
on a native macOS + Windows + Linux matrix (macOS needs a real runner for its cgo
injector). To cut a release, push a `vX.Y.Z` tag:

```sh
git tag v0.1.0 && git push origin v0.1.0
```

The workflow creates a **draft** GitHub release and attaches the macOS `.dmg`,
Windows `-setup.exe`, Linux `.tar.gz` + `.deb`, and `checksums.txt`. Review and
publish the draft. You can also run it manually (**Actions → Release →
Run workflow**) with a version number, which creates the tag for you.

Packaging assets live under [`build/`](build/): the app icon
(`cd build && go run gen_appicon.go` regenerates `icon/appicon.png` +
`icon/app.ico`), the macOS `Info.plist`, the NSIS installer script, and the nfpm
`.deb` config.

**Code signing** is wired up but inert until secrets are set, so unsigned releases
work out of the box. Add these repository secrets to activate it:

- macOS: `APPLE_CERTIFICATE`, `APPLE_CERTIFICATE_PASSWORD`, `APPLE_SIGNING_IDENTITY`
  (signing) and `APPLE_ID`, `APPLE_PASSWORD`, `APPLE_TEAM_ID` (notarization).
- Windows: `WINDOWS_CERTIFICATE`, `WINDOWS_CERTIFICATE_PASSWORD` (Authenticode).
