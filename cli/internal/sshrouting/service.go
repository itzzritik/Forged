package sshrouting

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/vault"
)

var ErrRouteMemoryLocked = errors.New("SSH route memory is locked")

type PrepareRequest struct {
	Attempt      string
	ClientPID    int
	CWD          string
	Host         string
	OriginalHost string
	User         string
	Port         string
	Branch       string
	Target       Target
}

type Attempt struct {
	Token       string
	ClientPID   int
	Target      Target
	Operation   OperationClass
	Candidates  []string
	HadExact    bool
	ProbeProved bool
	LastKey     string
	Created     time.Time
}

type Service struct {
	mu            sync.RWMutex
	paths         config.Paths
	keyStore      *vault.KeyStore
	cachedKeys    []vault.Key
	cachedRoutes  map[string]vault.SSHRoute
	now           func() time.Time
	attempts      map[string]Attempt
	clientAttempt map[int]string
	prober        ProviderProber
	onMutation    func(reason string)
}

func NewService(paths config.Paths, keyStore *vault.KeyStore) *Service {
	return &Service{
		paths:         paths,
		keyStore:      keyStore,
		now:           func() time.Time { return time.Now().UTC() },
		attempts:      map[string]Attempt{},
		clientAttempt: map[int]string{},
		prober:        NewProviderProber(paths.AgentSocket()),
	}
}

func (s *Service) SetKeyStore(keyStore *vault.KeyStore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keyStore = keyStore
	if keyStore != nil {
		s.refreshCacheFromKeyStoreLocked(keyStore)
	}
}

func (s *Service) SetOnMutation(fn func(reason string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onMutation = fn
}

func (s *Service) Prepare(req PrepareRequest) error {
	if err := validateAttemptToken(req.Attempt); err != nil {
		return err
	}

	now := s.now()
	operation := InspectProcess(req.ClientPID).Operation
	target, err := s.resolveTarget(req, operation)
	if err != nil {
		return err
	}
	if operation == OperationUnknown && target.Kind == TargetSSH {
		operation = OperationSSHAuth
	}

	keyStore, routes, keys := s.routingSnapshot()
	if keyStore == nil && len(keys) == 0 {
		_ = WriteRouteSnippet(s.paths.SSHRouteRuntimeDir(), req.Attempt, nil)
		return ErrRouteMemoryLocked
	}

	refs, err := BuildKeyRefs(keys, s.paths.SSHManagedKeysDir())
	if err != nil {
		return fmt.Errorf("Building SSH key refs: %w", err)
	}
	if err := SyncPublicHintFiles(s.paths.SSHManagedKeysDir(), refs, now); err != nil {
		return fmt.Errorf("Syncing SSH public key hints: %w", err)
	}
	_ = CleanupRouteRuntime(s.paths.SSHRouteRuntimeDir(), now.Add(-routeSnippetTTL))

	plan := PlanCandidatesForRequest(PlanRequest{
		Target:    target,
		Operation: operation,
		Routes:    routes,
		Keys:      keys,
		Limit:     3,
	})
	refByFingerprint := KeyRefsByFingerprint(refs)

	selected := append([]string(nil), plan.Fingerprints...)
	if plan.HadExact {
		if exact := exactProvenFingerprints(plan); len(exact) > 0 {
			selected = exact
		}
	}
	probeProved := false
	if target.Kind == TargetGit && !plan.HadExact && keyStore != nil {
		probed, proved, err := s.probeGitProvider(target, operation, plan, refByFingerprint)
		if err != nil {
			return err
		}
		if proved || probed != nil {
			selected = probed
			probeProved = proved
		}
	}
	if target.Kind == TargetSSH && keyStore != nil {
		probed, proved, err := s.probeSSHServer(target, operation, plan)
		if err != nil {
			return err
		}
		if proved || probed != nil {
			selected = probed
			probeProved = proved
		}
	}

	s.mu.Lock()
	s.expireBeforeLocked(now.Add(-routeSnippetTTL))
	attemptKey := routeAttemptKey(req.Attempt, req.ClientPID)
	s.attempts[attemptKey] = Attempt{
		Token:       req.Attempt,
		ClientPID:   req.ClientPID,
		Target:      target,
		Operation:   operation,
		Candidates:  append([]string(nil), selected...),
		HadExact:    plan.HadExact,
		ProbeProved: probeProved,
		Created:     now,
	}
	s.clientAttempt[req.ClientPID] = attemptKey
	tokenCandidates := s.candidatesForTokenLocked(req.Attempt)
	s.mu.Unlock()

	tokenRefs := refsForFingerprints(tokenCandidates, refByFingerprint)
	if err := WriteRouteSnippet(s.paths.SSHRouteRuntimeDir(), req.Attempt, tokenRefs); err != nil {
		return fmt.Errorf("Writing SSH route snippet: %w", err)
	}
	return nil
}

func (s *Service) Success(attempt string, clientPID int) error {
	s.mu.Lock()
	current, ok := s.attemptBySuccessLocked(attempt, clientPID)
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("Attempt %q not found", attempt)
	}
	s.deleteAttemptLocked(current)
	remaining := s.candidatesForTokenLocked(current.Token)
	s.mu.Unlock()

	if len(remaining) == 0 {
		RemoveRouteSnippet(s.paths.SSHRouteRuntimeDir(), current.Token)
	} else {
		keyStore, _, keys := s.routingSnapshot()
		if keyStore != nil {
			keys = keyStore.List()
		}
		refs, err := BuildKeyRefs(keys, s.paths.SSHManagedKeysDir())
		if err == nil {
			_ = WriteRouteSnippet(s.paths.SSHRouteRuntimeDir(), current.Token, refsForFingerprints(remaining, KeyRefsByFingerprint(refs)))
		}
	}

	keyStore, _, _ := s.routingSnapshot()
	if current.LastKey == "" || keyStore == nil {
		return nil
	}
	if current.Target.Kind == TargetGit {
		return nil
	}
	if err := keyStore.RecordSSHRouteProof(
		current.Target.Canonical,
		current.LastKey,
		vault.SSHRouteProofSSHAuth,
		current.Operation.String(),
		s.now(),
	); err != nil {
		return err
	}
	s.refreshCacheFromKeyStore()
	s.notifyMutation("ssh_route_learned")
	return nil
}

