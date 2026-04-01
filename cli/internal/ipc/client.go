package ipc

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

type Client struct {
	socketPath string
}

func NewClient(socketPath string) *Client {
	return &Client{socketPath: socketPath}
}

func (c *Client) Call(command string, args any) (Response, error) {
	conn, err := net.DialTimeout("unix", c.socketPath, 2*time.Second)
	if err != nil {
		return Response{}, fmt.Errorf("daemon is not running. Start it with: forged start")
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(30 * time.Second))

	var rawArgs json.RawMessage
	if args != nil {
		b, err := json.Marshal(args)
		if err != nil {
			return Response{}, fmt.Errorf("marshaling args: %w", err)
		}
		rawArgs = b
	}

	req := Request{Command: command, Args: rawArgs}
	if err := WriteMessage(conn, req); err != nil {
		return Response{}, fmt.Errorf("sending request: %w", err)
	}

	var resp Response
	if err := ReadMessage(conn, &resp); err != nil {
		return Response{}, fmt.Errorf("reading response: %w", err)
	}

	if resp.Status == "error" {
		return resp, fmt.Errorf("%s", resp.Error)
	}

	return resp, nil
}
