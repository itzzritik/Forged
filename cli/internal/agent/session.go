package agent

import (
	"context"
	"fmt"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type RouteSessions interface {
	AllowedFingerprints(clientPID int) ([]string, bool)
	RecordSignature(clientPID int, fingerprint string)
}

type sessionAgent struct {
	base      *ForgedAgent
	ctx       context.Context
	clientPID int
	routes    RouteSessions
}

func (s *sessionAgent) List() ([]*agent.Key, error) {
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	allowed := map[string]struct{}{}
	fingerprints, routed := s.routes.AllowedFingerprints(s.clientPID)
	if !routed {
		return s.base.listForClient(s.ctx, s.clientPID)
	}
	for _, fingerprint := range fingerprints {
		allowed[fingerprint] = struct{}{}
	}
	if len(allowed) == 0 {
		return nil, nil
	}

	activityLog := s.base.activityLog()
	if err := s.base.ensurePrivateKeyAccess(s.ctx); err != nil {
		recordSSHListDenial(s.ctx, activityLog, s.clientPID, err)
		return nil, err
	}
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}

	s.base.recordAgentAccess("ssh_agent_list")

	s.base.mu.RLock()
	defer s.base.mu.RUnlock()
	if s.base.keyStore == nil {
		return nil, fmt.Errorf("Vault is locked")
	}

	keys := s.base.keyStore.List()
	out := make([]*agent.Key, 0, len(keys))
	for _, key := range keys {
		if _, ok := allowed[key.Fingerprint]; !ok {
			continue
		}
		pub, err := parsePublicKey(key.PublicKey)
		if err != nil {
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
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	s.base.recordAgentAccess("ssh_agent_sign")

	allowed := map[string]struct{}{}
	fingerprints, routed := s.routes.AllowedFingerprints(s.clientPID)
	if !routed {
		return s.base.signWithFlagsForClient(s.ctx, key, data, flags, s.clientPID)
	}
	activityLog := s.base.activityLog()
	for _, fingerprint := range fingerprints {
		allowed[fingerprint] = struct{}{}
	}
	if len(allowed) == 0 {
		recordSSHSignActivity(s.ctx, activityLog, "denied", "", s.clientPID)
		return nil, fmt.Errorf("No key is allowed for this SSH route")
	}

	if err := s.base.ensurePrivateKeyAccess(s.ctx); err != nil {
		recordSSHSignDenial(s.ctx, activityLog, s.clientPID, err)
		return nil, err
	}
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}

	s.base.mu.RLock()
	defer s.base.mu.RUnlock()

	if s.base.keyStore == nil {
		recordSSHSignActivity(s.ctx, activityLog, "failed", "", s.clientPID)
		return nil, fmt.Errorf("Vault is locked")
	}

	signer, name, fingerprint, err := s.base.keyStore.SignerByPublicKey(key)
	if err != nil {
		s.base.mu.RUnlock()
		refreshErr := s.base.refreshMissingKey(s.ctx, "sign_missing_key")
		s.base.mu.RLock()
		if ctxErr := s.ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		if refreshErr == nil && s.base.keyStore != nil {
			signer, name, fingerprint, err = s.base.keyStore.SignerByPublicKey(key)
		}
		if err != nil {
			recordSSHSignActivity(s.ctx, activityLog, "failed", "", s.clientPID)
			return nil, err
		}
	}

	if _, ok := allowed[fingerprint]; !ok {
		recordSSHSignActivity(s.ctx, activityLog, "denied", fingerprint, s.clientPID)
		return nil, fmt.Errorf("Key not allowed for client")
	}
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}

	sig, err := signWithFlags(signer, data, flags)
	if err != nil {
		recordSSHSignActivity(s.ctx, activityLog, "failed", fingerprint, s.clientPID)
		return nil, fmt.Errorf("Signing with key %s: %w", name, err)
	}
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}

	s.routes.RecordSignature(s.clientPID, fingerprint)
	s.base.keyStore.RecordUsage(name)
	recordSSHSignActivity(s.ctx, activityLog, "success", fingerprint, s.clientPID)
	return sig, nil
}

func (s *sessionAgent) Add(key agent.AddedKey) error   { return s.base.Add(key) }
func (s *sessionAgent) Remove(key ssh.PublicKey) error { return s.base.Remove(key) }
func (s *sessionAgent) RemoveAll() error               { return s.base.RemoveAll() }
func (s *sessionAgent) Lock(passphrase []byte) error   { return s.base.Lock(passphrase) }
func (s *sessionAgent) Unlock(passphrase []byte) error { return s.base.Unlock(passphrase) }
func (s *sessionAgent) Signers() ([]ssh.Signer, error) {
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	if _, routed := s.routes.AllowedFingerprints(s.clientPID); !routed {
		return s.base.signers(s.ctx)
	}
	return nil, fmt.Errorf("Signers are not exposed for routed SSH clients")
}
func (s *sessionAgent) Extension(name string, payload []byte) ([]byte, error) {
	return s.base.Extension(name, payload)
}

func signWithFlags(signer ssh.Signer, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	var algorithm string
	if flags&agent.SignatureFlagRsaSha256 != 0 {
		algorithm = ssh.KeyAlgoRSASHA256
	} else if flags&agent.SignatureFlagRsaSha512 != 0 {
		algorithm = ssh.KeyAlgoRSASHA512
	}
	if algorithm == "" {
		return signer.Sign(nil, data)
	}
	if algorithmSigner, ok := signer.(ssh.AlgorithmSigner); ok {
		return algorithmSigner.SignWithAlgorithm(nil, data, algorithm)
	}
	return signer.Sign(nil, data)
}
