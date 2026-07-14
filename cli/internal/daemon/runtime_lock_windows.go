//go:build windows

package daemon

import "github.com/itzzritik/forged/cli/internal/config"

type runtimeLock struct{}

func acquireRuntimeLock(config.Paths) (*runtimeLock, error) {
	return &runtimeLock{}, nil
}

func (*runtimeLock) Close() {}