func (s *Service) RecordSignature(clientPID int, fingerprint string) {
	s.mu.Lock()
	attempt, ok := s.attemptByPIDLocked(clientPID)
	if !ok {
		s.mu.Unlock()
		return
	}
	attempt.LastKey = fingerprint
	s.attempts[routeAttemptKey(attempt.Token, attempt.ClientPID)] = attempt
	s.mu.Unlock()
}

func (s *Service) AllowedFingerprints(clientPID int) ([]string, bool) {
	s.mu.RLock()
	attempt, ok := s.attemptByPIDLocked(clientPID)
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	return append([]string(nil), attempt.Candidates...), true
}

func (s *Service) AttemptByPID(clientPID int) (Attempt, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.attemptByPIDLocked(clientPID)
}

func (s *Service) attemptByPIDLocked(clientPID int) (Attempt, bool) {
	key, ok := s.clientAttempt[clientPID]
	if !ok {
		return Attempt{}, false
	}
	attempt, ok := s.attempts[key]
	return attempt, ok
}

func (s *Service) attemptBySuccessLocked(token string, clientPID int) (Attempt, bool) {
	if clientPID > 0 {
		if attempt, ok := s.attemptByPIDLocked(clientPID); ok && attempt.Token == token {
			return attempt, true
		}
	}
	for _, attempt := range s.attempts {
		if attempt.Token == token {
			return attempt, true
		}
	}
	return Attempt{}, false
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
		RemoveRouteSnippet(s.paths.SSHRouteRuntimeDir(), attempt.Token)
	}
}

func (s *Service) deleteAttempt(attempt Attempt) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleteAttemptLocked(attempt)
}

