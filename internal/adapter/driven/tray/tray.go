//go:build darwin || windows

package tray

import (
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
	t.mu.Lock()
	last := t.last
	t.mu.Unlock()
	t.statusItm = systray.AddMenuItem(statusText(last), "Player lock status")
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

var _ port.StatusPresenter = (*Tray)(nil)
