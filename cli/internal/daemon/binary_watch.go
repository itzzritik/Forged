package daemon

import (
	"os"
	"time"
)

const binaryWatchInterval = 15 * time.Second

// Package managers replace the installed binary without telling the daemon;
// stopping once the replacement settles lets launchd/systemd restart it on
// the new build. With reinstall, path is the source of the installed copy,
// which is refreshed first. A missing binary never stops it, so the service
// can't loop.
func (d *Daemon) watchInstalledBinary(path string, started os.FileInfo, reinstall bool) {
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
		if reinstall {
			if err := InstallBinaries(path); err != nil {
				d.logger.Warn("installing upgraded binary failed", "path", path, "error", err)
				started, pending = current, nil
				continue
			}
		}
		d.logger.Info("installed binary changed; restarting", "path", path)
		d.Stop()
		return
	}
}

func sameBinary(a, b os.FileInfo) bool {
	return os.SameFile(a, b) && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}
