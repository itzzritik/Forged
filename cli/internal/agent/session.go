package agent

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/itzzritik/forged/cli/internal/keytypes"
	"github.com/itzzritik/forged/cli/internal/platform"
	"github.com/itzzritik/forged/cli/internal/vault"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type RouteSessions interface {
	OpenRouteSession(client platform.ProcessInstance) (func(), bool)
	AllowedFingerprints(client platform.ProcessInstance) ([]string, bool)
	BeginSignature(client platform.ProcessInstance, fingerprint string) (func(), bool)
	RecordSignature(client platform.ProcessInstance, fingerprint string)
}

type sessionAgent struct {
	base      *ForgedAgent
	ctx       context.Context
	client    platform.ProcessInstance
	clientPID int
	routes    RouteSessions
	denied    bool

	mu                    sync.Mutex
	routeRelease          func()
	routeLatched          bool
	releaseAfterOperation bool
	closed                bool
}

func (s *sessionAgent) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	release := s.routeRelease
	s.routeRelease = nil
	s.mu.Unlock()
	if release != nil {
		release()
	}
}

func (s *sessionAgent) releaseRouteScope() {
	s.mu.Lock()
	release := s.routeRelease
	s.routeRelease = nil
	s.routeLatched = false
	s.mu.Unlock()
	if release != nil {
		release()
	}
}

func (s *sessionAgent) acquireRouteScope() (bool, func()) {
	s.mu.Lock()
	if s.denied || s.closed || s.routeLatched {
		s.mu.Unlock()
		return true, func() {}
	}
	s.mu.Unlock()

	release, scoped := s.routes.OpenRouteSession(s.client)
	if !scoped {
		return false, release
	}

	s.mu.Lock()
	if s.closed || s.routeLatched {
		s.mu.Unlock()
		release()
		return true, func() {}
	}
	s.routeLatched = true
	s.routeRelease = release
	s.mu.Unlock()
	return true, func() {}
}

func (s *sessionAgent) List() ([]*agent.Key, error) {
	if s.releaseAfterOperation {
		defer s.releaseRouteScope()
	}
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	if s.denied {
		return nil, nil
	}
	routed, release := s.acquireRouteScope()
	defer release()
	if !routed {
		return s.base.listForClient(s.ctx, s.client.PID)
	}
	allowed := map[string]struct{}{}
	fingerprints, routed := s.routes.AllowedFingerprints(s.client)
	if !routed {
		return nil, nil
	}
	for _, fingerprint := range fingerprints {
		allowed[fingerprint] = struct{}{}
	}
	if len(allowed) == 0 {
		return nil, nil
	}

	activityLog := s.base.activityLog()
	if err := s.base.ensurePrivateKeyAccess(s.ctx); err != nil {
		recordSSHListDenial(s.ctx, activityLog, s.client.PID, err)
		return nil, err
	}
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	fingerprints, routed = s.routes.AllowedFingerprints(s.client)
	if !routed || len(fingerprints) == 0 {
		return nil, nil
	}
	clear(allowed)
	for _, fingerprint := range fingerprints {
		allowed[fingerprint] = struct{}{}
	}

	s.base.recordAgentAccess("ssh_agent_list")

	s.base.mu.RLock()
	defer s.base.mu.RUnlock()
	if s.base.keyStore == nil {
		return nil, fmt.Errorf("Vault is locked")
	}

	byFingerprint := map[string]vault.Key{}
	for _, key := range s.base.keyStore.List() {
		if _, ok := allowed[key.Fingerprint]; ok {
			byFingerprint[key.Fingerprint] = key
		}
	}
	// OpenSSH offers agent keys in list order, so the route's ranking must survive here.
	out := make([]*agent.Key, 0, len(byFingerprint))
	for _, fingerprint := range fingerprints {
		key, ok := byFingerprint[fingerprint]
		if !ok {
			continue
		}
		delete(byFingerprint, fingerprint)
		pub, err := parsePublicKey(key.PublicKey)
		if err != nil {
			continue
		}
		if !keytypes.SupportsSSHSigning(pub.Type()) {
			continue
		}
		out = append(out, &agent.Key{
			Format:  pub.Type(),
			Blob:    pub.Marshal(),
			Comment: sanitizeKeyComment(key.Name),
		})
	}
	return out, nil
}

func (s *sessionAgent) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	return s.SignWithFlags(key, data, 0)
}

