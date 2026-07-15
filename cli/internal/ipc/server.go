package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"

	"github.com/itzzritik/forged/cli/internal/activity"
	"github.com/itzzritik/forged/cli/internal/buildinfo"
	"github.com/itzzritik/forged/cli/internal/platform"
	"github.com/itzzritik/forged/cli/internal/sensitiveauth"
	"github.com/itzzritik/forged/cli/internal/sshrouting"
	forgedsync "github.com/itzzritik/forged/cli/internal/sync"
	"github.com/itzzritik/forged/cli/internal/vault"
)

type SSHRouteHandler interface {
	PrepareContext(context.Context, sshrouting.PrepareRequest) error
	Success(attempt string, clientPID int) error
	DebugSnapshot() (sshrouting.DebugSnapshot, error)
	Clear(target string) error
	ClearAll() error
}

type Server struct {
	socketPath     string
	stateMu        sync.RWMutex
	lifecycleMu    sync.Mutex
	vault          *vault.Vault
	keyStore       *vault.KeyStore
	activityLog    *activity.ActivityLog
	listener       net.Listener
	connections    map[net.Conn]struct{}
	stopping       bool
	logger         *slog.Logger
	wg             sync.WaitGroup
	syncBus        *forgedsync.Bus
	syncError      string
	syncLink       func(SyncLinkArgs) error
	syncUnlink     func() error
	accountReplace func(AccountCredentialsArgs) error
	accountClear   func() error
	authBroker     *sensitiveauth.Broker
	onKeyChange    func()
	onVaultChange  func(string)
	onReadSync     func()
	sshRoutes      SSHRouteHandler
}

func (s *Server) SetSyncBus(bus *forgedsync.Bus) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.syncBus = bus
}

func (s *Server) SetSyncError(err string) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.syncError = err
}

func (s *Server) SetSyncLinkHandler(handler func(SyncLinkArgs) error) {
	s.syncLink = handler
}

func (s *Server) SetSyncUnlinkHandler(handler func() error) {
	s.syncUnlink = handler
}

func (s *Server) SetAccountReplaceHandler(handler func(AccountCredentialsArgs) error) {
	s.accountReplace = handler
}

func (s *Server) SetAccountClearHandler(handler func() error) {
	s.accountClear = handler
}

func (s *Server) SetSensitiveAuthBroker(broker *sensitiveauth.Broker) {
	s.authBroker = broker
}

func (s *Server) SetVaultState(v *vault.Vault, ks *vault.KeyStore) {
	s.stateMu.Lock()
	s.vault = v
	s.keyStore = ks
	s.stateMu.Unlock()
}

func (s *Server) SetOnKeyChange(fn func()) {
	s.onKeyChange = fn
}

func (s *Server) SetOnVaultChange(fn func(string)) {
	s.onVaultChange = fn
}

func (s *Server) SetOnReadSync(fn func()) {
	s.onReadSync = fn
}

func (s *Server) SetSSHRouteHandler(handler SSHRouteHandler) {
	s.sshRoutes = handler
}

func NewServer(socketPath string, v *vault.Vault, ks *vault.KeyStore, al *activity.ActivityLog, logger *slog.Logger) *Server {
	return &Server{
		socketPath:  socketPath,
		vault:       v,
		keyStore:    ks,
		activityLog: al,
		logger:      logger,
		connections: make(map[net.Conn]struct{}),
	}
}

func (s *Server) Start() error {
	s.lifecycleMu.Lock()
	if s.stopping {
		s.lifecycleMu.Unlock()
		return fmt.Errorf("starting IPC server: already stopping")
	}
	if s.listener != nil {
		s.lifecycleMu.Unlock()
		return fmt.Errorf("starting IPC server: already started")
	}
	ln, err := platform.Listen(s.socketPath)
	if err != nil {
		s.lifecycleMu.Unlock()
		return err
	}
	s.listener = ln
	s.wg.Add(1)
	s.lifecycleMu.Unlock()

	go func() {
		defer s.wg.Done()
		s.acceptLoop(ln)
	}()

	return nil
}

