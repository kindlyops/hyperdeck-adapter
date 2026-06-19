//go:build windows

package injector

import (
	"fmt"
	"time"
	"unsafe"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
	"golang.org/x/sys/windows"
)

// Delays that make synthesized input reliable. Mirrors the macOS injector: the OS
// needs time for a newly-foregrounded window to settle before keys arrive, and
// events posted too rapidly (or just before a short-lived process like injcheck
// exits) can be dropped before delivery. keyHold gives apps a moment to observe
// the key-down before the key-up.
const (
	focusSettle = 120 * time.Millisecond
	afterKey    = 25 * time.Millisecond
	keyHold     = 15 * time.Millisecond
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procEnumWindows              = user32.NewProc("EnumWindows")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procGetWindowTextW           = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW     = user32.NewProc("GetWindowTextLengthW")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procAttachThreadInput        = user32.NewProc("AttachThreadInput")
	procBringWindowToTop         = user32.NewProc("BringWindowToTop")
	procSetFocus                 = user32.NewProc("SetFocus")
	procShowWindow               = user32.NewProc("ShowWindow")
	procIsIconic                 = user32.NewProc("IsIconic")
	procSystemParametersInfoW    = user32.NewProc("SystemParametersInfoW")
	procSendInput                = user32.NewProc("SendInput")
	procPostMessageW             = user32.NewProc("PostMessageW")
	procMapVirtualKeyW           = user32.NewProc("MapVirtualKeyW")
	procGetWindowThreadID        = kernel32.NewProc("GetCurrentThreadId")
)

const (
	swRestore = 9

	spiGetForegroundLockTimeout = 0x2000
	spiSetForegroundLockTimeout = 0x2001
	spifSendChange              = 0x0002

	inputKeyboard      = 1
	keyeventfExtended  = 0x0001
	keyeventfKeyUp     = 0x0002
	mapvkVKToVSC       = 0
	wmKeyDown          = 0x0100
	wmKeyUp            = 0x0101
	lparamKeyUpFlags   = 0xC0000000 // bits 30 (previous-down) + 31 (transition/up)
	lparamExtendedFlag = 0x01000000 // bit 24
)

// extendedVKs are virtual keys that live in the extended block of the keyboard;
// their scan codes need the extended bit set for both SendInput and PostMessage.
var extendedVKs = map[uint16]bool{
	0x25: true, 0x26: true, 0x27: true, 0x28: true, // arrows
	0x2D: true, 0x2E: true, // insert, delete
}

// keybdInput mirrors Win32 KEYBDINPUT (winuser.h).
type keybdInput struct {
	Vk        uint16
	Scan      uint16
	Flags     uint32
	Time      uint32
	ExtraInfo uintptr
}

// input mirrors Win32 INPUT for keyboard events. Must be exactly 40 bytes on
// amd64 or SendInput silently rejects it; the trailing pad makes the union as
// wide as the (larger) mouse variant. Verified by sizeofInput below.
type input struct {
	Type uint32
	Ki   keybdInput
	_    [8]byte
}

// Injector is the union of the two OS-facing driven ports.
type Injector interface {
	Focus(w domain.Window) error
	SendKeys(w domain.Window, chords []domain.Chord) error
	OpenWindows() ([]domain.Window, error)
}

// New returns the Windows injector. No special permission is required to
// enumerate windows or synthesize input at the same integrity level, so unlike
// macOS there is nothing to gate on here.
func New() (Injector, error) { return &winInjector{}, nil }

type winInjector struct{}

