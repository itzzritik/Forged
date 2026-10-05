//go:build windows

package platform

import (
	"fmt"
	"net"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func PipeServerProcessID(path string) (int, error) {
	conn, err := Dial(path, 500*time.Millisecond)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	handle, ok := conn.(interface{ Fd() uintptr })
	var pid uint32
	if !ok || windows.GetNamedPipeServerProcessId(windows.Handle(handle.Fd()), &pid) != nil || pid == 0 {
		return 0, ErrPeerPIDUnavailable
	}
	return int(pid), nil
}

// Pipe names derive from the public user SID; only this user (or an admin)
// can own a pipe as that SID, and Low integrity marks a sandboxed creator.
func verifyPipeServerUser(conn net.Conn) error {
	handle, ok := conn.(interface{ Fd() uintptr })
	if !ok {
		return ErrPeerIdentityUnavailable
	}
	sd, err := windows.GetSecurityInfo(windows.Handle(handle.Fd()), windows.SE_KERNEL_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.LABEL_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Errorf("%w: reading pipe owner: %v", ErrPeerIdentityUnavailable, err)
	}
	owner, _, err := sd.Owner()
	if err != nil || owner == nil {
		return fmt.Errorf("%w: pipe has no owner", ErrPeerIdentityUnavailable)
	}
	current, err := currentUserSID()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrPeerIdentityUnavailable, err)
	}
	if !windows.EqualSid(owner, current) {
		if admins, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid); err == nil && windows.EqualSid(owner, admins) {
			return fmt.Errorf("%w: pipe is owned by Administrators; a Forged daemon started from an elevated terminal is still running, stop it and reopen Forged", ErrPeerIdentityMismatch)
		}
		return fmt.Errorf("%w: pipe is owned by %s", ErrPeerIdentityMismatch, owner.String())
	}
	if rid, ok := mandatoryLabelRID(sd); ok && rid < securityMandatoryMediumRID {
		return fmt.Errorf("%w: pipe was created at integrity level 0x%x", ErrPeerIdentityMismatch, rid)
	}
	return nil
}

const (
	systemMandatoryLabelACEType = 0x11
	securityMandatoryMediumRID  = 0x2000
)

// Objects without an explicit mandatory label are implicitly Medium.
func mandatoryLabelRID(sd *windows.SECURITY_DESCRIPTOR) (uint32, bool) {
	sacl, _, err := sd.SACL()
	if err != nil || sacl == nil {
		return 0, false
	}
	for i := uint32(0); i < uint32(sacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if windows.GetAce(sacl, i, &ace) != nil || ace.Header.AceType != systemMandatoryLabelACEType {
			continue
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if count := sid.SubAuthorityCount(); count > 0 {
			return sid.SubAuthority(uint32(count) - 1), true
		}
	}
	return 0, false
}
