//go:build e2e && windows

package e2e

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/itzzritik/forged/cli/internal/platform"
	"golang.org/x/sys/windows"
)

// TestPipeIdentityChecks covers the client-side pipe checks: a pipe created
// the way the daemon creates it is accepted, one a sandboxed (Low
// integrity) process of the same user could create is rejected.
func TestPipeIdentityChecks(t *testing.T) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sid := user.User.Sid.String()

	good := `\\.\pipe\forged-e2e-good-` + randomHex(6)
	ln, err := platform.Listen(good)
	if err != nil {
		t.Fatalf("platform.Listen: %v", err)
	}
	defer ln.Close()
	go acceptAndClose(ln)
	conn, err := platform.Dial(good, 2*time.Second)
	if err != nil {
		t.Fatalf("dial daemon-style pipe: %v", err)
	}
	conn.Close()

	low := `\\.\pipe\forged-e2e-low-` + randomHex(6)
	lowLn, err := winio.ListenPipe(low, &winio.PipeConfig{
		SecurityDescriptor: "O:" + sid + "D:P(A;;GA;;;" + sid + ")S:(ML;;NW;;;LW)",
	})
	if err != nil {
		t.Fatalf("creating low-integrity pipe: %v", err)
	}
	defer lowLn.Close()
	go acceptAndClose(lowLn)
	if conn, err := platform.Dial(low, 2*time.Second); err == nil {
		conn.Close()
		t.Fatal("dial accepted a pipe labeled Low integrity")
	} else if !errors.Is(err, platform.ErrPeerIdentityMismatch) {
		t.Fatalf("low-integrity pipe rejected with %v, want ErrPeerIdentityMismatch", err)
	}
}

func acceptAndClose(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		conn.Close()
	}
}
