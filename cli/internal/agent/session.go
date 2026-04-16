package agent

import (
	"fmt"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type RouteSessions interface {
	AllowedFingerprints(clientPID int) []string
	RecordSignature(clientPID int, fingerprint string)
}

type sessionAgent struct {
	base      *ForgedAgent
	clientPID int
	routes    RouteSessions
}

func (s *sessionAgent) List() ([]*agent.Key, error) {
	allowed := map[string]struct{}{}
	for _, fingerprint := range s.routes.AllowedFingerprints(s.clientPID) {
		allowed[fingerprint] = struct{}{}
	}
	if len(allowed) == 0 {
		return s.base.List()
	}

	s.base.recordAgentAccess("ssh_agent_list")

	s.base.mu.RLock()
	defer s.base.mu.RUnlock()
	if s.base.locked {
		return nil, nil
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
			Comment: key.Name,
		})
	}
	return out, nil
}

func (s *sessionAgent) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	return s.SignWithFlags(key, data, 0)
}

func (s *sessionAgent) SignWithFlags(key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	s.base.recordAgentAccess("ssh_agent_sign")

	allowed := map[string]struct{}{}
	for _, fingerprint := range s.routes.AllowedFingerprints(s.clientPID) {
		allowed[fingerprint] = struct{}{}
	}
	if len(allowed) == 0 {
		return s.base.SignWithFlags(key, data, flags)
	}

	s.base.mu.RLock()
	defer s.base.mu.RUnlock()

	if s.base.locked {
		return nil, fmt.Errorf("agent is locked")
	}

	signer, name, fingerprint, err := s.base.findSigner(key)
	if err != nil {
		s.base.mu.RUnlock()
		if refreshErr := s.base.refreshMissingKey("sign_missing_key"); refreshErr == nil {
			s.base.mu.RLock()
			signer, name, fingerprint, err = s.base.findSigner(key)
		} else {
			s.base.mu.RLock()
		}
		if err != nil {
			return nil, err
		}
	}

	if _, ok := allowed[fingerprint]; !ok {
		return nil, fmt.Errorf("key not allowed for client")
	}

	sig, err := signWithFlags(signer, data, flags)
	if err != nil {
		return nil, fmt.Errorf("signing with key %s: %w", name, err)
	}

	s.routes.RecordSignature(s.clientPID, fingerprint)
	s.base.keyStore.RecordUsage(name)
	return sig, nil
}

func (s *sessionAgent) Add(key agent.AddedKey) error   { return s.base.Add(key) }
func (s *sessionAgent) Remove(key ssh.PublicKey) error { return s.base.Remove(key) }
func (s *sessionAgent) RemoveAll() error               { return s.base.RemoveAll() }
func (s *sessionAgent) Lock(passphrase []byte) error   { return s.base.Lock(passphrase) }
func (s *sessionAgent) Unlock(passphrase []byte) error { return s.base.Unlock(passphrase) }
func (s *sessionAgent) Signers() ([]ssh.Signer, error) { return s.base.Signers() }
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
