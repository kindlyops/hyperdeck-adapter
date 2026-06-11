package domain

// TransportState is the modeled (open-loop) play state of the deck.
type TransportState int

const (
	StateStopped TransportState = iota
	StatePlaying
)

// HyperDeckStatus maps the modeled state to a HyperDeck transport "status" value.
func (s TransportState) HyperDeckStatus() string {
	if s == StatePlaying {
		return "play"
	}
	return "stopped"
}

// TransportInfo is the payload of a "transport info" response.
type TransportInfo struct {
	Status string
	Speed  int
	ClipID int
	SlotID int
}

// SlotInfo is the payload of a "slot info" response.
type SlotInfo struct {
	Present bool
	SlotID  int
}

// DeviceInfo is the payload of a "device info" response.
type DeviceInfo struct {
	ProtocolVersion string
	Model           string
	UniqueID        string
}
