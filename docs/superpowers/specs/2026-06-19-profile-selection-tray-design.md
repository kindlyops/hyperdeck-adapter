# Profile selection via tray submenu

## Problem

Today the adapter auto-matches a running player window against *all* loaded
profiles and locks onto the first match (`internal/core/app/lockmanager.go`,
`firstMatch`). There is no way for an operator to say "only use the VLC
profile" — useful when several configured players could be running, or to force
a known-good profile during a live event. The tray exposes only Re-home and
Quit.

## Goal

Let the operator pin exactly one active profile from the system tray, with an
"Auto (match any)" option that restores today's behavior. The choice persists
across restarts in a small state file and is honored by headless runs.

## Behavior

- The tray gains a **"Profile"** submenu listing **"Auto (match any)"** plus
  every profile `id` from `profiles.yaml`. A checkmark marks the active entry.
  Selection is radio-style: picking one clears the others.
- **Auto** (active id `""`): unchanged behavior — match any running player,
  first match wins.
- **Pinned profile** (active id set): the `LockManager` only considers that one
  profile. If its window is running, it locks; if not, the adapter stays
  unlocked and unlocks from any other player it was previously on.
- Selection persists to a separate state file and is re-applied on the next
  launch, including headless `-no-tray` runs (which honor the file but have no
  menu to change it).

## Components

Each unit has one purpose, a defined interface, and is testable in isolation.

### 1. `config.SelectionStore` (new)

Owns `selection.json`, stored alongside `profiles.yaml` in the OS config dir.

- `Load() (string, error)` — returns the active profile id. Missing file
  returns `("", nil)` (Auto). Malformed file returns an error.
- `Save(id string) error` — writes the selection. Empty id clears to Auto.

JSON (not YAML) so the app never round-trips the user's commented
`profiles.yaml`. Schema: `{"active_profile": "<id>"}` (absent/empty = Auto).

Pure I/O; no domain logic, no knowledge of which ids are valid.

### 2. `LockManager` (modified)

- Holds a mutex-guarded `active string`.
- `SetActive(id string)` — updates `active` and triggers an immediate `Poll()`
  so the lock state updates instantly on selection.
- `firstMatch` filters candidate profiles to `active` when set; when `active`
  is `""` it scans all profiles in order (today's path, unchanged).

The `LockManager` holds the selection (rather than being rebuilt with a
filtered profile slice on each change) so the tray callback path stays a single
`SetActive` call that both updates matching and re-polls.

### 3. `tray.Tray` (modified)

- Renders the "Profile" submenu from a profile-id list plus the current
  selection, with a checkmark on the active entry.
- Fires an `onSelectProfile(id string)` callback on click (`""` = Auto) and
  updates the checkmarks.
- The "which item is checked" resolution is extracted to a pure, testable
  function; only the thin systray calls remain untested.

### 4. `main.go` (wiring)

- Load the selection from `SelectionStore` at startup.
- Apply it to the `LockManager` (`SetActive`).
- Pass profile ids + current selection + the `onSelectProfile` callback to the
  tray.
- Callback: `Save(id)` → `SetActive(id)` (which re-polls and presents).

## Data flow

```
startup:  selection.json --Load--> LockManager.SetActive --> Poll (lock/unlock)
                         \--------> tray renders checkmarks
click:    tray onSelectProfile(id) --> Save(id) --> SetActive(id) --> Poll --> Present
```

## Edge cases & error handling

- **Stale selection**: a persisted id no longer present in `profiles.yaml` is
  logged as a warning and falls back to Auto. Do not crash; do not silently
  lock nothing without explanation.
- **Save failure**: log the error; the in-memory selection still takes effect
  for the session. Fail loud, don't block the UI.
- **Headless**: `-no-tray` applies the persisted selection at startup; there is
  no interactive change path.

## Testing

- `SelectionStore`: save/load round-trip; missing file → `("", nil)`;
  malformed file → error.
- `LockManager`: a pinned profile matches only itself even when another player
  is running; pinned-but-not-running stays unlocked; Auto (`""`) preserves the
  existing match-any tests; switching active re-locks on the triggered Poll.
- `tray`: checkmark-resolution function maps (profiles, selected) to the checked
  entry, including an unknown id → Auto.

## Out of scope

- Force-locking a profile whose window is not running (override semantics).
- Enabling/disabling a subset of profiles (multi-select).
- A web or native-window settings UI.
- A CLI flag to set the selection in headless mode.
