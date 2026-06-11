// Package port declares the hexagon's driving and driven port interfaces.
package port

import "github.com/kindlyops/hyperdeck-adapter/internal/core/domain"

// Transport is a driving (inbound) port: the deck's command surface.
type Transport interface {
	Play() error
	Stop() error
	Record() error
	Goto(clipID int) error
	Next() error
	Prev() error
	Rehome() error
}

// Query is a driving (inbound) port: the deck's read surface.
type Query interface {
	TransportInfo() domain.TransportInfo
	Clips() domain.ClipList
	SlotInfo() domain.SlotInfo
	DeviceInfo() domain.DeviceInfo
}
