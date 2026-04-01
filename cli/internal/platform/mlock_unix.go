//go:build !windows

package platform

import "syscall"

func Mlock(b []byte) error {
	return syscall.Mlock(b)
}

func Munlock(b []byte) error {
	return syscall.Munlock(b)
}
