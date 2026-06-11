package hyperdeck

import (
	"bufio"
	"fmt"
	"net"
	"strings"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

// Server accepts controller connections and serves the protocol.
type Server struct {
	responder *Responder
}

// NewServer wires a server to the inbound ports.
func NewServer(t port.Transport, q port.Query) *Server {
	return &Server{responder: NewResponder(t, q)}
}

// Serve accepts connections on ln until it is closed.
func (s *Server) Serve(ln net.Listener) error {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	// Greeting banner.
	fmt.Fprintf(conn, "%d connection info:\r\nprotocol version: 1.11\r\nmodel: HyperDeck Studio Mini\r\n\r\n", CodeConnectionInfo)

	rd := bufio.NewReader(conn)
	for {
		block, err := readCommandBlock(rd)
		if err != nil {
			return
		}
		if strings.TrimSpace(block) == "" {
			continue
		}
		cmd, perr := ParseCommand(block)
		if perr != nil {
			fmt.Fprintf(conn, "%d syntax error\r\n", CodeSyntaxError)
			continue
		}
		if _, err := conn.Write([]byte(s.responder.Handle(cmd))); err != nil {
			return
		}
		if cmd.Name == "quit" {
			return
		}
	}
}

// readCommandBlock reads one command: a single line, or — when the first line
// ends with ':' — lines up to and including a terminating blank line.
func readCommandBlock(rd *bufio.Reader) (string, error) {
	first, err := rd.ReadString('\n')
	if err != nil {
		return "", err
	}
	trimmed := strings.TrimRight(first, "\r\n")
	if !strings.HasSuffix(trimmed, ":") {
		return first, nil
	}
	var b strings.Builder
	b.WriteString(first)
	for {
		line, err := rd.ReadString('\n')
		if err != nil {
			return "", err
		}
		b.WriteString(line)
		if strings.TrimSpace(line) == "" {
			return b.String(), nil
		}
	}
}
