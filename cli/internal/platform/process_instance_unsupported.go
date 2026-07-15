//go:build !linux && !darwin

package platform

func ProcessInfoForPID(int) (ProcessInfo, error) {
	return ProcessInfo{}, ErrProcessIdentityUnavailable
}
