package sensitiveauth

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type leaseState struct {
	mu           sync.Mutex
	unlocked     bool
	exportTokens map[string]time.Time
}

func newLeaseState() *leaseState {
	return &leaseState{
		exportTokens: make(map[string]time.Time),
	}
}

func (s *leaseState) CanView(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.unlocked
}

func (s *leaseState) GrantView(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.unlocked = true
}

func (s *leaseState) IsUnlocked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.unlocked
}

func (s *leaseState) IssueExportToken(now time.Time) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	token := uuid.NewString()
	s.exportTokens[token] = now.Add(ExportTokenTTL)
	return token
}

func (s *leaseState) ConsumeExportToken(token string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for issuedToken, expiresAt := range s.exportTokens {
		if now.After(expiresAt) {
			delete(s.exportTokens, issuedToken)
		}
	}

	expiresAt, ok := s.exportTokens[token]
	if !ok {
		return false
	}
	delete(s.exportTokens, token)
	return now.Before(expiresAt)
}

func (s *leaseState) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.unlocked = false
	s.exportTokens = make(map[string]time.Time)
}
