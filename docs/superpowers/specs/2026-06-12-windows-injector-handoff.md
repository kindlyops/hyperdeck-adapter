# Windows Injector — Implementation Handoff

**Date:** 2026-06-12
**For:** an agent working on a **Windows** machine with Go installed.
**Status:** ready to implement. The macOS injector is the reference implementation; this is the Windows twin.

## Goal

Replace the stub at `internal/adapter/driven/injector/injector_windows.go` with a real
implementation that enumerates windows, focuses an app, and delivers synthesized
keystrokes (foreground and background) — then verify it against **VLC on Windows**.

No cgo. Use `golang.org/x/sys/windows` (already a dependency, `v0.46.0`) to call
`user32.dll` / `kernel32.dll`.

## Background you need

This project is a cross-platform Go app (`hyperdeck-adapter`) that emulates a
Blackmagic HyperDeck over TCP and translates HyperDeck commands into OS
keystrokes for a locked-on local media player. Architecture is hexagonal; the
injector is a driven adapter behind one interface, build-tagged per OS:

- `injector_darwin.go` + `keymap_darwin.go` + `cinject_darwin.{h,m}` — **done and
  verified** (CoreGraphics + AppKit). **Read these as your template.**
- `injector_noop.go` — `!windows && !darwin` fallback.
- `injector_windows.go` — **the stub you are replacing.**
- `injector_mock.go` — in-memory test double (must keep satisfying the interface).

You don't need to check the macOS/Linux builds — your file is `windows`-tagged, so
those builds exclude it. Your job is the `windows` build plus the live VLC test.

## The contract (do not change these signatures)

`injector_windows.go` must keep declaring this exact interface (identical to the
darwin/noop variants) and a `New()` returning it:

```go
//go:build windows

package injector

// Injector is the union of the two OS-facing driven ports.
type Injector interface {
	Focus(w domain.Window) error
	SendKeys(w domain.Window, chords []domain.Chord) error
	OpenWindows() ([]domain.Window, error)
}

func New() (Injector, error)
```

Domain types you consume (`internal/core/domain`):

```go
type Window struct {
	Handle  uintptr // platform-defined; see convention below
	Title   string
	Process string  // executable/owner name used for profile matching
}

type Chord struct {
	Mods []Modifier // ModCtrl, ModAlt, ModShift, ModCmd
	Key  string     // lowercase base key, e.g. "space", "n", "right", "."
}
```

### `Window.Handle` convention (Windows)

Store the **HWND** (window handle) in `Handle`. (macOS stores the pid instead —
`Handle` is opaque to the core, so each platform stores what its `Focus`/`SendKeys`
need.) `OpenWindows` sets `Handle = HWND`; `Focus` and `SendKeys` use that HWND.

`RequestAccessibility()` already has a non-darwin no-op (`accessibility_other.go`),
so the Windows build needs nothing there — `injcheck trust` will just print
"granted". Leave it.

## Files to create / modify

| File | Action |
|---|---|
| `internal/adapter/driven/injector/injector_windows.go` | **Replace** stub with real impl (build tag `windows`) |
| `internal/adapter/driven/injector/keymap_windows.go` | **Create** — pure VK-code + modifier mapping (build tag `windows`) |
| `internal/adapter/driven/injector/keymap_windows_test.go` | **Create** — table tests for the mapping (build tag `windows`) |
| `examples/profiles.yaml` | **Add** a Windows VLC profile (or a separate `examples/profiles.windows.yaml`) with verified Windows keys |
| `docs/superpowers/specs/2026-06-11-hyperdeck-adapter-design.md` | **Update** "Open items" to mark the Windows injector done, recording findings |

Keep the pure mapping in its own file (`keymap_windows.go`) so it is unit-testable
without invoking any Win32 call — exactly as `keymap_darwin.go` separates
`keyCode`/`eventFlags` from the cgo syscall layer.

## Implementation detail per method

### `OpenWindows() ([]domain.Window, error)`

Enumerate top-level windows; for each, capture HWND, title, and owning process
exe name. Filter to **visible** windows with a non-empty title (this drops tray
helpers and tool windows). Recommended Win32:

