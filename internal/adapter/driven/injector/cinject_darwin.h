#ifndef CINJECT_DARWIN_H
#define CINJECT_DARWIN_H

#include <stdint.h>

// HDWindow is a flattened, Go-friendly view of one on-screen window.
typedef struct {
  int64_t pid;
  int64_t windowNumber;
  char owner[256]; // owning application name (no Screen Recording perm needed)
  char title[256]; // window title; empty without Screen Recording permission
} HDWindow;

// hdListWindows fills up to max entries and returns the count written.
int hdListWindows(HDWindow *out, int max);

// hdPostKeyToPid synthesizes a key press (down then up) for the given CGKeyCode
// with the given CGEventFlags modifier mask, delivered directly to the target
// process. This does not require the process to be frontmost (background mode);
// the caller may still focus it first for foreground mode.
void hdPostKeyToPid(int64_t pid, uint16_t keycode, uint64_t flags);

// hdActivatePID brings the application with the given pid to the foreground.
// Returns 1 on success, 0 otherwise.
int hdActivatePID(int64_t pid);

// hdAXTrusted reports whether this process is trusted for Accessibility, which
// is required for synthesized keyboard events to be delivered. 1 = trusted.
int hdAXTrusted(void);

// hdAXPrompt is like hdAXTrusted but, when not trusted, asks macOS to show the
// permission dialog and add this process to the Accessibility list.
int hdAXPrompt(void);

#endif
