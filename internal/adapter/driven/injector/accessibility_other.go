//go:build !darwin

package injector

// RequestAccessibility is a no-op on platforms without macOS Accessibility TCC;
// it reports that input is permitted.
func RequestAccessibility() bool { return true }
