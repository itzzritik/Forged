package daemon

import (
	"time"

	"github.com/itzzritik/forged/cli/internal/update"
)

const (
	latestCheckDelay    = time.Minute
	latestCheckInterval = time.Hour
)

func (d *Daemon) watchLatestVersion() {
	wait := time.NewTimer(latestCheckDelay)
	defer wait.Stop()
	for {
		select {
		case <-d.stop:
			return
		case <-wait.C:
		}
		d.checkLatestVersion()
		wait.Reset(latestCheckInterval)
	}
}

func (d *Daemon) checkLatestVersion() {
	if err := update.Refresh(d.paths); err != nil {
		d.logger.Debug("checking for a newer forged failed", "error", err)
	}
}
