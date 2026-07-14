package ipc

import (
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
	conn, err := platform.Dial(c.socketPath, 2*time.Second)
	if err != nil {
		return Response{}, fmt.Errorf("%w. Open Forged to start it", ErrDaemonNotRunning)
	}
	defer conn.Close()

	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	conn.SetDeadline(time.Now().Add(timeout))

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
		return Response{}, fmt.Errorf("Sending request: %w", err)
	}

	var resp Response
	if err := ReadMessage(conn, &resp); err != nil {
		return Response{}, fmt.Errorf("Reading response: %w", err)
	}

	if resp.Status == "error" {
		return resp, fmt.Errorf("%s", resp.Error)
	}

	return resp, nil
}
