# Profile selection via tray submenu — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let the operator pin one active profile from the system tray (with "Auto (match any)" restoring today's behavior), persisted across restarts.

**Architecture:** A new `config.SelectionStore` owns a `selection.json` state file. The `LockManager` gains a mutex-guarded `active` profile id that filters matching. The tray renders a "Profile" submenu whose clicks call back into the composition root to persist the choice and re-point the matcher. `main.go` wires the three together and applies the persisted selection at startup (including headless runs).

**Tech Stack:** Go 1.26, `fyne.io/systray`, `encoding/json`, standard `testing`.

---

## File Structure

- **Create** `internal/adapter/driven/config/selection.go` — `SelectionStore` (JSON state file I/O).
- **Create** `internal/adapter/driven/config/selection_test.go` — round-trip, missing-file, malformed-file tests.
- **Create** `internal/adapter/driven/tray/profilemenu.go` — pure `checkedProfile` resolver (no build tag, compiles on every platform).
- **Create** `internal/adapter/driven/tray/profilemenu_test.go` — `checkedProfile` tests (run on CI/Linux).
- **Modify** `internal/core/app/lockmanager.go` — add `mu`/`active`, `SetActive`, filter in `firstMatch`.
- **Modify** `internal/core/app/lockmanager_test.go` — pin tests + `mittiProfileForLock` helper.
- **Modify** `internal/adapter/driven/tray/tray.go` (darwin/windows) — submenu fields, `New` signature, `onReady` submenu, `selectProfile`.
- **Modify** `internal/adapter/driven/tray/tray_noop.go` (linux/CI) — matching `New` signature.
- **Modify** `cmd/hyperdeck-adapter/main.go` — selection path/load/validate, `onSelect` callback, `ui` signature, `SetActive` at startup.

---

## Task 1: SelectionStore (state file I/O)

**Files:**
- Create: `internal/adapter/driven/config/selection.go`
- Test: `internal/adapter/driven/config/selection_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/adapter/driven/config/selection_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSelectionStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "selection.json")
	s := NewSelectionStore(path)
	if err := s.Save("vlc"); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got != "vlc" {
		t.Errorf("got %q want %q", got, "vlc")
	}
}

func TestSelectionStoreMissingFileIsAuto(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	got, err := NewSelectionStore(path).Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got != "" {
		t.Errorf("got %q want empty (Auto)", got)
	}
}

func TestSelectionStoreMalformedFileErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSelectionStore(path).Load(); err == nil {
		t.Error("expected error for malformed selection file")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/adapter/driven/config/ -run TestSelectionStore -v`
Expected: FAIL — `undefined: NewSelectionStore`.

- [ ] **Step 3: Write the implementation**

Create `internal/adapter/driven/config/selection.go`:

```go
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SelectionStore persists the pinned profile id (empty = Auto / match-any) to a
// JSON file kept separate from profiles.yaml, so the user's commented config is
// never rewritten.
type SelectionStore struct{ path string }

// NewSelectionStore returns a SelectionStore backed by path.
func NewSelectionStore(path string) *SelectionStore { return &SelectionStore{path: path} }

type selectionSchema struct {
	ActiveProfile string `json:"active_profile"`
}

// Load returns the pinned profile id. A missing file is not an error and yields
// "" (Auto); a malformed file returns an error.
func (s *SelectionStore) Load() (string, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read selection %q: %w", s.path, err)
	}
	var sel selectionSchema
	if err := json.Unmarshal(data, &sel); err != nil {
		return "", fmt.Errorf("parse selection %q: %w", s.path, err)
	}
	return sel.ActiveProfile, nil
}

// Save writes the pinned profile id (empty clears to Auto), creating parent
// directories as needed.
func (s *SelectionStore) Save(id string) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create selection dir for %q: %w", s.path, err)
	}
	data, err := json.Marshal(selectionSchema{ActiveProfile: id})
	if err != nil {
		return fmt.Errorf("encode selection: %w", err)
	}
	if err := os.WriteFile(s.path, data, 0o644); err != nil {
		return fmt.Errorf("write selection %q: %w", s.path, err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/driven/config/ -run TestSelectionStore -v`
