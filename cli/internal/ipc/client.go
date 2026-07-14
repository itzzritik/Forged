package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/itzzritik/forged/cli/internal/platform"
)

var ErrDaemonNotRunning = errors.New("Daemon is not running")

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
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Response{}, fmt.Errorf("Connecting to daemon: %w", ctxErr)
		}
		return Response{}, fmt.Errorf("%w. Open Forged to start it", ErrDaemonNotRunning)
	}
	defer conn.Close()

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