func (s *Server) BeginStop() {
	s.lifecycleMu.Lock()
	if s.stopping {
		s.lifecycleMu.Unlock()
		return
	}
	s.stopping = true
	listener := s.listener
	connections := make([]net.Conn, 0, len(s.connections))
	for conn := range s.connections {
		connections = append(connections, conn)
	}
	s.lifecycleMu.Unlock()

	if listener != nil {
		_ = listener.Close()
	}
	for _, conn := range connections {
		_ = conn.Close()
	}
}

func (s *Server) Wait() {
	s.wg.Wait()
}

func (s *Server) Stop() {
	s.BeginStop()
	s.Wait()
}

func (s *Server) acceptLoop(listener net.Listener) {
	var retryDelay time.Duration
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			//lint:ignore SA1019 Listener implementations use Temporary to classify retriable accept failures.
			if netErr, ok := err.(net.Error); ok && netErr.Temporary() {
				if retryDelay == 0 {
					retryDelay = 5 * time.Millisecond
				} else {
					retryDelay = min(2*retryDelay, time.Second)
				}
				s.logger.Warn("temporary IPC accept failure", "error", err, "retry_in", retryDelay)
				time.Sleep(retryDelay)
				continue
			}
			s.logger.Error("IPC accept loop stopped", "error", err)
			return
		}
		retryDelay = 0
		if !s.admit(conn) {
			_ = conn.Close()
			return
		}
		go func(conn net.Conn) {
			defer s.wg.Done()
			defer s.release(conn)
			defer conn.Close()
			s.handleConn(conn)
		}(conn)
	}
}

func (s *Server) admit(conn net.Conn) bool {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.stopping {
		return false
	}
	s.connections[conn] = struct{}{}
	s.wg.Add(1)
	return true
}

func (s *Server) release(conn net.Conn) {
	s.lifecycleMu.Lock()
	delete(s.connections, conn)
	s.lifecycleMu.Unlock()
}

func (s *Server) handleConn(conn net.Conn) {
	deadline := time.Now().Add(60 * time.Second)
	conn.SetDeadline(deadline)

	var req Request
	if err := ReadMessage(conn, &req); err != nil {
		return
	}
	defer clear(req.Args)

	switch req.Command {
	case CmdSSHRoutePrepare:
		deadline = time.Now().Add(SSHRoutePrepareCallTimeout + 5*time.Second)
		conn.SetDeadline(deadline)
	case CmdSensitiveAuth, CmdSensitivePassword:
		deadline = time.Now().Add(5 * time.Minute)
		conn.SetDeadline(deadline)
	}

	s.logger.Debug("ipc request", "command", req.Command)

	ctx, cancel, watchDone := requestContext(conn, deadline)
	defer func() {
		cancel()
		_ = conn.Close()
		<-watchDone
	}()

	resp := s.dispatch(ctx, req)
	defer clear(resp.Data)
	writeErr := WriteMessage(conn, resp)
	if err := resp.finalizeDelivery(writeErr == nil && ctx.Err() == nil); err != nil && writeErr == nil {
		s.logger.Debug("sensitive authorization not delivered", "error", err)
	}
}

func requestContext(conn net.Conn, deadline time.Time) (context.Context, context.CancelFunc, <-chan struct{}) {
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	done := make(chan struct{})
	go func() {
		defer close(done)
		var extra [1]byte
		_, err := conn.Read(extra[:])
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return
		}
		cancel()
	}()
	return ctx, cancel, done
}