Expected: PASS (all three).

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/driven/config/selection.go internal/adapter/driven/config/selection_test.go
git commit -m "feat(config): persist pinned profile selection to JSON state file"
```

---

## Task 2: LockManager pinning

**Files:**
- Modify: `internal/core/app/lockmanager.go`
- Test: `internal/core/app/lockmanager_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/core/app/lockmanager_test.go`:

```go
func TestLockManagerPinnedMatchesOnlyActive(t *testing.T) {
	m := injector.NewMock()
	m.Windows = []domain.Window{
		{Process: "vlc.exe", Title: "x - VLC media player"},
		{Process: "Mitti", Title: "Mitti"},
	}
	s := NewSession()
	pres := &fakePresenter{}
	profiles := []domain.Profile{vlcProfileForLock(), mittiProfileForLock()}
	lm := NewLockManager(s, m, profiles, pres,
		func(domain.Profile) port.ClipSource { return fakeClipSource{} },
		func(domain.Profile) port.StateProbe { return noProbe{} })

	lm.SetActive("mitti") // pins mitti even though vlc also matches; triggers Poll

	p, _, ok := s.Active()
	if !ok || p.ID != "mitti" {
		t.Fatalf("expected mitti locked, got ok=%v id=%q", ok, p.ID)
	}
}

func TestLockManagerPinnedNotRunningStaysUnlocked(t *testing.T) {
	m := injector.NewMock()
	m.Windows = []domain.Window{{Process: "vlc.exe", Title: "x - VLC media player"}}
	s := NewSession()
	pres := &fakePresenter{}
	profiles := []domain.Profile{vlcProfileForLock(), mittiProfileForLock()}
	lm := NewLockManager(s, m, profiles, pres,
		func(domain.Profile) port.ClipSource { return fakeClipSource{} },
		func(domain.Profile) port.StateProbe { return noProbe{} })

	lm.SetActive("mitti") // mitti is not in the window list

	if _, _, ok := s.Active(); ok {
		t.Error("expected unlocked when the pinned profile is not running")
	}
}

