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

	"github.com/itzzritik/forged/cli/internal/activity"
	"github.com/itzzritik/forged/cli/internal/keytypes"
	"github.com/itzzritik/forged/cli/internal/platform"
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
	activity *activity.ActivityLog
}

type contextAgent struct {
	*ForgedAgent
	ctx       context.Context
	clientPID int
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

func (a *ForgedAgent) SetActivityLog(activityLog *activity.ActivityLog) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.activity = activityLog
}

func (a *ForgedAgent) ForClientPID(clientPID int) agent.ExtendedAgent {
	return a.ForClientPIDContext(context.Background(), clientPID)
}

func (a *ForgedAgent) ForContext(ctx context.Context) agent.ExtendedAgent {
	return &contextAgent{ForgedAgent: a, ctx: ctx}
}

func (a *ForgedAgent) ForClientPIDContext(ctx context.Context, clientPID int) agent.ExtendedAgent {
	process, err := platform.ProcessInfoForPID(clientPID)
	if err != nil {
		return a.ForDeniedClientContext(ctx, clientPID)
	}
	return a.ForClientProcessContext(ctx, process.Instance)
}

// ForClientProcessContext scopes routing to each operation. Long-lived agent
// connections must use ForClientProcessSessionContext and its release function.
func (a *ForgedAgent) ForClientProcessContext(ctx context.Context, process platform.ProcessInstance) agent.ExtendedAgent {
	scoped, _ := a.ForClientProcessSessionContext(ctx, process)
	if session, ok := scoped.(*sessionAgent); ok {
		session.releaseAfterOperation = true
	}
	return scoped
}

// ForClientProcessSessionContext returns a client-scoped agent plus the
// connection release function that must run after serving the peer.
func (a *ForgedAgent) ForClientProcessSessionContext(ctx context.Context, process platform.ProcessInstance) (agent.ExtendedAgent, func()) {
	a.mu.RLock()
	routes := a.routes
	a.mu.RUnlock()
	if routes == nil {
		return &contextAgent{ForgedAgent: a, ctx: ctx, clientPID: process.PID}, func() {}
	}
	session := &sessionAgent{
		base:   a,
		ctx:    ctx,
		client: process,
		routes: routes,
	}
	return session, session.Close
}

func (a *ForgedAgent) ForDeniedClientContext(ctx context.Context, clientPID int) agent.ExtendedAgent {
	return &sessionAgent{base: a, ctx: ctx, clientPID: clientPID, denied: true}
}

func (a *ForgedAgent) List() ([]*agent.Key, error) {
	return a.listForClient(context.Background(), 0)
}

func (a *ForgedAgent) listForClient(ctx context.Context, clientPID int) ([]*agent.Key, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	a.recordAgentAccess("ssh_agent_list")
	activityLog := a.activityLog()

	if err := a.ensurePrivateKeyAccess(ctx); err != nil {
		recordSSHListDenial(ctx, activityLog, clientPID, err)
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
		if !keytypes.SupportsSSHSigning(pub.Type()) {
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
	return a.signWithFlagsForClient(context.Background(), key, data, 0, 0)
}

func (a *ForgedAgent) SignWithFlags(key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	return a.signWithFlagsForClient(context.Background(), key, data, flags, 0)
}

func (a *ForgedAgent) signWithFlagsForClient(ctx context.Context, key ssh.PublicKey, data []byte, flags agent.SignatureFlags, clientPID int) (*ssh.Signature, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	a.recordAgentAccess("ssh_agent_sign")
	activityLog := a.activityLog()

	a.mu.RLock()
	if a.keyStore == nil {
		a.mu.RUnlock()
		if err := a.ensurePrivateKeyAccess(ctx); err != nil {
			recordSSHSignDenial(ctx, activityLog, clientPID, err)
			return nil, err
		}
		a.mu.RLock()
	}
	a.mu.RUnlock()

	if err := a.ensurePrivateKeyAccess(ctx); err != nil {
		recordSSHSignDenial(ctx, activityLog, clientPID, err)
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.keyStore == nil {
		recordSSHSignActivity(ctx, activityLog, "failed", "", clientPID)
		return nil, fmt.Errorf("Vault is locked")
	}

	signer, name, fingerprint, err := a.keyStore.SignerByPublicKey(key)
	if errors.Is(err, vault.ErrKeyNotFound) {
		a.mu.RUnlock()
		refreshErr := a.refreshMissingKey(ctx, "sign_missing_key")
		a.mu.RLock()
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		if refreshErr == nil && a.keyStore != nil {
			signer, name, fingerprint, err = a.keyStore.SignerByPublicKey(key)
		}
	}
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		recordSSHSignActivity(ctx, activityLog, "failed", "", clientPID)
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	sig, err := signWithFlags(signer, data, flags)

	if err != nil {
		recordSSHSignActivity(ctx, activityLog, "failed", fingerprint, clientPID)
		return nil, fmt.Errorf("Signing with key %s: %w", name, err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	a.keyStore.RecordUsage(name)
	recordSSHSignActivity(ctx, activityLog, "success", fingerprint, clientPID)
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

func (a *ForgedAgent) activityLog() *activity.ActivityLog {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.activity
}

func recordSSHSignActivity(ctx context.Context, activityLog *activity.ActivityLog, result, fingerprint string, clientPID int) {
	if ctx.Err() != nil || activityLog == nil {
		return
	}
	if clientPID < 1 {
		clientPID = 0
	}
	activityLog.Record(activity.ActivityEvent{
		Type:        "ssh_agent_sign",
		Fingerprint: fingerprint,
		Result:      result,
		ClientPID:   clientPID,
	})
}

func recordSSHSignDenial(ctx context.Context, activityLog *activity.ActivityLog, clientPID int, err error) {
	if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, sensitiveauth.ErrAuthenticationCanceled) {
		return
	}
	recordSSHSignActivity(ctx, activityLog, "denied", "", clientPID)
}

func recordSSHListDenial(ctx context.Context, activityLog *activity.ActivityLog, clientPID int, err error) {
	if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, sensitiveauth.ErrAuthenticationCanceled) {
		return
	}
	if activityLog == nil {
		return
	}
	if clientPID < 1 {
		clientPID = 0
	}
	activityLog.Record(activity.ActivityEvent{
		Type:      "ssh_agent_list",
		Result:    "denied",
		ClientPID: clientPID,
	})
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
	return a.ForgedAgent.listForClient(a.ctx, a.clientPID)
}

func (a *contextAgent) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	return a.ForgedAgent.signWithFlagsForClient(a.ctx, key, data, 0, a.clientPID)
}

func (a *contextAgent) SignWithFlags(key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	return a.ForgedAgent.signWithFlagsForClient(a.ctx, key, data, flags, a.clientPID)
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
