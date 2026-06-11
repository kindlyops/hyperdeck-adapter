// Package hyperdeck implements the HyperDeck Ethernet Protocol as a driving adapter.
package hyperdeck

// Response codes (HyperDeck Ethernet Protocol v1.11). Verify against the manual.
const (
	CodeOK             = 200
	CodeSlotInfo       = 202
	CodeDeviceInfo     = 204
	CodeClipsInfo      = 205
	CodeTransportInfo  = 208
	CodeConnectionInfo = 500
	CodeSyntaxError    = 100
	CodeInvalidState   = 150
)