func mittiProfileForLock() domain.Profile {
	return domain.Profile{
		ID:     "mitti",
		Match:  domain.Match{Process: []string{"Mitti"}},
		Keymap: domain.Keymap{domain.KeyPlay: {Key: "enter"}},
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/core/app/ -run TestLockManagerPinned -v`
Expected: FAIL — `lm.SetActive undefined`.

- [ ] **Step 3: Add the `sync` import and struct fields**

In `internal/core/app/lockmanager.go`, change the import block and the `LockManager` struct:

```go
import (
	"sync"
	"time"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

// LockManager binds a running player to the session by matching profiles.
type LockManager struct {
	session   *Session
	windows   port.WindowEnumerator
	profiles  []domain.Profile
	presenter port.StatusPresenter
	clipsFor  ClipSourceFactory
	probeFor  StateProbeFactory

	mu     sync.Mutex
	active string // pinned profile id; "" means Auto / match any
}
```

- [ ] **Step 4: Switch `NewLockManager` to a named-field literal**

Replace the body of `NewLockManager` (the positional literal would break with the new fields):

```go
func NewLockManager(
	s *Session,
	w port.WindowEnumerator,
	profiles []domain.Profile,
	presenter port.StatusPresenter,
	clipsFor ClipSourceFactory,
	probeFor StateProbeFactory,
) *LockManager {
	return &LockManager{
		session:   s,
		windows:   w,
		profiles:  profiles,
		presenter: presenter,
		clipsFor:  clipsFor,
		probeFor:  probeFor,
	}
}
```

- [ ] **Step 5: Add `SetActive` and filter `firstMatch`**

Add `SetActive` (place it just after `Poll`) and replace `firstMatch`:

```go
// SetActive pins the matcher to one profile id; "" restores Auto (match any).
// It re-polls immediately so the lock state reflects the new selection.
func (lm *LockManager) SetActive(id string) {
	lm.mu.Lock()
	lm.active = id
	lm.mu.Unlock()
	lm.Poll()
}

func (lm *LockManager) firstMatch(windows []domain.Window) (domain.Profile, domain.Window, bool) {
	lm.mu.Lock()
	active := lm.active
	lm.mu.Unlock()
	for _, p := range lm.profiles {
		if active != "" && p.ID != active {
			continue
		}
		for _, w := range windows {
			if p.MatchesWindow(w) {
				return p, w, true
			}
		}
	}
	return domain.Profile{}, domain.Window{}, false
}
```

- [ ] **Step 6: Run the package tests**

Run: `go test ./internal/core/app/ -v`
Expected: PASS — the two new `TestLockManagerPinned*` tests pass and the existing `TestLockManagerLocksOnMatch` / `TestLockManagerUnlocksWhenGone` (which never call `SetActive`, so `active == ""`) still pass.

- [ ] **Step 7: Commit**

```bash
git add internal/core/app/lockmanager.go internal/core/app/lockmanager_test.go
git commit -m "feat(app): pin LockManager matching to a selected profile"
```

---

## Task 3: Tray checkmark resolver (pure function)

**Files:**
- Create: `internal/adapter/driven/tray/profilemenu.go`
- Test: `internal/adapter/driven/tray/profilemenu_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/adapter/driven/tray/profilemenu_test.go`:

```go
package tray

import "testing"

func TestCheckedProfile(t *testing.T) {
	profiles := []string{"vlc", "mitti"}
	cases := []struct {
		name   string
		active string
		want   string
	}{
		{"auto when empty", "", ""},
		{"known id checks itself", "mitti", "mitti"},
		{"unknown id falls back to auto", "ghost", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := checkedProfile(profiles, c.active); got != c.want {
				t.Errorf("checkedProfile(%v, %q) = %q want %q", profiles, c.active, got, c.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapter/driven/tray/ -run TestCheckedProfile -v`
Expected: FAIL — `undefined: checkedProfile`.

- [ ] **Step 3: Write the implementation**

Create `internal/adapter/driven/tray/profilemenu.go` (no build tag — it compiles on every platform so the test runs on CI/Linux):

```go
package tray

// checkedProfile returns the profile id whose menu entry should show a
// checkmark: active when it names a known profile, otherwise "" (the Auto entry).
func checkedProfile(profiles []string, active string) string {
	if active == "" {
		return ""
	}
	for _, id := range profiles {
		if id == active {
			return active
		}
	}
	return ""
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/adapter/driven/tray/ -run TestCheckedProfile -v`
Expected: PASS (all three sub-cases).

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/driven/tray/profilemenu.go internal/adapter/driven/tray/profilemenu_test.go
git commit -m "feat(tray): resolve which profile entry shows a checkmark"
```

---

## Task 4: Tray submenu + main.go wiring

This task changes `tray.New`'s signature and updates every caller in the same commit, so `go build ./...` stays green.

**Files:**
- Modify: `internal/adapter/driven/tray/tray.go`
- Modify: `internal/adapter/driven/tray/tray_noop.go`
- Modify: `cmd/hyperdeck-adapter/main.go`

- [ ] **Step 1: Extend the desktop `Tray` struct and `New`**

In `internal/adapter/driven/tray/tray.go`, replace the `Tray` struct and `New`:

```go
// Tray presents lock status and exposes Profile / Re-home / Quit menu actions.
type Tray struct {
	mu        sync.Mutex
	statusItm *systray.MenuItem
	onRehome  func()
	onQuit    func()
	last      domain.LockState

	profiles     []string
	active       string
	onSelect     func(string)
	profileItems map[string]*systray.MenuItem // keyed by profile id; "" = Auto
}

// New returns a Tray. onRehome/onQuit/onSelect are invoked from menu clicks;
// profiles lists selectable profile ids and active is the initially pinned id
// ("" = Auto).
func New(onRehome, onQuit func(), profiles []string, active string, onSelect func(string)) *Tray {
	return &Tray{
		onRehome: onRehome,
		onQuit:   onQuit,
		profiles: profiles,
		active:   active,
		onSelect: onSelect,
	}
}
```

- [ ] **Step 2: Add the Profile submenu in `onReady`**

In `internal/adapter/driven/tray/tray.go`, replace the body of `onReady` with:

```go
func (t *Tray) onReady() {
	t.mu.Lock()
	last := t.last
	t.mu.Unlock()
	systray.SetIcon(lockIcon(last.Locked))
	systray.SetTitle("HD○")
	systray.SetTooltip("HyperDeck Adapter")
	t.statusItm = systray.AddMenuItem(statusText(last), "Player lock status")
	t.statusItm.Disable()
	systray.AddSeparator()
	t.addProfileMenu()
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

// addProfileMenu builds the Profile submenu: an "Auto (match any)" entry plus
// one checkbox per profile id, with a checkmark on the active selection.
func (t *Tray) addProfileMenu() {
	profMenu := systray.AddMenuItem("Profile", "Pin which profile the adapter uses")
	checked := checkedProfile(t.profiles, t.active)
	t.profileItems = make(map[string]*systray.MenuItem, len(t.profiles)+1)

	auto := profMenu.AddSubMenuItemCheckbox("Auto (match any)", "Match any running player", checked == "")
	t.profileItems[""] = auto
	go func() {
		for range auto.ClickedCh {
			t.selectProfile("")
		}
	}()

	for _, id := range t.profiles {
		item := profMenu.AddSubMenuItemCheckbox(id, "Pin the "+id+" profile", checked == id)
		t.profileItems[id] = item
		go func(id string, item *systray.MenuItem) {
			for range item.ClickedCh {
				t.selectProfile(id)
			}
		}(id, item)
	}
}

// selectProfile records the new pinned id, moves the checkmark to it, and
// notifies the composition root via onSelect.
func (t *Tray) selectProfile(id string) {
	t.mu.Lock()
	t.active = id
	for key, item := range t.profileItems {
		if key == id {
			item.Check()
		} else {
			item.Uncheck()
		}
	}
	t.mu.Unlock()
	if t.onSelect != nil {
		t.onSelect(id)
	}
}
```

- [ ] **Step 3: Update the no-op `Tray.New` signature to match**

In `internal/adapter/driven/tray/tray_noop.go`, replace `New` (the headless build has no menu, so the extra arguments are accepted and ignored):

```go
// New returns a no-op Tray with the same API as the desktop implementation.
// profiles/active/onSelect are unused: this platform has no interactive menu.
func New(onRehome, onQuit func(), profiles []string, active string, onSelect func(string)) *Tray {
	return &Tray{onRehome: onRehome, onQuit: onQuit}
}
```

- [ ] **Step 4: Wire selection load + callback in `main.go`**

In `cmd/hyperdeck-adapter/main.go`, replace the block from `profiles, err := config.NewStore...` down through the `lm := app.NewLockManager(...)` / `rec := app.NewReconciler(...)` lines, and the later `lm.Poll()` line. New `main` body (the section between loading the injector and starting goroutines):

```go
	profiles, err := config.NewStore(*configPath).Load()
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}

	selStore := config.NewSelectionStore(defaultSelectionPath())
	active, err := selStore.Load()
	if err != nil {
		slog.Warn("load selection; defaulting to Auto", "err", err)
		active = ""
	}
	active = validateActive(active, profiles)

	inj, err := injector.New()
	if err != nil {
		slog.Error("init injector", "err", err)
		os.Exit(1)
	}
	if !injector.RequestAccessibility() {
		slog.Warn("input permission not granted; keystrokes will not be delivered until this binary is enabled in the OS input/accessibility settings")
	}

	session := app.NewSession()
	uiaEngine := uia.New()
	controller := controlRouter{
		domain.ControlAPI: vlchttp.New(),
		domain.ControlUIA: uiaEngine,
	}
	deck := app.NewVirtualDeck(session, inj, app.WithController(controller))
	clk := clock.New()

	var lm *app.LockManager
	onSelect := func(id string) {
		if err := selStore.Save(id); err != nil {
			slog.Warn("save selection", "err", err)
		}
		lm.SetActive(id)
	}

	presenter, run := ui(*noTray, deck, profileIDs(profiles), active, onSelect)

	lm = app.NewLockManager(session, inj, profiles, presenter,
		func(p domain.Profile) port.ClipSource { return clipsource.New(p) },
		func(p domain.Profile) port.StateProbe { return stateprobe.New(p, uiaEngine) })
	rec := app.NewReconciler(session)

	srv := hyperdeck.NewServer(deck, deck)
	ln, err := net.Listen("tcp", *bind)
	if err != nil {
		slog.Error("listen", "addr", *bind, "err", err)
		os.Exit(1)
	}

	lm.SetActive(active) // apply persisted selection and lock immediately if a matching player is running
	go func() { _ = srv.Serve(ln) }()
	go lm.Run(clk, *interval)
	go rec.Run(clk, *interval)
```

Note: this replaces the old `lm.Poll() // lock immediately...` line — `SetActive(active)` performs the initial poll.

- [ ] **Step 5: Update the `ui` helper and add the two wiring helpers**

In `cmd/hyperdeck-adapter/main.go`, replace `ui` and add the helpers near `defaultConfigPath`:

```go
// ui returns the status presenter and the blocking run loop for the chosen mode.
func ui(noTray bool, deck *app.VirtualDeck, profiles []string, active string, onSelect func(string)) (port.StatusPresenter, func()) {
	if noTray {
		return logPresenter{}, waitForSignal
	}
	t := tray.New(func() { _ = deck.Rehome() }, func() { os.Exit(0) }, profiles, active, onSelect)
	return t, t.Run
}

// profileIDs extracts the ordered profile ids for the tray menu.
func profileIDs(profiles []domain.Profile) []string {
	ids := make([]string, len(profiles))
	for i, p := range profiles {
		ids[i] = p.ID
	}
	return ids
}

// validateActive keeps a pinned id only when it still names a loaded profile;
// an unknown id (renamed/removed profile) falls back to Auto with a warning.
func validateActive(active string, profiles []domain.Profile) string {
	if active == "" {
		return ""
	}
	for _, p := range profiles {
		if p.ID == active {
			return active
		}
	}
	slog.Warn("pinned profile not found; falling back to Auto", "profile", active)
	return ""
}

// defaultSelectionPath is the JSON state file holding the pinned profile id,
// stored alongside profiles.yaml in the OS config dir.
func defaultSelectionPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "selection.json"
	}
	return filepath.Join(dir, "hyperdeck-adapter", "selection.json")
}
```

- [ ] **Step 6: Build for every platform path**

Run:
```bash
go build ./...
GOOS=linux  GOARCH=amd64 CGO_ENABLED=0 go build ./...
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...
```
Expected: all three succeed. The native build exercises the darwin tray (`tray.go`), the linux build exercises the no-op tray (`tray_noop.go`), and the windows build exercises the windows tray path — all now agree on `New`'s signature.

- [ ] **Step 7: Run the full test suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/adapter/driven/tray/tray.go internal/adapter/driven/tray/tray_noop.go cmd/hyperdeck-adapter/main.go
git commit -m "feat(tray): add Profile submenu to pin the active profile"
```

---

## Task 5: Final verification

**Files:** none (verification + formatting only).

- [ ] **Step 1: Format check**

Run: `gofmt -l .`
Expected: no output (no files need formatting). If a path prints, run `gofmt -w <path>` and re-check.

- [ ] **Step 2: Vet**

Run: `go vet ./...`
Expected: no output.

- [ ] **Step 3: Full test + race on the changed packages**

Run: `go test -race ./internal/core/app/ ./internal/adapter/driven/config/ ./internal/adapter/driven/tray/`
Expected: PASS — confirms `SetActive`/`firstMatch` and the tray's `selectProfile` are race-clean under the mutex.

- [ ] **Step 4: Manual smoke (optional, on macOS)**

Run: `go run ./cmd/hyperdeck-adapter` then open the tray → Profile submenu. Pick a profile; confirm the checkmark moves and a `selection.json` appears under `~/Library/Application Support/hyperdeck-adapter/`. Quit and relaunch; confirm the same entry is checked.

- [ ] **Step 5: Final commit (only if Step 1 rewrote files)**

```bash
git add -A
git commit -m "style: gofmt profile selection changes"
```

---

## Self-Review Notes

- **Spec coverage:** Behavior (Auto/pin) → Tasks 2 & 4; `SelectionStore` → Task 1; `LockManager` filter/`SetActive` → Task 2; tray submenu + checkmark → Tasks 3 & 4; `main.go` wiring → Task 4; stale-selection fallback → `validateActive` (Task 4 Step 5); save-failure logging → `onSelect` (Task 4 Step 4); headless honors selection → `lm.SetActive(active)` runs regardless of UI (Task 4 Step 4); JSON not YAML → Task 1. All testing-matrix rows map to tests in Tasks 1–3.
- **Type consistency:** `SetActive(string)`, `checkedProfile([]string, string) string`, `New(onRehome, onQuit func(), profiles []string, active string, onSelect func(string))`, `selection_test` uses `NewSelectionStore`/`Save`/`Load` exactly as defined.
- **Naming:** the pinned-id field is `active` in both `LockManager` and `Tray`; the JSON key is `active_profile`.