func (s *sessionAgent) SignWithFlags(key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	if s.releaseAfterOperation {
		defer s.releaseRouteScope()
	}
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	if s.denied {
		return nil, fmt.Errorf("SSH route client identity could not be verified")
	}
	routed, release := s.acquireRouteScope()
	defer release()
	if !routed {
		return s.base.signWithFlagsForClient(s.ctx, key, data, flags, s.client.PID)
	}
	s.base.recordAgentAccess("ssh_agent_sign")

	allowed := map[string]struct{}{}
	fingerprints, routed := s.routes.AllowedFingerprints(s.client)
	if !routed {
		return nil, fmt.Errorf("No key is allowed for this SSH route")
	}
	activityLog := s.base.activityLog()
	for _, fingerprint := range fingerprints {
		allowed[fingerprint] = struct{}{}
	}
	if len(allowed) == 0 {
		recordSSHSignActivity(s.ctx, activityLog, "denied", "", s.client.PID)
		return nil, fmt.Errorf("No key is allowed for this SSH route")
	}

	if err := s.base.ensurePrivateKeyAccess(s.ctx); err != nil {
		recordSSHSignDenial(s.ctx, activityLog, s.client.PID, err)
		return nil, err
	}
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}

	lookupFingerprint := func() (string, error) {
		s.base.mu.RLock()
		defer s.base.mu.RUnlock()
		if s.base.keyStore == nil {
			return "", fmt.Errorf("Vault is locked")
		}
		_, _, fingerprint, err := s.base.keyStore.SignerByPublicKey(key)
		return fingerprint, err
	}
	fingerprint, err := lookupFingerprint()
	if errors.Is(err, vault.ErrKeyNotFound) {
		refreshErr := s.base.refreshMissingKey(s.ctx, "sign_missing_key")
		if ctxErr := s.ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		if refreshErr == nil {
			fingerprint, err = lookupFingerprint()
		}
	}
	if err != nil {
		if ctxErr := s.ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		recordSSHSignActivity(s.ctx, activityLog, "failed", "", s.client.PID)
		return nil, err
	}

	if _, ok := allowed[fingerprint]; !ok {
		recordSSHSignActivity(s.ctx, activityLog, "denied", fingerprint, s.client.PID)
		return nil, fmt.Errorf("Key not allowed for client")
	}
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}

	release, authorized := s.routes.BeginSignature(s.client, fingerprint)
	if !authorized {
		recordSSHSignActivity(s.ctx, activityLog, "denied", fingerprint, s.client.PID)
		return nil, fmt.Errorf("Key not allowed for client")
	}

	s.base.mu.RLock()
	keyStore := s.base.keyStore
	if keyStore == nil {
		s.base.mu.RUnlock()
		release()
		recordSSHSignActivity(s.ctx, activityLog, "failed", "", s.client.PID)
		return nil, fmt.Errorf("Vault is locked")
	}
	signer, name, currentFingerprint, err := keyStore.SignerByPublicKey(key)
	if err != nil {
		s.base.mu.RUnlock()
		release()
		if ctxErr := s.ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		recordSSHSignActivity(s.ctx, activityLog, "failed", "", s.client.PID)
		return nil, err
	}
	_, allowedCurrent := allowed[currentFingerprint]
	if currentFingerprint != fingerprint || !allowedCurrent {
		s.base.mu.RUnlock()
		release()
		recordSSHSignActivity(s.ctx, activityLog, "denied", currentFingerprint, s.client.PID)
		return nil, fmt.Errorf("Key not allowed for client")
	}
	if err := s.ctx.Err(); err != nil {
		s.base.mu.RUnlock()
		release()
		return nil, err
	}
	sig, err := signWithFlags(signer, data, flags)
	s.base.mu.RUnlock()
	release()
	if err != nil {
		recordSSHSignActivity(s.ctx, activityLog, "failed", fingerprint, s.client.PID)
		return nil, fmt.Errorf("Signing with key %s: %w", name, err)
	}
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}

	s.routes.RecordSignature(s.client, fingerprint)
	s.base.mu.RLock()
	if s.base.keyStore == keyStore {
		keyStore.RecordUsage(name)
	}
	s.base.mu.RUnlock()
	recordSSHSignActivity(s.ctx, activityLog, "success", fingerprint, s.client.PID)
	return sig, nil
}

func (s *sessionAgent) Add(key agent.AddedKey) error   { return s.base.Add(key) }
func (s *sessionAgent) Remove(key ssh.PublicKey) error { return s.base.Remove(key) }
func (s *sessionAgent) RemoveAll() error               { return s.base.RemoveAll() }
func (s *sessionAgent) Lock(passphrase []byte) error   { return s.base.Lock(passphrase) }
func (s *sessionAgent) Unlock(passphrase []byte) error { return s.base.Unlock(passphrase) }
func (s *sessionAgent) Signers() ([]ssh.Signer, error) {
	if s.releaseAfterOperation {
		defer s.releaseRouteScope()
	}
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	if s.denied {
		return nil, fmt.Errorf("Signers are not exposed for unverified SSH clients")
	}
	routed, release := s.acquireRouteScope()
	defer release()
	if !routed {
		return s.base.signers(s.ctx)
	}
	return nil, fmt.Errorf("Signers are not exposed for routed SSH clients")
}
func (s *sessionAgent) Extension(name string, payload []byte) ([]byte, error) {
	return s.base.Extension(name, payload)
}

func signWithFlags(signer ssh.Signer, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	var algorithm string
	switch flags {
	case 0:
		return signer.Sign(nil, data)
	case agent.SignatureFlagRsaSha256:
		algorithm = ssh.KeyAlgoRSASHA256
	case agent.SignatureFlagRsaSha512:
		algorithm = ssh.KeyAlgoRSASHA512
	default:
		return nil, fmt.Errorf("Unsupported SSH agent signature flags: %d", flags)
	}
	if algorithmSigner, ok := signer.(ssh.AlgorithmSigner); ok {
		return algorithmSigner.SignWithAlgorithm(nil, data, algorithm)
	}
	return nil, fmt.Errorf("SSH agent signer does not support %s signatures", algorithm)
}
