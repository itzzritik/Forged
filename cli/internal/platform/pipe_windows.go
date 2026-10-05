//go:build windows

package platform

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

var currentUserPipes struct {
	once  sync.Once
	sid   *windows.SID
	agent string
	ctl   string
	err   error
}

// CurrentUserPipePaths returns opaque, per-user named-pipe paths derived from
// the current process token. The SID is never exposed in a pipe name.
func CurrentUserPipePaths() (agent, ctl string, err error) {
	currentUserPipes.once.Do(func() {
		token := windows.GetCurrentProcessToken()
		user, tokenErr := token.GetTokenUser()
		if tokenErr != nil {
			currentUserPipes.err = fmt.Errorf("%w: reading process token user: %v", ErrCurrentUserPipeIdentity, tokenErr)
			return
		}
		if user == nil || user.User.Sid == nil {
			currentUserPipes.err = fmt.Errorf("%w: process token has no user SID", ErrCurrentUserPipeIdentity)
			return
		}

		copied, copyErr := user.User.Sid.Copy()
		sid := user.User.Sid.String()
		if copyErr != nil || sid == "" {
			currentUserPipes.err = fmt.Errorf("%w: converting process token SID", ErrCurrentUserPipeIdentity)
			return
		}
		currentUserPipes.sid = copied
		currentUserPipes.agent = userPipePath("agent", sid)
		currentUserPipes.ctl = userPipePath("ctl", sid)
	})
	return currentUserPipes.agent, currentUserPipes.ctl, currentUserPipes.err
}

func CurrentUserPipeIdentityError() error {
	_, _, err := CurrentUserPipePaths()
	return err
}

func currentUserSID() (*windows.SID, error) {
	if _, _, err := CurrentUserPipePaths(); err != nil {
		return nil, err
	}
	return currentUserPipes.sid, nil
}

func userPipePath(kind, sid string) string {
	digest := sha256.Sum256([]byte("forged/windows-pipe/v1/" + kind + "\x00" + sid))
	return `\\.\pipe\forged-` + kind + "-v1-" + hex.EncodeToString(digest[:])
}

func IsSocketAlive(path string) bool {
	timeout := 500 * time.Millisecond
	conn, err := winio.DialPipe(path, &timeout)
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