func (s *Server) dispatch(ctx context.Context, req Request) Response {
	switch req.Command {
	case CmdList:
		return s.handleList(ctx)
	case CmdAdd:
		return s.handleAdd(req.Args)
	case CmdGenerate:
		return s.handleGenerate(req.Args)
	case CmdRemove:
		return s.handleRemove(req.Args)
	case CmdRename:
		return s.handleRename(req.Args)
	case CmdExport:
		return s.handleExport(ctx, req.Args)
	case CmdView:
		return s.handleView(ctx, req.Args)
	case CmdExportAll:
		return s.handleExportAll(req.Args)
	case CmdActivity:
		return s.handleActivity(req.Args)
	case CmdSyncTrigger:
		return s.handleSyncTrigger(ctx, req.Args)
	case CmdSyncLink:
		return s.handleSyncLink(req.Args)
	case CmdSyncUnlink:
		return s.handleSyncUnlink()
	case CmdAccountReplace:
		return s.handleAccountReplace(req.Args)
	case CmdAccountClear:
		return s.handleAccountClear()
	case CmdSSHRoutePrepare:
		return s.handleSSHRoutePrepare(ctx, req.Args)
	case CmdSSHRouteSuccess:
		return s.handleSSHRouteSuccess(req.Args)
	case CmdSSHRoutesList:
		return s.handleSSHRoutesList(ctx)
	case CmdSSHRouteClear:
		return s.handleSSHRouteClear(req.Args)
	case CmdSSHRoutesClearAll:
		return s.handleSSHRoutesClearAll()
	case CmdSensitiveAuth:
		return s.handleSensitiveAuth(ctx, req.Args)
	case CmdSensitivePassword:
		return s.handleSensitivePassword(ctx, req.Args)
	case CmdSensitiveLock:
		return s.handleSensitiveLock()
	case "status":
		return s.handleStatus()
	default:
		return ErrorResponse(fmt.Errorf("Unknown command: %s", req.Command))
	}
}

func (s *Server) handleSSHRoutePrepare(deliveryCtx context.Context, raw json.RawMessage) Response {
	if s.sshRoutes == nil {
		return ErrorResponse(fmt.Errorf("SSH routing unavailable"))
	}
	workCtx, cancel := context.WithTimeout(deliveryCtx, SSHRoutePrepareWorkTimeout)
	defer cancel()

	var args SSHRoutePrepareArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return ErrorResponse(fmt.Errorf("Invalid args: %w", err))
	}

	req := sshrouting.PrepareRequest{
		Attempt:      args.Attempt,
		ClientPID:    args.ClientPID,
		CWD:          args.CWD,
		Host:         args.Host,
		OriginalHost: args.OriginalHost,
		User:         args.User,
		Port:         args.Port,
	}
	if err := s.sshRoutes.PrepareContext(workCtx, req); err != nil {
		if errors.Is(err, sshrouting.ErrRouteMemoryLocked) {
			finalize, authErr := s.ensureExternalSession(deliveryCtx)
			if authErr != nil {
				return ErrorResponse(authErr)
			}
			err = s.sshRoutes.PrepareContext(workCtx, req)
			if err == nil {
				resp := OkResponse(nil)
				resp.finalize = finalize
				return resp
			}
			if finalize != nil {
				_ = finalize(false)
			}
		}
		return ErrorResponse(err)
	}

	return OkResponse(nil)
}

func (s *Server) ensureExternalSession(ctx context.Context) (func(bool) error, error) {
	s.stateMu.RLock()
	locked := s.keyStore == nil
	s.stateMu.RUnlock()
	if !locked || s.authBroker == nil {
		return nil, nil
	}
	result, finalize, err := s.authBroker.BeginAuthorize(ctx, sensitiveauth.ActionExternal, false)
	if err != nil {
		return nil, err
	}
	if result.PasswordRequired {
		if finalize != nil {
			_ = finalize(false)
		}
		return nil, fmt.Errorf("System Auth is required for external use")
	}
	return finalize, nil
}

func (s *Server) handleSSHRouteSuccess(raw json.RawMessage) Response {
	if s.sshRoutes == nil {
		return ErrorResponse(fmt.Errorf("SSH routing unavailable"))
	}

	var args SSHRouteSuccessArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return ErrorResponse(fmt.Errorf("Invalid args: %w", err))
	}

	if err := s.sshRoutes.Success(args.Attempt, args.ClientPID); err != nil {
		return ErrorResponse(err)
	}

	return OkResponse(nil)
}

