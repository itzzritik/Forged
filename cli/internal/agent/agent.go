package agent

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/itzzritik/forged/cli/internal/sensitiveauth"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/itzzritik/forged/cli/internal/vault"
)

var errAgentLockUnsupported = errors.New("ssh agent locking is not supported")

type ForgedAgent struct {
	mu       sync.RWMutex
	keyStore *vault.KeyStore
	auth     SensitiveAuthorizer
	syncBus  SyncCoordinator
	routes   RouteSessions
}

type contextAgent struct {
	*ForgedAgent
	ctx context.Context
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
	return a.ForClientPIDContext(context.Background(), clientPID)
}

func (a *ForgedAgent) ForContext(ctx context.Context) agent.ExtendedAgent {
	return &contextAgent{ForgedAgent: a, ctx: ctx}
}

func (a *ForgedAgent) ForClientPIDContext(ctx context.Context, clientPID int) agent.ExtendedAgent {
	a.mu.RLock()
	routes := a.routes
	a.mu.RUnlock()
	if routes == nil {
		return a.ForContext(ctx)
	}
	return &sessionAgent{
		base:      a,
		ctx:       ctx,
		clientPID: clientPID,
		routes:    routes,
	}
}

func (a *ForgedAgent) List() ([]*agent.Key, error) {
	return a.list(context.Background())
}

func (a *ForgedAgent) list(ctx context.Context) ([]*agent.Key, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	a.recordAgentAccess("ssh_agent_list")

	if err := a.ensurePrivateKeyAccess(ctx); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.keyStore == nil {
		return nil, fmt.Errorf("Vault is locked")
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
			Comment: sanitizeKeyComment(k.Name),
		})
	}
	return out, nil
}

func sanitizeKeyComment(name string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return unicode.ReplacementChar
		}
		return r
	}, name)
}

func (a *ForgedAgent) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	return a.signWithFlags(context.Background(), key, data, 0)
}

func (a *ForgedAgent) SignWithFlags(key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	return a.signWithFlags(context.Background(), key, data, flags)
}

func (a *ForgedAgent) signWithFlags(ctx context.Context, key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	a.recordAgentAccess("ssh_agent_sign")

	a.mu.RLock()
	if a.keyStore == nil {
		a.mu.RUnlock()
		if err := a.ensurePrivateKeyAccess(ctx); err != nil {
			return nil, err
		}
		a.mu.RLock()
	}
	a.mu.RUnlock()

	if err := a.ensurePrivateKeyAccess(ctx); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.keyStore == nil {
		return nil, fmt.Errorf("Vault is locked")
	}

	signer, name, _, err := a.keyStore.SignerByPublicKey(key)
	if err != nil {
		a.mu.RUnlock()
		refreshErr := a.refreshMissingKey(ctx, "sign_missing_key")
		a.mu.RLock()
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		if refreshErr == nil && a.keyStore != nil {
			signer, name, _, err = a.keyStore.SignerByPublicKey(key)
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
	if err := ctx.Err(); err != nil {
		return nil, err
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
		return nil, fmt.Errorf("Signing with key %s: %w", name, err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	a.keyStore.RecordUsage(name)
	return sig, nil
}

func (a *ForgedAgent) Add(key agent.AddedKey) error {
	return fmt.Errorf("Use the Forged Key tab to import or generate keys")
}

func (a *ForgedAgent) Remove(key ssh.PublicKey) error {
	return fmt.Errorf("Use the Forged Key tab to remove keys")
}

func (a *ForgedAgent) RemoveAll() error {
	return fmt.Errorf("Use the Forged Key tab to remove keys")
}

func (a *ForgedAgent) Lock([]byte) error {
	return errAgentLockUnsupported
}

func (a *ForgedAgent) Unlock([]byte) error {
	return errAgentLockUnsupported
}

func (a *ForgedAgent) Signers() ([]ssh.Signer, error) {
	return a.signers(context.Background())
}

func (a *ForgedAgent) signers(ctx context.Context) ([]ssh.Signer, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	a.recordAgentAccess("ssh_agent_signers")

	a.mu.RLock()
	if a.keyStore == nil {
		a.mu.RUnlock()
		if err := a.ensurePrivateKeyAccess(ctx); err != nil {
			return nil, err
		}
		a.mu.RLock()
	}
	a.mu.RUnlock()

	if err := a.ensurePrivateKeyAccess(ctx); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.keyStore == nil {
		return nil, fmt.Errorf("Vault is locked")
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

func (a *ForgedAgent) refreshMissingKey(ctx context.Context, reason string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	a.mu.RLock()
	syncBus := a.syncBus
	a.mu.RUnlock()

	if syncBus == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
	defer cancel()
	return syncBus.RefreshMissingKey(ctx, reason)
}

func (a *ForgedAgent) ensurePrivateKeyAccess(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	a.mu.RLock()
	auth := a.auth
	a.mu.RUnlock()

	if auth == nil || auth.IsUnlocked() {
		return nil
	}

	result, err := auth.Authorize(ctx, sensitiveauth.ActionExternal)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if result.PasswordRequired {
		return fmt.Errorf("System Auth is required for external use")
	}
	return nil
}

func (a *contextAgent) List() ([]*agent.Key, error) {
	return a.ForgedAgent.list(a.ctx)
}

func (a *contextAgent) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	return a.ForgedAgent.signWithFlags(a.ctx, key, data, 0)
}

func (a *contextAgent) SignWithFlags(key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	return a.ForgedAgent.signWithFlags(a.ctx, key, data, flags)
}

func (a *contextAgent) Signers() ([]ssh.Signer, error) {
	return a.ForgedAgent.signers(a.ctx)
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
var _ agent.ExtendedAgent = (*contextAgent)(nil)
