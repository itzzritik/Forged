package sensitiveauth

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

type HelperClient struct {
	logger    *slog.Logger
	path      string
	cmd       *exec.Cmd
	stdin     *bufio.Writer
	responses map[string]chan HelperResponse
	onLock    func()
	mu        sync.Mutex
	nextID    atomic.Uint64
}

func NewHelperClient(path string, logger *slog.Logger) *HelperClient {
	return &HelperClient{
		logger:    logger,
		path:      path,
		responses: map[string]chan HelperResponse{},
	}
}

func (c *HelperClient) Start(ctx context.Context, onLock func()) error {
	cmd := exec.CommandContext(ctx, c.path)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return err
	}

	c.cmd = cmd
	c.stdin = bufio.NewWriter(stdin)
	c.onLock = onLock

	go c.readLoop(bufio.NewScanner(stdout))

	subscribeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if _, err := c.do(subscribeCtx, NewSubscribeLocksRequest(c.id())); err != nil {
		_ = c.Close()
		return err
	}

	return nil
}

func (c *HelperClient) Close() error {
	c.mu.Lock()
	cmd := c.cmd
	c.cmd = nil
	c.stdin = nil
	for id, ch := range c.responses {
		delete(c.responses, id)
		close(ch)
	}
	c.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if err := cmd.Process.Kill(); err != nil {
		return err
	}
	_, _ = cmd.Process.Wait()
	return nil
}

func (c *HelperClient) Authorize(ctx context.Context, action Action) error {
	resp, err := c.do(ctx, NewAuthorizeRequest(c.id(), action))
	if err != nil {
		return ErrNativeUnavailable
	}

	switch resp.Status {
	case helperStatusOK:
		return nil
	case helperStatusCanceled:
		return ErrAuthenticationCanceled
	case helperStatusUnavailable:
		return ErrNativeUnavailable
	default:
		return ErrAuthenticationFailed
	}
}

func (c *HelperClient) do(ctx context.Context, req HelperRequest) (HelperResponse, error) {
	ch := make(chan HelperResponse, 1)

	c.mu.Lock()
	if c.stdin == nil {
		c.mu.Unlock()
		return HelperResponse{}, ErrNativeUnavailable
	}
	c.responses[req.ID] = ch
	if err := json.NewEncoder(c.stdin).Encode(req); err != nil {
		delete(c.responses, req.ID)
		c.mu.Unlock()
		return HelperResponse{}, err
	}
	if err := c.stdin.Flush(); err != nil {
		delete(c.responses, req.ID)
		c.mu.Unlock()
		return HelperResponse{}, err
	}
	c.mu.Unlock()

	select {
	case resp, ok := <-ch:
		if !ok {
			return HelperResponse{}, ErrNativeUnavailable
		}
		return resp, nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.responses, req.ID)
		c.mu.Unlock()
		return HelperResponse{}, ctx.Err()
	}
}

func (c *HelperClient) readLoop(scanner *bufio.Scanner) {
	for scanner.Scan() {
		var resp HelperResponse
		if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
			continue
		}

		if resp.Type == helperTypeEvent && resp.Status == helperEventSessionLocked {
			if c.onLock != nil {
				c.onLock()
			}
			continue
		}

		c.mu.Lock()
		ch := c.responses[resp.ID]
		delete(c.responses, resp.ID)
		c.mu.Unlock()
		if ch != nil {
			ch <- resp
		}
	}

	c.mu.Lock()
	for id, ch := range c.responses {
		delete(c.responses, id)
		close(ch)
	}
	c.mu.Unlock()
}

func (c *HelperClient) id() string {
	return fmt.Sprintf("req-%d", c.nextID.Add(1))
}
