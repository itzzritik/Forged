//go:build darwin

package platform

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

func ProcessInfoForPID(pid int) (ProcessInfo, error) {
	if pid <= 0 {
		return ProcessInfo{}, fmt.Errorf("%w: invalid pid", ErrProcessNotFound)
	}

	process, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		if errors.Is(err, unix.ESRCH) || errors.Is(err, unix.ENOENT) || errors.Is(err, unix.EIO) {
			// kern.proc.pid reports EIO after a PID exits on current macOS.
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

// ProcessStartedNoLaterThan reports whether candidate began at or before reference.
// It fails closed when either process start identity cannot be ordered.
func ProcessStartedNoLaterThan(candidate, reference ProcessInstance) (bool, error) {
	candidateSeconds, candidateMicros, err := darwinProcessStart(candidate)
	if err != nil {
		return false, err
	}
	referenceSeconds, referenceMicros, err := darwinProcessStart(reference)
	if err != nil {
		return false, err
	}
	if candidateSeconds != referenceSeconds {
		return candidateSeconds < referenceSeconds, nil
	}
	return candidateMicros <= referenceMicros, nil
}

func darwinProcessStart(instance ProcessInstance) (int64, int64, error) {
	if !instance.Valid() {
		return 0, 0, ErrProcessIdentityUnavailable
	}
	secondsText, microsText, ok := strings.Cut(instance.ID, ":")
	if !ok {
		return 0, 0, fmt.Errorf("%w: malformed darwin process start identity", ErrProcessIdentityUnavailable)
	}
	seconds, err := strconv.ParseInt(secondsText, 10, 64)
	if err != nil || seconds < 0 {
		return 0, 0, fmt.Errorf("%w: invalid darwin process start seconds", ErrProcessIdentityUnavailable)
	}
	micros, err := strconv.ParseInt(microsText, 10, 64)
	if err != nil || micros < 0 || micros >= 1_000_000 || (seconds == 0 && micros == 0) {
		return 0, 0, fmt.Errorf("%w: invalid darwin process start microseconds", ErrProcessIdentityUnavailable)
	}
	return seconds, micros, nil
}
