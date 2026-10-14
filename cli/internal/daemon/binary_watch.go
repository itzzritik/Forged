package daemon

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const binaryWatchInterval = 15 * time.Second

// Package managers replace the installed binary without telling the daemon;
// stopping once the replacement settles lets launchd/systemd restart it on
// the new build. With reinstall, path is the source of the installed copy,
// which is refreshed first. A missing binary never stops it, so the service
// can't loop.
func (d *Daemon) watchInstalledBinary(path string, started os.FileInfo, reinstall bool, executable string) {
	running, err := fileDigest(executable)
	if err != nil {
		d.logger.Warn("reading daemon executable failed; upgrade watch disabled", "path", executable, "error", err)
		return
	}
	ticker := time.NewTicker(binaryWatchInterval)
	defer ticker.Stop()
	var pending os.FileInfo
	for {
		select {
		case <-d.stop:
			return
		case <-ticker.C:
		}
		current, err := os.Stat(path)
		if err != nil || sameBinary(current, started) {
			pending = nil
			continue
		}
		if pending == nil || !sameBinary(current, pending) {
			pending = current
			continue
		}
		// A build that cannot run would crash-loop the service and take the agent down with it.
		if err := runsVersion(path); err != nil {
			d.logger.Warn("upgraded binary does not run; keeping the current daemon", "path", path, "error", err)
			started, pending = current, nil
			continue
		}
		if reinstall {
			if err := installBinariesInto(filepath.Dir(filepath.Dir(executable)), path); err != nil {
				d.logger.Warn("installing upgraded binary failed", "path", path, "error", err)
				started, pending = current, nil
				continue
			}
		}
		digest, err := fileDigest(executable)
		if err != nil {
			continue
		}
		// A restart drops the unlocked session; a reinstall of the same build must not.
		if bytes.Equal(digest, running) {
			started, pending = current, nil
			continue
		}
		d.logger.Info("installed binary changed; restarting", "path", path)
		d.Stop()
		return
	}
}

func runsVersion(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, path, "version").Run()
}

func sameBinary(a, b os.FileInfo) bool {
	return os.SameFile(a, b) && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

func fileDigest(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, err
	}
	return hash.Sum(nil), nil
}
