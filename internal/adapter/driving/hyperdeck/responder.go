package hyperdeck

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

// Responder turns parsed commands into port calls and formatted responses.
type Responder struct {
	transport port.Transport
	query     port.Query
}

// NewResponder wires a responder to the inbound ports.
func NewResponder(t port.Transport, q port.Query) *Responder {
	return &Responder{transport: t, query: q}
}

// Handle executes one command and returns the full response text.
func (r *Responder) Handle(cmd Command) string {
	switch cmd.Name {
	case "ping":
		return ack()
	case "play":
		return r.ackErr(r.transport.Play())
	case "stop":
		return r.ackErr(r.transport.Stop())
	case "record":
		return r.ackErr(r.transport.Record())
	case "goto":
		return r.handleGoto(cmd)
	case "transport info":
		return r.transportInfo()
	case "clips get":
		return r.clips()
	case "slot info":
		return r.slotInfo()
	case "device info":
		return r.deviceInfo()
	case "notify", "remote", "configuration":
		return ack() // accepted; async emission handled by the server
	case "quit":
		return ack()
	default:
		return fmt.Sprintf("%d syntax error\r\n", CodeSyntaxError)
	}
}

func (r *Responder) handleGoto(cmd Command) string {
	idStr, ok := cmd.Params["clip id"]
	if !ok {
		return fmt.Sprintf("%d syntax error\r\n", CodeSyntaxError)
	}
	id, err := strconv.Atoi(strings.TrimPrefix(idStr, "+"))
	if err != nil {
		return fmt.Sprintf("%d syntax error\r\n", CodeSyntaxError)
	}
	return r.ackErr(r.transport.Goto(id))
}

func (r *Responder) transportInfo() string {
	ti := r.query.TransportInfo()
	var b strings.Builder
	fmt.Fprintf(&b, "%d transport info:\r\n", CodeTransportInfo)
	fmt.Fprintf(&b, "status: %s\r\n", ti.Status)
	fmt.Fprintf(&b, "speed: %d\r\n", ti.Speed)
	fmt.Fprintf(&b, "clip id: %d\r\n", ti.ClipID)
	fmt.Fprintf(&b, "slot id: %d\r\n", ti.SlotID)
	b.WriteString("\r\n")
	return b.String()
}

func (r *Responder) clips() string {
	clips := r.query.Clips()
	var b strings.Builder
	fmt.Fprintf(&b, "%d clips info:\r\n", CodeClipsInfo)
	fmt.Fprintf(&b, "clip count: %d\r\n", len(clips))
	for _, c := range clips {
		fmt.Fprintf(&b, "%d: %s %s %s\r\n", c.ID, c.Name, c.Timecode, c.Duration)
	}
	b.WriteString("\r\n")
	return b.String()
}

func (r *Responder) slotInfo() string {
	si := r.query.SlotInfo()
	status := "mounted"
	if !si.Present {
		status = "empty"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d slot info:\r\n", CodeSlotInfo)
	fmt.Fprintf(&b, "slot id: %d\r\n", si.SlotID)
	fmt.Fprintf(&b, "status: %s\r\n", status)
	b.WriteString("\r\n")
	return b.String()
}

func (r *Responder) deviceInfo() string {
	di := r.query.DeviceInfo()
	var b strings.Builder
	fmt.Fprintf(&b, "%d device info:\r\n", CodeDeviceInfo)
	fmt.Fprintf(&b, "protocol version: %s\r\n", di.ProtocolVersion)
	fmt.Fprintf(&b, "model: %s\r\n", di.Model)
	fmt.Fprintf(&b, "unique id: %s\r\n", di.UniqueID)
	b.WriteString("\r\n")
	return b.String()
}

func (r *Responder) ackErr(err error) string {
	if err != nil {
		return fmt.Sprintf("%d invalid state\r\n", CodeInvalidState)
	}
	return ack()
}

func ack() string { return fmt.Sprintf("%d ok\r\n", CodeOK) }
