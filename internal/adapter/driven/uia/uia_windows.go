//go:build windows

// Package uia implements port.PlayerController by invoking a player's UI
// Automation controls. It is the control backend for ControlUIA profiles, used
// for UWP/Store apps (e.g. Example Player) whose transport can't be driven by
// background keystrokes: synthesized keys are foreground-only and UWP CoreWindows
// ignore posted key messages, but every XAML control is exposed as a UIA element
// that can be Invoke()d by AutomationId.
//
// Invoking a UWP control activates the app (brings it foreground); that is a
// property of the platform, not avoidable here.
//
// All COM work runs on a single dedicated, OS-thread-locked goroutine that owns
// the COM apartment and the IUIAutomation instance — COM apartment rules require
// consistent thread affinity, and transport commands are issued serially.
package uia

import (
	"fmt"
	"log/slog"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
	"golang.org/x/sys/windows"
)

var (
	ole32                = windows.NewLazySystemDLL("ole32.dll")
	procCoInitializeEx   = ole32.NewProc("CoInitializeEx")
	procCoCreateInstance = ole32.NewProc("CoCreateInstance")
	oleaut32             = windows.NewLazySystemDLL("oleaut32.dll")
	procSysAllocString   = oleaut32.NewProc("SysAllocString")
	procSysFreeString    = oleaut32.NewProc("SysFreeString")
)

// CLSID_CUIAutomation {FF48DBA4-60EF-4201-AA87-54103EEF594E}
var clsidCUIAutomation = windows.GUID{Data1: 0xFF48DBA4, Data2: 0x60EF, Data3: 0x4201, Data4: [8]byte{0xAA, 0x87, 0x54, 0x10, 0x3E, 0xEF, 0x59, 0x4E}}

// IID_IUIAutomation {30CBE57D-D9D0-452A-AB13-7AC5AC4825EE}
var iidIUIAutomation = windows.GUID{Data1: 0x30CBE57D, Data2: 0xD9D0, Data3: 0x452A, Data4: [8]byte{0xAB, 0x13, 0x7A, 0xC5, 0xAC, 0x48, 0x25, 0xEE}}

const (
	clsctxInprocServer  = 1
	coinitMultithreaded = 0
	vtBSTR              = 8

	treeScopeDescendants = 4
	uiaAutomationIdProp  = 30011
	uiaInvokePatternID   = 10000

	// IUIAutomation vtable indices (after the 3 IUnknown slots).
	mAutoElementFromHandle       = 6
	mAutoCreatePropertyCondition = 23
	// IUIAutomationElement vtable indices.
	mElemFindFirst         = 5
	mElemGetCurrentPattern = 16
	mElemGetCurrentName    = 23
	// IUIAutomationInvokePattern vtable index.
	mInvokeInvoke = 3
)

// variant is the 16-byte VARIANT used to carry a BSTR into CreatePropertyCondition.
type variant struct {
	vt         uint16
	r1, r2, r3 uint16
	val        uintptr
}

// vtable views a COM object's method table as an array of function pointers, so
// methods are reached by index without uintptr arithmetic (keeps go vet happy).
type vtable struct{ m [64]uintptr }

// op selects which COM action the worker performs for a request.
type op int

const (
	opInvoke op = iota // invoke the element
	opName             // read the element's Name
)

// Engine drives and reads a player's transport via UI Automation. It owns a COM
// apartment on a single worker goroutine; construct with New. It serves both the
// player-controller (invoke) and state-probe (read Name) roles.
type Engine struct {
	reqs chan request
}

type request struct {
	op   op
	hwnd uintptr
	aid  string
	res  chan result
}

type result struct {
	name string
	err  error
}

// New starts the COM worker and returns an Engine.
func New() *Engine {
	e := &Engine{reqs: make(chan request)}
	go e.worker()
	return e
}

// Control invokes the UIA element whose AutomationId the profile maps to key, on
// the window identified by w.Handle (the target HWND). Implements port.PlayerController.
func (e *Engine) Control(p domain.Profile, w domain.Window, key domain.KeyName) error {
	aid := p.UIA[key]
	if aid == "" {
		return nil // no element mapped for this action: acked no-op
	}
	return e.do(opInvoke, w.Handle, aid).err
}

// Name reads the Name of the element with automationId under hwnd. Returns ""
// (no error) when the element is not present (e.g. no clip open).
func (e *Engine) Name(hwnd uintptr, automationID string) (string, error) {
	r := e.do(opName, hwnd, automationID)
	return r.name, r.err
}

func (e *Engine) do(o op, hwnd uintptr, aid string) result {
	res := make(chan result, 1)
	e.reqs <- request{op: o, hwnd: hwnd, aid: aid, res: res}
	return <-res
}

