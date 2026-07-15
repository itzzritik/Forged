package sensitiveauth

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type leaseState struct {
	mu               sync.Mutex
	activeUntil      time.Time
	exportTokens     map[string]time.Time
	privateKeyTokens map[string]time.Time
}

func newLeaseState() *leaseState {
	return &leaseState{
		exportTokens:     make(map[string]time.Time),
		privateKeyTokens: make(map[string]time.Time),
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

	expired := !s.activeUntil.IsZero() && !now.Before(s.activeUntil)
	s.pruneLocked(now)
	return expired
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

func (s *leaseState) RevokeExportToken(token string) {
	if token == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.exportTokens, token)
}

func (s *leaseState) IssuePrivateKeyToken(now time.Time) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pruneLocked(now)
	token := uuid.NewString()
	s.privateKeyTokens[token] = now.Add(ExportTokenTTL)
	return token
}

func (s *leaseState) ConsumePrivateKeyToken(token string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pruneLocked(now)

	expiresAt, ok := s.privateKeyTokens[token]
	if !ok {
		return false
	}
	delete(s.privateKeyTokens, token)
	return now.Before(expiresAt)
}

func (s *leaseState) RevokePrivateKeyToken(token string) {
	if token == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.privateKeyTokens, token)
}

func (s *leaseState) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.activeUntil = time.Time{}
	s.exportTokens = make(map[string]time.Time)
	s.privateKeyTokens = make(map[string]time.Time)
}

func (s *leaseState) pruneLocked(now time.Time) {
	for issuedToken, expiresAt := range s.exportTokens {
		if !now.Before(expiresAt) {
			delete(s.exportTokens, issuedToken)
		}
	}
	for issuedToken, expiresAt := range s.privateKeyTokens {
		if !now.Before(expiresAt) {
			delete(s.privateKeyTokens, issuedToken)
		}
	}
}
