package sshrouting

import (
	"fmt"
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
	s.attempts[req.Attempt] = Attempt{
		Token:      req.Attempt,
		ClientPID:  req.ClientPID,
		Target:     target,
		Candidates: append([]string(nil), plan.Fingerprints...),
		HadExact:   plan.HadExact,
		Created:    s.now(),
	}
	s.clientAttempt[req.ClientPID] = req.Attempt
	return nil
}

func (s *Service) Success(attempt string) error {
	current, ok := s.attempts[attempt]
	if !ok {
		return fmt.Errorf("attempt %q not found", attempt)
	}
	defer s.deleteAttempt(current)

	if current.LastKey == "" {
		return nil
	}
	return s.keyStore.RecordSSHRoute(current.Target.Canonical, current.LastKey, s.now())
}

func (s *Service) RecordSignature(clientPID int, fingerprint string) {
	attempt, ok := s.AttemptByPID(clientPID)
	if !ok {
		return
	}
	attempt.LastKey = fingerprint
	s.attempts[attempt.Token] = attempt
}

func (s *Service) AllowedFingerprints(clientPID int) []string {
	attempt, ok := s.AttemptByPID(clientPID)
	if !ok {
		return nil
	}
	return append([]string(nil), attempt.Candidates...)
}

func (s *Service) AttemptByPID(clientPID int) (Attempt, bool) {
	token, ok := s.clientAttempt[clientPID]
	if !ok {
		return Attempt{}, false
	}
	attempt, ok := s.attempts[token]
	return attempt, ok
}

func (s *Service) ExpireBefore(cutoff time.Time) {
	for _, attempt := range s.attempts {
		if attempt.Created.After(cutoff) {
			continue
		}
		if attempt.HadExact && attempt.LastKey != "" {
			_ = s.keyStore.ClearSSHRoute(attempt.Target.Canonical, s.now())
		}
		s.deleteAttempt(attempt)
	}
}

func (s *Service) deleteAttempt(attempt Attempt) {
	delete(s.attempts, attempt.Token)
	delete(s.clientAttempt, attempt.ClientPID)
}
