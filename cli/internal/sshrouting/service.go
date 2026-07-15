package sshrouting

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/platform"
	"github.com/itzzritik/forged/cli/internal/vault"
)

var ErrRouteMemoryLocked = errors.New("SSH route memory is locked")

type SessionChecker interface {
	IsUnlocked() bool
}

type PrepareRequest struct {
	Attempt      string
	ClientPID    int
	HelperPID    int
	CWD          string
	Host         string
	OriginalHost string
	User         string
	Port         string
	Branch       string
	Target       Target
}

type Attempt struct {
	Token           string
	ClientPID       int
	Process         platform.ProcessInstance
	Target          Target
	Operation       OperationClass
	Candidates      []string
	HadExact        bool
	ProbeProved     bool
	LastKey         string
	Created         time.Time
	ExpiresAt       time.Time
	SuccessRecorded bool
	Leases          int
}

type Service struct {
	mu                 sync.RWMutex
	paths              config.Paths
	keyStore           *vault.KeyStore
	sessionChecker     SessionChecker
	cachedKeys         []vault.Key
	cachedRoutes       map[string]vault.SSHRoute
	now                func() time.Time
	attempts           map[string]Attempt
	attemptOwners      map[string]uint64
	attemptSeq         uint64
	clientAttempt      map[int]string
	unscoped           map[platform.ProcessInstance]int
	unscopedChanged    chan struct{}
	runtimeUntrusted   bool
	runtimeWriteFailed bool
	prober             ProviderProber
	onMutation         func(reason string)
}

func NewService(paths config.Paths, keyStore *vault.KeyStore) *Service {
	s := &Service{
		paths:           paths,
		keyStore:        keyStore,
		now:             func() time.Time { return time.Now().UTC() },
		attempts:        map[string]Attempt{},
		attemptOwners:   map[string]uint64{},
		clientAttempt:   map[int]string{},
		unscoped:        map[platform.ProcessInstance]int{},
		unscopedChanged: make(chan struct{}),
		prober:          NewProviderProber(paths.AgentSocket()),
	}
	s.restoreRouteState()
	return s
}

