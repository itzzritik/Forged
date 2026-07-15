package activity

import (
	"sync"
	"time"
)

const (
	nonSuccessDedupeWindow = 30 * time.Second
	nonSuccessDedupeSize   = 64
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

type activityDedupeEntry struct {
	typeName    string
	result      string
	keyName     string
	fingerprint string
	remoteHost  string
	clientPID   int
	recordedAt  time.Time
}

type ActivityLog struct {
	mu             sync.Mutex
	events         []ActivityEvent
	max            int
	nonSuccess     [nonSuccessDedupeSize]activityDedupeEntry
	nonSuccessNext int
}

func NewActivityLog(max int) *ActivityLog {
	return &ActivityLog{max: max}
}

func (al *ActivityLog) Record(e ActivityEvent) {
	al.mu.Lock()
	defer al.mu.Unlock()

	now := time.Now().UTC()
	if e.Timestamp.IsZero() {
		e.Timestamp = now
	}
	if e.Result != "success" && al.duplicateNonSuccess(e, now) {
		return
	}

	al.events = append(al.events, e)
	if len(al.events) > al.max {
		al.events = al.events[len(al.events)-al.max:]
	}
}

func (al *ActivityLog) duplicateNonSuccess(e ActivityEvent, now time.Time) bool {
	for _, previous := range al.nonSuccess {
		if previous.recordedAt.IsZero() || now.Sub(previous.recordedAt) >= nonSuccessDedupeWindow {
			continue
		}
		if previous.typeName == e.Type &&
			previous.result == e.Result &&
			previous.keyName == e.KeyName &&
			previous.fingerprint == e.Fingerprint &&
			previous.remoteHost == e.RemoteHost &&
			previous.clientPID == e.ClientPID {
			return true
		}
	}

	al.nonSuccess[al.nonSuccessNext] = activityDedupeEntry{
		typeName:    e.Type,
		result:      e.Result,
		keyName:     e.KeyName,
		fingerprint: e.Fingerprint,
		remoteHost:  e.RemoteHost,
		clientPID:   e.ClientPID,
		recordedAt:  now,
	}
	al.nonSuccessNext = (al.nonSuccessNext + 1) % len(al.nonSuccess)
	return false
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
