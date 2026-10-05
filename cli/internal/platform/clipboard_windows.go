//go:build windows

package platform

import (
	"errors"
	"fmt"
	"runtime"
	"time"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
)

var (
	user32                       = windows.NewLazySystemDLL("user32.dll")
	procOpenClipboard            = user32.NewProc("OpenClipboard")
	procCloseClipboard           = user32.NewProc("CloseClipboard")
	procEmptyClipboard           = user32.NewProc("EmptyClipboard")
	procGetClipboardData         = user32.NewProc("GetClipboardData")
	procSetClipboardData         = user32.NewProc("SetClipboardData")
	procRegisterClipboardFormatW = user32.NewProc("RegisterClipboardFormatW")
	procGlobalAlloc              = modkernel32.NewProc("GlobalAlloc")
	procGlobalFree               = modkernel32.NewProc("GlobalFree")
	procGlobalLock               = modkernel32.NewProc("GlobalLock")
	procGlobalUnlock             = modkernel32.NewProc("GlobalUnlock")
	procGlobalSize               = modkernel32.NewProc("GlobalSize")
	errClipboardUnavailable      = errors.New("clipboard is in use by another application")
	sensitiveClipboardFormats    = []string{"ExcludeClipboardContentFromMonitorProcessing", "CanIncludeInClipboardHistory", "CanUploadToCloudClipboard"}
)

// nil clears; sensitive writes stay out of Win+V history and Cloud Clipboard.
func ClipboardWriteText(value []byte, sensitive bool) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := openClipboard(); err != nil {
		return err
	}
	defer procCloseClipboard.Call()
	if ok, _, err := procEmptyClipboard.Call(); ok == 0 {
		return fmt.Errorf("emptying clipboard: %w", err)
	}
	if value == nil {
		return nil
	}
	if sensitive {
		var disallow [4]byte
		for _, format := range sensitiveClipboardFormats {
			name, _ := windows.UTF16PtrFromString(format)
			id, _, err := procRegisterClipboardFormatW.Call(uintptr(unsafe.Pointer(name)))
			if id == 0 {
				return fmt.Errorf("registering %s: %w", format, err)
			}
			if err := setClipboardBytes(uint32(id), disallow[:]); err != nil {
				return err
			}
		}
	}
	units := encodeUTF16(value)
	raw := unsafe.Slice((*byte)(unsafe.Pointer(&units[0])), len(units)*2)
	defer clear(units)
	return setClipboardBytes(cfUnicodeText, raw)
}

func ClipboardReadText() ([]byte, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := openClipboard(); err != nil {
		return nil, err
	}
	defer procCloseClipboard.Call()
	handle, _, _ := procGetClipboardData.Call(cfUnicodeText)
	if handle == 0 {
		return []byte{}, nil
	}
	ptr, _, err := procGlobalLock.Call(handle)
	if ptr == 0 {
		return nil, fmt.Errorf("locking clipboard data: %w", err)
	}
	defer procGlobalUnlock.Call(handle)
	size, _, _ := procGlobalSize.Call(handle)
	return decodeUTF16(unsafe.Slice((*uint16)(globalMemory(ptr)), size/2)), nil
}

func openClipboard() error {
	for attempt := 0; attempt < 20; attempt++ {
		if ok, _, _ := procOpenClipboard.Call(0); ok != 0 {
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return errClipboardUnavailable
}

func setClipboardBytes(format uint32, data []byte) error {
	mem, _, err := procGlobalAlloc.Call(gmemMoveable, uintptr(len(data)))
	if mem == 0 {
		return fmt.Errorf("allocating clipboard data: %w", err)
	}
	ptr, _, err := procGlobalLock.Call(mem)
	if ptr == 0 {
		procGlobalFree.Call(mem)
		return fmt.Errorf("locking clipboard data: %w", err)
	}
	copy(unsafe.Slice((*byte)(globalMemory(ptr)), len(data)), data)
	procGlobalUnlock.Call(mem)
	if ok, _, err := procSetClipboardData.Call(uintptr(format), mem); ok == 0 {
		procGlobalFree.Call(mem)
		return fmt.Errorf("setting clipboard data: %w", err)
	}
	return nil
}

// GlobalLock memory is OS-owned, so this conversion cannot race the GC.
func globalMemory(p uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&p))
}

// No intermediate strings: they could not be zeroed after holding a key.
func encodeUTF16(value []byte) []uint16 {
	units := make([]uint16, 0, len(value)+1)
	for len(value) > 0 {
		r, size := utf8.DecodeRune(value)
		units = utf16.AppendRune(units, r)
		value = value[size:]
	}
	return append(units, 0)
}

func decodeUTF16(units []uint16) []byte {
	out := make([]byte, 0, len(units))
	for i := 0; i < len(units) && units[i] != 0; i++ {
		r := rune(units[i])
		if utf16.IsSurrogate(r) && i+1 < len(units) {
			if pair := utf16.DecodeRune(r, rune(units[i+1])); pair != utf8.RuneError {
				r = pair
				i++
			}
		}
		out = utf8.AppendRune(out, r)
	}
	return out
}
