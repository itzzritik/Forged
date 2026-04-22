package sensitiveauth

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type leaseState struct {
	mu           sync.Mutex
	activeUntil  time.Time
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

	s.pruneLocked(now)
	return !s.activeUntil.IsZero() && now.Before(s.activeUntil)
}

func (s *leaseState) GrantView(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.activeUntil = now.Add(SharedSessionTTL)
	s.pruneLocked(now)
}

func (s *leaseState) IsUnlocked(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pruneLocked(now)
	return !s.activeUntil.IsZero() && now.Before(s.activeUntil)
}

func (s *leaseState) IsExpired(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pruneLocked(now)
	return !s.activeUntil.IsZero() && !now.Before(s.activeUntil)
}

func (s *leaseState) IssueExportToken(now time.Time) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pruneLocked(now)
	token := uuid.NewString()
	s.exportTokens[token] = now.Add(ExportTokenTTL)
	return token
}

func (s *leaseState) ConsumeExportToken(token string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pruneLocked(now)

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

	s.activeUntil = time.Time{}
	s.exportTokens = make(map[string]time.Time)
}

func (s *leaseState) pruneLocked(now time.Time) {
	if !s.activeUntil.IsZero() && !now.Before(s.activeUntil) {
		s.activeUntil = time.Time{}
	}
	for issuedToken, expiresAt := range s.exportTokens {
		if !now.Before(expiresAt) {
			delete(s.exportTokens, issuedToken)
		}
	}
}
