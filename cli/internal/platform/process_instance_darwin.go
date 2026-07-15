//go:build darwin

package platform

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

func ProcessInfoForPID(pid int) (ProcessInfo, error) {
	if pid <= 0 {
		return ProcessInfo{}, fmt.Errorf("%w: invalid pid", ErrProcessNotFound)
	}

	process, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		if errors.Is(err, unix.ESRCH) || errors.Is(err, unix.ENOENT) {
			return ProcessInfo{}, fmt.Errorf("%w: pid %d", ErrProcessNotFound, pid)
		}
		return ProcessInfo{}, fmt.Errorf("reading process info: %w", err)
	}
	if process == nil || int(process.Proc.P_pid) != pid {
		return ProcessInfo{}, fmt.Errorf("%w: pid %d", ErrProcessNotFound, pid)
	}
	started := process.Proc.P_starttime
	if started.Sec == 0 && started.Usec == 0 {
		return ProcessInfo{}, fmt.Errorf("%w: missing process start time", ErrProcessIdentityUnavailable)
	}

	return ProcessInfo{
		Instance: ProcessInstance{
			PID: pid,
			ID:  fmt.Sprintf("%d:%d", started.Sec, started.Usec),
		},
		ParentPID: int(process.Eproc.Ppid),
	}, nil
}