func (s *Server) handleSSHRoutesList(ctx context.Context) Response {
	if s.sshRoutes == nil {
		return ErrorResponse(fmt.Errorf("SSH routing unavailable"))
	}
	if _, err := s.requireKeyStore(); err != nil {
		return ErrorResponse(err)
	}

	s.refreshForRead(ctx, "ssh_routes_list")
	if _, err := s.requireKeyStore(); err != nil {
		return ErrorResponse(err)
	}
	snapshot, err := s.sshRoutes.DebugSnapshot()
	if err != nil {
		return ErrorResponse(err)
	}
	return OkResponse(snapshot)
}

func (s *Server) handleSSHRouteClear(raw json.RawMessage) Response {
	if s.sshRoutes == nil {
		return ErrorResponse(fmt.Errorf("SSH routing unavailable"))
	}

	var args SSHRouteClearArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return ErrorResponse(fmt.Errorf("Invalid args: %w", err))
	}
	if _, err := s.requireKeyStore(); err != nil {
		return ErrorResponse(err)
	}
	if err := s.sshRoutes.Clear(args.Target); err != nil {
		return ErrorResponse(err)
	}
	return OkResponse(nil)
}

func (s *Server) handleSSHRoutesClearAll() Response {
	if s.sshRoutes == nil {
		return ErrorResponse(fmt.Errorf("SSH routing unavailable"))
	}
	if _, err := s.requireKeyStore(); err != nil {
		return ErrorResponse(err)
	}

	if err := s.sshRoutes.ClearAll(); err != nil {
		return ErrorResponse(err)
	}
	return OkResponse(nil)
}

func (s *Server) handleList(ctx context.Context) Response {
	if _, err := s.requireKeyStore(); err != nil {
		return ErrorResponse(err)
	}
	s.refreshForRead(ctx, "list_keys")

	keyStore, err := s.requireKeyStore()
	if err != nil {
		return ErrorResponse(err)
	}
	keys := keyStore.List()
	type keyInfo struct {
		Name        string `json:"name"`
		Type        string `json:"type"`
		Fingerprint string `json:"fingerprint"`
		Comment     string `json:"comment,omitempty"`
	}
	out := make([]keyInfo, len(keys))
	for i, k := range keys {
		out[i] = keyInfo{Name: k.Name, Type: k.Type, Fingerprint: k.Fingerprint, Comment: k.Comment}
	}
	return OkResponse(map[string]any{"keys": out})
}

type addArgs struct {
	Name       string `json:"name"`
	PrivateKey string `json:"private_key"`
	Comment    string `json:"comment"`
}

func (s *Server) handleAdd(raw json.RawMessage) Response {
	var a addArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return ErrorResponse(fmt.Errorf("Invalid args: %w", err))
	}
	keyStore, err := s.requireKeyStore()
	if err != nil {
		return ErrorResponse(err)
	}
	key, err := keyStore.Add(a.Name, []byte(a.PrivateKey), a.Comment)
	if err != nil {
		return ErrorResponse(err)
	}
	s.afterKeyMutation("key_added")
	return OkResponse(map[string]string{
		"name":        key.Name,
		"type":        key.Type,
		"fingerprint": key.Fingerprint,
		"public_key":  key.PublicKey,
	})
}

type generateArgs struct {
	Name    string `json:"name"`
	Comment string `json:"comment"`
}

func (s *Server) handleGenerate(raw json.RawMessage) Response {
	var a generateArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return ErrorResponse(fmt.Errorf("Invalid args: %w", err))
	}
	keyStore, err := s.requireKeyStore()
	if err != nil {
		return ErrorResponse(err)
	}
	key, err := keyStore.Generate(a.Name, a.Comment)
	if err != nil {
		return ErrorResponse(err)
	}
	s.afterKeyMutation("key_generated")
	return OkResponse(map[string]string{
		"name":        key.Name,
		"type":        key.Type,
		"fingerprint": key.Fingerprint,
		"public_key":  key.PublicKey,
	})
}

