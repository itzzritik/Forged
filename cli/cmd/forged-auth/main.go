//go:build linux || windows

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"sync"

	"github.com/itzzritik/forged/cli/internal/sensitiveauth"
)

type helperCall struct {
	cancel   context.CancelFunc
	canceled bool
	complete bool
}

type helperCalls struct {
	mu     sync.Mutex
	active map[string]*helperCall
	seen   map[string]struct{}
	closed bool
	wg     sync.WaitGroup
}

func newHelperCalls() *helperCalls {
	return &helperCalls{
		active: make(map[string]*helperCall),
		seen:   make(map[string]struct{}),
	}
}

func (c *helperCalls) Start(req sensitiveauth.HelperRequest, run func(context.Context) sensitiveauth.HelperResponse, emit func(sensitiveauth.HelperResponse)) {
	if req.ID == "" {
		emit(sensitiveauth.HelperResponse{Type: req.Type, Status: "failed", Provider: providerName(), Message: "missing request id"})
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	call := &helperCall{cancel: cancel}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		cancel()
		return
	}
	if _, exists := c.seen[req.ID]; exists {
		c.mu.Unlock()
		cancel()
		return
	}
	c.seen[req.ID] = struct{}{}
	c.active[req.ID] = call
	c.wg.Add(1)
	c.mu.Unlock()
	go func() {
		defer c.wg.Done()
		resp := run(ctx)
		cancel()

		c.mu.Lock()
		current := c.active[req.ID] == call
		canceled := call.canceled
		if current && !canceled {
			call.complete = true
		}
		c.mu.Unlock()
		if current && !canceled {
			emit(resp)
		}
		c.mu.Lock()
		if c.active[req.ID] == call {
			delete(c.active, req.ID)
		}
		c.mu.Unlock()
	}()
}

func (c *helperCalls) Cancel(id string) {
	c.mu.Lock()
	call := c.active[id]
	shouldCancel := call != nil && !call.complete
	if shouldCancel {
		call.canceled = true
	}
	c.mu.Unlock()
	if shouldCancel {
		call.cancel()
	}
}

func (c *helperCalls) Close() {
	c.mu.Lock()
	c.closed = true
	calls := make([]*helperCall, 0, len(c.active))
	for _, call := range c.active {
		if !call.complete {
			call.canceled = true
			calls = append(calls, call)
		}
	}
	c.mu.Unlock()
	for _, call := range calls {
		call.cancel()
	}
	c.wg.Wait()
}

func main() {
	var writeMu sync.Mutex
	emit := func(resp sensitiveauth.HelperResponse) {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = json.NewEncoder(os.Stdout).Encode(resp)
	}
	calls := newHelperCalls()
	helperCtx, stopHelper := context.WithCancel(context.Background())
	var background sync.WaitGroup
	defer func() {
		stopHelper()
		calls.Close()
		background.Wait()
	}()

	scanner := bufio.NewScanner(os.Stdin)
	background.Add(1)
	go func() {
		defer background.Done()
		startLockLoop(helperCtx, func() {
			emit(sensitiveauth.HelperResponse{
				Type:     "event",
				Status:   "session_locked",
				Provider: providerName(),
			})
		})
	}()

	for scanner.Scan() {
		var req sensitiveauth.HelperRequest
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			continue
		}

		switch req.Type {
		case "authorize":
			calls.Start(req, func(ctx context.Context) sensitiveauth.HelperResponse {
				action, err := sensitiveauth.ParseAction(req.Action)
				if err != nil {
					return sensitiveauth.HelperResponse{
						ID:       req.ID,
						Type:     req.Type,
						Status:   "failed",
						Provider: providerName(),
						Message:  err.Error(),
					}
				}
				return sensitiveauth.HelperResponse{
					ID:       req.ID,
					Type:     req.Type,
					Status:   authorize(ctx, action),
					Provider: providerName(),
				}
			}, emit)
		case "cancel":
			calls.Cancel(req.ID)
		case "collect-password":
			calls.Start(req, func(context.Context) sensitiveauth.HelperResponse {
				// TODO(windows): wire CredUIPromptForWindowsCredentials for a native
				// password popup. Until then, report unavailable so the broker falls
				// back to the "open Forged" deny message instead of hanging.
				return sensitiveauth.HelperResponse{
					ID:       req.ID,
					Type:     req.Type,
					Status:   "unavailable_by_platform",
					Provider: providerName(),
				}
			}, emit)
		case "subscribe-locks":
			calls.Start(req, func(context.Context) sensitiveauth.HelperResponse {
				return sensitiveauth.HelperResponse{
					ID:       req.ID,
					Type:     req.Type,
					Status:   "ok",
					Provider: providerName(),
				}
			}, emit)
		case "status":
			calls.Start(req, func(ctx context.Context) sensitiveauth.HelperResponse {
				return sensitiveauth.HelperResponse{
					ID:       req.ID,
					Type:     req.Type,
					Status:   status(ctx),
					Provider: providerName(),
				}
			}, emit)
		default:
			calls.Start(req, func(context.Context) sensitiveauth.HelperResponse {
				return sensitiveauth.HelperResponse{
					ID:       req.ID,
					Type:     req.Type,
					Status:   "failed",
					Provider: providerName(),
					Message:  "unsupported request",
				}
			}, emit)
		}
	}
}
