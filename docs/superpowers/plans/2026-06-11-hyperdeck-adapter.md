# HyperDeck Adapter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a cross-platform Go tray app that emulates one Blackmagic HyperDeck on TCP 9993 and translates HyperDeck commands into keystrokes for a locked-on local player (VLC, Example Player, Mitti).

**Architecture:** Hexagonal (ports & adapters). A pure domain core (`internal/core`) defines driving ports (`Transport`, `Query`) and driven ports (`KeyInjector`, `WindowEnumerator`, `ClipSource`, `StateProbe`, `StatusPresenter`, `ProfileStore`, `Clock`). Adapters implement the ports; the composition root (`cmd/hyperdeck-adapter`) wires them and selects the OS injector by build tag. The entire core compiles and tests on any OS against in-memory fakes.

**Tech Stack:** Go 1.23+, `gopkg.in/yaml.v3` (config), `pgregory.net/rapid` (property tests), `golang.org/x/sys/windows` (Windows injection), cgo + CoreGraphics/Accessibility (macOS injection), `fyne.io/systray` (tray UI).

**Reference:** Design spec at `docs/superpowers/specs/2026-06-11-hyperdeck-adapter-design.md`. HyperDeck client reference (protocol shape) in `Blackmagic_HyperDeck_Developer_SDK_1.0.zip` → `HyperDeck.py`.

**Module path:** `github.com/kindlyops/hyperdeck-adapter`

---

## File structure

```
go.mod
internal/core/domain/        chord.go transport.go clip.go profile.go window.go    (pure value objects)
internal/core/port/          inbound.go outbound.go                                (interfaces only)
internal/core/app/           session.go virtualdeck.go lockmanager.go reconciler.go errors.go
internal/adapter/driving/hyperdeck/   codes.go parser.go responder.go server.go
internal/adapter/driven/injector/     injector_mock.go injector_noop.go injector_windows.go injector_darwin.go
internal/adapter/driven/clipsource/   playlist.go positional.go mitti.go factory.go
internal/adapter/driven/stateprobe/   titleregex.go none.go factory.go
internal/adapter/driven/config/       store.go
internal/adapter/driven/tray/         tray.go
internal/adapter/driven/clock/        clock.go
cmd/hyperdeck-adapter/       main.go
testdata/                    sample.m3u sample.xspf profiles.yaml
.github/workflows/ci.yml
```

**Phase map (each phase ends green and committed):**
- Phase 1 (Tasks 1–4): scaffold + pure domain types — TDD, runs everywhere.
- Phase 2 (Task 5): ports + in-memory fakes.
- Phase 3 (Tasks 6–9): core application services — the heart, fully TDD.
- Phase 4 (Tasks 10–12): config, clipsource, stateprobe driven adapters — TDD.
- Phase 5 (Tasks 13–15): HyperDeck protocol driving adapter + end-to-end test.
- Phase 6 (Tasks 16–18): OS injector, tray, clock driven adapters — **manually verified** on Win/macOS.
- Phase 7 (Task 19): composition root, cross-compile, smoke checklist.

---

## Phase 1 — Scaffold and pure domain

### Task 1: Project scaffold

**Files:**
- Create: `go.mod`, `internal/core/domain/doc.go`, `.github/workflows/ci.yml`

- [ ] **Step 1: Initialize the module**

Run:
```bash
cd /Users/emurphy/kindlyops/hyperdeck-adapter
go mod init github.com/kindlyops/hyperdeck-adapter
```
Expected: creates `go.mod` with `go 1.23` (or the installed toolchain version).

- [ ] **Step 2: Add a package doc file so the module builds**

Create `internal/core/domain/doc.go`:
```go
// Package domain holds the pure value objects of the virtual HyperDeck.
// It imports nothing outside the standard library and contains no I/O.
package domain
```

- [ ] **Step 3: Verify it builds**

Run: `go build ./...`
Expected: no output, exit 0.

- [ ] **Step 4: Add CI workflow**

Create `.github/workflows/ci.yml`:
```yaml
name: ci
on:
  push:
    branches: [main]
  pull_request:
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.23"
      - run: go vet ./...
      - run: go test ./... -race -count=1
```

- [ ] **Step 5: Commit**

```bash
git add go.mod internal/core/domain/doc.go .github/workflows/ci.yml
git commit -m "chore: scaffold module and CI"
```

---

### Task 2: Chord parsing

A `Chord` is one keypress with modifiers, e.g. `ctrl+right`, `cmd+esc`, `space`.

**Files:**
- Create: `internal/core/domain/chord.go`
- Test: `internal/core/domain/chord_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/core/domain/chord_test.go`:
```go
package domain

import (
	"reflect"
	"testing"
)

func TestParseChord(t *testing.T) {
	cases := []struct {
		in   string
		want Chord
	}{
		{"space", Chord{Key: "space"}},
		{"s", Chord{Key: "s"}},
		{"ctrl+right", Chord{Mods: []Modifier{ModCtrl}, Key: "right"}},
		{"cmd+esc", Chord{Mods: []Modifier{ModCmd}, Key: "esc"}},
		{"CTRL+Shift+Up", Chord{Mods: []Modifier{ModCtrl, ModShift}, Key: "up"}},
	}
	for _, c := range cases {
		got, err := ParseChord(c.in)
		if err != nil {
			t.Fatalf("ParseChord(%q) error: %v", c.in, err)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("ParseChord(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestParseChordErrors(t *testing.T) {
	for _, in := range []string{"", "ctrl+", "+s", "bogusmod+x"} {
		if _, err := ParseChord(in); err == nil {
			t.Errorf("ParseChord(%q) expected error, got nil", in)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/domain/ -run TestParseChord -v`
Expected: FAIL — `undefined: Chord` / `undefined: ParseChord`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/core/domain/chord.go`:
```go
package domain

import (
	"fmt"
	"strings"
)

// Modifier is a keyboard modifier key.
type Modifier string

const (
	ModCtrl  Modifier = "ctrl"
	ModAlt   Modifier = "alt"
	ModShift Modifier = "shift"
	ModCmd   Modifier = "cmd"
)

var knownModifiers = map[string]Modifier{
	"ctrl": ModCtrl, "alt": ModAlt, "shift": ModShift, "cmd": ModCmd,
}

// Chord is a single keystroke: zero or more modifiers plus one base key.
type Chord struct {
	Mods []Modifier
	Key  string
}

// ParseChord parses strings like "ctrl+right" or "space" into a Chord.
// Matching is case-insensitive. The base key must be the last token.
func ParseChord(s string) (Chord, error) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(s)), "+")
	if len(parts) == 0 || parts[len(parts)-1] == "" {
		return Chord{}, fmt.Errorf("parse chord %q: empty key", s)
	}
	var c Chord
	for i, p := range parts {
		if p == "" {
			return Chord{}, fmt.Errorf("parse chord %q: empty token", s)
		}
		if i == len(parts)-1 {
			c.Key = p
			continue
		}
		mod, ok := knownModifiers[p]
		if !ok {
			return Chord{}, fmt.Errorf("parse chord %q: unknown modifier %q", s, p)
		}
		c.Mods = append(c.Mods, mod)
	}
	return c, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/domain/ -run TestParseChord -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/domain/chord.go internal/core/domain/chord_test.go
git commit -m "feat(domain): chord parsing"
```

---

### Task 3: Transport state, clip, window value objects

**Files:**
- Create: `internal/core/domain/transport.go`, `internal/core/domain/clip.go`, `internal/core/domain/window.go`
- Test: `internal/core/domain/transport_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/core/domain/transport_test.go`:
```go
package domain

import "testing"

func TestTransportStateHyperDeckStatus(t *testing.T) {
	if StateStopped.HyperDeckStatus() != "stopped" {
		t.Errorf("StateStopped = %q", StateStopped.HyperDeckStatus())
	}
	if StatePlaying.HyperDeckStatus() != "play" {
		t.Errorf("StatePlaying = %q", StatePlaying.HyperDeckStatus())
	}
}

