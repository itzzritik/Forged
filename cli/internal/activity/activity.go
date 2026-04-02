package activity

import (
	"sync"
	"time"
)

type ActivityEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"`
	KeyName     string    `json:"key_name"`
	Fingerprint string    `json:"fingerprint"`
	RemoteHost  string    `json:"remote_host,omitempty"`
	Result      string    `json:"result"`
	ClientPID   int       `json:"client_pid,omitempty"`
}

type ActivityLog struct {
	mu     sync.Mutex
	events []ActivityEvent
	max    int
}

func NewActivityLog(max int) *ActivityLog {
	return &ActivityLog{max: max}
}

func (al *ActivityLog) Record(e ActivityEvent) {
	al.mu.Lock()
	defer al.mu.Unlock()

	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}

	al.events = append(al.events, e)
	if len(al.events) > al.max {
		al.events = al.events[len(al.events)-al.max:]
	}
}

func (al *ActivityLog) Recent(limit int) []ActivityEvent {
	al.mu.Lock()
	defer al.mu.Unlock()

	if limit <= 0 || limit > len(al.events) {
		limit = len(al.events)
	}

	out := make([]ActivityEvent, limit)
	copy(out, al.events[len(al.events)-limit:])

	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}
