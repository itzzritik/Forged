package platform

import "errors"

var (
	ErrProcessNotFound            = errors.New("process not found")
	ErrProcessIdentityUnavailable = errors.New("process identity unavailable")
)

// ProcessInstance identifies one process lifetime, not just its reusable PID.
type ProcessInstance struct {
	PID int    `json:"pid"`
	ID  string `json:"id"`
}

func (p ProcessInstance) Valid() bool {
	return p.PID > 0 && p.ID != ""
}

// ProcessInfo includes the identity and current parent of a process.
type ProcessInfo struct {
	Instance  ProcessInstance
	ParentPID int
}

// SameProcessInstance reports whether instance still names the same process.
// A missing process is a definite non-match; other lookup failures are unknown.
func SameProcessInstance(instance ProcessInstance) (bool, error) {
	if !instance.Valid() {
		return false, ErrProcessIdentityUnavailable
	}
	current, err := ProcessInfoForPID(instance.PID)
	if errors.Is(err, ErrProcessNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return current.Instance == instance, nil
}