func TestClipListIndexing(t *testing.T) {
	cl := ClipList{{ID: 1, Name: "a"}, {ID: 2, Name: "b"}}
	if cl.Len() != 2 {
		t.Fatalf("Len = %d", cl.Len())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/domain/ -run 'TestTransport|TestClip' -v`
Expected: FAIL — undefined identifiers.

- [ ] **Step 3: Write minimal implementations**

Create `internal/core/domain/transport.go`:
```go
package domain

// TransportState is the modeled (open-loop) play state of the deck.
type TransportState int

const (
	StateStopped TransportState = iota
	StatePlaying
)

// HyperDeckStatus maps the modeled state to a HyperDeck transport "status" value.
func (s TransportState) HyperDeckStatus() string {
	if s == StatePlaying {
		return "play"
	}
	return "stopped"
}

// TransportInfo is the payload of a "transport info" response.
type TransportInfo struct {
	Status  string
	Speed   int
	ClipID  int
	SlotID  int
}

// SlotInfo is the payload of a "slot info" response.
type SlotInfo struct {
	Present bool
	SlotID  int
}

// DeviceInfo is the payload of a "device info" response.
type DeviceInfo struct {
	ProtocolVersion string
	Model           string
	UniqueID        string
}
```

Create `internal/core/domain/clip.go`:
```go
package domain

// Clip is one entry in the deck's clip list.
type Clip struct {
	ID       int
	Name     string
	Timecode string
	Duration string
}

// ClipList is the ordered set of clips the controller can navigate.
type ClipList []Clip

// Len returns the number of clips.
func (c ClipList) Len() int { return len(c) }
```

Create `internal/core/domain/window.go`:
```go
package domain

// Window identifies a target OS window the injector can act on.
type Window struct {
	Handle  uintptr
	Title   string
	Process string
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/domain/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/domain/transport.go internal/core/domain/clip.go internal/core/domain/window.go internal/core/domain/transport_test.go
git commit -m "feat(domain): transport, clip, window value objects"
```

---

### Task 4: Profile and window matching

**Files:**
- Create: `internal/core/domain/profile.go`
- Test: `internal/core/domain/profile_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/core/domain/profile_test.go`:
```go
package domain

import "testing"

func vlcProfile() Profile {
	return Profile{
		ID:        "vlc",
		Match:     Match{Process: []string{"vlc.exe", "VLC"}, TitleRegex: "VLC media player"},
		Injection: InjectionBackground,
		Keymap: Keymap{
			KeyPlay: {Key: "space"},
			KeyStop: {Key: "s"},
			KeyNext: {Key: "n"},
			KeyPrev: {Key: "p"},
		},
	}
}

func TestProfileMatchesWindow(t *testing.T) {
	p := vlcProfile()
	if !p.MatchesWindow(Window{Process: "vlc.exe", Title: "Big Buck Bunny - VLC media player"}) {
		t.Error("expected vlc.exe + title to match")
	}
	if p.MatchesWindow(Window{Process: "chrome.exe", Title: "VLC media player"}) {
		t.Error("wrong process should not match")
	}
	if p.MatchesWindow(Window{Process: "vlc.exe", Title: "Something Else"}) {
		t.Error("title regex mismatch should not match")
	}
}

func TestProfileMatchEmptyTitleRegexMatchesAnyTitle(t *testing.T) {
	p := Profile{Match: Match{Process: []string{"Mitti"}}}
	if !p.MatchesWindow(Window{Process: "Mitti", Title: "anything"}) {
		t.Error("empty title regex should match any title")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/domain/ -run TestProfile -v`
Expected: FAIL — undefined `Profile`, `Match`, `Keymap`, `InjectionBackground`, key constants.

- [ ] **Step 3: Write minimal implementation**

Create `internal/core/domain/profile.go`:
```go
package domain

import "regexp"

// KeyName is a logical transport action mapped to a chord by a profile.
type KeyName string

const (
	KeyPlay   KeyName = "play"
	KeyStop   KeyName = "stop"
	KeyRecord KeyName = "record"
	KeyNext   KeyName = "next"
	KeyPrev   KeyName = "prev"
)

// InjectionMode selects how keystrokes reach the target window.
type InjectionMode string

const (
	InjectionFocus      InjectionMode = "focus"
	InjectionBackground InjectionMode = "background"
)

// Keymap maps logical actions to concrete chords.
type Keymap map[KeyName]Chord

// Match describes how to recognize the player's window.
type Match struct {
	Process    []string
	TitleRegex string
}

// ClipSourceConfig selects and parameterizes the clip source strategy.
type ClipSourceConfig struct {
	Type  string // "playlist_file" | "positional" | "mitti"
	Path  string
	Count int
}

// StateConfig selects and parameterizes best-effort state detection.
type StateConfig struct {
	Type    string // "title_regex" | "none"
	Playing string // regex applied to the window title when Type == "title_regex"
}

// Profile is one player application's complete mapping definition.
type Profile struct {
	ID             string
	Match          Match
	Injection      InjectionMode
	Keymap         Keymap
	PlayStopToggle bool // when true, the play key toggles play/stop (e.g. Example Player Space)
	ClipSource     ClipSourceConfig
	State          StateConfig
	Homing         []Chord
}

// MatchesWindow reports whether w belongs to this profile.
func (p Profile) MatchesWindow(w Window) bool {
	matched := false
	for _, name := range p.Match.Process {
		if name == w.Process {
			matched = true
			break
		}
	}
	if !matched {
		return false
	}
	if p.Match.TitleRegex == "" {
		return true
	}
	re, err := regexp.Compile(p.Match.TitleRegex)
	if err != nil {
		return false
	}
	return re.MatchString(w.Title)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/domain/ -v`
Expected: PASS (all domain tests).

- [ ] **Step 5: Commit**

```bash
git add internal/core/domain/profile.go internal/core/domain/profile_test.go
git commit -m "feat(domain): profile and window matching"
```

---

## Phase 2 — Ports and fakes

### Task 5: Port interfaces and in-memory fakes

**Files:**
- Create: `internal/core/port/inbound.go`, `internal/core/port/outbound.go`
- Create: `internal/adapter/driven/injector/injector_mock.go`
- Test: `internal/adapter/driven/injector/injector_mock_test.go`

- [ ] **Step 1: Define the ports (no test yet — interfaces only)**

Create `internal/core/port/inbound.go`:
```go
// Package port declares the hexagon's driving and driven port interfaces.
package port

import "github.com/kindlyops/hyperdeck-adapter/internal/core/domain"

// Transport is a driving (inbound) port: the deck's command surface.
type Transport interface {
	Play() error
	Stop() error
	Record() error
	Goto(clipID int) error
	Next() error
	Prev() error
	Rehome() error
}

// Query is a driving (inbound) port: the deck's read surface.
type Query interface {
	TransportInfo() domain.TransportInfo
	Clips() domain.ClipList
	SlotInfo() domain.SlotInfo
	DeviceInfo() domain.DeviceInfo
}
```

Create `internal/core/port/outbound.go`:
```go
package port

import (
	"time"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

// KeyInjector is a driven (outbound) port: deliver keystrokes to a window.
type KeyInjector interface {
	Focus(w domain.Window) error
	SendKeys(w domain.Window, chords []domain.Chord) error
}

// WindowEnumerator is a driven port: list currently-open OS windows.
type WindowEnumerator interface {
	OpenWindows() ([]domain.Window, error)
}

// ClipSource is a driven port: produce the active clip list.
type ClipSource interface {
	List() (domain.ClipList, error)
}

// StateProbe is a driven port: best-effort real-state detection.
type StateProbe interface {
	Detect(w domain.Window) (domain.TransportState, bool)
}

// StatusPresenter is a driven port: reflect lock status in the UI.
type StatusPresenter interface {
	Present(lock domain.LockState)
}

// ProfileStore is a driven port: load validated profiles.
type ProfileStore interface {
	Load() ([]domain.Profile, error)
}

// Clock is a driven port: a tick source the reconciler/locator poll on.
type Clock interface {
	Tick(d time.Duration) <-chan time.Time
}
```

- [ ] **Step 2: Add LockState to domain (referenced by the presenter port)**

Add to `internal/core/domain/window.go`:
```go

// LockState is the current player-lock status.
type LockState struct {
	Locked  bool
	Profile *Profile
	Window  Window
}
```

- [ ] **Step 3: Write the failing test for the mock injector**

Create `internal/adapter/driven/injector/injector_mock_test.go`:
```go
package injector

import (
	"testing"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

func TestMockRecordsCalls(t *testing.T) {
	m := NewMock()
	w := domain.Window{Process: "vlc.exe"}
	if err := m.Focus(w); err != nil {
		t.Fatal(err)
	}
	if err := m.SendKeys(w, []domain.Chord{{Key: "space"}}); err != nil {
		t.Fatal(err)
	}
	if len(m.Focused) != 1 || m.Focused[0] != w {
		t.Errorf("Focused = %+v", m.Focused)
	}
	if len(m.Sent) != 1 || m.Sent[0].Chords[0].Key != "space" {
		t.Errorf("Sent = %+v", m.Sent)
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/adapter/driven/injector/ -v`
Expected: FAIL — undefined `NewMock`.

- [ ] **Step 5: Write the mock**

Create `internal/adapter/driven/injector/injector_mock.go`:
```go
package injector

import "github.com/kindlyops/hyperdeck-adapter/internal/core/domain"

// SentKeys records one SendKeys call.
type SentKeys struct {
	Window domain.Window
	Chords []domain.Chord
}

// Mock is an in-memory KeyInjector + WindowEnumerator for tests.
type Mock struct {
	Windows   []domain.Window // returned by OpenWindows
	Focused   []domain.Window
	Sent      []SentKeys
	FocusErr  error
	SendErr   error
	EnumErr   error
}

// NewMock returns an empty Mock.
func NewMock() *Mock { return &Mock{} }

func (m *Mock) Focus(w domain.Window) error {
	if m.FocusErr != nil {
		return m.FocusErr
	}
	m.Focused = append(m.Focused, w)
	return nil
}

func (m *Mock) SendKeys(w domain.Window, chords []domain.Chord) error {
	if m.SendErr != nil {
		return m.SendErr
	}
	m.Sent = append(m.Sent, SentKeys{Window: w, Chords: chords})
	return nil
}

func (m *Mock) OpenWindows() ([]domain.Window, error) {
	if m.EnumErr != nil {
		return nil, m.EnumErr
	}
	return m.Windows, nil
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./... -v`
Expected: PASS (domain + injector mock).

- [ ] **Step 7: Commit**

```bash
git add internal/core/port internal/core/domain/window.go internal/adapter/driven/injector
git commit -m "feat(core): ports and mock injector"
```

---

## Phase 3 — Core application services

### Task 6: Session (shared core state)

`Session` is the mutex-guarded state shared by `VirtualDeck`, `LockManager`, and `Reconciler`. It is pure — it imports only `domain` and `port`.

**Files:**
- Create: `internal/core/app/session.go`, `internal/core/app/errors.go`
- Test: `internal/core/app/session_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/core/app/session_test.go`:
```go
package app

import (
	"testing"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

func TestSessionLockUnlock(t *testing.T) {
	s := NewSession()
	if _, _, ok := s.Active(); ok {
		t.Fatal("new session should be unlocked")
	}
	p := domain.Profile{ID: "vlc"}
	w := domain.Window{Process: "vlc.exe"}
	s.Lock(p, w, nil, nil)
	gotP, gotW, ok := s.Active()
	if !ok || gotP.ID != "vlc" || gotW.Process != "vlc.exe" {
		t.Fatalf("Active = %+v %+v %v", gotP, gotW, ok)
	}
	s.Unlock()
	if _, _, ok := s.Active(); ok {
		t.Fatal("session should be unlocked after Unlock")
	}
}

func TestSessionStateAndClip(t *testing.T) {
	s := NewSession()
	s.SetState(domain.StatePlaying)
	if s.State() != domain.StatePlaying {
		t.Error("state not set")
	}
	s.SetClips(domain.ClipList{{ID: 1}, {ID: 2}})
	if s.Clips().Len() != 2 {
		t.Error("clips not set")
	}
	s.SetCurrentClip(2)
	if s.CurrentClip() != 2 {
		t.Error("current clip not set")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/app/ -v`
Expected: FAIL — undefined `NewSession`.

- [ ] **Step 3: Write the implementation**

Create `internal/core/app/errors.go`:
```go
package app

import "errors"

// ErrNotLocked is returned when a transport command arrives with no locked player.
var ErrNotLocked = errors.New("no player locked")
```

Create `internal/core/app/session.go`:
```go
package app

import (
	"sync"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

// Session is the mutex-guarded shared state of the running deck.
type Session struct {
	mu          sync.Mutex
	lock        domain.LockState
	state       domain.TransportState
	clips       domain.ClipList
	currentClip int
	clipSource  port.ClipSource
	probe       port.StateProbe
}

// NewSession returns an unlocked session in the stopped state.
func NewSession() *Session {
	return &Session{currentClip: 1}
}

// Lock binds an active profile, window, clip source, and state probe.
func (s *Session) Lock(p domain.Profile, w domain.Window, cs port.ClipSource, sp port.StateProbe) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prof := p
	s.lock = domain.LockState{Locked: true, Profile: &prof, Window: w}
	s.clipSource = cs
	s.probe = sp
	s.state = domain.StateStopped
	s.currentClip = 1
}

// Unlock clears the active binding.
func (s *Session) Unlock() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lock = domain.LockState{}
	s.clipSource = nil
	s.probe = nil
	s.clips = nil
}

// Active returns the locked profile and window, or ok=false when unlocked.
func (s *Session) Active() (domain.Profile, domain.Window, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.lock.Locked || s.lock.Profile == nil {
		return domain.Profile{}, domain.Window{}, false
	}
	return *s.lock.Profile, s.lock.Window, true
}

// LockState returns a copy of the current lock status.
func (s *Session) LockState() domain.LockState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lock
}

func (s *Session) State() domain.TransportState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *Session) SetState(st domain.TransportState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = st
}

func (s *Session) Clips() domain.ClipList {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.clips
}

func (s *Session) SetClips(c domain.ClipList) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clips = c
}

func (s *Session) CurrentClip() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.currentClip
}

func (s *Session) SetCurrentClip(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentClip = n
}

// ClipSource returns the active clip source (nil when unlocked).
func (s *Session) ClipSource() port.ClipSource {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.clipSource
}

// Probe returns the active state probe (nil when unlocked).
func (s *Session) Probe() port.StateProbe {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.probe
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/app/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/app/session.go internal/core/app/errors.go internal/core/app/session_test.go
git commit -m "feat(core): session shared state"
```

---

### Task 7: VirtualDeck — transport commands

This is the crown jewel: toggle resolution and goto-delta. Implements `port.Transport`.

**Files:**
- Create: `internal/core/app/virtualdeck.go`
- Test: `internal/core/app/virtualdeck_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/core/app/virtualdeck_test.go`:
```go
package app

import (
	"testing"

	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/injector"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

func lockedSession(p domain.Profile, clips domain.ClipList) *Session {
	s := NewSession()
	s.Lock(p, domain.Window{Process: "x"}, nil, nil)
	s.SetClips(clips)
	return s
}

func sentKeys(m *injector.Mock) []string {
	var out []string
	for _, s := range m.Sent {
		for _, c := range s.Chords {
			out = append(out, c.Key)
		}
	}
	return out
}

func discreteProfile() domain.Profile {
	return domain.Profile{
		ID:        "vlc",
		Injection: domain.InjectionBackground,
		Keymap: domain.Keymap{
			domain.KeyPlay: {Key: "space"},
			domain.KeyStop: {Key: "s"},
			domain.KeyNext: {Key: "n"},
			domain.KeyPrev: {Key: "p"},
		},
	}
}

func toggleProfile() domain.Profile {
	return domain.Profile{
		ID:             "example",
		Injection:      domain.InjectionFocus,
		PlayStopToggle: true,
		Keymap: domain.Keymap{
			domain.KeyPlay: {Key: "space"},
			domain.KeyNext: {Key: "right", Mods: []domain.Modifier{domain.ModCtrl}},
			domain.KeyPrev: {Key: "left", Mods: []domain.Modifier{domain.ModCtrl}},
		},
	}
}

func TestPlayStopDiscrete(t *testing.T) {
	m := injector.NewMock()
	d := NewVirtualDeck(lockedSession(discreteProfile(), nil), m)
	if err := d.Play(); err != nil {
		t.Fatal(err)
	}
	if err := d.Stop(); err != nil {
		t.Fatal(err)
	}
	got := sentKeys(m)
	want := []string{"space", "s"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("keys = %v, want %v", got, want)
	}
}

func TestToggleSuppressesRedundant(t *testing.T) {
	m := injector.NewMock()
	s := lockedSession(toggleProfile(), nil)
	d := NewVirtualDeck(s, m)
	// play when stopped -> sends space; play again -> nothing.
	_ = d.Play()
	_ = d.Play()
	// stop when playing -> sends space; stop again -> nothing.
	_ = d.Stop()
	_ = d.Stop()
	got := sentKeys(m)
	if len(got) != 2 {
		t.Fatalf("expected 2 keypresses, got %v", got)
	}
}

func TestToggleFocusesFirst(t *testing.T) {
	m := injector.NewMock()
	d := NewVirtualDeck(lockedSession(toggleProfile(), nil), m)
	_ = d.Play()
	if len(m.Focused) != 1 {
		t.Errorf("focus mode should focus before sending; Focused=%v", m.Focused)
	}
}

func TestGotoComputesDelta(t *testing.T) {
	m := injector.NewMock()
	s := lockedSession(discreteProfile(), domain.ClipList{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}, {ID: 5}})
	d := NewVirtualDeck(s, m)
	// current starts at 1; goto 4 -> three "next".
	if err := d.Goto(4); err != nil {
		t.Fatal(err)
	}
	if got := sentKeys(m); len(got) != 3 || got[0] != "n" {
		t.Errorf("goto 4 keys = %v, want 3x n", got)
	}
	if s.CurrentClip() != 4 {
		t.Errorf("current clip = %d, want 4", s.CurrentClip())
	}
	// goto 2 -> two "prev".
	m.Sent = nil
	if err := d.Goto(2); err != nil {
		t.Fatal(err)
	}
	if got := sentKeys(m); len(got) != 2 || got[0] != "p" {
		t.Errorf("goto 2 keys = %v, want 2x p", got)
	}
}

func TestGotoClampsToRange(t *testing.T) {
	m := injector.NewMock()
	s := lockedSession(discreteProfile(), domain.ClipList{{ID: 1}, {ID: 2}})
	d := NewVirtualDeck(s, m)
	if err := d.Goto(99); err != nil {
		t.Fatal(err)
	}
	if s.CurrentClip() != 2 {
		t.Errorf("clamped current = %d, want 2", s.CurrentClip())
	}
}

func TestRecordUndefinedIsNoop(t *testing.T) {
	m := injector.NewMock()
	d := NewVirtualDeck(lockedSession(discreteProfile(), nil), m)
	if err := d.Record(); err != nil {
		t.Fatalf("record with no mapping should be a silent no-op, got %v", err)
	}
	if len(m.Sent) != 0 {
		t.Errorf("record should send nothing, sent %v", m.Sent)
	}
}

func TestCommandsWithoutLockError(t *testing.T) {
	m := injector.NewMock()
	d := NewVirtualDeck(NewSession(), m)
	if err := d.Play(); err != ErrNotLocked {
		t.Errorf("expected ErrNotLocked, got %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/core/app/ -run 'TestPlay|TestToggle|TestGoto|TestRecord|TestCommands' -v`
Expected: FAIL — undefined `NewVirtualDeck`.

- [ ] **Step 3: Write the implementation**

Create `internal/core/app/virtualdeck.go`:
```go
package app

import (
	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

// VirtualDeck implements port.Transport and port.Query over a Session.
type VirtualDeck struct {
	session  *Session
	injector port.KeyInjector
	device   domain.DeviceInfo
}

// NewVirtualDeck wires the deck to its shared session and key injector.
func NewVirtualDeck(s *Session, inj port.KeyInjector) *VirtualDeck {
	return &VirtualDeck{
		session:  s,
		injector: inj,
		device: domain.DeviceInfo{
			ProtocolVersion: "1.11",
			Model:           "HyperDeck Studio Mini",
			UniqueID:        "hyperdeck-adapter",
		},
	}
}

// Play moves the deck to the playing state.
func (d *VirtualDeck) Play() error {
	p, w, ok := d.session.Active()
	if !ok {
		return ErrNotLocked
	}
	if p.PlayStopToggle && d.session.State() == domain.StatePlaying {
		return nil
	}
	d.session.SetState(domain.StatePlaying)
	return d.send(p, w, domain.KeyPlay)
}

// Stop moves the deck to the stopped state.
func (d *VirtualDeck) Stop() error {
	p, w, ok := d.session.Active()
	if !ok {
		return ErrNotLocked
	}
	if p.PlayStopToggle {
		if d.session.State() != domain.StatePlaying {
			return nil
		}
		d.session.SetState(domain.StateStopped)
		return d.send(p, w, domain.KeyPlay) // toggle shares the play key
	}
	d.session.SetState(domain.StateStopped)
	return d.send(p, w, domain.KeyStop)
}

// Record sends the record key if the profile defines one; otherwise no-op.
func (d *VirtualDeck) Record() error {
	p, w, ok := d.session.Active()
	if !ok {
		return ErrNotLocked
	}
	return d.send(p, w, domain.KeyRecord)
}

// Goto navigates to a 1-based clip id via repeated next/prev keys.
func (d *VirtualDeck) Goto(clipID int) error {
	p, w, ok := d.session.Active()
	if !ok {
		return ErrNotLocked
	}
	target := clamp(clipID, 1, max(1, d.session.Clips().Len()))
	delta := target - d.session.CurrentClip()
	key := domain.KeyNext
	if delta < 0 {
		key = domain.KeyPrev
		delta = -delta
	}
	for i := 0; i < delta; i++ {
		if err := d.send(p, w, key); err != nil {
			return err
		}
	}
	d.session.SetCurrentClip(target)
	return nil
}

// Next advances one clip.
func (d *VirtualDeck) Next() error { return d.step(domain.KeyNext, +1) }

// Prev rewinds one clip.
func (d *VirtualDeck) Prev() error { return d.step(domain.KeyPrev, -1) }

// Rehome runs the profile's homing sequence and resets modeled state.
func (d *VirtualDeck) Rehome() error {
	p, w, ok := d.session.Active()
	if !ok {
		return ErrNotLocked
	}
	if p.Injection == domain.InjectionFocus {
		if err := d.injector.Focus(w); err != nil {
			return err
		}
	}
	if len(p.Homing) > 0 {
		if err := d.injector.SendKeys(w, p.Homing); err != nil {
			return err
		}
	}
	d.session.SetState(domain.StateStopped)
	d.session.SetCurrentClip(1)
	return nil
}

func (d *VirtualDeck) step(key domain.KeyName, delta int) error {
	p, w, ok := d.session.Active()
	if !ok {
		return ErrNotLocked
	}
	n := clamp(d.session.CurrentClip()+delta, 1, max(1, d.session.Clips().Len()))
	if err := d.send(p, w, key); err != nil {
		return err
	}
	d.session.SetCurrentClip(n)
	return nil
}

func (d *VirtualDeck) send(p domain.Profile, w domain.Window, key domain.KeyName) error {
	chord, ok := p.Keymap[key]
	if !ok {
		return nil // unmapped action -> acked no-op
	}
	if p.Injection == domain.InjectionFocus {
		if err := d.injector.Focus(w); err != nil {
			return err
		}
	}
	return d.injector.SendKeys(w, []domain.Chord{chord})
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
```

> Note: `max` is a Go 1.21+ builtin. If the toolchain is older, add a local `func max(a, b int) int`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/core/app/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/app/virtualdeck.go internal/core/app/virtualdeck_test.go
git commit -m "feat(core): virtualdeck transport with toggle and goto-delta"
```

---

### Task 8: VirtualDeck — query surface

**Files:**
- Modify: `internal/core/app/virtualdeck.go`
- Test: `internal/core/app/virtualdeck_query_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/core/app/virtualdeck_query_test.go`:
```go
package app

import (
	"testing"

	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/injector"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

func TestTransportInfoReflectsState(t *testing.T) {
	m := injector.NewMock()
	s := lockedSession(discreteProfile(), domain.ClipList{{ID: 1}, {ID: 2}})
	d := NewVirtualDeck(s, m)
	_ = d.Play()
	ti := d.TransportInfo()
	if ti.Status != "play" {
		t.Errorf("status = %q, want play", ti.Status)
	}
}

func TestSlotInfoTracksLock(t *testing.T) {
	m := injector.NewMock()
	s := NewSession()
	d := NewVirtualDeck(s, m)
	if d.SlotInfo().Present {
		t.Error("unlocked slot should be absent")
	}
	s.Lock(discreteProfile(), domain.Window{}, nil, nil)
	if !d.SlotInfo().Present {
		t.Error("locked slot should be present")
	}
}

func TestDeviceInfoStable(t *testing.T) {
	d := NewVirtualDeck(NewSession(), injector.NewMock())
	if d.DeviceInfo().Model == "" || d.DeviceInfo().ProtocolVersion == "" {
		t.Error("device info must be populated")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/app/ -run 'TestTransportInfo|TestSlotInfo|TestDeviceInfo' -v`
Expected: FAIL — undefined methods.

- [ ] **Step 3: Add query methods to `virtualdeck.go`**

Append to `internal/core/app/virtualdeck.go`:
```go

// TransportInfo reports the modeled transport state.
func (d *VirtualDeck) TransportInfo() domain.TransportInfo {
	speed := 0
	if d.session.State() == domain.StatePlaying {
		speed = 100
	}
	return domain.TransportInfo{
		Status: d.session.State().HyperDeckStatus(),
		Speed:  speed,
		ClipID: d.session.CurrentClip(),
		SlotID: 1,
	}
}

// Clips returns the active clip list.
func (d *VirtualDeck) Clips() domain.ClipList {
	return d.session.Clips()
}

// SlotInfo reports a present slot when a player is locked.
func (d *VirtualDeck) SlotInfo() domain.SlotInfo {
	_, _, ok := d.session.Active()
	return domain.SlotInfo{Present: ok, SlotID: 1}
}

// DeviceInfo reports the emulated deck identity.
func (d *VirtualDeck) DeviceInfo() domain.DeviceInfo {
	return d.device
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/core/app/ -v`
Expected: PASS.

- [ ] **Step 5: Verify VirtualDeck satisfies both ports — add a compile-time assertion**

Append to `internal/core/app/virtualdeck.go`:
```go

var (
	_ port.Transport = (*VirtualDeck)(nil)
	_ port.Query     = (*VirtualDeck)(nil)
)
```

Run: `go build ./...`
Expected: compiles (proves interface conformance).

- [ ] **Step 6: Commit**

```bash
git add internal/core/app/virtualdeck.go internal/core/app/virtualdeck_query_test.go
git commit -m "feat(core): virtualdeck query surface"
```

---

### Task 9: LockManager and Reconciler

`LockManager` enumerates windows, matches a profile, and locks/unlocks via the `Session`, notifying the `StatusPresenter`. It builds the clip source and state probe through injected factories. `Reconciler` refreshes clips and corrects modeled state on each clock tick.

**Files:**
- Create: `internal/core/app/lockmanager.go`, `internal/core/app/reconciler.go`
- Test: `internal/core/app/lockmanager_test.go`, `internal/core/app/reconciler_test.go`

- [ ] **Step 1: Write the failing LockManager test**

Create `internal/core/app/lockmanager_test.go`:
```go
package app

import (
	"testing"

	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/injector"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

type fakePresenter struct{ last domain.LockState }

func (f *fakePresenter) Present(l domain.LockState) { f.last = l }

type fakeClipSource struct{ clips domain.ClipList }

func (f fakeClipSource) List() (domain.ClipList, error) { return f.clips, nil }

func TestLockManagerLocksOnMatch(t *testing.T) {
	m := injector.NewMock()
	m.Windows = []domain.Window{{Process: "vlc.exe", Title: "x - VLC media player"}}
	s := NewSession()
	pres := &fakePresenter{}
	profiles := []domain.Profile{vlcProfileForLock()}
	csFactory := func(domain.Profile) port.ClipSource { return fakeClipSource{} }
	spFactory := func(domain.Profile) port.StateProbe { return noProbe{} }
	lm := NewLockManager(s, m, profiles, pres, csFactory, spFactory)

	lm.Poll()

	if _, _, ok := s.Active(); !ok {
		t.Fatal("expected lock after matching poll")
	}
	if !pres.last.Locked {
		t.Error("presenter should have been notified of lock")
	}
}

func TestLockManagerUnlocksWhenGone(t *testing.T) {
	m := injector.NewMock()
	m.Windows = []domain.Window{{Process: "vlc.exe", Title: "x - VLC media player"}}
	s := NewSession()
	pres := &fakePresenter{}
	lm := NewLockManager(s, m, []domain.Profile{vlcProfileForLock()}, pres,
		func(domain.Profile) port.ClipSource { return fakeClipSource{} },
		func(domain.Profile) port.StateProbe { return noProbe{} })
	lm.Poll()
	m.Windows = nil // player closed
	lm.Poll()
	if _, _, ok := s.Active(); ok {
		t.Error("expected unlock after player disappears")
	}
	if pres.last.Locked {
		t.Error("presenter should reflect unlock")
	}
}

func vlcProfileForLock() domain.Profile {
	return domain.Profile{
		ID:    "vlc",
		Match: domain.Match{Process: []string{"vlc.exe"}, TitleRegex: "VLC media player"},
		Keymap: domain.Keymap{domain.KeyPlay: {Key: "space"}},
	}
}

type noProbe struct{}

func (noProbe) Detect(domain.Window) (domain.TransportState, bool) {
	return domain.StateStopped, false
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/app/ -run TestLockManager -v`
Expected: FAIL — undefined `NewLockManager`.

- [ ] **Step 3: Write LockManager**

Create `internal/core/app/lockmanager.go`:
```go
package app

import (
	"time"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

// ClipSourceFactory builds a clip source for a profile (provided by the composition root).
type ClipSourceFactory func(domain.Profile) port.ClipSource

// StateProbeFactory builds a state probe for a profile.
type StateProbeFactory func(domain.Profile) port.StateProbe

// LockManager binds a running player to the session by matching profiles.
type LockManager struct {
	session   *Session
	windows   port.WindowEnumerator
	profiles  []domain.Profile
	presenter port.StatusPresenter
	clipsFor  ClipSourceFactory
	probeFor  StateProbeFactory
}

// NewLockManager wires a lock manager.
func NewLockManager(
	s *Session,
	w port.WindowEnumerator,
	profiles []domain.Profile,
	presenter port.StatusPresenter,
	clipsFor ClipSourceFactory,
	probeFor StateProbeFactory,
) *LockManager {
	return &LockManager{s, w, profiles, presenter, clipsFor, probeFor}
}

// Poll runs one match cycle: lock on first match, unlock when the locked window is gone.
func (lm *LockManager) Poll() {
	windows, err := lm.windows.OpenWindows()
	if err != nil {
		windows = nil
	}
	if profile, win, ok := lm.firstMatch(windows); ok {
		if cur, _, locked := lm.session.Active(); !locked || cur.ID != profile.ID {
			lm.session.Lock(profile, win, lm.clipsFor(profile), lm.probeFor(profile))
			lm.presenter.Present(lm.session.LockState())
		}
		return
	}
	if _, _, locked := lm.session.Active(); locked {
		lm.session.Unlock()
		lm.presenter.Present(lm.session.LockState())
	}
}

// Run polls on every clock tick until the channel closes.
func (lm *LockManager) Run(clock port.Clock, every time.Duration) {
	for range clock.Tick(every) {
		lm.Poll()
	}
}

func (lm *LockManager) firstMatch(windows []domain.Window) (domain.Profile, domain.Window, bool) {
	for _, p := range lm.profiles {
		for _, w := range windows {
			if p.MatchesWindow(w) {
				return p, w, true
			}
		}
	}
	return domain.Profile{}, domain.Window{}, false
}
```

- [ ] **Step 4: Run LockManager test to verify it passes**

Run: `go test ./internal/core/app/ -run TestLockManager -v`
Expected: PASS.

- [ ] **Step 5: Write the failing Reconciler test**

Create `internal/core/app/reconciler_test.go`:
```go
package app

import (
	"testing"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

type playingProbe struct{}

func (playingProbe) Detect(domain.Window) (domain.TransportState, bool) {
	return domain.StatePlaying, true
}

func TestReconcilerRefreshesClipsAndState(t *testing.T) {
	s := NewSession()
	s.Lock(discreteProfile(), domain.Window{}, fakeClipSource{clips: domain.ClipList{{ID: 1, Name: "a"}}}, playingProbe{})
	r := NewReconciler(s)
	r.Tick()
	if s.Clips().Len() != 1 {
		t.Errorf("clips not refreshed: %v", s.Clips())
	}
	if s.State() != domain.StatePlaying {
		t.Errorf("state not corrected to playing")
	}
}

func TestReconcilerNoopWhenUnlocked(t *testing.T) {
	s := NewSession()
	r := NewReconciler(s)
	r.Tick() // must not panic with nil clip source / probe
}
```

- [ ] **Step 6: Run Reconciler test to verify it fails**

Run: `go test ./internal/core/app/ -run TestReconciler -v`
Expected: FAIL — undefined `NewReconciler`.

- [ ] **Step 7: Write Reconciler**

Create `internal/core/app/reconciler.go`:
```go
package app

import (
	"time"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

// Reconciler refreshes the clip list and corrects modeled state from the probe.
type Reconciler struct {
	session *Session
}

// NewReconciler wires a reconciler to the session.
func NewReconciler(s *Session) *Reconciler {
	return &Reconciler{session: s}
}

// Tick runs one reconciliation cycle. Safe to call when unlocked.
func (r *Reconciler) Tick() {
	_, w, ok := r.session.Active()
	if !ok {
		return
	}
	if cs := r.session.ClipSource(); cs != nil {
		if clips, err := cs.List(); err == nil {
			r.session.SetClips(clips)
		}
	}
	if probe := r.session.Probe(); probe != nil {
		if state, detected := probe.Detect(w); detected {
			r.session.SetState(state)
		}
	}
}

// Run reconciles on every clock tick until the channel closes.
func (r *Reconciler) Run(clock port.Clock, every time.Duration) {
	for range clock.Tick(every) {
		r.Tick()
	}
}
```

- [ ] **Step 8: Run all core tests**

Run: `go test ./internal/core/... -race -v`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/core/app/lockmanager.go internal/core/app/reconciler.go internal/core/app/lockmanager_test.go internal/core/app/reconciler_test.go
git commit -m "feat(core): lock manager and reconciler"
```

---

## Phase 4 — Config, clipsource, stateprobe adapters

### Task 10: Config ProfileStore (YAML)

**Files:**
- Create: `internal/adapter/driven/config/store.go`, `testdata/profiles.yaml`
- Test: `internal/adapter/driven/config/store_test.go`

- [ ] **Step 1: Add the YAML dependency**

Run: `go get gopkg.in/yaml.v3@latest`
Expected: updates `go.mod`/`go.sum`.

- [ ] **Step 2: Create the test fixture**

Create `testdata/profiles.yaml`:
```yaml
profiles:
  - id: vlc
    match: { process: ["vlc.exe", "VLC"], title_regex: "VLC media player" }
    injection: background
    keymap: { play: "space", stop: "s", next: "n", prev: "p" }
    play_stop_toggle: false
    clip_source: { type: playlist_file, path: "/tmp/playlist.m3u" }
    state: { type: title_regex, playing: ".+ - VLC" }
    homing: ["s"]
  - id: example_player
    match: { process: ["Example Player"] }
    injection: focus
    keymap: { play: "space", next: "ctrl+right", prev: "ctrl+left" }
    play_stop_toggle: true
    clip_source: { type: positional, count: 50 }
    state: { type: none }
```

- [ ] **Step 3: Write the failing test**

Create `internal/adapter/driven/config/store_test.go`:
```go
package config

import (
	"testing"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

func TestLoadProfiles(t *testing.T) {
	profiles, err := NewStore("../../../../testdata/profiles.yaml").Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 {
		t.Fatalf("got %d profiles", len(profiles))
	}
	vlc := profiles[0]
	if vlc.ID != "vlc" || vlc.Injection != domain.InjectionBackground {
		t.Errorf("vlc profile wrong: %+v", vlc)
	}
	if vlc.Keymap[domain.KeyNext].Key != "n" {
		t.Errorf("vlc next key = %+v", vlc.Keymap[domain.KeyNext])
	}
	example := profiles[1]
	if !example.PlayStopToggle {
		t.Error("example should be play_stop_toggle")
	}
	if example.Keymap[domain.KeyNext].Key != "right" || len(example.Keymap[domain.KeyNext].Mods) != 1 {
		t.Errorf("example next chord = %+v", example.Keymap[domain.KeyNext])
	}
}

func TestLoadRejectsMissingPlayKey(t *testing.T) {
	_, err := loadBytes([]byte(`profiles:
  - id: bad
    match: { process: ["x"] }
    injection: focus
    keymap: { next: "n" }
`))
	if err == nil {
		t.Fatal("expected validation error for missing play key")
	}
}

func TestLoadRejectsBadInjection(t *testing.T) {
	_, err := loadBytes([]byte(`profiles:
  - id: bad
    match: { process: ["x"] }
    injection: telepathy
    keymap: { play: "space" }
`))
	if err == nil {
		t.Fatal("expected validation error for bad injection mode")
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/adapter/driven/config/ -v`
Expected: FAIL — undefined `NewStore`, `loadBytes`.

- [ ] **Step 5: Write the store**

Create `internal/adapter/driven/config/store.go`:
```go
// Package config implements port.ProfileStore over a YAML file.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

// Store loads profiles from a YAML file path.
type Store struct{ path string }

// NewStore returns a ProfileStore reading from path.
func NewStore(path string) *Store { return &Store{path: path} }

// Load reads and validates the profile file.
func (s *Store) Load() ([]domain.Profile, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", s.path, err)
	}
	return loadBytes(data)
}

type fileSchema struct {
	Profiles []profileSchema `yaml:"profiles"`
}

type profileSchema struct {
	ID        string            `yaml:"id"`
	Match     matchSchema       `yaml:"match"`
	Injection string            `yaml:"injection"`
	Keymap    map[string]string `yaml:"keymap"`
	Toggle    bool              `yaml:"play_stop_toggle"`
	Clip      clipSchema        `yaml:"clip_source"`
	State     stateSchema       `yaml:"state"`
	Homing    []string          `yaml:"homing"`
}

type matchSchema struct {
	Process    []string `yaml:"process"`
	TitleRegex string   `yaml:"title_regex"`
}

type clipSchema struct {
	Type  string `yaml:"type"`
	Path  string `yaml:"path"`
	Count int    `yaml:"count"`
}

type stateSchema struct {
	Type    string `yaml:"type"`
	Playing string `yaml:"playing"`
}

func loadBytes(data []byte) ([]domain.Profile, error) {
	var f fileSchema
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	profiles := make([]domain.Profile, 0, len(f.Profiles))
	for _, ps := range f.Profiles {
		p, err := convert(ps)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	return profiles, nil
}

func convert(ps profileSchema) (domain.Profile, error) {
	if ps.ID == "" {
		return domain.Profile{}, fmt.Errorf("profile missing id")
	}
	mode := domain.InjectionMode(ps.Injection)
	if mode != domain.InjectionFocus && mode != domain.InjectionBackground {
		return domain.Profile{}, fmt.Errorf("profile %q: invalid injection %q (want focus|background)", ps.ID, ps.Injection)
	}
	keymap := domain.Keymap{}
	for name, spec := range ps.Keymap {
		chord, err := domain.ParseChord(spec)
		if err != nil {
			return domain.Profile{}, fmt.Errorf("profile %q key %q: %w", ps.ID, name, err)
		}
		keymap[domain.KeyName(name)] = chord
	}
	if _, ok := keymap[domain.KeyPlay]; !ok {
		return domain.Profile{}, fmt.Errorf("profile %q: missing required 'play' key", ps.ID)
	}
	var homing []domain.Chord
	for _, spec := range ps.Homing {
		chord, err := domain.ParseChord(spec)
		if err != nil {
			return domain.Profile{}, fmt.Errorf("profile %q homing %q: %w", ps.ID, spec, err)
		}
		homing = append(homing, chord)
	}
	return domain.Profile{
		ID:             ps.ID,
		Match:          domain.Match{Process: ps.Match.Process, TitleRegex: ps.Match.TitleRegex},
		Injection:      mode,
		Keymap:         keymap,
		PlayStopToggle: ps.Toggle,
		ClipSource:     domain.ClipSourceConfig{Type: ps.Clip.Type, Path: ps.Clip.Path, Count: ps.Clip.Count},
		State:          domain.StateConfig{Type: ps.State.Type, Playing: ps.State.Playing},
		Homing:         homing,
	}, nil
}

var _ port.ProfileStore = (*Store)(nil)
```

> Add the import `"github.com/kindlyops/hyperdeck-adapter/internal/core/port"` to `store.go`'s import block for the assertion on the last line.

- [ ] **Step 6: Verify the import and assertion compile**

Run: `go build ./internal/adapter/driven/config/`
Expected: compiles — the `var _ port.ProfileStore = (*Store)(nil)` line proves `Store` satisfies the port.

- [ ] **Step 7: Run tests to verify they pass**

Run: `go test ./internal/adapter/driven/config/ -v`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/adapter/driven/config testdata/profiles.yaml go.mod go.sum
git commit -m "feat(config): yaml profile store with validation"
```

---

### Task 11: ClipSource adapters (playlist, positional, mitti) + factory

**Files:**
- Create: `internal/adapter/driven/clipsource/playlist.go`, `positional.go`, `mitti.go`, `factory.go`
- Create: `testdata/sample.m3u`, `testdata/sample.xspf`
- Test: `internal/adapter/driven/clipsource/clipsource_test.go`

- [ ] **Step 1: Create fixtures**

Create `testdata/sample.m3u`:
```
#EXTM3U
#EXTINF:123,Intro Clip
/media/intro.mp4
#EXTINF:200,Main Segment
/media/main.mp4
```

Create `testdata/sample.xspf`:
```xml
<?xml version="1.0" encoding="UTF-8"?>
<playlist version="1" xmlns="http://xspf.org/ns/0/">
  <trackList>
    <track><title>Intro Clip</title><location>file:///media/intro.mp4</location></track>
    <track><title>Main Segment</title><location>file:///media/main.mp4</location></track>
  </trackList>
</playlist>
```

- [ ] **Step 2: Write the failing test**

Create `internal/adapter/driven/clipsource/clipsource_test.go`:
```go
package clipsource

import (
	"testing"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

func TestPlaylistM3U(t *testing.T) {
	clips, err := NewPlaylist("../../../../testdata/sample.m3u").List()
	if err != nil {
		t.Fatal(err)
	}
	if len(clips) != 2 || clips[0].Name != "Intro Clip" || clips[0].ID != 1 {
		t.Errorf("m3u clips = %+v", clips)
	}
}

func TestPlaylistXSPF(t *testing.T) {
	clips, err := NewPlaylist("../../../../testdata/sample.xspf").List()
	if err != nil {
		t.Fatal(err)
	}
	if len(clips) != 2 || clips[1].Name != "Main Segment" {
		t.Errorf("xspf clips = %+v", clips)
	}
}

func TestPositional(t *testing.T) {
	clips, err := NewPositional(3).List()
	if err != nil {
		t.Fatal(err)
	}
	if len(clips) != 3 || clips[2].ID != 3 || clips[2].Name != "Clip 3" {
		t.Errorf("positional clips = %+v", clips)
	}
}

func TestFactory(t *testing.T) {
	p := domain.Profile{ClipSource: domain.ClipSourceConfig{Type: "positional", Count: 2}}
	cs := New(p)
	clips, _ := cs.List()
	if len(clips) != 2 {
		t.Errorf("factory positional = %+v", clips)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/adapter/driven/clipsource/ -v`
Expected: FAIL — undefined `NewPlaylist`, `NewPositional`, `New`.

- [ ] **Step 4: Write the playlist adapter**

Create `internal/adapter/driven/clipsource/playlist.go`:
```go
// Package clipsource implements port.ClipSource strategies.
package clipsource

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

// Playlist reads clip names from an .m3u or .xspf file.
type Playlist struct{ path string }

// NewPlaylist returns a playlist-backed clip source.
func NewPlaylist(path string) *Playlist { return &Playlist{path: path} }

// List parses the playlist file into a clip list.
func (p *Playlist) List() (domain.ClipList, error) {
	switch strings.ToLower(filepath.Ext(p.path)) {
	case ".xspf":
		return p.listXSPF()
	default:
		return p.listM3U()
	}
}

func (p *Playlist) listM3U() (domain.ClipList, error) {
	f, err := os.Open(p.path)
	if err != nil {
		return nil, fmt.Errorf("open playlist %q: %w", p.path, err)
	}
	defer f.Close()

	var clips domain.ClipList
	name := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(line, "#EXTINF:"):
			if comma := strings.Index(line, ","); comma >= 0 {
				name = strings.TrimSpace(line[comma+1:])
			}
		case line == "" || strings.HasPrefix(line, "#"):
			// skip directives and blanks
		default:
			label := name
			if label == "" {
				label = filepath.Base(line)
			}
			clips = append(clips, domain.Clip{ID: len(clips) + 1, Name: label})
			name = ""
		}
	}
	return clips, sc.Err()
}

type xspfFile struct {
	Tracks []struct {
		Title    string `xml:"title"`
		Location string `xml:"location"`
	} `xml:"trackList>track"`
}

func (p *Playlist) listXSPF() (domain.ClipList, error) {
	data, err := os.ReadFile(p.path)
	if err != nil {
		return nil, fmt.Errorf("read playlist %q: %w", p.path, err)
	}
	var parsed xspfFile
	if err := xml.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("parse xspf %q: %w", p.path, err)
	}
	var clips domain.ClipList
	for _, tr := range parsed.Tracks {
		name := tr.Title
		if name == "" {
			name = filepath.Base(tr.Location)
		}
		clips = append(clips, domain.Clip{ID: len(clips) + 1, Name: name})
	}
	return clips, nil
}
```

- [ ] **Step 5: Write positional, mitti, and factory**

Create `internal/adapter/driven/clipsource/positional.go`:
```go
package clipsource

import (
	"fmt"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

// Positional produces a fixed number of generic clip slots.
type Positional struct{ count int }

// NewPositional returns a positional clip source with n slots.
func NewPositional(n int) *Positional { return &Positional{count: n} }

// List returns n generically-named clips.
func (p *Positional) List() (domain.ClipList, error) {
	clips := make(domain.ClipList, 0, p.count)
	for i := 1; i <= p.count; i++ {
		clips = append(clips, domain.Clip{ID: i, Name: fmt.Sprintf("Clip %d", i)})
	}
	return clips, nil
}
```

Create `internal/adapter/driven/clipsource/mitti.go`:
```go
package clipsource

import "github.com/kindlyops/hyperdeck-adapter/internal/core/domain"

// Mitti is a best-effort clip source for Mitti. Until the proprietary playlist
// format is parsed, it falls back to a positional list of the configured size.
// Tracked as an open item in the design spec.
type Mitti struct{ fallback *Positional }

// NewMitti returns a Mitti clip source with a positional fallback of n slots.
func NewMitti(n int) *Mitti { return &Mitti{fallback: NewPositional(n)} }

// List returns the best-effort clip list.
func (m *Mitti) List() (domain.ClipList, error) { return m.fallback.List() }
```

Create `internal/adapter/driven/clipsource/factory.go`:
```go
package clipsource

import (
	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

// New builds the clip source named by the profile's clip_source.type.
func New(p domain.Profile) port.ClipSource {
	cfg := p.ClipSource
	switch cfg.Type {
	case "playlist_file":
		return NewPlaylist(cfg.Path)
	case "mitti":
		return NewMitti(defaultCount(cfg.Count))
	default: // "positional" and unknown -> positional
		return NewPositional(defaultCount(cfg.Count))
	}
}

func defaultCount(n int) int {
	if n <= 0 {
		return 1
	}
	return n
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/adapter/driven/clipsource/ -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/adapter/driven/clipsource testdata/sample.m3u testdata/sample.xspf
git commit -m "feat(clipsource): playlist, positional, mitti, factory"
```

---

### Task 12: StateProbe adapters (title regex, none) + factory

**Files:**
- Create: `internal/adapter/driven/stateprobe/titleregex.go`, `none.go`, `factory.go`
- Test: `internal/adapter/driven/stateprobe/stateprobe_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/adapter/driven/stateprobe/stateprobe_test.go`:
```go
package stateprobe

import (
	"testing"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

func TestTitleRegexDetectsPlaying(t *testing.T) {
	p := NewTitleRegex(".+ - VLC")
	st, ok := p.Detect(domain.Window{Title: "Movie - VLC"})
	if !ok || st != domain.StatePlaying {
		t.Errorf("got %v %v", st, ok)
	}
	st, ok = p.Detect(domain.Window{Title: "idle"})
	if !ok || st != domain.StateStopped {
		t.Errorf("non-match should report stopped+detected; got %v %v", st, ok)
	}
}

func TestNoneNeverDetects(t *testing.T) {
	_, ok := None{}.Detect(domain.Window{Title: "x"})
	if ok {
		t.Error("none probe must not claim detection")
	}
}

func TestFactory(t *testing.T) {
	none := New(domain.Profile{State: domain.StateConfig{Type: "none"}})
	if _, ok := none.Detect(domain.Window{}); ok {
		t.Error("factory none should not detect")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapter/driven/stateprobe/ -v`
Expected: FAIL — undefined identifiers.

- [ ] **Step 3: Write the adapters**

Create `internal/adapter/driven/stateprobe/titleregex.go`:
```go
// Package stateprobe implements port.StateProbe strategies.
package stateprobe

import (
	"regexp"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

// TitleRegex infers playing state from the window title.
type TitleRegex struct{ re *regexp.Regexp }

// NewTitleRegex compiles pattern; an invalid pattern yields a probe that never detects.
func NewTitleRegex(pattern string) *TitleRegex {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return &TitleRegex{re: nil}
	}
	return &TitleRegex{re: re}
}

// Detect reports playing when the title matches, stopped when it does not.
func (t *TitleRegex) Detect(w domain.Window) (domain.TransportState, bool) {
	if t.re == nil {
		return domain.StateStopped, false
	}
	if t.re.MatchString(w.Title) {
		return domain.StatePlaying, true
	}
	return domain.StateStopped, true
}
```

Create `internal/adapter/driven/stateprobe/none.go`:
```go
package stateprobe

import "github.com/kindlyops/hyperdeck-adapter/internal/core/domain"

// None performs no detection; the modeled state is authoritative.
type None struct{}

// Detect always reports not-detected.
func (None) Detect(domain.Window) (domain.TransportState, bool) {
	return domain.StateStopped, false
}
```

Create `internal/adapter/driven/stateprobe/factory.go`:
```go
package stateprobe

import (
	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

// New builds the state probe named by the profile's state.type.
func New(p domain.Profile) port.StateProbe {
	if p.State.Type == "title_regex" {
		return NewTitleRegex(p.State.Playing)
	}
	return None{}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/driven/stateprobe/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/driven/stateprobe
git commit -m "feat(stateprobe): title-regex and none probes"
```

---

## Phase 5 — HyperDeck protocol driving adapter

### Task 13: Protocol codes and command parser

The wire protocol: commands are newline-terminated; a command with parameters has a trailing colon on the first line followed by `key: value` lines and a blank line. Responses: `<code> <text>`; multi-line responses end the first line with `:` and terminate on a blank line.

**Files:**
- Create: `internal/adapter/driving/hyperdeck/codes.go`, `parser.go`
- Test: `internal/adapter/driving/hyperdeck/parser_test.go`

> **Before coding:** confirm the exact response-code numbers and the `device info`/`transport info` field names against the HyperDeck "Ethernet Protocol" section of the HyperDeck Studio manual (the SDK zip ships only the intro deck). The codes below match the published protocol (v1.11) and the SDK reference client (`transport info` = 208, `clips get` = 205). If the manual differs, change only `codes.go` and the field names in `responder.go` (Task 14).

- [ ] **Step 1: Write the failing parser test**

Create `internal/adapter/driving/hyperdeck/parser_test.go`:
```go
package hyperdeck

import (
	"reflect"
	"testing"
)

func TestParseSimpleCommand(t *testing.T) {
	cmd, err := ParseCommand("play\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Name != "play" || len(cmd.Params) != 0 {
		t.Errorf("got %+v", cmd)
	}
}

func TestParseCommandWithParams(t *testing.T) {
	raw := "play:\r\nsingle clip: true\r\nspeed: 100\r\n\r\n"
	cmd, err := ParseCommand(raw)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"single clip": "true", "speed": "100"}
	if cmd.Name != "play" || !reflect.DeepEqual(cmd.Params, want) {
		t.Errorf("got %+v", cmd)
	}
}

func TestParseGoto(t *testing.T) {
	cmd, err := ParseCommand("goto: clip id: 3\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Name != "goto" || cmd.Params["clip id"] != "3" {
		t.Errorf("got %+v", cmd)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapter/driving/hyperdeck/ -run TestParse -v`
Expected: FAIL — undefined `ParseCommand`.

- [ ] **Step 3: Write codes and parser**

Create `internal/adapter/driving/hyperdeck/codes.go`:
```go
// Package hyperdeck implements the HyperDeck Ethernet Protocol as a driving adapter.
package hyperdeck

// Response codes (HyperDeck Ethernet Protocol v1.11). Verify against the manual.
const (
	CodeOK             = 200
	CodeSlotInfo       = 202
	CodeDeviceInfo     = 204
	CodeClipsInfo      = 205
	CodeTransportInfo  = 208
	CodeConnectionInfo = 500
	CodeSyntaxError    = 100
	CodeInvalidState   = 150
)
```

Create `internal/adapter/driving/hyperdeck/parser.go`:
```go
package hyperdeck

import (
	"fmt"
	"strings"
)

// Command is a parsed HyperDeck request.
type Command struct {
	Name   string
	Params map[string]string
}

// ParseCommand parses one full command block (single- or multi-line).
func ParseCommand(raw string) (Command, error) {
	lines := splitLines(raw)
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return Command{}, fmt.Errorf("empty command")
	}
	cmd := Command{Params: map[string]string{}}
	first := strings.TrimSpace(lines[0])

	// Inline form: "goto: clip id: 3" -> name "goto", param "clip id"="3".
	if name, rest, ok := strings.Cut(first, ":"); ok {
		cmd.Name = strings.TrimSpace(name)
		rest = strings.TrimSpace(rest)
		if rest != "" {
			if k, v, ok := strings.Cut(rest, ":"); ok {
				cmd.Params[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		}
	} else {
		cmd.Name = first
	}

	// Block form: subsequent "key: value" lines until a blank line.
	for _, line := range lines[1:] {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			break
		}
		if k, v, ok := strings.Cut(line, ":"); ok {
			cmd.Params[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return cmd, nil
}

func splitLines(raw string) []string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	return strings.Split(raw, "\n")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/driving/hyperdeck/ -run TestParse -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/driving/hyperdeck/codes.go internal/adapter/driving/hyperdeck/parser.go internal/adapter/driving/hyperdeck/parser_test.go
git commit -m "feat(hyperdeck): command parser and response codes"
```

---

### Task 14: Protocol responder (command → port calls → response text)

**Files:**
- Create: `internal/adapter/driving/hyperdeck/responder.go`
- Test: `internal/adapter/driving/hyperdeck/responder_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/adapter/driving/hyperdeck/responder_test.go`:
```go
package hyperdeck

import (
	"strings"
	"testing"

	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/injector"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/app"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

func newTestDeck() *app.VirtualDeck {
	s := app.NewSession()
	s.Lock(domain.Profile{
		ID:        "vlc",
		Injection: domain.InjectionBackground,
		Keymap:    domain.Keymap{domain.KeyPlay: {Key: "space"}, domain.KeyStop: {Key: "s"}},
	}, domain.Window{}, nil, nil)
	s.SetClips(domain.ClipList{{ID: 1, Name: "Intro", Timecode: "00:00:00:00", Duration: "00:00:10:00"}})
	return app.NewVirtualDeck(s, injector.NewMock())
}

func TestRespondPlayAck(t *testing.T) {
	d := newTestDeck()
	r := NewResponder(d, d)
	out := r.Handle(Command{Name: "play"})
	if !strings.HasPrefix(out, "200 ok") {
		t.Errorf("play response = %q", out)
	}
}

func TestRespondTransportInfo(t *testing.T) {
	d := newTestDeck()
	r := NewResponder(d, d)
	_ = r.Handle(Command{Name: "play"})
	out := r.Handle(Command{Name: "transport info"})
	if !strings.HasPrefix(out, "208 transport info:") {
		t.Errorf("transport info head = %q", out)
	}
	if !strings.Contains(out, "status: play") {
		t.Errorf("transport info body missing status: %q", out)
	}
	if !strings.HasSuffix(out, "\r\n\r\n") {
		t.Errorf("multi-line response must end with blank line: %q", out)
	}
}

func TestRespondClips(t *testing.T) {
	d := newTestDeck()
	r := NewResponder(d, d)
	out := r.Handle(Command{Name: "clips get"})
	if !strings.HasPrefix(out, "205 clips info:") || !strings.Contains(out, "Intro") {
		t.Errorf("clips response = %q", out)
	}
}

func TestRespondUnknownCommand(t *testing.T) {
	d := newTestDeck()
	r := NewResponder(d, d)
	out := r.Handle(Command{Name: "frobnicate"})
	if !strings.HasPrefix(out, "100 ") {
		t.Errorf("unknown command should be 1xx: %q", out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapter/driving/hyperdeck/ -run TestRespond -v`
Expected: FAIL — undefined `NewResponder`.

- [ ] **Step 3: Write the responder**

Create `internal/adapter/driving/hyperdeck/responder.go`:
```go
package hyperdeck

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

// Responder turns parsed commands into port calls and formatted responses.
type Responder struct {
	transport port.Transport
	query     port.Query
}

// NewResponder wires a responder to the inbound ports.
func NewResponder(t port.Transport, q port.Query) *Responder {
	return &Responder{transport: t, query: q}
}

// Handle executes one command and returns the full response text.
func (r *Responder) Handle(cmd Command) string {
	switch cmd.Name {
	case "ping":
		return ack()
	case "play":
		return r.ackErr(r.transport.Play())
	case "stop":
		return r.ackErr(r.transport.Stop())
	case "record":
		return r.ackErr(r.transport.Record())
	case "goto":
		return r.handleGoto(cmd)
	case "transport info":
		return r.transportInfo()
	case "clips get":
		return r.clips()
	case "slot info":
		return r.slotInfo()
	case "device info":
		return r.deviceInfo()
	case "notify", "remote", "configuration":
		return ack() // accepted; async emission handled by the server
	case "quit":
		return ack()
	default:
		return fmt.Sprintf("%d syntax error\r\n", CodeSyntaxError)
	}
}

func (r *Responder) handleGoto(cmd Command) string {
	idStr, ok := cmd.Params["clip id"]
	if !ok {
		return fmt.Sprintf("%d syntax error\r\n", CodeSyntaxError)
	}
	id, err := strconv.Atoi(strings.TrimPrefix(idStr, "+"))
	if err != nil {
		return fmt.Sprintf("%d syntax error\r\n", CodeSyntaxError)
	}
	return r.ackErr(r.transport.Goto(id))
}

func (r *Responder) transportInfo() string {
	ti := r.query.TransportInfo()
	var b strings.Builder
	fmt.Fprintf(&b, "%d transport info:\r\n", CodeTransportInfo)
	fmt.Fprintf(&b, "status: %s\r\n", ti.Status)
	fmt.Fprintf(&b, "speed: %d\r\n", ti.Speed)
	fmt.Fprintf(&b, "clip id: %d\r\n", ti.ClipID)
	fmt.Fprintf(&b, "slot id: %d\r\n", ti.SlotID)
	b.WriteString("\r\n")
	return b.String()
}

func (r *Responder) clips() string {
	clips := r.query.Clips()
	var b strings.Builder
	fmt.Fprintf(&b, "%d clips info:\r\n", CodeClipsInfo)
	fmt.Fprintf(&b, "clip count: %d\r\n", len(clips))
	for _, c := range clips {
		fmt.Fprintf(&b, "%d: %s %s %s\r\n", c.ID, c.Name, c.Timecode, c.Duration)
	}
	b.WriteString("\r\n")
	return b.String()
}

func (r *Responder) slotInfo() string {
	si := r.query.SlotInfo()
	status := "mounted"
	if !si.Present {
		status = "empty"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d slot info:\r\n", CodeSlotInfo)
	fmt.Fprintf(&b, "slot id: %d\r\n", si.SlotID)
	fmt.Fprintf(&b, "status: %s\r\n", status)
	b.WriteString("\r\n")
	return b.String()
}

func (r *Responder) deviceInfo() string {
	di := r.query.DeviceInfo()
	var b strings.Builder
	fmt.Fprintf(&b, "%d device info:\r\n", CodeDeviceInfo)
	fmt.Fprintf(&b, "protocol version: %s\r\n", di.ProtocolVersion)
	fmt.Fprintf(&b, "model: %s\r\n", di.Model)
	fmt.Fprintf(&b, "unique id: %s\r\n", di.UniqueID)
	b.WriteString("\r\n")
	return b.String()
}

func (r *Responder) ackErr(err error) string {
	if err != nil {
		return fmt.Sprintf("%d invalid state\r\n", CodeInvalidState)
	}
	return ack()
}

func ack() string { return fmt.Sprintf("%d ok\r\n", CodeOK) }
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/driving/hyperdeck/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/driving/hyperdeck/responder.go internal/adapter/driving/hyperdeck/responder_test.go
git commit -m "feat(hyperdeck): responder mapping commands to ports"
```

---

### Task 15: TCP server + end-to-end integration test

**Files:**
- Create: `internal/adapter/driving/hyperdeck/server.go`
- Test: `internal/adapter/driving/hyperdeck/server_test.go`

- [ ] **Step 1: Write the failing end-to-end test**

Create `internal/adapter/driving/hyperdeck/server_test.go`:
```go
package hyperdeck

import (
	"bufio"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/injector"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/app"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

func TestServerEndToEnd(t *testing.T) {
	mock := injector.NewMock()
	s := app.NewSession()
	s.Lock(domain.Profile{
		ID:        "vlc",
		Injection: domain.InjectionBackground,
		Keymap:    domain.Keymap{domain.KeyPlay: {Key: "space"}, domain.KeyStop: {Key: "s"}},
	}, domain.Window{Process: "vlc.exe"}, nil, nil)
	deck := app.NewVirtualDeck(s, mock)

	srv := NewServer(deck, deck)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(ln)
	defer ln.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	rd := bufio.NewReader(conn)

	// Server greets with a 500 connection info banner.
	banner, _ := rd.ReadString('\n')
	if !strings.HasPrefix(banner, "500 connection info:") {
		t.Errorf("banner = %q", banner)
	}
	drainBlank(rd)

	// Send "play" and expect "200 ok" + a recorded keystroke.
	conn.Write([]byte("play\r\n"))
	resp, _ := rd.ReadString('\n')
	if !strings.HasPrefix(resp, "200 ok") {
		t.Errorf("play resp = %q", resp)
	}
	// Allow the handler goroutine to record the keystroke.
	time.Sleep(50 * time.Millisecond)
	if len(mock.Sent) != 1 || mock.Sent[0].Chords[0].Key != "space" {
		t.Errorf("expected space keystroke, got %+v", mock.Sent)
	}
}

func drainBlank(rd *bufio.Reader) {
	for {
		line, err := rd.ReadString('\n')
		if err != nil || strings.TrimSpace(line) == "" {
			return
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapter/driving/hyperdeck/ -run TestServer -v`
Expected: FAIL — undefined `NewServer`.

- [ ] **Step 3: Write the server**

Create `internal/adapter/driving/hyperdeck/server.go`:
```go
package hyperdeck

import (
	"bufio"
	"fmt"
	"net"
	"strings"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

// Server accepts controller connections and serves the protocol.
type Server struct {
	responder *Responder
}

// NewServer wires a server to the inbound ports.
func NewServer(t port.Transport, q port.Query) *Server {
	return &Server{responder: NewResponder(t, q)}
}

// Serve accepts connections on ln until it is closed.
func (s *Server) Serve(ln net.Listener) error {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	// Greeting banner.
	fmt.Fprintf(conn, "%d connection info:\r\nprotocol version: 1.11\r\nmodel: HyperDeck Studio Mini\r\n\r\n", CodeConnectionInfo)

	rd := bufio.NewReader(conn)
	for {
		block, err := readCommandBlock(rd)
		if err != nil {
			return
		}
		if strings.TrimSpace(block) == "" {
			continue
		}
		cmd, perr := ParseCommand(block)
		if perr != nil {
			fmt.Fprintf(conn, "%d syntax error\r\n", CodeSyntaxError)
			continue
		}
		if _, err := conn.Write([]byte(s.responder.Handle(cmd))); err != nil {
			return
		}
		if cmd.Name == "quit" {
			return
		}
	}
}

// readCommandBlock reads one command: a single line, or — when the first line
// ends with ':' — lines up to and including a terminating blank line.
func readCommandBlock(rd *bufio.Reader) (string, error) {
	first, err := rd.ReadString('\n')
	if err != nil {
		return "", err
	}
	trimmed := strings.TrimRight(first, "\r\n")
	if !strings.HasSuffix(trimmed, ":") {
		return first, nil
	}
	var b strings.Builder
	b.WriteString(first)
	for {
		line, err := rd.ReadString('\n')
		if err != nil {
			return "", err
		}
		b.WriteString(line)
		if strings.TrimSpace(line) == "" {
			return b.String(), nil
		}
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/driving/hyperdeck/ -race -v`
Expected: PASS.

- [ ] **Step 5: Run the full suite**

Run: `go test ./... -race -count=1`
Expected: PASS across all packages.

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/driving/hyperdeck/server.go internal/adapter/driving/hyperdeck/server_test.go
git commit -m "feat(hyperdeck): tcp server and end-to-end test"
```

---

## Phase 6 — OS adapters (manually verified)

> These adapters call platform APIs that cannot be unit-tested on CI. Each implements a port already exercised by core tests, so correctness reduces to "does the real OS deliver the keystroke." Verification is the manual smoke checklist in Task 19. Keep these files small and free of logic — they are pure translation to syscalls.

### Task 16: Injector adapters (noop, windows, darwin)

**Files:**
- Create: `internal/adapter/driven/injector/injector_noop.go` (build tag `!windows && !darwin`)
- Create: `internal/adapter/driven/injector/injector_windows.go` (build tag `windows`)
- Create: `internal/adapter/driven/injector/injector_darwin.go` (build tag `darwin`)
- Test: `internal/adapter/driven/injector/injector_noop_test.go`

- [ ] **Step 1: Write the noop adapter and its test (runs on CI/Linux)**

Create `internal/adapter/driven/injector/injector_noop.go`:
```go
//go:build !windows && !darwin

package injector

import (
	"log/slog"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

// New returns the platform injector. On unsupported platforms it logs and no-ops,
// so the protocol server still runs for development against a controller.
func New() (Injector, error) {
	slog.Warn("key injection is not supported on this platform; running in no-op mode")
	return noop{}, nil
}

// Injector is the union of the two OS-facing driven ports.
type Injector interface {
	Focus(w domain.Window) error
	SendKeys(w domain.Window, chords []domain.Chord) error
	OpenWindows() ([]domain.Window, error)
}

type noop struct{}

func (noop) Focus(domain.Window) error                      { return nil }
func (noop) SendKeys(domain.Window, []domain.Chord) error   { return nil }
func (noop) OpenWindows() ([]domain.Window, error)          { return nil, nil }
```

Create `internal/adapter/driven/injector/injector_noop_test.go`:
```go
//go:build !windows && !darwin

package injector

import "testing"

func TestNoopInjectorSatisfiesInterface(t *testing.T) {
	inj, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if err := inj.SendKeys(injectorWindow(), nil); err != nil {
		t.Errorf("noop send should succeed: %v", err)
	}
}
```

Add a tiny test helper `internal/adapter/driven/injector/helpers_test.go`:
```go
package injector

import "github.com/kindlyops/hyperdeck-adapter/internal/core/domain"

func injectorWindow() domain.Window { return domain.Window{Process: "x"} }
```

> Also add a compile-time assertion that `*Mock` satisfies `Injector`. Append to `injector_mock.go`:
> ```go
> var _ Injector = (*Mock)(nil)
> ```

- [ ] **Step 2: Run the noop test**

Run: `go test ./internal/adapter/driven/injector/ -v`
Expected: PASS (on macOS/Linux).

- [ ] **Step 3: Write the Windows adapter**

Create `internal/adapter/driven/injector/injector_windows.go`:
```go
//go:build windows

package injector

import (
	"fmt"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
	"golang.org/x/sys/windows"
)

// Injector is the union of the two OS-facing driven ports.
type Injector interface {
	Focus(w domain.Window) error
	SendKeys(w domain.Window, chords []domain.Chord) error
	OpenWindows() ([]domain.Window, error)
}

// New returns the Windows injector.
func New() (Injector, error) { return &winInjector{}, nil }

type winInjector struct{}

// Focus brings the target window to the foreground (focus injection mode).
// Implementation: user32!SetForegroundWindow(HWND). See design risk note on
// focus stealing.
func (w *winInjector) Focus(win domain.Window) error {
	// TODO(windows-smoke): call SetForegroundWindow via golang.org/x/sys/windows.
	// Left as a single syscall; verified manually in Task 19.
	return fmt.Errorf("not implemented: build and verify on Windows")
}

func (w *winInjector) SendKeys(win domain.Window, chords []domain.Chord) error {
	// focus mode -> SendInput(INPUT_KEYBOARD ...) with VK codes + modifiers.
	// background mode -> PostMessageW(WM_KEYDOWN/WM_KEYUP) to the HWND.
	return fmt.Errorf("not implemented: build and verify on Windows")
}

func (w *winInjector) OpenWindows() ([]domain.Window, error) {
	// EnumWindows + GetWindowTextW + GetWindowThreadProcessId + module base name.
	return nil, fmt.Errorf("not implemented: build and verify on Windows")
}

var _ = windows.NewLazySystemDLL // anchor the dependency import
```

> The Windows syscall bodies are intentionally not pre-written as guessed code. Implementing them is a focused sub-task done **on a Windows machine** where each call is verified immediately (Task 19 checklist). Each method is a thin, logic-free translation: VK-code mapping table for chords, `SendInput` for focus mode, `PostMessageW` for background mode, `EnumWindows` for enumeration.

- [ ] **Step 4: Write the macOS adapter skeleton**

Create `internal/adapter/driven/injector/injector_darwin.go`:
```go
//go:build darwin

package injector

import (
	"fmt"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

// Injector is the union of the two OS-facing driven ports.
type Injector interface {
	Focus(w domain.Window) error
	SendKeys(w domain.Window, chords []domain.Chord) error
	OpenWindows() ([]domain.Window, error)
}

// New returns the macOS injector. Requires the Accessibility permission.
func New() (Injector, error) { return &macInjector{}, nil }

type macInjector struct{}

// Focus activates the target application (AX/NSRunningApplication activate).
func (m *macInjector) Focus(win domain.Window) error {
	return fmt.Errorf("not implemented: build and verify on macOS")
}

// SendKeys posts CGEvent keyboard events for each chord.
func (m *macInjector) SendKeys(win domain.Window, chords []domain.Chord) error {
	return fmt.Errorf("not implemented: build and verify on macOS")
}

// OpenWindows lists on-screen windows via CGWindowListCopyWindowInfo.
func (m *macInjector) OpenWindows() ([]domain.Window, error) {
	return nil, fmt.Errorf("not implemented: build and verify on macOS")
}
```

> The macOS bodies use cgo against CoreGraphics (`CGEventCreateKeyboardEvent`, `CGEventPost`) and `CGWindowListCopyWindowInfo`, plus an Accessibility-permission check (`AXIsProcessTrusted`). Written and verified **on a Mac** in Task 19; kept logic-free.

- [ ] **Step 5: Verify cross-compilation of the build-tagged files**

Run:
```bash
GOOS=windows GOARCH=amd64 go build ./internal/adapter/driven/injector/
GOOS=darwin  GOARCH=arm64 go build ./internal/adapter/driven/injector/
go test ./internal/adapter/driven/injector/ -v
```
Expected: all three compile; the noop test passes on the host.

- [ ] **Step 6: Add the Windows syscall dependency to go.mod**

Run: `go get golang.org/x/sys/windows@latest`
Expected: dependency recorded (used only by the `windows` build).

- [ ] **Step 7: Commit**

```bash
git add internal/adapter/driven/injector go.mod go.sum
git commit -m "feat(injector): noop + windows/darwin adapter skeletons"
```

---

### Task 17: Tray status presenter and menu

**Files:**
- Create: `internal/adapter/driven/tray/tray.go`
- Test: `internal/adapter/driven/tray/tray_test.go`

- [ ] **Step 1: Add the systray dependency**

Run: `go get fyne.io/systray@latest`
Expected: dependency recorded.

- [ ] **Step 2: Write the failing test for the icon-state mapping (pure, testable)**

Create `internal/adapter/driven/tray/tray_test.go`:
```go
package tray

import (
	"testing"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

func TestStatusText(t *testing.T) {
	if got := statusText(domain.LockState{Locked: false}); got != "Disconnected — no player" {
		t.Errorf("unlocked text = %q", got)
	}
	p := domain.Profile{ID: "vlc"}
	got := statusText(domain.LockState{Locked: true, Profile: &p, Window: domain.Window{Title: "Movie - VLC"}})
	if got == "" || got == "Disconnected — no player" {
		t.Errorf("locked text = %q", got)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/adapter/driven/tray/ -v`
Expected: FAIL — undefined `statusText`.

- [ ] **Step 4: Write the tray adapter**

Create `internal/adapter/driven/tray/tray.go`:
```go
// Package tray implements port.StatusPresenter and the menu over fyne.io/systray.
package tray

import (
	"fmt"
	"sync"

	"fyne.io/systray"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

// Tray presents lock status and exposes Re-home / Quit menu actions.
type Tray struct {
	mu        sync.Mutex
	statusItm *systray.MenuItem
	onRehome  func()
	onQuit    func()
	last      domain.LockState
}

// New returns a Tray. onRehome/onQuit are invoked from menu clicks.
func New(onRehome, onQuit func()) *Tray {
	return &Tray{onRehome: onRehome, onQuit: onQuit}
}

// Present updates the tray to reflect the current lock state (driven port).
func (t *Tray) Present(lock domain.LockState) {
	t.mu.Lock()
	t.last = lock
	t.mu.Unlock()
	if t.statusItm != nil {
		t.statusItm.SetTitle(statusText(lock))
	}
	if lock.Locked {
		systray.SetTitle("HD●")
	} else {
		systray.SetTitle("HD○")
	}
}

// Run starts the systray event loop. Blocks until quit; call from main goroutine.
func (t *Tray) Run() {
	systray.Run(t.onReady, func() {})
}

func (t *Tray) onReady() {
	systray.SetTitle("HD○")
	systray.SetTooltip("HyperDeck Adapter")
	t.statusItm = systray.AddMenuItem(statusText(t.last), "Player lock status")
	t.statusItm.Disable()
	systray.AddSeparator()
	rehome := systray.AddMenuItem("Re-home", "Run the homing sequence")
	quit := systray.AddMenuItem("Quit", "Exit the adapter")
	go func() {
		for {
			select {
			case <-rehome.ClickedCh:
				if t.onRehome != nil {
					t.onRehome()
				}
			case <-quit.ClickedCh:
				if t.onQuit != nil {
					t.onQuit()
				}
				systray.Quit()
				return
			}
		}
	}()
}

func statusText(lock domain.LockState) string {
	if !lock.Locked || lock.Profile == nil {
		return "Disconnected — no player"
	}
	return fmt.Sprintf("Locked: %s (%s)", lock.Profile.ID, lock.Window.Title)
}

var _ port.StatusPresenter = (*Tray)(nil)
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/adapter/driven/tray/ -v`
Expected: PASS (the `statusText` test; the systray loop is exercised manually).

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/driven/tray go.mod go.sum
git commit -m "feat(tray): status presenter and menu"
```

---

### Task 18: System clock adapter

**Files:**
- Create: `internal/adapter/driven/clock/clock.go`
- Test: `internal/adapter/driven/clock/clock_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/adapter/driven/clock/clock_test.go`:
```go
package clock

import (
	"testing"
	"time"
)

func TestTickFires(t *testing.T) {
	c := New()
	ch := c.Tick(10 * time.Millisecond)
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("expected a tick within 1s")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapter/driven/clock/ -v`
Expected: FAIL — undefined `New`.

- [ ] **Step 3: Write the clock**

Create `internal/adapter/driven/clock/clock.go`:
```go
// Package clock implements port.Clock over the system clock.
package clock

import (
	"time"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

// Clock ticks on real time.
type Clock struct{}

// New returns a system clock.
func New() *Clock { return &Clock{} }

// Tick returns a channel that fires every d.
func (c *Clock) Tick(d time.Duration) <-chan time.Time {
	return time.NewTicker(d).C
}

var _ port.Clock = (*Clock)(nil)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/driven/clock/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/driven/clock
git commit -m "feat(clock): system clock adapter"
```

---

## Phase 7 — Composition root and delivery

### Task 19: main.go wiring, build, and smoke checklist

**Files:**
- Create: `cmd/hyperdeck-adapter/main.go`

- [ ] **Step 1: Write the composition root**

Create `cmd/hyperdeck-adapter/main.go`:
```go
// Command hyperdeck-adapter runs the virtual HyperDeck tray application.
package main

import (
	"flag"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/clipsource"
	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/clock"
	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/config"
	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/injector"
	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/stateprobe"
	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/tray"
	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driving/hyperdeck"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/app"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

func main() {
	configPath := flag.String("config", defaultConfigPath(), "path to profiles.yaml")
	bind := flag.String("bind", "0.0.0.0:9993", "TCP listen address")
	interval := flag.Duration("poll", time.Second, "lock/reconcile poll interval")
	flag.Parse()

	profiles, err := config.NewStore(*configPath).Load()
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}

	inj, err := injector.New()
	if err != nil {
		slog.Error("init injector", "err", err)
		os.Exit(1)
	}

	session := app.NewSession()
	deck := app.NewVirtualDeck(session, inj)

	clk := clock.New()
	t := tray.New(func() { _ = deck.Rehome() }, func() { os.Exit(0) })

	lm := app.NewLockManager(session, inj, profiles, t,
		func(p domain.Profile) port.ClipSource { return clipsource.New(p) },
		func(p domain.Profile) port.StateProbe { return stateprobe.New(p) })
	rec := app.NewReconciler(session)

	srv := hyperdeck.NewServer(deck, deck)
	ln, err := net.Listen("tcp", *bind)
	if err != nil {
		slog.Error("listen", "addr", *bind, "err", err)
		os.Exit(1)
	}

	go func() { _ = srv.Serve(ln) }()
	go lm.Run(clk, *interval)
	go rec.Run(clk, *interval)

	slog.Info("hyperdeck-adapter started", "bind", *bind, "profiles", len(profiles))
	t.Run() // blocks on the tray event loop
}

func defaultConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "profiles.yaml"
	}
	return dir + "/hyperdeck-adapter/profiles.yaml"
}
```

- [ ] **Step 2: Build the host binary**

Run: `go build ./cmd/hyperdeck-adapter`
Expected: builds on the host OS. (On Linux it uses the noop injector.)

- [ ] **Step 3: Verify cross-compilation**

Run:
```bash
GOOS=windows GOARCH=amd64 go build -o /tmp/hda.exe ./cmd/hyperdeck-adapter
```
Expected: builds. (macOS build needs cgo and is done on a Mac: `GOOS=darwin go build` with `CGO_ENABLED=1` once the darwin injector bodies exist.)

> Note: `fyne.io/systray` requires cgo on macOS/Windows for the real tray. The Linux/CI `go test ./...` path does not invoke `systray.Run`, so tests stay cgo-free. Building the shipping macOS/Windows binaries is done on those platforms.

- [ ] **Step 4: Run the full suite once more**

Run: `go test ./... -race -count=1 && go vet ./...`
Expected: PASS, no vet warnings.

- [ ] **Step 5: Commit**

```bash
git add cmd/hyperdeck-adapter/main.go
git commit -m "feat: composition root wiring"
```

- [ ] **Step 6: Manual smoke checklist (run on real OS + controller)**

Document results in the PR. This is where the OS injector bodies (Task 16) get implemented and verified one call at a time:

1. **Windows / VLC:** start VLC with a playlist, run the adapter, point Bitfocus Companion's HyperDeck module at the host IP:9993. Verify: clip list appears in Companion; Play/Stop/Next/Prev move VLC; tray shows "Locked: vlc".
2. **Windows / Example Player:** verify `Space`-toggle play/stop never double-toggles; Next/Prev use `Ctrl+Arrow`.
3. **macOS / Mitti:** grant Accessibility permission; verify Enter=play, Space=pause, Cmd+Esc=panic, Cmd+Down/Up=next/prev; tray menu-bar shows lock state.
4. **ATEM:** add the adapter as a HyperDeck by IP in ATEM Software Control; verify it is accepted (device info), transport buttons work, and status reflects play/stop.
5. **Lock loss:** close the player; verify tray flips to "Disconnected" and `slot info` reports empty.

---

## Self-review

**Spec coverage:**
- Hexagonal core, ports, adapters, composition root → Tasks 5–9, 19. ✓
- Driving adapter (HyperDeck TCP), `200 ok`/`208`/`205`/`500` framing → Tasks 13–15. ✓
- Per-profile injection (`focus`/`background`), toggle semantics, goto-delta → Tasks 7, 16. ✓
- Clip model (playlist/positional/mitti) → Task 11. ✓
- Best-effort detection + modeled reconcile → Tasks 9, 12. ✓
- One app/one deck, profile auto-select via window match → Tasks 4, 9. ✓
- YAML profiles + validation → Task 10. ✓
- Homing on explicit command → Task 7 (`Rehome`), Task 17 (tray), Task 19 (wiring). ✓
- Tray lock/unlock indication → Tasks 17, 9. ✓
- Cross-platform + mock/noop for CI → Tasks 5, 16, 19. ✓
- Property tests for the parser → **gap addressed below.**

**Gap found & fixed inline:** the spec calls for property tests on the protocol parser; Task 13 uses table tests. Add this optional step after Task 13 Step 4 if `rapid` is desired:
```bash
go get pgregory.net/rapid@latest
```
```go
// internal/adapter/driving/hyperdeck/parser_property_test.go
package hyperdeck

import (
	"testing"

	"pgregory.net/rapid"
)

func TestParseNeverPanics(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.String().Draw(t, "raw")
		_, _ = ParseCommand(s) // must never panic on arbitrary input
	})
}
```

**Placeholder scan:** the only deliberately-deferred code is the Windows/macOS injector syscall bodies (Task 16) and the manual checklist (Task 19) — both flagged explicitly as on-device work with concrete API names, not vague TODOs. The `store.go` Step 5/6 placeholder pointer is removed in Step 6.

**Type consistency:** `Session`, `VirtualDeck`, `LockManager`, `Reconciler`, `ClipSourceFactory`, `StateProbeFactory`, port names, and `KeyName`/`Chord`/`Profile` fields are used identically across Tasks 5–19. The `Injector` union interface (Task 16) is satisfied by `*Mock` (assertion added) and consumed by `main.go`.

---

## Notes for the executor

- **TDD discipline:** every code task is test-first. Do not skip the "verify it fails" step — it proves the test exercises new behavior.
- **Commit cadence:** one commit per task as written.
- **Spec refinement recorded here:** `play_stop_toggle` (bool) supersedes the spec's `toggle_keys` sketch. If you prefer, update the spec's YAML examples to match before starting Phase 4.
- **Protocol fidelity:** before Phase 5 ships, cross-check `codes.go` and the `device info`/`transport info` field names against the HyperDeck Studio manual's Ethernet Protocol section. Only `codes.go` and `responder.go` change if the manual differs.
