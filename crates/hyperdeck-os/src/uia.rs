//! UI Automation control + state reads (port of `internal/adapter/driven/uia`).
//!
//! Backend for `control: uia` profiles (UWP/Store apps like Example Player whose
//! transport can't be driven by background keystrokes): every XAML control is a
//! UIA element invoked by AutomationId. The same engine also reads an element's
//! Name for the UIA state probe.
//!
//! All COM work runs on one dedicated thread that owns the MTA apartment and the
//! `IUIAutomation` instance (COM apartment rules require thread affinity);
//! requests are serviced serially over a channel.

use std::ffi::c_void;
use std::sync::mpsc::{channel, Receiver, Sender};
use std::thread;

use windows::core::{BSTR, VARIANT};
use windows::Win32::Foundation::HWND;
use windows::Win32::System::Com::{
    CoCreateInstance, CoInitializeEx, CLSCTX_INPROC_SERVER, COINIT_MULTITHREADED,
};
use windows::Win32::UI::Accessibility::{
    CUIAutomation, IUIAutomation, IUIAutomationElement, IUIAutomationInvokePattern,
    TreeScope_Descendants, UIA_AutomationIdPropertyId, UIA_InvokePatternId,
};

use hyperdeck_core::domain::{KeyName, Profile, Window};
use hyperdeck_core::error::{DeckError, DeckResult};
use hyperdeck_core::port::PlayerController;
use hyperdeck_core::stateprobe::ElementNamer;

enum Op {
    Invoke,
    Name,
}

struct Req {
    op: Op,
    hwnd: usize,
    aid: String,
    res: Sender<DeckResult<String>>,
}

/// Drives and reads a player's transport via UI Automation. Owns a COM apartment
/// on a single worker thread; construct with [`Engine::new`].
pub struct Engine {
    reqs: Sender<Req>,
}

impl Engine {
    /// Starts the COM worker thread and returns an engine.
    pub fn new() -> Self {
        let (tx, rx) = channel::<Req>();
        thread::spawn(move || worker(rx));
        Engine { reqs: tx }
    }

    fn call(&self, op: Op, hwnd: usize, aid: &str) -> DeckResult<String> {
        let (rtx, rrx) = channel();
        self.reqs
            .send(Req {
                op,
                hwnd,
                aid: aid.to_string(),
                res: rtx,
            })
            .map_err(|_| DeckError::Other("uia: worker stopped".into()))?;
        rrx.recv()
            .map_err(|_| DeckError::Other("uia: no response".into()))?
    }
}

impl Default for Engine {
    fn default() -> Self {
        Self::new()
    }
}

fn worker(rx: Receiver<Req>) {
    // SAFETY: standard COM init on this dedicated thread.
    unsafe {
        let _ = CoInitializeEx(None, COINIT_MULTITHREADED);
    }
    let automation: Option<IUIAutomation> =
        unsafe { CoCreateInstance(&CUIAutomation, None, CLSCTX_INPROC_SERVER).ok() };
    let Some(automation) = automation else {
        for req in rx {
            let _ = req
                .res
                .send(Err(DeckError::Other("uia: automation unavailable".into())));
        }
        return;
    };
    for req in rx {
        let r = match req.op {
            Op::Invoke => invoke(&automation, req.hwnd, &req.aid).map(|_| String::new()),
            Op::Name => get_name(&automation, req.hwnd, &req.aid),
        };
        let _ = req.res.send(r);
    }
}

/// Returns the first descendant of `hwnd`'s element with the given AutomationId,
/// or `None` when no such element exists right now (e.g. no clip open).
fn find_element(
    automation: &IUIAutomation,
    hwnd: usize,
    aid: &str,
) -> DeckResult<Option<IUIAutomationElement>> {
    if hwnd == 0 {
        return Err(DeckError::Other("uia: nil window handle".into()));
    }
    let h = HWND(hwnd as *mut c_void);
    let win_el = unsafe { automation.ElementFromHandle(h) }
        .map_err(|e| DeckError::Other(format!("uia: ElementFromHandle: {e}")))?;
    let cond = unsafe {
        automation
            .CreatePropertyCondition(UIA_AutomationIdPropertyId, &VARIANT::from(BSTR::from(aid)))
    }
    .map_err(|e| DeckError::Other(format!("uia: CreatePropertyCondition: {e}")))?;
    // FindFirst returns an error / null when nothing matches; treat as not found.
    match unsafe { win_el.FindFirst(TreeScope_Descendants, &cond) } {
        Ok(el) => Ok(Some(el)),
        Err(_) => Ok(None),
    }
}

fn invoke(automation: &IUIAutomation, hwnd: usize, aid: &str) -> DeckResult<()> {
    let Some(el) = find_element(automation, hwnd, aid)? else {
        return Ok(()); // control not present: acked no-op
    };
    let pattern: IUIAutomationInvokePattern =
        unsafe { el.GetCurrentPatternAs(UIA_InvokePatternId) }
            .map_err(|e| DeckError::Other(format!("uia: {aid:?} has no Invoke pattern: {e}")))?;
    unsafe { pattern.Invoke() }
        .map_err(|e| DeckError::Other(format!("uia: Invoke({aid:?}): {e}")))?;
    Ok(())
}

fn get_name(automation: &IUIAutomation, hwnd: usize, aid: &str) -> DeckResult<String> {
    let Some(el) = find_element(automation, hwnd, aid)? else {
        return Ok(String::new()); // not present: not detectable
    };
    match unsafe { el.CurrentName() } {
        Ok(bstr) => Ok(bstr.to_string()),
        Err(_) => Ok(String::new()),
    }
}

impl PlayerController for Engine {
    fn control(&self, p: &Profile, w: &Window, key: KeyName) -> DeckResult<()> {
        let Some(aid) = p.uia.get(&key) else {
            return Ok(()); // no element mapped: acked no-op
        };
        if aid.is_empty() {
            return Ok(());
        }
        self.call(Op::Invoke, w.handle, aid).map(|_| ())
    }
}

impl ElementNamer for Engine {
    fn name(&self, hwnd: usize, automation_id: &str) -> DeckResult<String> {
        self.call(Op::Name, hwnd, automation_id)
    }
}