func (e *Engine) worker() {
	runtime.LockOSThread() // the COM apartment and IUIAutomation are bound to this thread
	procCoInitializeEx.Call(0, coinitMultithreaded)

	var automation unsafe.Pointer
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidCUIAutomation)), 0, clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidIUIAutomation)), uintptr(unsafe.Pointer(&automation)))
	if int32(hr) < 0 || automation == nil {
		slog.Error("uia: CoCreateInstance(CUIAutomation) failed", "hr", fmt.Sprintf("0x%x", uint32(hr)))
		for req := range e.reqs {
			req.res <- result{err: fmt.Errorf("uia: automation unavailable")}
		}
		return
	}

	for req := range e.reqs {
		switch req.op {
		case opName:
			name, err := getName(automation, req.hwnd, req.aid)
			req.res <- result{name: name, err: err}
		default:
			req.res <- result{err: invoke(automation, req.hwnd, req.aid)}
		}
	}
}

// findElement returns the first descendant of hwnd's element with the given
// AutomationId, or nil if none. The caller must release a non-nil result.
func findElement(automation unsafe.Pointer, hwnd uintptr, aid string) (unsafe.Pointer, error) {
	if hwnd == 0 {
		return nil, fmt.Errorf("uia: nil window handle")
	}

	var winEl unsafe.Pointer
	if r := comCall(automation, mAutoElementFromHandle, hwnd, uintptr(unsafe.Pointer(&winEl))); int32(r) < 0 || winEl == nil {
		return nil, fmt.Errorf("uia: ElementFromHandle(0x%x) failed: 0x%x", hwnd, uint32(r))
	}
	defer release(winEl)

	bstr := sysAllocString(aid)
	v := variant{vt: vtBSTR, val: bstr}
	var cond unsafe.Pointer
	r := comCall(automation, mAutoCreatePropertyCondition, uintptr(uiaAutomationIdProp), uintptr(unsafe.Pointer(&v)), uintptr(unsafe.Pointer(&cond)))
	procSysFreeString.Call(bstr)
	if int32(r) < 0 || cond == nil {
		return nil, fmt.Errorf("uia: CreatePropertyCondition failed: 0x%x", uint32(r))
	}
	defer release(cond)

	var el unsafe.Pointer
	if r := comCall(winEl, mElemFindFirst, treeScopeDescendants, uintptr(cond), uintptr(unsafe.Pointer(&el))); int32(r) < 0 {
		return nil, fmt.Errorf("uia: FindFirst(%q) failed: 0x%x", aid, uint32(r))
	}
	return el, nil
}

// invoke finds the element with the given AutomationId under hwnd and invokes it.
func invoke(automation unsafe.Pointer, hwnd uintptr, aid string) error {
	el, err := findElement(automation, hwnd, aid)
	if err != nil {
		return err
	}
	if el == nil {
		// No such control right now (e.g. no clip open): acked no-op.
		slog.Warn("uia: control not found; is media open?", "automationId", aid)
		return nil
	}
	defer release(el)

	var pat unsafe.Pointer
	if r := comCall(el, mElemGetCurrentPattern, uintptr(uiaInvokePatternID), uintptr(unsafe.Pointer(&pat))); int32(r) < 0 || pat == nil {
		return fmt.Errorf("uia: %q has no Invoke pattern: 0x%x", aid, uint32(r))
	}
	defer release(pat)

	if r := comCall(pat, mInvokeInvoke); int32(r) < 0 {
		return fmt.Errorf("uia: Invoke(%q) failed: 0x%x", aid, uint32(r))
	}
	return nil
}

// getName reads the Name property of the element with the given AutomationId.
func getName(automation unsafe.Pointer, hwnd uintptr, aid string) (string, error) {
	el, err := findElement(automation, hwnd, aid)
	if err != nil {
		return "", err
	}
	if el == nil {
		return "", nil // not present (e.g. controls hidden / no clip): not detectable
	}
	defer release(el)

	var bstr unsafe.Pointer
	if r := comCall(el, mElemGetCurrentName, uintptr(unsafe.Pointer(&bstr))); int32(r) < 0 || bstr == nil {
		return "", nil
	}
	name := windows.UTF16PtrToString((*uint16)(bstr))
	procSysFreeString.Call(uintptr(bstr))
	return name, nil
}

// comCall dispatches method idx of the COM object's vtable with the given args.
// this is the interface pointer; args follow the implicit receiver.
func comCall(this unsafe.Pointer, idx int, args ...uintptr) uintptr {
	vt := *(**vtable)(this)
	ret, _, _ := syscall.SyscallN(vt.m[idx], append([]uintptr{uintptr(this)}, args...)...)
	return ret
}

func release(this unsafe.Pointer) {
	if this != nil {
		comCall(this, 2) // IUnknown::Release
	}
}

func sysAllocString(s string) uintptr {
	p, err := windows.UTF16PtrFromString(s)
	if err != nil {
		return 0
	}
	r, _, _ := procSysAllocString.Call(uintptr(unsafe.Pointer(p)))
	return r
}

var _ port.PlayerController = (*Engine)(nil)