- `EnumWindows(callback, 0)` — `user32`. The callback receives each HWND. Use
  `syscall.NewCallback`. Append HWNDs to a slice captured in the closure.
- `IsWindowVisible(hwnd)` — `user32`. Skip invisible.
- `GetWindowTextLengthW` + `GetWindowTextW` — `user32`. Title (UTF-16 → string).
  No special permission needed on Windows (unlike macOS Screen Recording).
- `GetWindowThreadProcessId(hwnd, &pid)` — `user32`. Gets the owning pid.
- Map pid → exe base name. Build the map **once per call** via
  `CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0)` +
  `Process32FirstW`/`Process32NextW` (all in `golang.org/x/sys/windows`), reading
  `ProcessEntry32.ExeFile` and `ProcessID`. (Alternative: `OpenProcess` +
  `QueryFullProcessImageNameW` per pid, then base name.)

`Process` should be the exe name as it appears for matching — e.g. `vlc.exe`. Run
`injcheck list vlc` to confirm the exact string and put that in the profile's
`match.process`.

> Reference draft (verify on device — Win32 struct layouts are unforgiving):
> ```go
> var user32 = windows.NewLazySystemDLL("user32.dll")
> var procEnumWindows = user32.NewProc("EnumWindows")
> var procIsWindowVisible = user32.NewProc("IsWindowVisible")
> var procGetWindowTextW = user32.NewProc("GetWindowTextW")
> var procGetWindowTextLengthW = user32.NewProc("GetWindowTextLengthW")
> var procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
>
> func (w *winInjector) OpenWindows() ([]domain.Window, error) {
> 	names := processNames() // pid -> exe base name, via Toolhelp snapshot
> 	var out []domain.Window
> 	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
> 		if visible, _, _ := procIsWindowVisible.Call(hwnd); visible == 0 {
> 			return 1 // continue
> 		}
> 		title := windowText(hwnd) // GetWindowTextLengthW + GetWindowTextW
> 		if title == "" {
> 			return 1
> 		}
> 		var pid uint32
> 		procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
> 		out = append(out, domain.Window{
> 			Handle:  hwnd,
> 			Title:   title,
> 			Process: names[pid],
> 		})
> 		return 1
> 	})
> 	procEnumWindows.Call(cb, 0)
> 	return out, nil
> }
> ```

### `Focus(w domain.Window) error`

Bring `HWND = w.Handle` to the foreground: `SetForegroundWindow(hwnd)` (`user32`).

⚠️ **Windows restricts `SetForegroundWindow`.** A background process often cannot
steal foreground — Windows flashes the taskbar instead. This is the Windows analog
of the macOS activation-policy issue we hit. Mitigations, in order of preference:
1. Prefer **background** injection (`PostMessage`, below) which needs no focus.
2. If focus is required, use the `AttachThreadInput` trick: attach the calling
   thread's input to the target window's thread, call `SetForegroundWindow`, then
   detach. Optionally `ShowWindow(hwnd, SW_RESTORE)` first if minimized.
3. After focusing, **sleep ~120ms** (focus-settle) before sending keys — the macOS
   impl learned this is necessary; Windows foreground changes are also async.

Return a clear error if `SetForegroundWindow` reports failure.

### `SendKeys(w domain.Window, chords []domain.Chord) error`

Two modes. The **core** decides which by calling `Focus` first or not (focus-mode
profiles call `Focus` then `SendKeys`; background-mode profiles call `SendKeys`
only). So `SendKeys` itself should deliver to the foreground via `SendInput`
**and/or** post to the specific HWND via `PostMessageW`. Recommended approach that
matches the core's model: **deliver to the target HWND with `PostMessageW`** (works
backgrounded, like macOS `CGEventPostToPid`), and rely on `Focus` having been
called for focus-mode profiles.

Per the macOS lessons, add delays: a short **key-hold** between key-down and key-up,
and a **per-key flush** sleep (events posted right before a short-lived process —
like `injcheck` — exits can be dropped). Mirror `focusSettle`/`afterKey` constants.

**Foreground path — `SendInput` (`user32`):** build an array of `INPUT` structs
(keyboard). For a chord with modifiers: modifier-down(s) → key-down → key-up →
modifier-up(s) in reverse. Use `KEYEVENTF_KEYUP` (0x0002) for releases. On 64-bit,
`INPUT` is 40 bytes; lay it out exactly:

