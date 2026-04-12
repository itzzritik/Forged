package sync

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/itzzritik/forged/cli/internal/vault"
)

type Engine struct {
	vault    *vault.Vault
	client   *Client
	logger   *slog.Logger
	interval time.Duration
	stopCh   chan struct{}
	running  atomic.Bool
}

func NewEngine(v *vault.Vault, client *Client, logger *slog.Logger, interval time.Duration) *Engine {
	return &Engine{
		vault:    v,
		client:   client,
		logger:   logger,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

func (e *Engine) Start() {
	if e.running.Swap(true) {
		return
	}
	go e.loop()
}

func (e *Engine) Stop() {
	if e.running.Swap(false) {
		close(e.stopCh)
	}
}

func (e *Engine) loop() {
	e.syncOnce()

	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			e.syncOnce()
		case <-e.stopCh:
			return
		}
	}
}

func (e *Engine) syncOnce() {
	if err := e.push(); err != nil {
		e.logger.Debug("sync push failed, will retry", "error", err)
		e.retryWithBackoff(e.push)
	}
}

func (e *Engine) push() error {
	blob, err := e.vault.ExportForSync()
	if err != nil {
		return err
	}
	protectedKey := base64.StdEncoding.EncodeToString(e.vault.ProtectedKeyBytes())
	_, err = e.client.Push(blob, e.vault.KDFParams(), protectedKey, 0)
	return err
}

func (e *Engine) Pull() error {
	result, err := e.client.Pull()
	if err != nil {
		return err
	}

	plaintext, err := vault.DecryptCombined(e.vault.Key(), result.Blob)
	if err != nil {
		return err
	}

	var remote vault.VaultData
	if err := json.Unmarshal(plaintext, &remote); err != nil {
		return err
	}

	merged := MergeVaults(e.vault.Data, remote)
	e.vault.Data = merged
	return e.vault.Save()
}

func (e *Engine) retryWithBackoff(fn func() error) {
	delays := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		30 * time.Second,
		60 * time.Second,
		5 * time.Minute,
	}

	for _, delay := range delays {
		select {
		case <-e.stopCh:
			return
		case <-time.After(delay):
		}

		if err := fn(); err == nil {
			e.logger.Info("sync retry succeeded")
			return
		}
		e.logger.Debug("sync retry failed, backing off", "next_delay", delay*2)
	}
	e.logger.Warn("sync retries exhausted, will try again next interval")
}
