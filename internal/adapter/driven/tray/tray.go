//go:build darwin || windows

package tray

import (
	"sync"

	"fyne.io/systray"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

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

// Present updates the tray to reflect the current lock state (driven port).
func (t *Tray) Present(lock domain.LockState) {
	t.mu.Lock()
	t.last = lock
	t.mu.Unlock()
	if t.statusItm != nil {
		t.statusItm.SetTitle(statusText(lock))
	}
	systray.SetIcon(lockIcon(lock.Locked))
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

var _ port.StatusPresenter = (*Tray)(nil)
