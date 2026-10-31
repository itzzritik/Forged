package daemon

import (
	"context"
	"time"

	"github.com/itzzritik/forged/cli/internal/update"
)

const (
	latestCheckDelay    = time.Minute
	latestCheckInterval = 6 * time.Hour
	latestCheckTimeout  = 10 * time.Second
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
	ctx, cancel := context.WithTimeout(context.Background(), latestCheckTimeout)
	defer cancel()
	version, err := update.Latest(ctx)
	if err == nil {
		err = update.Save(d.paths, version)
	}
	if err != nil {
		d.logger.Debug("checking for a newer forged failed", "error", err)
	}
}
