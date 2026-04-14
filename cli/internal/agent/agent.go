package agent

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/itzzritik/forged/cli/internal/vault"
)

type ForgedAgent struct {
	mu       sync.RWMutex
	keyStore *vault.KeyStore
	locked   bool
	syncBus  SyncCoordinator
}

type SyncCoordinator interface {
	AgentAccess(reason string)
	RefreshMissingKey(ctx context.Context, reason string) error
}

func New(ks *vault.KeyStore) *ForgedAgent {
	return &ForgedAgent{keyStore: ks}
}

func (a *ForgedAgent) SetSyncCoordinator(syncBus SyncCoordinator) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.syncBus = syncBus
}

func (a *ForgedAgent) List() ([]*agent.Key, error) {
	a.recordAgentAccess("ssh_agent_list")

	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.locked {
		return nil, nil
	}

	keys := a.keyStore.List()
	out := make([]*agent.Key, 0, len(keys))
	for _, k := range keys {
		pub, err := parsePublicKey(k.PublicKey)
		if err != nil {
			continue
		}
		out = append(out, &agent.Key{
			Format:  pub.Type(),
			Blob:    pub.Marshal(),
			Comment: k.Name,
		})
	}
	return out, nil
}

func (a *ForgedAgent) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	return a.SignWithFlags(key, data, 0)
}

func (a *ForgedAgent) SignWithFlags(key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	a.recordAgentAccess("ssh_agent_sign")

	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.locked {
		return nil, fmt.Errorf("agent is locked")
	}

	signer, name, err := a.findSigner(key)
	if err != nil {
		a.mu.RUnlock()
		if refreshErr := a.refreshMissingKey("sign_missing_key"); refreshErr == nil {
			a.mu.RLock()
			signer, name, err = a.findSigner(key)
		} else {
			a.mu.RLock()
		}
		if err != nil {
			return nil, err
		}
	}

	var algo string
	if flags&agent.SignatureFlagRsaSha256 != 0 {
		algo = ssh.KeyAlgoRSASHA256
	} else if flags&agent.SignatureFlagRsaSha512 != 0 {
		algo = ssh.KeyAlgoRSASHA512
	}

	var sig *ssh.Signature
	if algo != "" {
		if as, ok := signer.(ssh.AlgorithmSigner); ok {
			sig, err = as.SignWithAlgorithm(nil, data, algo)
		} else {
			sig, err = signer.Sign(nil, data)
		}
	} else {
		sig, err = signer.Sign(nil, data)
	}

	if err != nil {
		return nil, fmt.Errorf("signing with key %s: %w", name, err)
	}

	a.keyStore.RecordUsage(name)
	return sig, nil
}

func (a *ForgedAgent) Add(key agent.AddedKey) error {
	return fmt.Errorf("use 'forged add' or 'forged generate' to add keys")
}

func (a *ForgedAgent) Remove(key ssh.PublicKey) error {
	return fmt.Errorf("use 'forged remove' to remove keys")
}

func (a *ForgedAgent) RemoveAll() error {
	return fmt.Errorf("use 'forged remove' to remove keys")
}

func (a *ForgedAgent) Lock(passphrase []byte) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.locked = true
	return nil
}

func (a *ForgedAgent) Unlock(passphrase []byte) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.locked = false
	return nil
}

func (a *ForgedAgent) Signers() ([]ssh.Signer, error) {
	a.recordAgentAccess("ssh_agent_signers")

	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.locked {
		return nil, nil
	}

	keys := a.keyStore.List()
	var signers []ssh.Signer
	for _, k := range keys {
		signer, err := ssh.ParsePrivateKey(k.PrivateKey)
		if err != nil {
			continue
		}
		signers = append(signers, signer)
	}
	return signers, nil
}

func (a *ForgedAgent) Extension(extensionType string, contents []byte) ([]byte, error) {
	return nil, agent.ErrExtensionUnsupported
}

func (a *ForgedAgent) findSigner(pub ssh.PublicKey) (ssh.Signer, string, error) {
	wanted := pub.Marshal()
	for _, k := range a.keyStore.List() {
		parsed, err := parsePublicKey(k.PublicKey)
		if err != nil {
			continue
		}
		if bytes.Equal(parsed.Marshal(), wanted) {
			signer, err := ssh.ParsePrivateKey(k.PrivateKey)
			if err != nil {
				return nil, "", fmt.Errorf("parsing private key for %s: %w", k.Name, err)
			}
			return signer, k.Name, nil
		}
	}
	return nil, "", fmt.Errorf("key not found in vault")
}

func parsePublicKey(authorizedKey string) (ssh.PublicKey, error) {
	pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(authorizedKey))
	if err != nil {
		return nil, err
	}
	return pub, nil
}

func (a *ForgedAgent) recordAgentAccess(reason string) {
	a.mu.RLock()
	syncBus := a.syncBus
	a.mu.RUnlock()

	if syncBus != nil {
		syncBus.AgentAccess(reason)
	}
}

func (a *ForgedAgent) refreshMissingKey(reason string) error {
	a.mu.RLock()
	syncBus := a.syncBus
	a.mu.RUnlock()

	if syncBus == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()
	return syncBus.RefreshMissingKey(ctx, reason)
}

var _ agent.ExtendedAgent = (*ForgedAgent)(nil)
