//go:build !windows

package daemon

func deleteAfterExit(int, []string) error { return nil }
