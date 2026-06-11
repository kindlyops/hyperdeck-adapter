package app

import (
	"testing"

	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/injector"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

func TestTransportInfoReflectsState(t *testing.T) {
	m := injector.NewMock()
	s := lockedSession(discreteProfile(), domain.ClipList{{ID: 1}, {ID: 2}})
	d := NewVirtualDeck(s, m)
	_ = d.Play()
	ti := d.TransportInfo()
	if ti.Status != "play" {
		t.Errorf("status = %q, want play", ti.Status)
	}
}

func TestSlotInfoTracksLock(t *testing.T) {
	m := injector.NewMock()
	s := NewSession()
	d := NewVirtualDeck(s, m)
	if d.SlotInfo().Present {
		t.Error("unlocked slot should be absent")
	}
	s.Lock(discreteProfile(), domain.Window{}, nil, nil)
	if !d.SlotInfo().Present {
		t.Error("locked slot should be present")
	}
}

func TestDeviceInfoStable(t *testing.T) {
	d := NewVirtualDeck(NewSession(), injector.NewMock())
	if d.DeviceInfo().Model == "" || d.DeviceInfo().ProtocolVersion == "" {
		t.Error("device info must be populated")
	}
}
