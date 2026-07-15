package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/platform"
)

var ErrDaemonNotRunning = errors.New("Daemon is not running")
var ErrDaemonIdentity = errors.New("Daemon identity could not be verified")

type Client struct {
	socketPath string
}

func NewClient(socketPath string) *Client {
	return &Client{socketPath: socketPath}
}

func (c *Client) Call(command string, args any) (Response, error) {
	return c.CallWithTimeout(command, args, 30*time.Second)
}

func (c *Client) CallWithTimeout(command string, args any, timeout time.Duration) (Response, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return c.CallContext(ctx, command, args)
}

func (c *Client) CallContext(ctx context.Context, command string, args any) (Response, error) {
	if err := ctx.Err(); err != nil {
		return Response{}, fmt.Errorf("Calling daemon: %w", err)
	}

	dialCtx, cancelDial := context.WithTimeout(ctx, 2*time.Second)
	conn, err := platform.DialContext(dialCtx, c.socketPath)
	cancelDial()
	if err != nil {
		if errors.Is(err, platform.ErrCurrentUserPipeIdentity) {
			return Response{}, fmt.Errorf("connecting to daemon: %w", err)
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Response{}, fmt.Errorf("Connecting to daemon: %w", ctxErr)
		}
		return Response{}, fmt.Errorf("%w. Open Forged to start it", ErrDaemonNotRunning)
	}
	defer conn.Close()
	if platform.ControlPeerCredentialsAvailable() {
		if err := verifyDaemonIdentity(conn, c.socketPath); err != nil {
			return Response{}, err
		}
	}

	stopClose := context.AfterFunc(ctx, func() {
		_ = conn.Close()
	})
	defer stopClose()

	var rawArgs json.RawMessage
	if args != nil {
		b, err := json.Marshal(args)
		if err != nil {
			clear(b)
			return Response{}, fmt.Errorf("Marshaling args: %w", err)
		}
		rawArgs = b
		args = nil
	}

	req := Request{Command: command, Args: rawArgs}
	err = WriteMessage(conn, req)
	clear(rawArgs)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Response{}, fmt.Errorf("Sending request: %w", ctxErr)
		}
		return Response{}, fmt.Errorf("Sending request: %w", err)
	}

	var resp Response
	if err := ReadMessage(conn, &resp); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Response{}, fmt.Errorf("Reading response: %w", ctxErr)
		}
		return Response{}, fmt.Errorf("Reading response: %w", err)
	}

	if resp.Status == "error" {
		return resp, fmt.Errorf("%s", resp.Error)
	}

	return resp, nil
}

func verifyDaemonIdentity(conn net.Conn, socketPath string) error {
	pid, err := readDaemonPID(socketPath)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDaemonIdentity, err)
	}
	if err := platform.VerifyDaemonPeer(conn, pid); err != nil {
		return fmt.Errorf("%w: %w", ErrDaemonIdentity, err)
	}
	currentPID, err := readDaemonPID(socketPath)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDaemonIdentity, err)
	}
	if currentPID != pid {
		return fmt.Errorf("%w: daemon restarted during verification", ErrDaemonIdentity)
	}
	return nil
}

func readDaemonPID(socketPath string) (int, error) {
	data, err := os.ReadFile(filepath.Join(filepath.Dir(socketPath), "daemon.pid"))
	if err != nil {
		return 0, fmt.Errorf("reading daemon pid: %w", err)
	}
	defer clear(data)
	pID, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pID <= 0 {
		return 0, fmt.Errorf("invalid daemon pid")
	}
	return pID, nil
}