func (s *Service) restoreRouteState() {
	attempts, untrusted := loadRouteState(s.paths.SSHRouteRuntimeDir())
	s.runtimeUntrusted = untrusted
	if untrusted {
		return
	}
	for _, attempt := range attempts {
		key := routeAttemptKey(attempt.Token, attempt.ClientPID)
		if _, exists := s.clientAttempt[attempt.ClientPID]; exists {
			s.runtimeUntrusted = true
			s.attempts = map[string]Attempt{}
			s.attemptOwners = map[string]uint64{}
			s.clientAttempt = map[int]string{}
			return
		}
		s.attemptSeq++
		s.attempts[key] = attempt
		s.attemptOwners[key] = s.attemptSeq
		s.clientAttempt[attempt.ClientPID] = key
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
	process, err := routeClientProcess(req.HelperPID, req.ClientPID)
	if err != nil {
		return err
	}

	now := s.now()
	currentRefs := s.routeRefs()
	attemptKey := routeAttemptKey(req.Attempt, req.ClientPID)
	var attempt Attempt
	var replacedToken string
	s.mu.Lock()
	s.reconcileRouteAttemptsLocked(now, currentRefs)
	if s.routeScopeGuardLocked() {
		s.mu.Unlock()
		return fmt.Errorf("SSH route runtime needs reset")
	}
	previous, hasPrevious := s.attemptByPIDLocked(req.ClientPID)
	if hasPrevious {
		if previous.Process == process {
			if previous.Token != req.Attempt {
				s.mu.Unlock()
				return fmt.Errorf("SSH route client already has a different route attempt")
			}
			if !previous.ExpiresAt.After(now) {
				s.mu.Unlock()
				return fmt.Errorf("SSH route authorization has expired")
			}
			if len(previous.Candidates) > 0 {
				if err := s.writeRouteSnippetLocked(previous.Token, currentRefs); err != nil {
					s.runtimeWriteFailed = true
					s.mu.Unlock()
					return fmt.Errorf("Refreshing SSH route slots: %w", err)
				}
				s.mu.Unlock()
				return nil
			}
			attempt = previous
		} else {
			replacedToken = previous.Token
			s.deleteAttemptLocked(previous)
		}
	}
	if !hasPrevious && len(s.attempts) >= routeStateMaxAttempts {
		s.mu.Unlock()
		return fmt.Errorf("too many active SSH route attempts")
	}
	if !attempt.Process.Valid() {
		attempt = newRouteAttempt(req.Attempt, process, now)
	}
	s.attemptSeq++
	owner := s.attemptSeq
	s.attempts[attemptKey] = attempt
	s.attemptOwners[attemptKey] = owner
	s.clientAttempt[req.ClientPID] = attemptKey
	if err := s.writeRouteStateLocked(); err != nil {
		s.runtimeWriteFailed = true
		s.mu.Unlock()
		return fmt.Errorf("Persisting SSH route guard: %w", err)
	}
	for _, token := range []string{replacedToken, req.Attempt} {
		if token == "" || (token == replacedToken && token == req.Attempt) {
			continue
		}
		if err := s.writeRouteSnippetLocked(token, currentRefs); err != nil {
			s.runtimeWriteFailed = true
			s.mu.Unlock()
			return fmt.Errorf("Refreshing SSH route slots: %w", err)
		}
	}
	s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}

	processContext := InspectProcessContext(ctx, req.ClientPID)
	if err := ctx.Err(); err != nil {
		return err
	}
	operation := processContext.Operation
	target, err := s.resolveTarget(ctx, req, processContext)
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

	attempt.Target = target
	attempt.Operation = operation
	attempt.Candidates = append([]string(nil), selected...)
	attempt.HadExact = plan.HadExact
	attempt.ProbeProved = probeProved
	attempt.LastKey = ""
	attempt.SuccessRecorded = false

	if err := s.waitForUnscopedRouteClient(ctx, process); err != nil {
		return err
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
	if s.routeScopeGuardLocked() {
		s.mu.Unlock()
		return fmt.Errorf("SSH route runtime needs reset")
	}
	available := s.candidatesForTokenWithAttemptLocked(req.Attempt, attemptKey, attempt)
	if !s.allCandidatesRepresentedLocked(req.Attempt, attemptKey, attempt, available) {
		s.demoteAttemptLocked(attemptKey, refByFingerprint)
		s.mu.Unlock()
		return fmt.Errorf("SSH route identity slots are full")
	}
	s.attempts[attemptKey] = attempt
	if err := s.writeRouteStateLocked(); err != nil {
		s.runtimeWriteFailed = true
		s.demoteAttemptLocked(attemptKey, refByFingerprint)
		s.mu.Unlock()
		return fmt.Errorf("Persisting SSH route authorization: %w", err)
	}
	if err := s.writeRouteSnippetLocked(req.Attempt, refByFingerprint); err != nil {
		s.runtimeWriteFailed = true
		s.demoteAttemptLocked(attemptKey, refByFingerprint)
		s.mu.Unlock()
		return fmt.Errorf("Writing SSH route snippet: %w", err)
	}
	if err := ctx.Err(); err != nil {
		s.demoteAttemptLocked(attemptKey, refByFingerprint)
		s.mu.Unlock()
		return err
	}
	s.recoverRouteRuntimeLocked(refByFingerprint)
	s.mu.Unlock()
	return nil
}

func (s *Service) Success(attempt string, clientPID, helperPID int) error {
	if err := validateAttemptToken(attempt); err != nil {
		return err
	}
	if clientPID <= 0 {
		return fmt.Errorf("SSH route client PID must be positive")
	}
	process, err := routeClientProcess(helperPID, clientPID)
	if err != nil {
		return err
	}
	keyStore, _, _ := s.routingSnapshot()
	refs := s.routeRefs()
	now := s.now()

	s.mu.Lock()
	s.reconcileRouteAttemptsLocked(now, refs)
	current, ok := s.attemptBySuccessLocked(attempt, clientPID)
	if !ok || current.Process != process {
		s.mu.Unlock()
		return fmt.Errorf("Attempt %q not found", attempt)
	}
	if !current.ExpiresAt.After(now) || len(current.Candidates) == 0 {
		s.mu.Unlock()
		return fmt.Errorf("Attempt %q has expired", attempt)
	}
	if current.SuccessRecorded {
		s.mu.Unlock()
		return nil
	}
	current.SuccessRecorded = true
	s.attempts[routeAttemptKey(current.Token, current.ClientPID)] = current
	s.mu.Unlock()

	if current.LastKey == "" || !s.keyStoreCurrent(keyStore) {
		return nil
	}
	if current.Target.Canonical == "" || current.Target.Kind == TargetGit {
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

// Slot verifies that the direct SSH child requesting a static identity slot
// owns that candidate. It returns an error so Match exec omits the slot.
func (s *Service) Slot(attempt string, clientPID, helperPID, slot int) error {
	if err := validateAttemptToken(attempt); err != nil {
		return err
	}
	if clientPID <= 0 {
		return fmt.Errorf("SSH route client PID must be positive")
	}
	if slot < 1 || slot > routeIdentitySlotCount {
		return fmt.Errorf("invalid SSH route identity slot")
	}
	process, err := routeClientProcess(helperPID, clientPID)
	if err != nil {
		return err
	}
	refs := s.routeRefs()
	now := s.now()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.reconcileRouteAttemptsLocked(now, refs)
	if s.routeScopeGuardLocked() {
		return fmt.Errorf("SSH route runtime needs reset")
	}
	current, ok := s.attemptBySuccessLocked(attempt, clientPID)
	if !ok || current.Process != process || !current.ExpiresAt.After(now) || len(current.Candidates) == 0 {
		return fmt.Errorf("SSH route identity slot is not authorized")
	}
	candidates := s.candidatesForTokenLocked(attempt)
	if slot > len(candidates) {
		return fmt.Errorf("SSH route identity slot is not authorized")
	}
	fingerprint := candidates[slot-1]
	if !containsFingerprint(current.Candidates, fingerprint) {
		return fmt.Errorf("SSH route identity slot is not authorized")
	}
	if !routeIdentitySlotMatches(s.paths.SSHRouteRuntimeDir(), attempt, slot, fingerprint) {
		return fmt.Errorf("SSH route identity slot is unavailable")
	}
	return nil
}

func (s *Service) RecordSignature(process platform.ProcessInstance, fingerprint string) {
	allowed, routed := s.AllowedFingerprints(process)
	if !routed || !containsFingerprint(allowed, fingerprint) {
		return
	}

	s.mu.Lock()
	attempt, ok := s.attemptByPIDLocked(process.PID)
	if ok && attempt.Process == process && attempt.ExpiresAt.After(s.now()) && containsFingerprint(s.allowedCandidatesForAttemptLocked(attempt), fingerprint) {
		attempt.LastKey = fingerprint
		s.attempts[routeAttemptKey(attempt.Token, attempt.ClientPID)] = attempt
	}
	s.mu.Unlock()
}

func (s *Service) AllowedFingerprints(process platform.ProcessInstance) ([]string, bool) {
	if !process.Valid() {
		return nil, true
	}
	now := s.now()
	refs := s.routeRefs()
	s.mu.Lock()
	s.reconcileRouteAttemptsLocked(now, refs)
	if s.routeScopeGuardLocked() {
		s.mu.Unlock()
		return nil, true
	}
	attempt, ok := s.attemptByPIDLocked(process.PID)
	if !ok {
		guarded := s.routeScopeGuardLocked()
		s.mu.Unlock()
		return nil, guarded
	}
	if attempt.Process != process {
		s.mu.Unlock()
		return nil, true
	}
	current, err := platform.SameProcessInstance(process)
	if err != nil {
		s.mu.Unlock()
		return nil, true
	}
	if !current {
		if attempt.Leases == 0 {
			token := attempt.Token
			s.deleteAttemptLocked(attempt)
			if err := s.writeRouteStateLocked(); err != nil {
				s.runtimeWriteFailed = true
			}
			if err := s.writeRouteSnippetLocked(token, refs); err != nil {
				s.runtimeWriteFailed = true
			}
		}
		s.recoverRouteRuntimeLocked(refs)
		s.mu.Unlock()
		return nil, true
	}
	fingerprints := s.allowedCandidatesForAttemptLocked(attempt)
	s.mu.Unlock()
	return fingerprints, true
}

// OpenRouteSession latches route scope for one admitted agent connection.
// A later PID exit or route cleanup can only deny that connection, never
// downgrade it to unrestricted agent access.
func (s *Service) OpenRouteSession(process platform.ProcessInstance) (func(), bool) {
	if !process.Valid() {
		return func() {}, true
	}
	refs := s.routeRefs()
	s.mu.Lock()
	s.reconcileRouteAttemptsLocked(s.now(), refs)
	if s.routeScopeGuardLocked() {
		s.mu.Unlock()
		return func() {}, true
	}
	attempt, ok := s.attemptByPIDLocked(process.PID)
	if ok && attempt.Process == process {
		attempt.Leases++
		s.attempts[routeAttemptKey(attempt.Token, attempt.ClientPID)] = attempt
		s.mu.Unlock()
		var once sync.Once
		return func() {
			once.Do(func() { s.releaseRouteSession(process) })
		}, true
	}
	current, err := platform.SameProcessInstance(process)
	if err != nil || !current {
		s.mu.Unlock()
		return func() {}, true
	}
	s.unscoped[process]++
	s.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() { s.releaseUnscopedRouteClient(process) })
	}, false
}

func (s *Service) releaseUnscopedRouteClient(process platform.ProcessInstance) {
	s.mu.Lock()
	if count := s.unscoped[process]; count > 1 {
		s.unscoped[process] = count - 1
	} else {
		delete(s.unscoped, process)
	}
	close(s.unscopedChanged)
	s.unscopedChanged = make(chan struct{})
	s.mu.Unlock()
}

func (s *Service) waitForUnscopedRouteClient(ctx context.Context, process platform.ProcessInstance) error {
	for {
		s.mu.Lock()
		waiting := s.unscoped[process] > 0
		changed := s.unscopedChanged
		s.mu.Unlock()
		if !waiting {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-changed:
		}
	}
}

func (s *Service) releaseRouteSession(process platform.ProcessInstance) {
	refs := s.routeRefs()
	s.mu.Lock()
	if attempt, ok := s.attemptByPIDLocked(process.PID); ok && attempt.Process == process && attempt.Leases > 0 {
		attempt.Leases--
		s.attempts[routeAttemptKey(attempt.Token, attempt.ClientPID)] = attempt
	}
	s.reconcileRouteAttemptsLocked(s.now(), refs)
	s.mu.Unlock()
}

// BeginSignature admits one routed signature and keeps route expiry from
// changing until the caller releases the returned function.
func (s *Service) BeginSignature(process platform.ProcessInstance, fingerprint string) (func(), bool) {
	for {
		allowed, routed := s.AllowedFingerprints(process)
		if !routed || !containsFingerprint(allowed, fingerprint) {
			return nil, false
		}

		s.mu.RLock()
		attempt, ok := s.attemptByPIDLocked(process.PID)
		if ok && attempt.Process == process && attempt.ExpiresAt.After(s.now()) && containsFingerprint(s.allowedCandidatesForAttemptLocked(attempt), fingerprint) {
			return s.mu.RUnlock, true
		}
		s.mu.RUnlock()
	}
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
	refs := s.routeRefs()
	s.mu.Lock()
	s.expireBeforeLocked(cutoff, refs)
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

func (s *Service) expireBeforeLocked(cutoff time.Time, refs map[string]KeyRef) {
	affected := map[string]struct{}{}
	changed := false
	for key, attempt := range s.attempts {
		if attempt.Created.After(cutoff) {
			continue
		}
		attempt.Candidates = nil
		attempt.HadExact = false
		attempt.ProbeProved = false
		attempt.LastKey = ""
		s.attempts[key] = attempt
		affected[attempt.Token] = struct{}{}
		changed = true
	}
	if changed {
		if err := s.writeRouteStateLocked(); err != nil {
			s.runtimeWriteFailed = true
		}
	}
	for token := range affected {
		if err := s.writeRouteSnippetLocked(token, refs); err != nil {
			s.runtimeWriteFailed = true
		}
	}
	s.recoverRouteRuntimeLocked(refs)
}

func (s *Service) writeRouteSnippetLocked(token string, refs map[string]KeyRef) error {
	return s.writeRouteSnippetForCandidatesLocked(token, s.candidatesForTokenLocked(token), refs)
}

func (s *Service) writeRouteSnippetForCandidatesLocked(token string, candidates []string, refs map[string]KeyRef) error {
	if len(candidates) == 0 {
		return RemoveRouteSnippet(s.paths.SSHRouteRuntimeDir(), token)
	}
	selected := refsForFingerprints(candidates, refs)
	if len(selected) != len(candidates) {
		return fmt.Errorf("missing SSH public key hint for route candidate")
	}
	return WriteRouteSnippet(s.paths.SSHRouteRuntimeDir(), token, selected)
}

func (s *Service) candidatesForTokenLocked(token string) []string {
	return s.candidatesForTokenWithAttemptLocked(token, "", Attempt{})
}

func (s *Service) candidatesForTokenWithAttemptLocked(token, attemptKey string, replacement Attempt) []string {
	attempts := make([]routeAttemptCandidate, 0, len(s.attempts))
	for key, attempt := range s.attempts {
		if attempt.Token != token {
			continue
		}
		if key == attemptKey {
			attempt = replacement
		}
		attempts = append(attempts, routeAttemptCandidate{key: key, attempt: attempt})
	}
	return candidatesForRouteAttempts(attempts)
}

func (s *Service) allCandidatesRepresentedLocked(token, attemptKey string, replacement Attempt, available []string) bool {
	for key, attempt := range s.attempts {
		if attempt.Token != token {
			continue
		}
		if key == attemptKey {
			attempt = replacement
		}
		for _, fingerprint := range attempt.Candidates {
			if !containsFingerprint(available, fingerprint) {
				return false
			}
		}
	}
	return true
}

func (s *Service) allowedCandidatesForAttemptLocked(attempt Attempt) []string {
	return candidatesForAttempt(attempt.Candidates, s.candidatesForTokenLocked(attempt.Token))
}

func candidatesForAttempt(candidates, available []string) []string {
	allowed := make([]string, 0, len(candidates))
	for _, fingerprint := range candidates {
		if containsFingerprint(available, fingerprint) {
			allowed = append(allowed, fingerprint)
		}
	}
	return allowed
}

func (s *Service) reconcileRouteAttemptsLocked(now time.Time, refs map[string]KeyRef) {
	affected := map[string]struct{}{}
	changed := false
	for key, attempt := range s.attempts {
		current, err := platform.SameProcessInstance(attempt.Process)
		if err == nil && !current && attempt.Leases == 0 {
			s.deleteAttemptLocked(attempt)
			affected[attempt.Token] = struct{}{}
			changed = true
			continue
		}
		if attempt.ExpiresAt.After(now) || len(attempt.Candidates) == 0 {
			continue
		}
		attempt.Candidates = nil
		attempt.HadExact = false
		attempt.ProbeProved = false
		attempt.LastKey = ""
		s.attempts[key] = attempt
		affected[attempt.Token] = struct{}{}
		changed = true
	}
	if changed {
		if err := s.writeRouteStateLocked(); err != nil {
			s.runtimeWriteFailed = true
		}
	}
	for token := range affected {
		if err := s.writeRouteSnippetLocked(token, refs); err != nil {
			s.runtimeWriteFailed = true
		}
	}
	s.recoverRouteRuntimeLocked(refs)
}

func (s *Service) demoteAttemptLocked(attemptKey string, refs map[string]KeyRef) {
	attempt, ok := s.attempts[attemptKey]
	if !ok {
		return
	}
	attempt.Candidates = nil
	attempt.HadExact = false
	attempt.ProbeProved = false
	attempt.LastKey = ""
	attempt.SuccessRecorded = false
	s.attempts[attemptKey] = attempt
	if err := s.writeRouteStateLocked(); err != nil {
		s.runtimeWriteFailed = true
	}
	if err := s.writeRouteSnippetLocked(attempt.Token, refs); err != nil {
		s.runtimeWriteFailed = true
	}
	s.recoverRouteRuntimeLocked(refs)
}

func (s *Service) routeScopeGuardLocked() bool {
	return s.runtimeUntrusted || s.runtimeWriteFailed
}

func (s *Service) recoverRouteRuntimeLocked(refs map[string]KeyRef) {
	if s.runtimeUntrusted || !s.runtimeWriteFailed {
		return
	}
	if err := s.writeRouteStateLocked(); err != nil {
		return
	}
	tokens := map[string]struct{}{}
	attempts := make([]Attempt, 0, len(s.attempts))
	for _, attempt := range s.attempts {
		tokens[attempt.Token] = struct{}{}
		attempts = append(attempts, attempt)
	}
	for token := range tokens {
		if err := s.writeRouteSnippetLocked(token, refs); err != nil {
			return
		}
	}
	artifacts, err := routeRuntimeArtifacts(s.paths.SSHRouteRuntimeDir())
	if err != nil {
		return
	}
	if !routeRuntimeMatchesState(s.paths.SSHRouteRuntimeDir(), artifacts, attempts) {
		return
	}
	s.runtimeWriteFailed = false
}

func (s *Service) routeRefs() map[string]KeyRef {
	_, _, keys := s.routingSnapshot()
	refs, err := BuildKeyRefs(keys, s.paths.SSHManagedKeysDir())
	if err != nil {
		return nil
	}
	return KeyRefsByFingerprint(refs)
}

func newRouteAttempt(token string, process platform.ProcessInstance, now time.Time) Attempt {
	return Attempt{
		Token:     token,
		ClientPID: process.PID,
		Process:   process,
		Created:   now,
		ExpiresAt: now.Add(routeSnippetTTL),
	}
}

func routeClientProcess(helperPID, clientPID int) (platform.ProcessInstance, error) {
	if helperPID <= 0 {
		return platform.ProcessInstance{}, fmt.Errorf("SSH route helper PID must be positive")
	}
	helper, err := platform.ProcessInfoForPID(helperPID)
	if err != nil {
		return platform.ProcessInstance{}, fmt.Errorf("Inspecting SSH route helper: %w", err)
	}
	if helper.ParentPID != clientPID {
		return platform.ProcessInstance{}, fmt.Errorf("SSH route helper is not owned by client PID %d", clientPID)
	}
	client, err := platform.ProcessInfoForPID(clientPID)
	if err != nil {
		return platform.ProcessInstance{}, fmt.Errorf("Inspecting SSH route client: %w", err)
	}
	confirmedHelper, err := platform.ProcessInfoForPID(helperPID)
	if err != nil {
		return platform.ProcessInstance{}, fmt.Errorf("Rechecking SSH route helper: %w", err)
	}
	if confirmedHelper.Instance != helper.Instance || confirmedHelper.ParentPID != clientPID {
		return platform.ProcessInstance{}, fmt.Errorf("SSH route helper parent changed during verification")
	}
	return client.Instance, nil
}

func containsFingerprint(fingerprints []string, fingerprint string) bool {
	for _, candidate := range fingerprints {
		if candidate == fingerprint {
			return true
		}
	}
	return false
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
