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

func main() {
	var writeMu sync.Mutex
	emit := func(resp sensitiveauth.HelperResponse) {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = json.NewEncoder(os.Stdout).Encode(resp)
	}

	scanner := bufio.NewScanner(os.Stdin)
	locks := make(chan struct{}, 1)

	go startLockLoop(func() {
		select {
		case locks <- struct{}{}:
		default:
		}
	})

	go func() {
		for range locks {
			emit(sensitiveauth.HelperResponse{
				Type:     "event",
				Status:   "session_locked",
				Provider: providerName(),
			})
		}
	}()

	for scanner.Scan() {
		var req sensitiveauth.HelperRequest
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			continue
		}

		switch req.Type {
		case "authorize":
			action, err := sensitiveauth.ParseAction(req.Action)
			if err != nil {
				emit(sensitiveauth.HelperResponse{
					ID:       req.ID,
					Type:     req.Type,
					Status:   "failed",
					Provider: providerName(),
					Message:  err.Error(),
				})
				continue
			}
			emit(sensitiveauth.HelperResponse{
				ID:       req.ID,
				Type:     req.Type,
				Status:   authorize(context.Background(), action),
				Provider: providerName(),
			})
		case "subscribe-locks", "status":
			emit(sensitiveauth.HelperResponse{
				ID:       req.ID,
				Type:     req.Type,
				Status:   "ok",
				Provider: providerName(),
			})
		default:
			emit(sensitiveauth.HelperResponse{
				ID:       req.ID,
				Type:     req.Type,
				Status:   "failed",
				Provider: providerName(),
				Message:  "unsupported request",
			})
		}
	}
}
