package sshrouting

import (
	"fmt"
	"sync"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/vault"
)

type PrepareRequest struct {
	Attempt   string
	ClientPID int
	CWD       string
	Host      string
	User      string
	Port      string
	Branch    string
	Target    Target
}

type Attempt struct {
	Token      string
	ClientPID  int
	Target     Target
	Candidates []string
	HadExact   bool
	LastKey    string
	Created    time.Time
}

type Service struct {
	mu            sync.RWMutex
	paths         config.Paths
	keyStore      *vault.KeyStore
	now           func() time.Time
	attempts      map[string]Attempt
	clientAttempt map[int]string
}

func NewService(paths config.Paths, keyStore *vault.KeyStore) *Service {
	return &Service{
		paths:         paths,
		keyStore:      keyStore,
		now:           func() time.Time { return time.Now().UTC() },
		attempts:      map[string]Attempt{},
		clientAttempt: map[int]string{},
	}
}

func (s *Service) Prepare(req PrepareRequest) error {
	target := req.Target
	if target.Canonical == "" {
		if resolved, err := ResolveGitTarget(req.CWD, req.Branch); err == nil {
			target = resolved
		} else {
			resolved, err := ResolveSSHTarget(PrepareInput{
				Host: req.Host,
				User: req.User,
				Port: req.Port,
			})
			if err != nil {
				return err
			}
			target = resolved
		}
	}

	routes := s.keyStore.SSHRoutes()
	plan := PlanCandidates(target, routes, s.keyStore.List(), 3)
	s.mu.Lock()
	s.attempts[req.Attempt] = Attempt{
		Token:      req.Attempt,
		ClientPID:  req.ClientPID,
		Target:     target,
		Candidates: append([]string(nil), plan.Fingerprints...),
		HadExact:   plan.HadExact,
		Created:    s.now(),
	}
	s.clientAttempt[req.ClientPID] = req.Attempt
	s.mu.Unlock()
	return nil
}

func (s *Service) Success(attempt string) error {
	s.mu.Lock()
	current, ok := s.attempts[attempt]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("attempt %q not found", attempt)
	}
	s.deleteAttemptLocked(current)
	s.mu.Unlock()

	if current.LastKey == "" {
		return nil
	}
	return s.keyStore.RecordSSHRoute(current.Target.Canonical, current.LastKey, s.now())
}

func (s *Service) RecordSignature(clientPID int, fingerprint string) {
	s.mu.Lock()
	attempt, ok := s.attemptByPIDLocked(clientPID)
	if !ok {
		s.mu.Unlock()
		return
	}
	attempt.LastKey = fingerprint
	s.attempts[attempt.Token] = attempt
	s.mu.Unlock()
}

func (s *Service) AllowedFingerprints(clientPID int) []string {
	s.mu.RLock()
	attempt, ok := s.attemptByPIDLocked(clientPID)
	s.mu.RUnlock()
	if !ok {
		return nil
	}
	return append([]string(nil), attempt.Candidates...)
}

func (s *Service) AttemptByPID(clientPID int) (Attempt, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.attemptByPIDLocked(clientPID)
}

func (s *Service) attemptByPIDLocked(clientPID int) (Attempt, bool) {
	token, ok := s.clientAttempt[clientPID]
	if !ok {
		return Attempt{}, false
	}
	attempt, ok := s.attempts[token]
	return attempt, ok
}

func (s *Service) ExpireBefore(cutoff time.Time) {
	var expired []Attempt
	s.mu.Lock()
	for _, attempt := range s.attempts {
		if attempt.Created.After(cutoff) {
			continue
		}
		expired = append(expired, attempt)
		s.deleteAttemptLocked(attempt)
	}
	s.mu.Unlock()

	for _, attempt := range expired {
		if attempt.HadExact && attempt.LastKey != "" {
			_ = s.keyStore.ClearSSHRoute(attempt.Target.Canonical, s.now())
		}
	}
}

func (s *Service) deleteAttempt(attempt Attempt) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleteAttemptLocked(attempt)
}

func (s *Service) deleteAttemptLocked(attempt Attempt) {
	delete(s.attempts, attempt.Token)
	delete(s.clientAttempt, attempt.ClientPID)
}
