//go:build windows

package platform

import (
	"fmt"

	"github.com/Microsoft/go-winio"
)

// Pipe paths kept here for callers that still hard-code them. New code should
// read these from config.Paths instead, but these constants stay as a
// fallback for parity with the old CtlPipeName / AgentPipeName references.
const (
	AgentPipeName = `\\.\pipe\forged-agent`
	CtlPipeName   = `\\.\pipe\forged-ctl`
)

func IsSocketAlive(path string) bool {
	conn, err := winio.DialPipe(path, nil)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// CleanStaleSocket reports whether a pipe with the same name is currently
// being served by another process. Named pipes auto-clean on close, so
// there is nothing to remove — we only need to refuse to start if the name
// is taken.
func CleanStaleSocket(path string) error {
	if IsSocketAlive(path) {
		return fmt.Errorf("Pipe %s is in use by another process", path)
	}
	return nil
}
