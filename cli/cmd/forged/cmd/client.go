package cmd

import (
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/ipc"
)

func ctlClient() *ipc.Client {
	return ipc.NewClient(config.DefaultPaths().CtlSocket())
}