type removeArgs struct {
	Name        string `json:"name"`
	Fingerprint string `json:"fingerprint"`
}

func (s *Server) handleRemove(raw json.RawMessage) Response {
	var a removeArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return ErrorResponse(fmt.Errorf("Invalid args: %w", err))
	}

	keyStore, err := s.requireKeyStore()
	if err != nil {
		return ErrorResponse(err)
	}
	if err := keyStore.Remove(a.Name, a.Fingerprint); err != nil {
		return ErrorResponse(err)
	}
	s.afterKeyMutation("key_removed")
	return OkResponse(map[string]string{"resolved_name": a.Name})
}

type renameArgs struct {
	OldName string `json:"old_name"`
	NewName string `json:"new_name"`
}

func (s *Server) handleRename(raw json.RawMessage) Response {
	var a renameArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return ErrorResponse(fmt.Errorf("Invalid args: %w", err))
	}

	resolvedOldName, err := s.resolveKeyName(a.OldName)
	if err != nil {
		return ErrorResponse(err)
	}
	keyStore, err := s.requireKeyStore()
	if err != nil {
		return ErrorResponse(err)
	}
	if err := keyStore.Rename(resolvedOldName, a.NewName); err != nil {
		return ErrorResponse(err)
	}
	s.afterKeyMutation("key_renamed")
	return OkResponse(map[string]string{
		"old_name": resolvedOldName,
		"new_name": a.NewName,
	})
}

type exportArgs struct {
	Name string `json:"name"`
}

func (s *Server) handleExport(ctx context.Context, raw json.RawMessage) Response {
	var a exportArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return ErrorResponse(fmt.Errorf("Invalid args: %w", err))
	}

	if _, err := s.requireKeyStore(); err != nil {
		return ErrorResponse(err)
	}
	s.refreshForRead(ctx, "export_key")

	resolvedName, err := s.resolveKeyName(a.Name)
	if err != nil {
		return ErrorResponse(err)
	}
	keyStore, err := s.requireKeyStore()
	if err != nil {
		return ErrorResponse(err)
	}
	pub, err := keyStore.Export(resolvedName)
	if err != nil {
		return ErrorResponse(err)
	}
	return OkResponse(map[string]string{
		"public_key":    pub,
		"resolved_name": resolvedName,
	})
}

type viewArgs struct {
	Name            string `json:"name"`
	Full            bool   `json:"full"`
	PrivateKeyToken string `json:"private_key_token"`
}

func (s *Server) handleView(ctx context.Context, raw json.RawMessage) Response {
	var a viewArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return ErrorResponse(fmt.Errorf("Invalid args: %w", err))
	}

	if _, err := s.requireKeyStore(); err != nil {
		return ErrorResponse(err)
	}
	s.refreshForRead(ctx, "view_key")

	resolvedName, err := s.resolveKeyName(a.Name)
	if err != nil {
		return ErrorResponse(err)
	}

	keyStore, err := s.requireKeyStore()
	if err != nil {
		return ErrorResponse(err)
	}
	key, ok := keyStore.Get(resolvedName)
	if !ok {
		return ErrorResponse(fmt.Errorf("Key %q not found", resolvedName))
	}

	if a.Full {
		if s.authBroker == nil || a.PrivateKeyToken == "" || !s.authBroker.ConsumePrivateKeyToken(a.PrivateKeyToken) {
			return ErrorResponse(fmt.Errorf("Private-key access requires fresh password authentication"))
		}
	}

	out := map[string]any{
		"resolved_name": resolvedName,
		"name":          key.Name,
		"type":          key.Type,
		"fingerprint":   key.Fingerprint,
		"public_key":    key.PublicKey,
		"comment":       key.Comment,
		"created_at":    key.CreatedAt.Format(time.RFC3339),
		"updated_at":    key.UpdatedAt.Format(time.RFC3339),
		"version":       key.Version,
		"device_origin": key.DeviceOrigin,
		"git_signing":   key.GitSigning,
	}
	if key.LastUsedAt != nil {
		out["last_used_at"] = key.LastUsedAt.Format(time.RFC3339)
	}
	if a.Full {
		privateKey, err := keyStore.PrivateKeyBytes(resolvedName)
		if err != nil {
			return ErrorResponse(err)
		}
		out["private_key"] = string(privateKey)
		for i := range privateKey {
			privateKey[i] = 0
		}
	}

	return OkResponse(out)
}

