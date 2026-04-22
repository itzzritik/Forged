package agent

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/itzzritik/forged/cli/internal/sensitiveauth"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/itzzritik/forged/cli/internal/vault"
)

type ForgedAgent struct {
	mu       sync.RWMutex
	keyStore *vault.KeyStore
	locked   bool
	auth     SensitiveAuthorizer
	syncBus  SyncCoordinator
	routes   RouteSessions
}

type SensitiveAuthorizer interface {
	IsUnlocked() bool
	Authorize(context.Context, sensitiveauth.Action) (sensitiveauth.AuthorizeResult, error)
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
	a.syncBus = normalizeSyncCoordinator(syncBus)
}

func (a *ForgedAgent) SetSensitiveAuthorizer(auth SensitiveAuthorizer) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.auth = auth
}

func (a *ForgedAgent) SetKeyStore(keyStore *vault.KeyStore) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.keyStore = keyStore
}

func (a *ForgedAgent) SetRouteSessions(routes RouteSessions) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.routes = routes
}

func (a *ForgedAgent) ForClientPID(clientPID int) agent.ExtendedAgent {
	a.mu.RLock()
	routes := a.routes
	a.mu.RUnlock()
	if routes == nil {
		return a
	}
	return &sessionAgent{
		base:      a,
		clientPID: clientPID,
		routes:    routes,
	}
}

func (a *ForgedAgent) List() ([]*agent.Key, error) {
	a.recordAgentAccess("ssh_agent_list")

	a.mu.RLock()
	if a.locked {
		a.mu.RUnlock()
		return nil, nil
	}
	a.mu.RUnlock()

	if err := a.ensurePrivateKeyAccess(); err != nil {
		return nil, err
	}

	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.keyStore == nil {
		return nil, fmt.Errorf("vault is locked")
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
	if a.locked {
		a.mu.RUnlock()
		return nil, fmt.Errorf("agent is locked")
	}
	if a.keyStore == nil {
		a.mu.RUnlock()
		if err := a.ensurePrivateKeyAccess(); err != nil {
			return nil, err
		}
		a.mu.RLock()
	}
	a.mu.RUnlock()

	if err := a.ensurePrivateKeyAccess(); err != nil {
		return nil, err
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.locked {
		return nil, fmt.Errorf("agent is locked")
	}
	if a.keyStore == nil {
		return nil, fmt.Errorf("vault is locked")
	}

	signer, name, _, err := a.keyStore.SignerByPublicKey(key)
	if err != nil {
		a.mu.RUnlock()
		if refreshErr := a.refreshMissingKey("sign_missing_key"); refreshErr == nil {
			a.mu.RLock()
			if a.keyStore != nil {
				signer, name, _, err = a.keyStore.SignerByPublicKey(key)
			}
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
	return fmt.Errorf("use the Forged Key tab to import or generate keys")
}

func (a *ForgedAgent) Remove(key ssh.PublicKey) error {
	return fmt.Errorf("use the Forged Key tab to remove keys")
}

func (a *ForgedAgent) RemoveAll() error {
	return fmt.Errorf("use the Forged Key tab to remove keys")
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
	if a.locked {
		a.mu.RUnlock()
		return nil, nil
	}
	if a.keyStore == nil {
		a.mu.RUnlock()
		if err := a.ensurePrivateKeyAccess(); err != nil {
			return nil, err
		}
		a.mu.RLock()
	}
	a.mu.RUnlock()

	if err := a.ensurePrivateKeyAccess(); err != nil {
		return nil, err
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.locked {
		return nil, nil
	}
	if a.keyStore == nil {
		return nil, fmt.Errorf("vault is locked")
	}

	return a.keyStore.Signers()
}

func (a *ForgedAgent) Extension(extensionType string, contents []byte) ([]byte, error) {
	return nil, agent.ErrExtensionUnsupported
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

func (a *ForgedAgent) ensurePrivateKeyAccess() error {
	a.mu.RLock()
	auth := a.auth
	a.mu.RUnlock()

	if auth == nil || auth.IsUnlocked() {
		return nil
	}

	result, err := auth.Authorize(context.Background(), sensitiveauth.ActionExternal)
	if err != nil {
		return err
	}
	if result.PasswordRequired {
		return fmt.Errorf("system authentication is required for external use")
	}
	return nil
}

func normalizeSyncCoordinator(syncBus SyncCoordinator) SyncCoordinator {
	if syncBus == nil {
		return nil
	}
	value := reflect.ValueOf(syncBus)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if value.IsNil() {
			return nil
		}
	}
	return syncBus
}

var _ agent.ExtendedAgent = (*ForgedAgent)(nil)
