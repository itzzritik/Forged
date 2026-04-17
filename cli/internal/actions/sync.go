package actions

import (
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/ipc"
)

func TriggerSync(paths config.Paths) error {
	creds, err := LoadCredentials(paths)
	if err != nil {
		return err
	}

	_, err = ipc.NewClient(paths.CtlSocket()).Call(ipc.CmdSyncTrigger, map[string]string{
		"server_url": creds.ServerURL,
		"token":      creds.Token,
	})
	return err
}