type exportAllArgs struct {
	Token string `json:"token"`
}

func (s *Server) handleExportAll(raw json.RawMessage) Response {
	if s.authBroker == nil {
		return ErrorResponse(fmt.Errorf("Sensitive auth broker unavailable"))
	}

	var a exportAllArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return ErrorResponse(fmt.Errorf("Invalid args: %w", err))
	}
	if a.Token == "" || !s.authBroker.ConsumeExportToken(a.Token) {
		return ErrorResponse(fmt.Errorf("Sensitive export requires fresh authentication"))
	}

	_, keyStore, err := s.requireVaultAndKeyStore()
	if err != nil {
		return ErrorResponse(err)
	}

	type exportedKey struct {
		Name        string `json:"name"`
		Type        string `json:"type"`
		PrivateKey  string `json:"private_key"`
		PublicKey   string `json:"public_key"`
		Fingerprint string `json:"fingerprint"`
		Comment     string `json:"comment"`
		GitSigning  bool   `json:"git_signing"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
	}

	keys := keyStore.List()
	exported := make([]exportedKey, 0, len(keys))
	for _, k := range keys {
		privateKey, err := keyStore.PrivateKeyBytes(k.Name)
		if err != nil {
			return ErrorResponse(fmt.Errorf("Decrypting key %s: %w", k.Name, err))
		}
		privPEM := string(privateKey)
		for i := range privateKey {
			privateKey[i] = 0
		}
		exported = append(exported, exportedKey{
			Name:        k.Name,
			Type:        k.Type,
			PrivateKey:  privPEM,
			PublicKey:   k.PublicKey,
			Fingerprint: k.Fingerprint,
			Comment:     k.Comment,
			GitSigning:  k.GitSigning,
			CreatedAt:   k.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:   k.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	return OkResponse(exported)
}

type activityArgs struct {
	Limit int `json:"limit"`
}

func (s *Server) handleActivity(raw json.RawMessage) Response {
	var a activityArgs
	if raw != nil {
		json.Unmarshal(raw, &a)
	}
	if a.Limit <= 0 {
		a.Limit = 50
	}
	events := s.activityLog.Recent(a.Limit)
	return OkResponse(map[string]any{"events": events})
}

type syncTriggerArgs struct {
	ServerURL string `json:"server_url"`
	Token     string `json:"token"`
}

func (s *Server) handleSyncTrigger(ctx context.Context, raw json.RawMessage) Response {
	var a syncTriggerArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return ErrorResponse(fmt.Errorf("Invalid args: %w", err))
	}

	if a.ServerURL == "" || a.Token == "" {
		return ErrorResponse(fmt.Errorf("Server URL and token required"))
	}

	_, _, err := s.requireVaultAndKeyStore()
	if err != nil {
		return ErrorResponse(err)
	}

	bus := s.currentSyncBus()
	if bus == nil {
		if syncErr := s.currentSyncError(); syncErr != "" {
			return ErrorResponse(errors.New(syncErr))
		}
		return ErrorResponse(fmt.Errorf("Sync is unavailable; restart Forged and try again"))
	}

	if err := bus.ForceSync(ctx, "manual_sync"); err != nil {
		return ErrorResponse(fmt.Errorf("Sync failed: %w", err))
	}
	state := bus.SnapshotState()
	return OkResponse(map[string]any{"version": state.LastKnownServerVersion})
}

func (s *Server) handleSyncLink(raw json.RawMessage) Response {
	var a SyncLinkArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return ErrorResponse(fmt.Errorf("Invalid args: %w", err))
	}
	if a.ServerURL == "" || a.Token == "" || a.UserID == "" {
		return ErrorResponse(fmt.Errorf("Server URL, token, and user ID required"))
	}
	if s.syncLink == nil {
		return ErrorResponse(fmt.Errorf("Sync link handler unavailable"))
	}
	if err := s.syncLink(a); err != nil {
		return ErrorResponse(err)
	}
	return OkResponse(nil)
}

func (s *Server) handleSyncUnlink() Response {
	if s.syncUnlink == nil {
		return ErrorResponse(fmt.Errorf("Sync unlink handler unavailable"))
	}
	if err := s.syncUnlink(); err != nil {
		return ErrorResponse(err)
	}
	return OkResponse(nil)
}

func (s *Server) handleAccountReplace(raw json.RawMessage) Response {
	var args AccountCredentialsArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return ErrorResponse(fmt.Errorf("Invalid args: %w", err))
	}
	if s.accountReplace == nil {
		return ErrorResponse(fmt.Errorf("Account replace handler unavailable"))
	}
	if err := s.accountReplace(args); err != nil {
		return ErrorResponse(err)
	}
	return OkResponse(nil)
}

func (s *Server) handleAccountClear() Response {
	if s.accountClear == nil {
		return ErrorResponse(fmt.Errorf("Account clear handler unavailable"))
	}
	if err := s.accountClear(); err != nil {
		return ErrorResponse(err)
	}
	return OkResponse(nil)
}

type sensitiveAuthArgs struct {
	Action string `json:"action"`
	Force  bool   `json:"force"`
}

type sensitivePasswordArgs struct {
	Action   string `json:"action"`
	Password string `json:"password"`
}

func (s *Server) handleSensitiveAuth(ctx context.Context, raw json.RawMessage) Response {
	if s.authBroker == nil {
		return ErrorResponse(fmt.Errorf("Sensitive auth broker unavailable"))
	}

	var a sensitiveAuthArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return ErrorResponse(fmt.Errorf("Invalid args: %w", err))
	}

	action, err := sensitiveauth.ParseAction(a.Action)
	if err != nil {
		return ErrorResponse(err)
	}

	result, finalize, err := s.authBroker.BeginAuthorize(ctx, action, a.Force)
	if err != nil {
		return ErrorResponse(err)
	}
	resp := OkResponse(result)
	resp.finalize = finalize
	return resp
}

func (s *Server) handleSensitivePassword(ctx context.Context, raw json.RawMessage) Response {
	if s.authBroker == nil {
		return ErrorResponse(fmt.Errorf("Sensitive auth broker unavailable"))
	}

	var a sensitivePasswordArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return ErrorResponse(fmt.Errorf("Invalid args: %w", err))
	}
	clear(raw)
	password := []byte(a.Password)
	a.Password = ""
	defer clear(password)

	action, err := sensitiveauth.ParseAction(a.Action)
	if err != nil {
		return ErrorResponse(err)
	}

	result, finalize, err := s.authBroker.BeginAuthorizeWithPassword(ctx, action, password)
	if err != nil {
		return ErrorResponse(err)
	}
	resp := OkResponse(result)
	resp.finalize = finalize
	return resp
}

func (s *Server) handleSensitiveLock() Response {
	if s.authBroker != nil {
		s.authBroker.Lock("manual_lock")
	}
	return OkResponse(nil)
}

func (s *Server) handleStatus() Response {
	buildID := buildinfo.CurrentID()
	status := map[string]any{
		"pid":                     os.Getpid(),
		"key_count":               s.keyCount(),
		"build_id":                buildID,
		"account_change_protocol": AccountChangeProtocol,
		"build": map[string]any{
			"id": buildID,
		},
	}

	if s.authBroker != nil {
		status["sensitive"] = map[string]any{
			"unlocked": s.authBroker.IsUnlocked(),
			"active":   s.hasActiveVaultSession(),
		}
	}

	if bus := s.currentSyncBus(); bus != nil {
		syncState := bus.SnapshotState()
		lastErr := syncState.LastError
		if recoveryErr := s.currentSyncError(); recoveryErr != "" {
			lastErr = recoveryErr
		}
		status["sync"] = map[string]any{
			"device_id":                 syncState.DeviceID,
			"dirty":                     syncState.Dirty,
			"last_error":                lastErr,
			"last_known_server_version": syncState.LastKnownServerVersion,
			"last_remote_check_at":      syncState.LastRemoteCheckAt,
			"last_successful_pull_at":   syncState.LastSuccessfulPullAt,
			"last_successful_push_at":   syncState.LastSuccessfulPushAt,
			"linked":                    syncState.LinkedUserID != "" && syncState.ServerURL != "",
			"linked_user_id":            syncState.LinkedUserID,
			"server_url":                syncState.ServerURL,
			"syncing":                   syncState.Syncing,
		}
	} else if syncErr := s.currentSyncError(); syncErr != "" {
		status["sync"] = map[string]any{"last_error": syncErr}
	}

	return OkResponse(status)
}

func (s *Server) refreshForRead(parent context.Context, reason string) {
	bus := s.currentSyncBus()
	if bus == nil {
		return
	}

	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()

	if err := bus.ForegroundRead(ctx, reason); err != nil {
		s.logger.Debug("foreground sync refresh failed", "reason", reason, "error", err)
	}
	if parent.Err() != nil {
		return
	}
	if s.onReadSync != nil {
		s.onReadSync()
	}
}

func (s *Server) afterKeyMutation(reason string) {
	s.afterVaultMutation(reason)
	if s.onKeyChange != nil {
		s.onKeyChange()
	}
}

func (s *Server) afterVaultMutation(reason string) {
	if s.onVaultChange != nil {
		s.onVaultChange(reason)
		return
	}
	if bus := s.currentSyncBus(); bus != nil {
		bus.LocalMutation(reason)
	}
}

func (s *Server) currentVaultState() (*vault.Vault, *vault.KeyStore) {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.vault, s.keyStore
}

func (s *Server) currentSyncBus() *forgedsync.Bus {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.syncBus
}

func (s *Server) currentSyncError() string {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.syncError
}

func (s *Server) requireKeyStore() (*vault.KeyStore, error) {
	if s.authBroker != nil && !s.authBroker.IsUnlocked() {
		return nil, fmt.Errorf("Vault is locked; open Forged to unlock")
	}
	_, keyStore := s.currentVaultState()
	if keyStore == nil {
		return nil, fmt.Errorf("Vault is locked; open Forged to unlock")
	}
	return keyStore, nil
}

func (s *Server) requireVaultAndKeyStore() (*vault.Vault, *vault.KeyStore, error) {
	if s.authBroker != nil && !s.authBroker.IsUnlocked() {
		return nil, nil, fmt.Errorf("Vault is locked; open Forged to unlock")
	}
	v, keyStore := s.currentVaultState()
	if v == nil || keyStore == nil {
		return nil, nil, fmt.Errorf("Vault is locked; open Forged to unlock")
	}
	return v, keyStore, nil
}

func (s *Server) keyCount() int {
	_, keyStore := s.currentVaultState()
	if keyStore == nil {
		return 0
	}
	return len(keyStore.List())
}

func (s *Server) hasActiveVaultSession() bool {
	v, keyStore := s.currentVaultState()
	return v != nil && keyStore != nil
}

func (s *Server) resolveKeyName(input string) (string, error) {
	keyStore, err := s.requireKeyStore()
	if err != nil {
		return "", err
	}
	return keyStore.ResolveName(input)
}