```go
type keybdInput struct {
	Vk      uint16
	Scan    uint16
	Flags   uint32
	Time    uint32
	ExtraInfo uintptr
}
type input struct {
	Type uint32
	Ki   keybdInput
	_    [8]byte // pad so sizeof(input) == 40 on amd64
}
// SendInput(nInputs, *input, sizeof(input))
```

**Background path — `PostMessageW` (`user32`):** post `WM_KEYDOWN` (0x0100) then
`WM_KEYUP` (0x0101) to the HWND with `wParam = VK`. Build a proper `lParam`:
bit 0–15 repeat count (1), bits 16–23 scan code from `MapVirtualKeyW(vk,
MAPVK_VK_TO_VSC)`, and for `WM_KEYUP` set bits 30 (previous state) and 31
(transition). **Known limitation:** many apps read modifier state via
`GetKeyState`, which `PostMessage` does **not** set, so **modified** chords
(ctrl+x) frequently do not register in background mode. Single-key chords (Space,
n, p, s) usually work. Document which VLC keys work background vs. need focus.

> Default policy suggestion: try `PostMessageW` to the HWND for both modes; if a
> profile's chords have modifiers and background fails in testing, mark that profile
> `injection: focus` and use `SendInput` after `Focus`. Make the choice explicit in
> the profile, like the macOS profiles do.

## Keymap (`keymap_windows.go`, pure & testable)

Map our lowercase key names → Windows Virtual-Key codes, and our modifiers → their
VKs. Cover at least the keys the profiles use plus a–z, 0–9, arrows, space, enter,
esc, tab, period, comma.

```go
//go:build windows

package injector

import "github.com/kindlyops/hyperdeck-adapter/internal/core/domain"

var windowsKeyCodes = map[string]uint16{
	"space": 0x20, "enter": 0x0D, "return": 0x0D, "tab": 0x09, "esc": 0x1B, "escape": 0x1B,
	"left": 0x25, "up": 0x26, "right": 0x27, "down": 0x28,
	"period": 0xBE, ".": 0xBE, "comma": 0xBC, ",": 0xBC,
	// letters: VK_A..VK_Z are 0x41..0x5A (ASCII uppercase)
	// digits:  VK_0..VK_9 are 0x30..0x39
	// fill a..z and 0..9 programmatically or explicitly
}

func keyCode(key string) (uint16, bool) {
	if c, ok := windowsKeyCodes[key]; ok {
		return c, true
	}
	if len(key) == 1 {
		ch := key[0]
		if ch >= 'a' && ch <= 'z' {
			return uint16(ch-'a') + 0x41, true
		}
		if ch >= '0' && ch <= '9' {
			return uint16(ch-'0') + 0x30, true
		}
	}
	return 0, false
}

func modifierVK(m domain.Modifier) (uint16, bool) {
	switch m {
	case domain.ModCtrl:
		return 0x11, true // VK_CONTROL
	case domain.ModShift:
		return 0x10, true // VK_SHIFT
	case domain.ModAlt:
		return 0x12, true // VK_MENU
	case domain.ModCmd:
		return 0x5B, true // VK_LWIN (rare in app shortcuts; included for parity)
	}
	return 0, false
}
```

`keymap_windows_test.go`: table-test `keyCode` ("space"→0x20, "n"→0x4E, "right"→0x27,
"."→0xBE, unknown→false) and `modifierVK` (ctrl→0x11, etc.). These run via
`go test` on Windows; CI on Linux will not exercise them (build-tagged), which is
expected.

## Windows gotchas checklist

- **`SetForegroundWindow` restriction** — see Focus above; prefer background or the
  `AttachThreadInput` workaround.
