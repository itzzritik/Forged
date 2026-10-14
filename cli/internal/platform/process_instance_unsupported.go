//go:build !linux && !darwin

package platform

func ProcessInfoForPID(int) (ProcessInfo, error) {
	return ProcessInfo{}, ErrProcessIdentityUnavailable
}

func ProcessStartedNoLaterThan(ProcessInstance, ProcessInstance) (bool, error) {
	return false, ErrProcessIdentityUnavailable
}

func ProcessInstances() (map[ProcessInstance]struct{}, error) {
	return nil, ErrProcessIdentityUnavailable
}
