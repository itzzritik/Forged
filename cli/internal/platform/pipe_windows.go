//go:build windows

package platform

import (
	"fmt"
	"net"
	"time"

	"github.com/Microsoft/go-winio"
)

const (
	AgentPipeName = `\\.\pipe\forged-agent`
	CtlPipeName   = `\\.\pipe\forged-ctl`
)

func ListenPipe(name string) (net.Listener, error) {
	cfg := &winio.PipeConfig{
		SecurityDescriptor: "D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GA;;;OW)",
		InputBufferSize:    65536,
		OutputBufferSize:   65536,
	}

	ln, err := winio.ListenPipe(name, cfg)
	if err != nil {
		return nil, fmt.Errorf("listening on pipe %s: %w", name, err)
	}
	return ln, nil
}

func IsSocketAlive(path string) bool {
	conn, err := winio.DialPipe(path, nil)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func CleanStaleSocket(path string) error {
	if IsSocketAlive(path) {
		return fmt.Errorf("pipe %s is in use by another process", path)
	}
	return nil
}

func DialPipe(name string) (net.Conn, error) {
	timeout := 2 * time.Second
	return winio.DialPipe(name, &timeout)
}