func (s *Service) deleteAttemptLocked(attempt Attempt) {
	delete(s.attempts, routeAttemptKey(attempt.Token, attempt.ClientPID))
	if legacy, ok := s.attempts[attempt.Token]; ok && legacy.ClientPID == attempt.ClientPID {
		delete(s.attempts, attempt.Token)
	}
	delete(s.clientAttempt, attempt.ClientPID)
}

func (s *Service) expireBeforeLocked(cutoff time.Time) {
	for _, attempt := range s.attempts {
		if attempt.Created.After(cutoff) {
			continue
		}
		s.deleteAttemptLocked(attempt)
		RemoveRouteSnippet(s.paths.SSHRouteRuntimeDir(), attempt.Token)
	}
}

func (s *Service) candidatesForTokenLocked(token string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, attempt := range s.attempts {
		if attempt.Token != token {
			continue
		}
		for _, fingerprint := range attempt.Candidates {
			if _, ok := seen[fingerprint]; ok {
				continue
			}
			seen[fingerprint] = struct{}{}
			out = append(out, fingerprint)
		}
	}
	return out
}

func (s *Service) routingSnapshot() (*vault.KeyStore, map[string]vault.SSHRoute, []vault.Key) {
	s.mu.RLock()
	keyStore := s.keyStore
	cachedRoutes := cloneRouteCache(s.cachedRoutes)
	cachedKeys := clonePublicRoutingKeys(s.cachedKeys)
	s.mu.RUnlock()

	if keyStore == nil {
		return nil, cachedRoutes, cachedKeys
	}

	routes := cloneRouteCache(keyStore.SSHRoutes())
	keys := publicRoutingKeys(keyStore.List())

	s.mu.Lock()
	if s.keyStore == keyStore {
		s.cachedRoutes = cloneRouteCache(routes)
		s.cachedKeys = clonePublicRoutingKeys(keys)
	}
	s.mu.Unlock()

	return keyStore, routes, keys
}

func (s *Service) refreshCacheFromKeyStore() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.keyStore != nil {
		s.refreshCacheFromKeyStoreLocked(s.keyStore)
	}
}

func (s *Service) refreshCacheFromKeyStoreLocked(keyStore *vault.KeyStore) {
	s.cachedRoutes = cloneRouteCache(keyStore.SSHRoutes())
	s.cachedKeys = publicRoutingKeys(keyStore.List())
}

func publicRoutingKeys(keys []vault.Key) []vault.Key {
	out := make([]vault.Key, 0, len(keys))
	for _, key := range keys {
		key.EncryptedPrivateKey = ""
		key.EncryptedCipherKey = ""
		key.PrivateKey = nil
		out = append(out, key)
	}
	return out
}

func clonePublicRoutingKeys(keys []vault.Key) []vault.Key {
	out := make([]vault.Key, len(keys))
	copy(out, keys)
	for i := range out {
		out[i].EncryptedPrivateKey = ""
		out[i].EncryptedCipherKey = ""
		out[i].PrivateKey = nil
	}
	return out
}

func cloneRouteCache(routes map[string]vault.SSHRoute) map[string]vault.SSHRoute {
	if len(routes) == 0 {
		return nil
	}
	out := make(map[string]vault.SSHRoute, len(routes))
	for target, route := range routes {
		if len(route.Attempts) > 0 {
			attempts := make(map[string]time.Time, len(route.Attempts))
			for fingerprint, attemptedAt := range route.Attempts {
				attempts[fingerprint] = attemptedAt
			}
			route.Attempts = attempts
		}
		out[target] = route
	}
	return out
}

func routeAttemptKey(token string, clientPID int) string {
	return fmt.Sprintf("%s:%d", token, clientPID)
}

func (s *Service) resolveTarget(req PrepareRequest, operation OperationClass) (Target, error) {
	if req.Target.Canonical != "" {
		return req.Target, nil
	}

	input := PrepareInput{
		Host:         req.Host,
		OriginalHost: req.OriginalHost,
		User:         req.User,
		Port:         req.Port,
	}
	if operation != OperationSSHAuth {
		if target, _, ok := ResolveProcessGitTarget(req.ClientPID, input); ok {
			return target, nil
		}
		if resolved, err := ResolveGitTargetForOperation(req.CWD, req.Branch, operation); err == nil {
			return resolved, nil
		}
	}
	resolved, err := ResolveSSHTarget(input)
	if err != nil {
		return Target{}, err
	}
	return resolved, nil
}

