package actions

import (
	"github.com/itzzritik/forged/cli/internal/accountauth"
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/ipc"
)

// refreshOnlySyncToken preserves the existing non-empty token wire shape for
// older daemons. Current daemons ignore it and refresh through their own
// credential source; it is never an account token or refresh secret.
const refreshOnlySyncToken = "forged-refresh-only"

func TriggerSync(paths config.Paths) error {
	creds, err := LoadCredentials(paths)
	if err != nil {
		return err
	}

	token := accountauth.CurrentToken(creds)
	if token == "" {
		token = refreshOnlySyncToken
	}
	_, err = ipc.NewClient(paths.CtlSocket()).CallWithTimeout(ipc.CmdSyncTrigger, map[string]string{
		"server_url": creds.ServerURL,
		"token":      token,
	}, ipc.ManualSyncCallTimeout)
	return err
}
