//go:build darwin

#import <Cocoa/Cocoa.h>
#import <CoreGraphics/CoreGraphics.h>
#import <ApplicationServices/ApplicationServices.h>

#include <unistd.h>

#include "cinject_darwin.h"

int hdListWindows(HDWindow *out, int max) {
  CFArrayRef windows = CGWindowListCopyWindowInfo(
      kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements,
      kCGNullWindowID);
  if (!windows) {
    return 0;
  }
  CFIndex n = CFArrayGetCount(windows);
  int count = 0;
  for (CFIndex i = 0; i < n && count < max; i++) {
    CFDictionaryRef d = (CFDictionaryRef)CFArrayGetValueAtIndex(windows, i);

    HDWindow *w = &out[count];
    w->pid = 0;
    w->windowNumber = 0;
    w->owner[0] = '\0';
    w->title[0] = '\0';

    CFNumberRef pidNum = (CFNumberRef)CFDictionaryGetValue(d, kCGWindowOwnerPID);
    if (pidNum) {
      CFNumberGetValue(pidNum, kCFNumberSInt64Type, &w->pid);
    }
    CFNumberRef winNum = (CFNumberRef)CFDictionaryGetValue(d, kCGWindowNumber);
    if (winNum) {
      CFNumberGetValue(winNum, kCFNumberSInt64Type, &w->windowNumber);
    }
    CFStringRef owner = (CFStringRef)CFDictionaryGetValue(d, kCGWindowOwnerName);
    if (owner) {
      CFStringGetCString(owner, w->owner, sizeof(w->owner), kCFStringEncodingUTF8);
    }
    CFStringRef title = (CFStringRef)CFDictionaryGetValue(d, kCGWindowName);
    if (title) {
      CFStringGetCString(title, w->title, sizeof(w->title), kCFStringEncodingUTF8);
    }
    count++;
  }
  CFRelease(windows);
  return count;
}

void hdPostKeyToPid(int64_t pid, uint16_t keycode, uint64_t flags) {
  CGEventSourceRef src = CGEventSourceCreate(kCGEventSourceStateHIDSystemState);
  CGEventRef down = CGEventCreateKeyboardEvent(src, (CGKeyCode)keycode, true);
  CGEventRef up = CGEventCreateKeyboardEvent(src, (CGKeyCode)keycode, false);
  if (flags != 0) {
    CGEventSetFlags(down, (CGEventFlags)flags);
    CGEventSetFlags(up, (CGEventFlags)flags);
  }
  CGEventPostToPid((pid_t)pid, down);
  usleep(12000); // 12ms key-hold so the target registers the press
  CGEventPostToPid((pid_t)pid, up);
  if (down) CFRelease(down);
  if (up) CFRelease(up);
  if (src) CFRelease(src);
}

int hdActivatePID(int64_t pid) {
  NSRunningApplication *app =
      [NSRunningApplication runningApplicationWithProcessIdentifier:(pid_t)pid];
  if (!app) {
    return 0;
  }
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
  BOOL ok = [app activateWithOptions:NSApplicationActivateIgnoringOtherApps];
#pragma clang diagnostic pop
  return ok ? 1 : 0;
}

int hdAXTrusted(void) { return AXIsProcessTrusted() ? 1 : 0; }

int hdAXPrompt(void) {
  const void *keys[] = {kAXTrustedCheckOptionPrompt};
  const void *values[] = {kCFBooleanTrue};
  CFDictionaryRef opts =
      CFDictionaryCreate(NULL, keys, values, 1,
                         &kCFCopyStringDictionaryKeyCallBacks,
                         &kCFTypeDictionaryValueCallBacks);
  Boolean trusted = AXIsProcessTrustedWithOptions(opts);
  CFRelease(opts);
  return trusted ? 1 : 0;
}
