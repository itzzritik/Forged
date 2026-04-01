package daemon

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/forgedkeys/forged/cli/internal/config"
	"github.com/forgedkeys/forged/cli/internal/platform"
	"github.com/forgedkeys/forged/cli/internal/vault"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Daemon struct {
	paths    config.Paths
	vault    *vault.Vault
	keyStore *vault.KeyStore
	logger   *slog.Logger
	stop     chan struct{}
}

func New(paths config.Paths) *Daemon {
	return &Daemon{
		paths: paths,
		stop:  make(chan struct{}),
	}
}

func (d *Daemon) Run(password []byte) error {
	if err := d.setupLogging(); err != nil {
		return fmt.Errorf("setting up logging: %w", err)
	}

	d.logger.Info("starting forged daemon")

	if err := d.cleanStaleState(); err != nil {
		return fmt.Errorf("cleaning stale state: %w", err)
	}

	if err := d.openVault(password); err != nil {
		return err
	}
	defer d.shutdown()

	if err := d.writePID(); err != nil {
		return err
	}

	d.logger.Info("daemon ready",
		"keys", len(d.keyStore.List()),
		"agent_socket", d.paths.AgentSocket(),
		"ctl_socket", d.paths.CtlSocket(),
	)

	d.waitForSignal()
	return nil
}

func (d *Daemon) KeyStore() *vault.KeyStore {
	return d.keyStore
}

func (d *Daemon) Stop() {
	close(d.stop)
}

func (d *Daemon) setupLogging() error {
	logPath := d.paths.LogFile()
	if err := os.MkdirAll(filepath.Dir(logPath), 0700); err != nil {
		return err
	}

	lj := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     30,
	}

	w := io.MultiWriter(os.Stderr, lj)
	d.logger = slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	return nil
}

func (d *Daemon) cleanStaleState() error {
	for _, sock := range []string{d.paths.AgentSocket(), d.paths.CtlSocket()} {
		if err := platform.CleanStaleSocket(sock); err != nil {
			return fmt.Errorf("socket %s: %w", sock, err)
		}
	}

	pidPath := d.paths.PIDFile()
	if data, err := os.ReadFile(pidPath); err == nil {
		if pid, err := strconv.Atoi(string(data)); err == nil {
			if process, err := os.FindProcess(pid); err == nil {
				if err := process.Signal(syscall.Signal(0)); err == nil {
					return fmt.Errorf("daemon already running (PID %d)", pid)
				}
			}
		}
		os.Remove(pidPath)
	}

	return nil
}

func (d *Daemon) openVault(password []byte) error {
	vaultPath := d.paths.VaultFile()

	var v *vault.Vault
	var err error

	if _, statErr := os.Stat(vaultPath); os.IsNotExist(statErr) {
		d.logger.Info("creating new vault", "path", vaultPath)
		v, err = vault.Create(vaultPath, password)
	} else {
		d.logger.Info("opening vault", "path", vaultPath)
		v, err = vault.Open(vaultPath, password)
	}

	if err != nil {
		return fmt.Errorf("vault: %w", err)
	}

	d.vault = v
	d.keyStore = vault.NewKeyStore(v)

	for _, key := range v.Data.Keys {
		platform.Mlock([]byte(key.PrivateKey))
	}

	return nil
}

func (d *Daemon) writePID() error {
	pidPath := d.paths.PIDFile()
	if err := os.MkdirAll(filepath.Dir(pidPath), 0700); err != nil {
		return fmt.Errorf("creating pid directory: %w", err)
	}
	return os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0600)
}

func (d *Daemon) waitForSignal() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	select {
	case s := <-sig:
		d.logger.Info("received signal", "signal", s)
	case <-d.stop:
		d.logger.Info("stop requested")
	}
}

func (d *Daemon) shutdown() {
	d.logger.Info("shutting down")

	if d.vault != nil {
		for _, key := range d.vault.Data.Keys {
			privBytes := []byte(key.PrivateKey)
			for i := range privBytes {
				privBytes[i] = 0
			}
			platform.Munlock(privBytes)
		}
		d.vault.Close()
	}

	os.Remove(d.paths.AgentSocket())
	os.Remove(d.paths.CtlSocket())
	os.Remove(d.paths.PIDFile())

	d.logger.Info("daemon stopped")
}

func IsRunning(paths config.Paths) (int, bool) {
	data, err := os.ReadFile(paths.PIDFile())
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return 0, false
	}
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return 0, false
	}
	return pid, true
}
