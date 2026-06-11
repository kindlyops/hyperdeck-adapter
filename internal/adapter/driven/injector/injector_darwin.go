//go:build darwin

package injector

/*
#cgo LDFLAGS: -framework Cocoa -framework CoreGraphics -framework ApplicationServices
#include <stdlib.h>
#include "cinject_darwin.h"
*/
import "C"

import (
	"fmt"
	"log/slog"
	"time"
	"unsafe"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

// Delays that make synthesized input reliable: the OS needs time for the
// activated app to become first responder, and HID events posted too rapidly (or
// just before a short-lived process exits) are dropped before delivery.
const (
	focusSettle = 120 * time.Millisecond
	afterKey    = 25 * time.Millisecond
)

// Injector is the union of the two OS-facing driven ports.
type Injector interface {
	Focus(w domain.Window) error
	SendKeys(w domain.Window, chords []domain.Chord) error
	OpenWindows() ([]domain.Window, error)
}

// New returns the macOS injector. Synthesized keystrokes require the
// Accessibility permission; if it is missing we warn but still return the
// injector so window enumeration works and the user can grant it.
func New() (Injector, error) {
	if C.hdAXTrusted() == 0 {
		slog.Warn("Accessibility permission not granted; keystrokes will not be delivered until enabled in System Settings > Privacy & Security > Accessibility")
	}
	return &macInjector{}, nil
}

// RequestAccessibility reports whether this process is trusted for Accessibility,
// prompting the user to grant it (via System Settings) when it is not.
func RequestAccessibility() bool { return C.hdAXPrompt() == 1 }

type macInjector struct{}

// Focus brings the target application (identified by pid stored in Handle) to
// the foreground via NSRunningApplication.
func (m *macInjector) Focus(win domain.Window) error {
	if C.hdActivatePID(C.int64_t(win.Handle)) == 0 {
		return fmt.Errorf("focus: could not activate pid %d (%s)", win.Handle, win.Process)
	}
	time.Sleep(focusSettle) // let the app become first responder before keys arrive
	return nil
}

// SendKeys posts CGEvent keyboard events for each chord to the focused app.
func (m *macInjector) SendKeys(win domain.Window, chords []domain.Chord) error {
	for _, c := range chords {
		code, ok := keyCode(c.Key)
		if !ok {
			return fmt.Errorf("sendkeys: no macOS key code for %q", c.Key)
		}
		C.hdPostKeyToPid(C.int64_t(win.Handle), C.uint16_t(code), C.uint64_t(eventFlags(c.Mods)))
		time.Sleep(afterKey) // space out events and let the last one flush
	}
	return nil
}

// OpenWindows lists on-screen windows via CGWindowListCopyWindowInfo. The owning
// process id is stored in Window.Handle so Focus can activate it later.
func (m *macInjector) OpenWindows() ([]domain.Window, error) {
	const max = 512
	buf := make([]C.HDWindow, max)
	n := int(C.hdListWindows((*C.HDWindow)(unsafe.Pointer(&buf[0])), C.int(max)))
	out := make([]domain.Window, 0, n)
	for i := range n {
		w := &buf[i]
		out = append(out, domain.Window{
			Handle:  uintptr(int64(w.pid)),
			Process: C.GoString(&w.owner[0]),
			Title:   C.GoString(&w.title[0]),
		})
	}
	return out, nil
}