- **UIPI / integrity level** — a non-elevated process cannot `SendInput`/`PostMessage`
  to an **elevated** window. Run VLC and the adapter at the same integrity level. If
  VLC is elevated, the injector must be too (or VLC won't receive input).
- **`PostMessage` modifiers** — not reflected in `GetKeyState`; modified background
  chords may no-op. Use `SendInput` (focus mode) for modified chords.
- **Struct sizes** — `INPUT` must be exactly 40 bytes on amd64; wrong size makes
  `SendInput` silently fail. Verify with a one-key smoke test first.
- **Delays** — replicate `focusSettle` (~120ms after focus) and `afterKey` (~25ms,
  doubles as the process-exit flush for `injcheck`).
- **UTF-16** — `GetWindowTextW` returns UTF-16; convert with
  `windows.UTF16ToString`.

## Verification plan (on Windows)

The repo ships a diagnostic tool, `cmd/injcheck`, that drives the injector directly
(it is how the macOS injector was verified). Build it and use it:

```
go build -o injcheck.exe ./cmd/injcheck
./injcheck.exe list vlc            # confirm VLC's window + process name
./injcheck.exe focus <hwnd>        # confirm SetForegroundWindow works
./injcheck.exe keys   <hwnd> space # focus-mode play/pause
./injcheck.exe bgkeys <hwnd> space # background play/pause (no focus)
```

`injcheck list` prints `HANDLE  PROCESS  TITLE`; on Windows HANDLE is the HWND.
Pass that HWND to `focus`/`keys`/`bgkeys`.

**Make it observable.** On Windows, VLC has no AppleScript; read its state via VLC's
HTTP interface instead:
1. VLC → Preferences (All) → Interface → Main interfaces → enable **Web**; set a
   password under Lua HTTP. Or launch `vlc.exe --extraintf http --http-password p`.
2. Read status: `GET http://127.0.0.1:8080/requests/status.json` (basic auth, empty
   user) → JSON `state` field is `playing`/`paused`/`stopped`, and current track
   info is present. Poll it before/after each injected key.

Verification steps (mirror the macOS pass):
1. Open VLC with 2–3 short media files (a positional playlist).
2. `list vlc` → record HWND + exact process name → set `match.process`.
3. `keys <hwnd> space` → status flips playing↔paused. Repeat to confirm toggle.
4. `keys <hwnd> <stopkey>` → status `stopped`.
5. `keys <hwnd> <nextkey>` / `<prevkey>` → current track changes.
6. `bgkeys <hwnd> space` with another window foreground → does VLC still toggle?
   Decide `injection: background` vs `focus` from this result, per chord type.

### Windows VLC keys

VLC's hotkeys differ from macOS. **Read the actual bindings** from
`%APPDATA%\vlc\vlcrc` (keys `key-play-pause`, `key-stop`, `key-next`, `key-prev`)
and use those. Cross-platform VLC defaults are typically single letters
(Space / s / n / p) rather than macOS's Cmd combos — but confirm from `vlcrc`, do
not assume. Put the confirmed keys in the Windows VLC profile.

## Acceptance criteria

- `GOOS=windows go build ./...` compiles on Windows; `go vet` clean; `gofmt -l .`
  empty.
- `go test ./internal/adapter/driven/injector/` passes on Windows (the new keymap
  tests run; existing mock test still passes).
- `injcheck` against VLC on Windows: enumerate finds VLC, focus works (or is
  consciously skipped in favor of background), and play/pause + stop + next/prev all
  take effect (confirmed via VLC's HTTP `status.json`).
- A Windows VLC profile with the verified keys is added to the examples.
- No regression to the macOS/Linux builds (your changes are all `windows`-tagged; do
  not touch portable files except adding the example profile and the spec note).
- Update the design spec's "Open items" to mark the Windows injector done and record
  any findings (e.g. which keys needed focus vs. background, UIPI notes).

## Reference: the macOS twin

Read these for the exact pattern you are mirroring:

- `internal/adapter/driven/injector/injector_darwin.go` — `New`, `Focus`
  (activate + `focusSettle`), `SendKeys` (`CGEventPostToPid` + `afterKey`),
  `OpenWindows`, and the `focusSettle`/`afterKey` constants.
- `internal/adapter/driven/injector/keymap_darwin.go` + `_test.go` — the pure,
  testable mapping split you should replicate for VK codes.
- `cmd/injcheck/main.go` — the verification harness (works as-is on Windows).
- `examples/profiles.yaml` — macOS VLC/the player/Mitti profiles; add the Windows VLC
  counterpart in the same shape.

When done, commit on a `feat/windows-injector` branch and hand back for review.