// Focus brings the target HWND to the foreground. Windows restricts
// SetForegroundWindow so a background process cannot normally steal focus; the
// AttachThreadInput trick (attach our input queue to the foreground window's
// thread for the duration of the call) is the standard workaround. Minimized
// windows are restored first.
func (w *winInjector) Focus(win domain.Window) error {
	hwnd := win.Handle
	if hwnd == 0 {
		return fmt.Errorf("focus: nil window handle")
	}

	if iconic, _, _ := procIsIconic.Call(hwnd); iconic != 0 {
		procShowWindow.Call(hwnd, swRestore)
	}

	// Windows lets a window block foreground steals for a timeout. Drop that
	// timeout to 0 around the call (restoring it after) so SetForegroundWindow is
	// honored — the standard, system-wide-safe workaround.
	var prevTimeout uintptr
	procSystemParametersInfoW.Call(spiGetForegroundLockTimeout, 0, uintptr(unsafe.Pointer(&prevTimeout)), 0)
	procSystemParametersInfoW.Call(spiSetForegroundLockTimeout, 0, 0, spifSendChange)

	fg, _, _ := procGetForegroundWindow.Call()
	curThread, _, _ := procGetWindowThreadID.Call()
	var fgThread uintptr
	if fg != 0 {
		fgThread, _, _ = procGetWindowThreadProcessId.Call(fg, 0)
	}

	// Attaching our input queue to the current foreground thread lets
	// SetForegroundWindow/SetFocus act as if the same thread owned focus.
	attached := fgThread != 0 && fgThread != curThread
	if attached {
		procAttachThreadInput.Call(curThread, fgThread, 1)
	}
	procBringWindowToTop.Call(hwnd)
	procSetForegroundWindow.Call(hwnd)
	procSetFocus.Call(hwnd)
	if attached {
		procAttachThreadInput.Call(curThread, fgThread, 0)
	}

	// Restore the user's foreground-lock timeout.
	procSystemParametersInfoW.Call(spiSetForegroundLockTimeout, 0, prevTimeout, spifSendChange)

	time.Sleep(focusSettle) // let the window become foreground before keys arrive

	if got, _, _ := procGetForegroundWindow.Call(); got != hwnd {
		return fmt.Errorf("focus: window 0x%x did not become foreground (got 0x%x); "+
			"check it is not elevated relative to this process", hwnd, got)
	}
	return nil
}

// SendKeys delivers chords to the target HWND. The delivery path is chosen by
// whether the target is currently the foreground window:
//
//   - Foreground (a focus-mode profile just called Focus, or the window already
//     has focus): use SendInput, which drives the real system input queue and
//     sets GetKeyState — the reliable path, and the only one that carries
//     modifiers. This is the Windows analog of focus-mode on macOS.
//   - Background (a background-mode profile sends without focusing): fall back to
//     PostMessageW on the HWND. This reaches the window without stealing focus,
//     but apps that route hotkeys through a focused child window (VLC does) or
//     read modifiers via GetKeyState may ignore it — a documented limitation,
//     mirrored by the macOS note that background works for unmodified keys.
//
// Posting WM_KEYDOWN to a frame window often does not reach the app's key
// handler, so we prefer SendInput whenever the target is foreground regardless of
// modifiers.
func (w *winInjector) SendKeys(win domain.Window, chords []domain.Chord) error {
	hwnd := win.Handle
	if hwnd == 0 {
		return fmt.Errorf("sendkeys: nil window handle")
	}
	fg, _, _ := procGetForegroundWindow.Call()
	foreground := fg == hwnd
	for _, c := range chords {
		vk, ok := keyCode(c.Key)
		if !ok {
			return fmt.Errorf("sendkeys: no Windows key code for %q", c.Key)
		}
		if foreground {
			if err := sendInputChord(c.Mods, vk); err != nil {
				return err
			}
		} else if err := postKey(hwnd, vk); err != nil {
			return err
		}
		time.Sleep(afterKey) // space out events and let the last one flush
	}
	return nil
}

