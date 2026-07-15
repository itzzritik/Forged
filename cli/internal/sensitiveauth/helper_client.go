package sensitiveauth

import (
	"bufio"
	"context"
	"encoding/base64"
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
	logger           *slog.Logger
	path             string
	run              *helperRun
	stdin            *bufio.Writer
	stdinPipe        io.WriteCloser
	responses        map[string]chan HelperResponse
	onLock           func()
	onExit           func()
	intentionalClose bool
	terminalNotified bool
	mu               sync.Mutex
	nextID           atomic.Uint64
}

type helperRun struct {
	cmd      *exec.Cmd
	done     chan struct{}
	waitOnce sync.Once
	waitErr  error
}

func (r *helperRun) reap() {
	r.waitOnce.Do(func() {
		r.waitErr = r.cmd.Wait()
		close(r.done)
	})
}

func NewHelperClient(path string, logger *slog.Logger) *HelperClient {
	return &HelperClient{
		logger:    logger,
		path:      path,
		responses: map[string]chan HelperResponse{},
	}
}

func (c *HelperClient) Start(ctx context.Context, onLock, onExit func()) error {
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
	run := &helperRun{cmd: cmd, done: make(chan struct{})}

	c.mu.Lock()
	c.run = run
	c.stdin = bufio.NewWriter(stdin)
	c.stdinPipe = stdin
	c.onLock = onLock
	c.onExit = onExit
	c.intentionalClose = false
	c.terminalNotified = false
	c.mu.Unlock()

	go c.readLoop(bufio.NewScanner(stdout), run)

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
	c.intentionalClose = true
	run := c.run
	stdin := c.stdinPipe
	for id := range c.responses {
		_ = c.writeRequestLocked(NewCancelRequest(id))
	}
	c.stdin = nil
	c.stdinPipe = nil
	for id, ch := range c.responses {
		delete(c.responses, id)
		close(ch)
	}
	c.mu.Unlock()
	if stdin != nil {
		_ = stdin.Close()
	}

	if run == nil || run.cmd.Process == nil {
		return nil
	}
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-run.done:
		return run.waitErr
	case <-timer.C:
	}
	_ = run.cmd.Process.Kill()
	<-run.done
	return nil
}

func (c *HelperClient) Authorize(ctx context.Context, action Action) (CapabilityState, error) {
	resp, err := c.do(ctx, NewAuthorizeRequest(c.id(), action))
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return CapabilityBroken, ctxErr
		}
		return CapabilityBroken, ErrNativeBroken
	}

	capability := capabilityFromHelperStatus(resp.Status)
	switch resp.Status {
	case helperStatusOK:
		return capability, nil
	case helperStatusCanceled:
		return capability, ErrAuthenticationCanceled
	case helperStatusUnavailablePlatform, helperStatusUnavailableEnv, helperStatusUnavailable:
		return capability, ErrNativeUnavailable
	case helperStatusBroken:
		return capability, ErrNativeBroken
	default:
		return capability, ErrAuthenticationFailed
	}
}

// CollectPassword shows a native master-password popup and returns the entered
// bytes. Caller owns and must zero the returned slice.
func (c *HelperClient) CollectPassword(ctx context.Context, reason string) ([]byte, error) {
	resp, err := c.do(ctx, NewCollectPasswordRequest(c.id(), reason))
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, ErrNativeBroken
	}

	switch resp.Status {
	case helperStatusOK:
		if resp.Secret == "" {
			return nil, ErrAuthenticationFailed
		}
		secret, decErr := base64.StdEncoding.DecodeString(resp.Secret)
		if decErr != nil || len(secret) == 0 {
			return nil, ErrAuthenticationFailed
		}
		return secret, nil
	case helperStatusCanceled:
		return nil, ErrAuthenticationCanceled
	case helperStatusUnavailablePlatform, helperStatusUnavailableEnv, helperStatusUnavailable:
		return nil, ErrNativeUnavailable
	default:
		return nil, ErrAuthenticationFailed
	}
}

func (c *HelperClient) Capability(ctx context.Context) (CapabilityState, error) {
	resp, err := c.do(ctx, NewStatusRequest(c.id()))
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return CapabilityBroken, ctxErr
		}
		return CapabilityBroken, ErrNativeBroken
	}
	return capabilityFromHelperStatus(resp.Status), nil
}

func (c *HelperClient) do(ctx context.Context, req HelperRequest) (HelperResponse, error) {
	if err := ctx.Err(); err != nil {
		return HelperResponse{}, err
	}
	ch := make(chan HelperResponse, 1)

	c.mu.Lock()
	c.responses[req.ID] = ch
	if err := c.writeRequestLocked(req); err != nil {
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
		_, pending := c.responses[req.ID]
		if pending {
			delete(c.responses, req.ID)
			_ = c.writeRequestLocked(NewCancelRequest(req.ID))
		}
		c.mu.Unlock()
		if !pending {
			if resp, ok := <-ch; ok {
				return resp, nil
			}
		}
		return HelperResponse{}, ctx.Err()
	}
}

func (c *HelperClient) writeRequestLocked(req HelperRequest) error {
	if c.stdin == nil {
		return ErrNativeUnavailable
	}
	if err := json.NewEncoder(c.stdin).Encode(req); err != nil {
		return err
	}
	return c.stdin.Flush()
}

func (c *HelperClient) readLoop(scanner *bufio.Scanner, run *helperRun) {
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
	if c.run != run {
		c.mu.Unlock()
		run.reap()
		return
	}
	for id, ch := range c.responses {
		delete(c.responses, id)
		close(ch)
	}
	c.stdin = nil
	c.stdinPipe = nil
	onExit := c.onExit
	unexpectedExit := !c.intentionalClose && !c.terminalNotified
	c.terminalNotified = true
	c.mu.Unlock()
	if unexpectedExit {
		_ = run.cmd.Process.Kill()
	}
	run.reap()
	if unexpectedExit && onExit != nil {
		onExit()
	}
	c.mu.Lock()
	if c.run == run {
		c.run = nil
	}
	c.mu.Unlock()
}

func (c *HelperClient) id() string {
	return fmt.Sprintf("req-%d", c.nextID.Add(1))
}

func capabilityFromHelperStatus(status string) CapabilityState {
	switch status {
	case helperStatusOK:
		return CapabilityAvailable
	case helperStatusUnavailablePlatform:
		return CapabilityUnavailableByPlatform
	case helperStatusUnavailableEnv, helperStatusUnavailable:
		return CapabilityUnavailableByEnv
	case helperStatusBroken:
		return CapabilityBroken
	default:
		return CapabilityBroken
	}
}