func (s *Service) probeGitProvider(target Target, operation OperationClass, plan CandidatePlan, refs map[string]KeyRef) ([]string, bool, error) {
	if _, ok := DetectProvider(target); !ok {
		return nil, false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), probeTotalTimeout)
	defer cancel()

	attempted := 0
	inconclusive := false
	for _, candidate := range plan.Candidates {
		ref, ok := refs[candidate.Fingerprint]
		if !ok {
			continue
		}
		attempted++
		result := s.prober.Probe(ctx, target, operation, ref)
		switch result.Status {
		case ProbeSuccess:
			proofOperation := operation
			if proofOperation == OperationUnknown {
				proofOperation = OperationRead
			}
			if s.keyStore != nil {
				if err := s.keyStore.RecordSSHRouteProof(
					target.Canonical,
					candidate.Fingerprint,
					vault.SSHRouteProofProviderProbe,
					proofOperation.String(),
					s.now(),
				); err != nil {
					return nil, false, err
				}
				s.refreshCacheFromKeyStore()
				s.notifyMutation("ssh_route_learned")
			}
			return []string{candidate.Fingerprint}, true, nil
		case ProbeDenied:
			continue
		case ProbeSkipped, ProbeInconclusive:
			inconclusive = true
		}
		if ctx.Err() != nil {
			inconclusive = true
			break
		}
	}
	if attempted > 0 && !inconclusive {
		return []string{}, false, nil
	}
	return nil, false, nil
}

func (s *Service) probeSSHServer(target Target, operation OperationClass, plan CandidatePlan) ([]string, bool, error) {
	if s.keyStore == nil {
		return nil, false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), probeTotalTimeout)
	defer cancel()

	attempted := 0
	inconclusive := false
	for _, candidate := range plan.Candidates {
		attempted++
		result := ProbeSSHServer(ctx, target, candidate, s.keyStore)
		switch result.Status {
		case ProbeSuccess:
			proofOperation := operation
			if proofOperation == OperationUnknown {
				proofOperation = OperationSSHAuth
			}
			if err := s.keyStore.RecordSSHRouteProof(
				target.Canonical,
				candidate.Fingerprint,
				vault.SSHRouteProofSSHAuth,
				proofOperation.String(),
				s.now(),
			); err != nil {
				return nil, false, err
			}
			s.refreshCacheFromKeyStore()
			s.notifyMutation("ssh_route_learned")
			return []string{candidate.Fingerprint}, true, nil
		case ProbeDenied:
			continue
		case ProbeSkipped:
			if result.Message == "host key is not trusted" || result.Message == "no known_hosts files found" {
				return nil, false, nil
			}
			inconclusive = true
		case ProbeInconclusive:
			inconclusive = true
		}
		if ctx.Err() != nil {
			inconclusive = true
			break
		}
	}
	if attempted > 0 && !inconclusive {
		return []string{}, false, nil
	}
	return nil, false, nil
}

func (s *Service) notifyMutation(reason string) {
	s.mu.RLock()
	fn := s.onMutation
	s.mu.RUnlock()
	if fn != nil {
		fn(reason)
	}
}

func refsForFingerprints(fingerprints []string, refs map[string]KeyRef) []KeyRef {
	out := make([]KeyRef, 0, len(fingerprints))
	seen := map[string]struct{}{}
	for _, fingerprint := range fingerprints {
		if _, ok := seen[fingerprint]; ok {
			continue
		}
		ref, ok := refs[fingerprint]
		if !ok {
			continue
		}
		seen[fingerprint] = struct{}{}
		out = append(out, ref)
	}
	return out
}

func exactProvenFingerprints(plan CandidatePlan) []string {
	var out []string
	for _, candidate := range plan.Candidates {
		if candidate.Proven && candidate.Reason == "exact" {
			out = append(out, candidate.Fingerprint)
		}
	}
	return out
}
