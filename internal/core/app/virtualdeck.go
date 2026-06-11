package app

import (
	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

// VirtualDeck implements port.Transport and port.Query over a Session.
type VirtualDeck struct {
	session  *Session
	injector port.KeyInjector
	device   domain.DeviceInfo
}

// NewVirtualDeck wires the deck to its shared session and key injector.
func NewVirtualDeck(s *Session, inj port.KeyInjector) *VirtualDeck {
	return &VirtualDeck{
		session:  s,
		injector: inj,
		device: domain.DeviceInfo{
			ProtocolVersion: "1.11",
			Model:           "HyperDeck Studio Mini",
			UniqueID:        "hyperdeck-adapter",
		},
	}
}

// Play moves the deck to the playing state.
func (d *VirtualDeck) Play() error {
	p, w, ok := d.session.Active()
	if !ok {
		return ErrNotLocked
	}
	if p.PlayStopToggle && d.session.State() == domain.StatePlaying {
		return nil
	}
	d.session.SetState(domain.StatePlaying)
	return d.send(p, w, domain.KeyPlay)
}

// Stop moves the deck to the stopped state.
func (d *VirtualDeck) Stop() error {
	p, w, ok := d.session.Active()
	if !ok {
		return ErrNotLocked
	}
	if p.PlayStopToggle {
		if d.session.State() != domain.StatePlaying {
			return nil
		}
		d.session.SetState(domain.StateStopped)
		return d.send(p, w, domain.KeyPlay) // toggle shares the play key
	}
	d.session.SetState(domain.StateStopped)
	return d.send(p, w, domain.KeyStop)
}

// Record sends the record key if the profile defines one; otherwise no-op.
func (d *VirtualDeck) Record() error {
	p, w, ok := d.session.Active()
	if !ok {
		return ErrNotLocked
	}
	return d.send(p, w, domain.KeyRecord)
}

// Goto navigates to a 1-based clip id via repeated next/prev keys.
func (d *VirtualDeck) Goto(clipID int) error {
	p, w, ok := d.session.Active()
	if !ok {
		return ErrNotLocked
	}
	target := clamp(clipID, 1, max(1, d.session.Clips().Len()))
	delta := target - d.session.CurrentClip()
	key := domain.KeyNext
	if delta < 0 {
		key = domain.KeyPrev
		delta = -delta
	}
	for i := 0; i < delta; i++ {
		if err := d.send(p, w, key); err != nil {
			return err
		}
	}
	d.session.SetCurrentClip(target)
	return nil
}

// Next advances one clip.
func (d *VirtualDeck) Next() error { return d.step(domain.KeyNext, +1) }

// Prev rewinds one clip.
func (d *VirtualDeck) Prev() error { return d.step(domain.KeyPrev, -1) }

// Rehome runs the profile's homing sequence and resets modeled state.
func (d *VirtualDeck) Rehome() error {
	p, w, ok := d.session.Active()
	if !ok {
		return ErrNotLocked
	}
	if p.Injection == domain.InjectionFocus {
		if err := d.injector.Focus(w); err != nil {
			return err
		}
	}
	if len(p.Homing) > 0 {
		if err := d.injector.SendKeys(w, p.Homing); err != nil {
			return err
		}
	}
	d.session.SetState(domain.StateStopped)
	d.session.SetCurrentClip(1)
	return nil
}

func (d *VirtualDeck) step(key domain.KeyName, delta int) error {
	p, w, ok := d.session.Active()
	if !ok {
		return ErrNotLocked
	}
	n := clamp(d.session.CurrentClip()+delta, 1, max(1, d.session.Clips().Len()))
	if err := d.send(p, w, key); err != nil {
		return err
	}
	d.session.SetCurrentClip(n)
	return nil
}

func (d *VirtualDeck) send(p domain.Profile, w domain.Window, key domain.KeyName) error {
	chord, ok := p.Keymap[key]
	if !ok {
		return nil // unmapped action -> acked no-op
	}
	if p.Injection == domain.InjectionFocus {
		if err := d.injector.Focus(w); err != nil {
			return err
		}
	}
	return d.injector.SendKeys(w, []domain.Chord{chord})
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// TransportInfo reports the modeled transport state.
func (d *VirtualDeck) TransportInfo() domain.TransportInfo {
	speed := 0
	if d.session.State() == domain.StatePlaying {
		speed = 100
	}
	return domain.TransportInfo{
		Status: d.session.State().HyperDeckStatus(),
		Speed:  speed,
		ClipID: d.session.CurrentClip(),
		SlotID: 1,
	}
}

// Clips returns the active clip list.
func (d *VirtualDeck) Clips() domain.ClipList {
	return d.session.Clips()
}

// SlotInfo reports a present slot when a player is locked.
func (d *VirtualDeck) SlotInfo() domain.SlotInfo {
	_, _, ok := d.session.Active()
	return domain.SlotInfo{Present: ok, SlotID: 1}
}

// DeviceInfo reports the emulated deck identity.
func (d *VirtualDeck) DeviceInfo() domain.DeviceInfo {
	return d.device
}

var (
	_ port.Transport = (*VirtualDeck)(nil)
	_ port.Query     = (*VirtualDeck)(nil)
)
