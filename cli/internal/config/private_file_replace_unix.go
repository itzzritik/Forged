//go:build !windows

package config

import "os"

func replacePrivateFile(source, target string, _ bool) error {
	return os.Rename(source, target)
}
