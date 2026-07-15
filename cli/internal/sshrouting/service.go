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

type SessionChecker interface {
	IsUnlocked() bool
}

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
	mu             sync.RWMutex
	paths          config.Paths
	keyStore       *vault.KeyStore
	sessionChecker SessionChecker
	cachedKeys     []vault.Key
	cachedRoutes   map[string]vault.SSHRoute
	now            func() time.Time
	attempts       map[string]Attempt
	attemptOwners  map[string]uint64
	attemptSeq     uint64
	clientAttempt  map[int]string
	prober         ProviderProber
	onMutation     func(reason string)
}

func NewService(paths config.Paths, keyStore *vault.KeyStore) *Service {
	return &Service{
		paths:         paths,
		keyStore:      keyStore,
		now:           func() time.Time { return time.Now().UTC() },
		attempts:      map[string]Attempt{},
		attemptOwners: map[string]uint64{},
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

func (s *Service) SetSessionChecker(sessionChecker SessionChecker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessionChecker = sessionChecker
}

func (s *Service) SetOnMutation(fn func(reason string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onMutation = fn
}

func (s *Service) PrepareContext(ctx context.Context, req PrepareRequest) error {
	if err := validateAttemptToken(req.Attempt); err != nil {
		return err
	}
	if req.ClientPID <= 0 {
		return fmt.Errorf("SSH route client PID must be positive")
	}

	now := s.now()
	attemptKey := routeAttemptKey(req.Attempt, req.ClientPID)
	s.mu.Lock()
	s.expireBeforeLocked(now.Add(-routeSnippetTTL))
	if previous, ok := s.attemptByPIDLocked(req.ClientPID); ok {
		s.deleteAttemptLocked(previous)
	}
	s.attemptSeq++
	owner := s.attemptSeq
	s.attempts[attemptKey] = Attempt{
		Token:     req.Attempt,
		ClientPID: req.ClientPID,
		Created:   now,
	}
	s.attemptOwners[attemptKey] = owner
	s.clientAttempt[req.ClientPID] = attemptKey
	s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}

	process := InspectProcessContext(ctx, req.ClientPID)
	if err := ctx.Err(); err != nil {
		return err
	}
	operation := process.Operation
	target, err := s.resolveTarget(ctx, req, process)
	if err != nil {
		return err
	}
	if operation == OperationUnknown && target.Kind == TargetSSH {
		operation = OperationSSHAuth
	}

	keyStore, routes, keys := s.routingSnapshot()
	if keyStore == nil && len(keys) == 0 {
		return ErrRouteMemoryLocked
	}

	refs, err := BuildKeyRefs(keys, s.paths.SSHManagedKeysDir())
	if err != nil {
		return fmt.Errorf("Building SSH key refs: %w", err)
	}
	if err := SyncPublicHintFiles(s.paths.SSHManagedKeysDir(), refs, now); err != nil {
		return fmt.Errorf("Syncing SSH public key hints: %w", err)
	}
	s.mu.Lock()
	_ = CleanupRouteRuntime(s.paths.SSHRouteRuntimeDir(), now.Add(-routeSnippetTTL))
	s.mu.Unlock()

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
		probed, proved, err := s.probeGitProvider(ctx, target, operation, plan, refByFingerprint, keyStore)
		if err != nil {
			return err
		}
		if proved || probed != nil {
			selected = probed
			probeProved = proved
		}
	}
	if target.Kind == TargetSSH && keyStore != nil {
		probed, proved, err := s.probeSSHServer(ctx, target, operation, plan, keyStore)
		if err != nil {
			return err
		}
		if proved || probed != nil {
			selected = probed
			probeProved = proved
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	attempt := Attempt{
		Token:       req.Attempt,
		ClientPID:   req.ClientPID,
		Target:      target,
		Operation:   operation,
		Candidates:  append([]string(nil), selected...),
		HadExact:    plan.HadExact,
		ProbeProved: probeProved,
		Created:     now,
	}

	s.mu.Lock()
	currentKey, ok := s.clientAttempt[req.ClientPID]
	if !ok || currentKey != attemptKey || s.attemptOwners[attemptKey] != owner {
		s.mu.Unlock()
		return fmt.Errorf("SSH route attempt was superseded")
	}
	if err := ctx.Err(); err != nil {
		s.mu.Unlock()
		return err
	}
	s.attempts[attemptKey] = attempt
	if err := s.writeRouteSnippetLocked(req.Attempt, refByFingerprint); err != nil {
		attempt.Candidates = nil
		attempt.HadExact = false
		attempt.ProbeProved = false
		s.attempts[attemptKey] = attempt
		_ = s.writeRouteSnippetLocked(req.Attempt, refByFingerprint)
		s.mu.Unlock()
		return fmt.Errorf("Writing SSH route snippet: %w", err)
	}
	if err := ctx.Err(); err != nil {
		attempt.Candidates = nil
		attempt.HadExact = false
		attempt.ProbeProved = false
		s.attempts[attemptKey] = attempt
		_ = s.writeRouteSnippetLocked(req.Attempt, refByFingerprint)
		s.mu.Unlock()
		return err
	}
	s.mu.Unlock()
	return nil
}

func (s *Service) Success(attempt string, clientPID int) error {
	if clientPID <= 0 {
		return fmt.Errorf("SSH route client PID must be positive")
	}
	keyStore, _, keys := s.routingSnapshot()
	refs, err := BuildKeyRefs(keys, s.paths.SSHManagedKeysDir())
	if err != nil {
		return fmt.Errorf("Building SSH key refs: %w", err)
	}
	refByFingerprint := KeyRefsByFingerprint(refs)

	s.mu.Lock()
	s.expireBeforeLocked(s.now().Add(-routeSnippetTTL))
	current, ok := s.attemptBySuccessLocked(attempt, clientPID)
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("Attempt %q not found", attempt)
	}
	s.deleteAttemptLocked(current)
	if err := s.writeRouteSnippetLocked(current.Token, refByFingerprint); err != nil {
		_ = s.writeRouteSnippetLocked(current.Token, refByFingerprint)
		s.mu.Unlock()
		return fmt.Errorf("Writing SSH route snippet: %w", err)
	}
	s.mu.Unlock()

	if current.LastKey == "" || !s.keyStoreCurrent(keyStore) {
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
		if !s.keyStoreCurrent(keyStore) {
			return nil
		}
		return err
	}
	s.refreshCacheFromKeyStore(keyStore)
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
	attempt, ok := s.attemptByPIDLocked(clientPID)
	return attempt, ok && attempt.Token == token
}

func (s *Service) ExpireBefore(cutoff time.Time) {
	s.mu.Lock()
	s.expireBeforeLocked(cutoff)
	s.mu.Unlock()
}

func (s *Service) deleteAttemptLocked(attempt Attempt) {
	attemptKey := routeAttemptKey(attempt.Token, attempt.ClientPID)
	delete(s.attempts, attemptKey)
	delete(s.attemptOwners, attemptKey)
	if legacy, ok := s.attempts[attempt.Token]; ok && legacy.ClientPID == attempt.ClientPID {
		delete(s.attempts, attempt.Token)
		delete(s.attemptOwners, attempt.Token)
	}
	delete(s.clientAttempt, attempt.ClientPID)
}

func (s *Service) expireBeforeLocked(cutoff time.Time) {
	affected := map[string]struct{}{}
	for _, attempt := range s.attempts {
		if attempt.Created.After(cutoff) {
			continue
		}
		s.deleteAttemptLocked(attempt)
		affected[attempt.Token] = struct{}{}
	}
	for token := range affected {
		if len(s.candidatesForTokenLocked(token)) == 0 {
			RemoveRouteSnippet(s.paths.SSHRouteRuntimeDir(), token)
		}
	}
}

func (s *Service) writeRouteSnippetLocked(token string, refs map[string]KeyRef) error {
	candidates := s.candidatesForTokenLocked(token)
	if len(candidates) == 0 {
		RemoveRouteSnippet(s.paths.SSHRouteRuntimeDir(), token)
		return nil
	}
	return WriteRouteSnippet(s.paths.SSHRouteRuntimeDir(), token, refsForFingerprints(candidates, refs))
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
	keyStore := s.currentKeyStore()
	s.mu.RLock()
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

func (s *Service) currentKeyStore() *vault.KeyStore {
	s.mu.RLock()
	keyStore := s.keyStore
	sessionChecker := s.sessionChecker
	s.mu.RUnlock()
	if keyStore == nil || (sessionChecker != nil && !sessionChecker.IsUnlocked()) {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.keyStore != keyStore {
		return nil
	}
	return keyStore
}

func (s *Service) keyStoreCurrent(keyStore *vault.KeyStore) bool {
	return keyStore != nil && s.currentKeyStore() == keyStore
}

func (s *Service) withCurrentKeyStore(keyStore *vault.KeyStore, fn func() error) error {
	if !s.keyStoreCurrent(keyStore) {
		return errProbeSessionLocked
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.keyStore != keyStore {
		return errProbeSessionLocked
	}
	return fn()
}

func (s *Service) refreshCacheFromKeyStore(keyStore *vault.KeyStore) {
	if !s.keyStoreCurrent(keyStore) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.keyStore == keyStore {
		s.refreshCacheFromKeyStoreLocked(keyStore)
	}
}

func (s *Service) refreshCacheFromKeyStoreLocked(keyStore *vault.KeyStore) {
	s.cachedRoutes = cloneRouteCache(keyStore.SSHRoutes())
	s.cachedKeys = publicRoutingKeys(keyStore.List())
}

func publicRoutingKeys(keys []vault.Key) []vault.Key {
	out := make([]vault.Key, 0, len(keys))
	for _, key := range keys {
		if !supportsSSHSigningKey(key) {
			continue
		}
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

func (s *Service) resolveTarget(ctx context.Context, req PrepareRequest, process ProcessContext) (Target, error) {
	if req.Target.Canonical != "" {
		return req.Target, nil
	}

	input := PrepareInput{
		Host:         req.Host,
		OriginalHost: req.OriginalHost,
		User:         req.User,
		Port:         req.Port,
	}
	if process.Operation != OperationSSHAuth {
		if process.RepoPath != "" {
			if target, err := targetFromRepoPath(input, process.RepoPath); err == nil {
				return target, nil
			}
		}
		if resolved, err := ResolveGitTargetForOperationContext(ctx, req.CWD, req.Branch, process.Operation); err == nil {
			return resolved, nil
		}
		if err := ctx.Err(); err != nil {
			return Target{}, err
		}
	}
	resolved, err := ResolveSSHTarget(input)
	if err != nil {
		return Target{}, err
	}
	return resolved, nil
}

func (s *Service) probeGitProvider(ctx context.Context, target Target, operation OperationClass, plan CandidatePlan, refs map[string]KeyRef, keyStore *vault.KeyStore) ([]string, bool, error) {
	if _, ok := DetectProvider(target); !ok {
		return nil, false, nil
	}
	probeCtx, cancel := context.WithTimeout(ctx, probeTotalTimeout)
	defer cancel()

	attempted := 0
	inconclusive := false
	for _, candidate := range plan.Candidates {
		ref, ok := refs[candidate.Fingerprint]
		if !ok {
			continue
		}
		if !s.keyStoreCurrent(keyStore) {
			return nil, false, nil
		}
		attempted++
		result := s.prober.Probe(probeCtx, target, operation, ref)
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		if !s.keyStoreCurrent(keyStore) {
			return nil, false, nil
		}
		switch result.Status {
		case ProbeSuccess:
			proofOperation := operation
			if proofOperation == OperationUnknown {
				proofOperation = OperationRead
			}
			if !s.keyStoreCurrent(keyStore) {
				return nil, false, nil
			}
			if err := keyStore.RecordSSHRouteProof(
				target.Canonical,
				candidate.Fingerprint,
				vault.SSHRouteProofProviderProbe,
				proofOperation.String(),
				s.now(),
			); err != nil {
				if !s.keyStoreCurrent(keyStore) {
					return nil, false, nil
				}
				return nil, false, err
			}
			s.refreshCacheFromKeyStore(keyStore)
			s.notifyMutation("ssh_route_learned")
			return []string{candidate.Fingerprint}, true, nil
		case ProbeDenied:
			continue
		case ProbeSkipped, ProbeInconclusive:
			inconclusive = true
		}
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		if probeCtx.Err() != nil {
			inconclusive = true
			break
		}
	}
	if attempted > 0 && !inconclusive {
		return []string{}, false, nil
	}
	return nil, false, nil
}

func (s *Service) probeSSHServer(ctx context.Context, target Target, operation OperationClass, plan CandidatePlan, keyStore *vault.KeyStore) ([]string, bool, error) {
	if !s.keyStoreCurrent(keyStore) {
		return nil, false, nil
	}
	probeCtx, cancel := context.WithTimeout(ctx, probeTotalTimeout)
	defer cancel()

	attempted := 0
	inconclusive := false
	for _, candidate := range plan.Candidates {
		if !s.keyStoreCurrent(keyStore) {
			return nil, false, nil
		}
		attempted++
		result := ProbeSSHServer(probeCtx, target, candidate, keyStore, func(fn func() error) error {
			return s.withCurrentKeyStore(keyStore, fn)
		})
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		if !s.keyStoreCurrent(keyStore) {
			return nil, false, nil
		}
		switch result.Status {
		case ProbeSuccess:
			proofOperation := operation
			if proofOperation == OperationUnknown {
				proofOperation = OperationSSHAuth
			}
			if !s.keyStoreCurrent(keyStore) {
				return nil, false, nil
			}
			if err := keyStore.RecordSSHRouteProof(
				target.Canonical,
				candidate.Fingerprint,
				vault.SSHRouteProofSSHAuth,
				proofOperation.String(),
				s.now(),
			); err != nil {
				if !s.keyStoreCurrent(keyStore) {
					return nil, false, nil
				}
				return nil, false, err
			}
			s.refreshCacheFromKeyStore(keyStore)
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
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		if probeCtx.Err() != nil {
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
