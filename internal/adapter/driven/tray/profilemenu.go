package tray

import "slices"

// checkedProfile returns the profile id whose menu entry should show a
// checkmark: active when it names a known profile, otherwise "" (the Auto entry).
func checkedProfile(profiles []string, active string) string {
	if active == "" || !slices.Contains(profiles, active) {
		return ""
	}
	return active
}
