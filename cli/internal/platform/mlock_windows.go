//go:build windows

package platform

import (
	"syscall"
	"unsafe"
)

var (
	kernel32      = syscall.NewLazyDLL("kernel32.dll")
	virtualLock   = kernel32.NewProc("VirtualLock")
	virtualUnlock = kernel32.NewProc("VirtualUnlock")
)

func Mlock(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	ret, _, err := virtualLock.Call(
		uintptr(unsafe.Pointer(&b[0])),
		uintptr(len(b)),
	)
	if ret == 0 {
		return err
	}
	return nil
}

func Munlock(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	ret, _, err := virtualUnlock.Call(
		uintptr(unsafe.Pointer(&b[0])),
		uintptr(len(b)),
	)
	if ret == 0 {
		return err
	}
	return nil
}