// postKey posts a WM_KEYDOWN/WM_KEYUP pair to the HWND (background-capable).
func postKey(hwnd uintptr, vk uint16) error {
	scan, _, _ := procMapVirtualKeyW.Call(uintptr(vk), mapvkVKToVSC)
	ext := uintptr(0)
	if extendedVKs[vk] {
		ext = lparamExtendedFlag
	}
	down := uintptr(1) | (scan&0xFF)<<16 | ext
	up := down | lparamKeyUpFlags
	if r, _, err := procPostMessageW.Call(hwnd, wmKeyDown, uintptr(vk), down); r == 0 {
		return fmt.Errorf("sendkeys: PostMessage WM_KEYDOWN vk=0x%x failed: %v", vk, err)
	}
	time.Sleep(keyHold)
	if r, _, err := procPostMessageW.Call(hwnd, wmKeyUp, uintptr(vk), up); r == 0 {
		return fmt.Errorf("sendkeys: PostMessage WM_KEYUP vk=0x%x failed: %v", vk, err)
	}
	return nil
}

// sendInputChord drives the system input queue: modifier-down(s) → key-down →
// key-up → modifier-up(s) in reverse. Goes to the foreground window.
func sendInputChord(mods []domain.Modifier, vk uint16) error {
	var seq []input
	var modVKs []uint16
	for _, m := range mods {
		mv, ok := modifierVK(m)
		if !ok {
			return fmt.Errorf("sendkeys: unknown modifier %q", m)
		}
		modVKs = append(modVKs, mv)
		seq = append(seq, keyEvent(mv, false))
	}
	seq = append(seq, keyEvent(vk, false), keyEvent(vk, true))
	for i := len(modVKs) - 1; i >= 0; i-- {
		seq = append(seq, keyEvent(modVKs[i], true))
	}

	n, _, err := procSendInput.Call(
		uintptr(len(seq)),
		uintptr(unsafe.Pointer(&seq[0])),
		unsafe.Sizeof(seq[0]),
	)
	if int(n) != len(seq) {
		return fmt.Errorf("sendkeys: SendInput inserted %d/%d events (UIPI/elevation?): %v",
			n, len(seq), err)
	}
	return nil
}

// keyEvent builds one keyboard INPUT for a virtual key (down, or up when up=true).
func keyEvent(vk uint16, up bool) input {
	scan, _, _ := procMapVirtualKeyW.Call(uintptr(vk), mapvkVKToVSC)
	var flags uint32
	if extendedVKs[vk] {
		flags |= keyeventfExtended
	}
	if up {
		flags |= keyeventfKeyUp
	}
	return input{
		Type: inputKeyboard,
		Ki:   keybdInput{Vk: vk, Scan: uint16(scan), Flags: flags},
	}
}

// OpenWindows enumerates visible, titled top-level windows. The HWND is stored in
// Window.Handle so Focus and SendKeys can act on it later.
func (w *winInjector) OpenWindows() ([]domain.Window, error) {
	names := processNames() // pid -> exe base name

	var out []domain.Window
	cb := windows.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		if visible, _, _ := procIsWindowVisible.Call(hwnd); visible == 0 {
			return 1 // continue enumeration
		}
		title := windowText(hwnd)
		if title == "" {
			return 1
		}
		var pid uint32
		procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		out = append(out, domain.Window{
			Handle:  hwnd,
			Title:   title,
			Process: names[pid],
		})
		return 1
	})
	if r, _, err := procEnumWindows.Call(cb, 0); r == 0 {
		return nil, fmt.Errorf("openwindows: EnumWindows failed: %v", err)
	}
	return out, nil
}

// windowText reads a window's title via GetWindowTextLengthW + GetWindowTextW.
func windowText(hwnd uintptr) string {
	n, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	got, _, _ := procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), n+1)
	return windows.UTF16ToString(buf[:got])
}

// processNames builds a pid → executable base name map via a Toolhelp snapshot,
// once per OpenWindows call. ExeFile is already a base name (e.g. "vlc.exe").
func processNames() map[uint32]string {
	names := make(map[uint32]string)
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return names
	}
	defer windows.CloseHandle(snap)

	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	if err := windows.Process32First(snap, &e); err != nil {
		return names
	}
	for {
		names[e.ProcessID] = windows.UTF16ToString(e.ExeFile[:])
		if err := windows.Process32Next(snap, &e); err != nil {
			break
		}
	}
	return names
}
