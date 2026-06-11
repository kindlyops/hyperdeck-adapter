//go:build !windows && !darwin

package injector

import "testing"

func TestNoopInjectorSatisfiesInterface(t *testing.T) {
	inj, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if err := inj.SendKeys(injectorWindow(), nil); err != nil {
		t.Errorf("noop send should succeed: %v", err)
	}
}
