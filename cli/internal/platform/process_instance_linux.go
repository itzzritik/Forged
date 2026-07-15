//go:build linux

package platform

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func ProcessInfoForPID(pid int) (ProcessInfo, error) {
	if pid <= 0 {
		return ProcessInfo{}, fmt.Errorf("%w: invalid pid", ErrProcessNotFound)
	}

	statPath := filepath.Join("/proc", strconv.Itoa(pid), "stat")
	stat, err := os.ReadFile(statPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ProcessInfo{}, fmt.Errorf("%w: pid %d", ErrProcessNotFound, pid)
		}
		return ProcessInfo{}, fmt.Errorf("reading process stat: %w", err)
	}
	closing := strings.LastIndex(string(stat), ")")
	if closing < 0 {
		return ProcessInfo{}, fmt.Errorf("%w: malformed process stat", ErrProcessIdentityUnavailable)
	}
	fields := strings.Fields(string(stat[closing+1:]))
	if len(fields) <= 19 {
		return ProcessInfo{}, fmt.Errorf("%w: incomplete process stat", ErrProcessIdentityUnavailable)
	}
	parentPID, err := strconv.Atoi(fields[1])
	if err != nil || parentPID < 0 {
		return ProcessInfo{}, fmt.Errorf("%w: invalid process parent", ErrProcessIdentityUnavailable)
	}
	startTicks, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return ProcessInfo{}, fmt.Errorf("%w: invalid process start time", ErrProcessIdentityUnavailable)
	}
	bootID, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return ProcessInfo{}, fmt.Errorf("reading boot id: %w", err)
	}
	bootIDText := strings.TrimSpace(string(bootID))
	if bootIDText == "" {
		return ProcessInfo{}, fmt.Errorf("%w: empty boot id", ErrProcessIdentityUnavailable)
	}

	return ProcessInfo{
		Instance:  ProcessInstance{PID: pid, ID: bootIDText + ":" + strconv.FormatUint(startTicks, 10)},
		ParentPID: parentPID,
	}, nil
}
