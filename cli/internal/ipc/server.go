package ipc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"

	"github.com/itzzritik/forged/cli/internal/activity"
	"github.com/itzzritik/forged/cli/internal/sensitiveauth"
	"github.com/itzzritik/forged/cli/internal/sshrouting"
	forgedsync "github.com/itzzritik/forged/cli/internal/sync"
	"github.com/itzzritik/forged/cli/internal/vault"
)

type SSHRouteHandler interface {
	Prepare(sshrouting.PrepareRequest) error
	Success(attempt string) error
}

type Server struct {
	socketPath  string
	vault       *vault.Vault
	keyStore    *vault.KeyStore
	activityLog *activity.ActivityLog
	listener    net.Listener
	logger      *slog.Logger
	wg          sync.WaitGroup
	syncBus     *forgedsync.Bus
	syncLink    func(SyncLinkArgs) error
	authBroker  *sensitiveauth.Broker
	onKeyChange func()
	onReadSync  func()
	sshRoutes   SSHRouteHandler
}

func (s *Server) SetSyncBus(bus *forgedsync.Bus) {
	s.syncBus = bus
}

func (s *Server) SetSyncLinkHandler(handler func(SyncLinkArgs) error) {
	s.syncLink = handler
}

func (s *Server) SetSensitiveAuthBroker(broker *sensitiveauth.Broker) {
	s.authBroker = broker
}

func (s *Server) SetOnKeyChange(fn func()) {
	s.onKeyChange = fn
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
	}
}

func (s *Server) Start() error {
	os.Remove(s.socketPath)

	ln, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", s.socketPath, err)
	}

	if err := os.Chmod(s.socketPath, 0600); err != nil {
		ln.Close()
		return fmt.Errorf("setting socket permissions: %w", err)
	}

	s.listener = ln

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.acceptLoop()
	}()

	return nil
}

func (s *Server) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}
	s.wg.Wait()
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer conn.Close()
			s.handleConn(conn)
		}()
	}
}

func (s *Server) handleConn(conn net.Conn) {
	conn.SetDeadline(time.Now().Add(60 * time.Second))

	var req Request
	if err := ReadMessage(conn, &req); err != nil {
		return
	}

	switch req.Command {
	case CmdSensitiveAuth, CmdSensitivePassword:
		conn.SetDeadline(time.Now().Add(5 * time.Minute))
	}

	s.logger.Debug("ipc request", "command", req.Command)

	resp := s.dispatch(req)
	WriteMessage(conn, resp)
}

func (s *Server) dispatch(req Request) Response {
	switch req.Command {
	case CmdList:
		return s.handleList()
	case CmdAdd:
		return s.handleAdd(req.Args)
	case CmdGenerate:
		return s.handleGenerate(req.Args)
	case CmdRemove:
		return s.handleRemove(req.Args)
	case CmdRename:
		return s.handleRename(req.Args)
	case CmdExport:
		return s.handleExport(req.Args)
	case CmdView:
		return s.handleView(req.Args)
	case CmdExportAll:
		return s.handleExportAll(req.Args)
	case CmdActivity:
		return s.handleActivity(req.Args)
	case CmdSyncTrigger:
		return s.handleSyncTrigger(req.Args)
	case CmdSyncLink:
		return s.handleSyncLink(req.Args)
	case CmdSSHRoutePrepare:
		return s.handleSSHRoutePrepare(req.Args)
	case CmdSSHRouteSuccess:
		return s.handleSSHRouteSuccess(req.Args)
	case CmdSensitiveAuth:
		return s.handleSensitiveAuth(req.Args)
	case CmdSensitivePassword:
		return s.handleSensitivePassword(req.Args)
	case CmdSensitiveLock:
		return s.handleSensitiveLock()
	case "status":
		return s.handleStatus()
	default:
		return ErrorResponse(fmt.Errorf("unknown command: %s", req.Command))
	}
}

func (s *Server) handleSSHRoutePrepare(raw json.RawMessage) Response {
	if s.sshRoutes == nil {
		return ErrorResponse(fmt.Errorf("ssh routing unavailable"))
	}

	var args SSHRoutePrepareArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return ErrorResponse(fmt.Errorf("invalid args: %w", err))
	}

	if err := s.sshRoutes.Prepare(sshrouting.PrepareRequest{
		Attempt:   args.Attempt,
		ClientPID: args.ClientPID,
		CWD:       args.CWD,
		Host:      args.Host,
		User:      args.User,
		Port:      args.Port,
	}); err != nil {
		return ErrorResponse(err)
	}

	return OkResponse(nil)
}

func (s *Server) handleSSHRouteSuccess(raw json.RawMessage) Response {
	if s.sshRoutes == nil {
		return ErrorResponse(fmt.Errorf("ssh routing unavailable"))
	}

	var args SSHRouteSuccessArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return ErrorResponse(fmt.Errorf("invalid args: %w", err))
	}

	if err := s.sshRoutes.Success(args.Attempt); err != nil {
		return ErrorResponse(err)
	}

	return OkResponse(nil)
}

func (s *Server) handleList() Response {
	s.refreshForRead("list")

	keys := s.keyStore.List()
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
		return ErrorResponse(fmt.Errorf("invalid args: %w", err))
	}
	key, err := s.keyStore.Add(a.Name, []byte(a.PrivateKey), a.Comment)
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
		return ErrorResponse(fmt.Errorf("invalid args: %w", err))
	}
	key, err := s.keyStore.Generate(a.Name, a.Comment)
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
	Name string `json:"name"`
}

func (s *Server) handleRemove(raw json.RawMessage) Response {
	var a removeArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return ErrorResponse(fmt.Errorf("invalid args: %w", err))
	}

	resolvedName, err := s.resolveKeyName(a.Name)
	if err != nil {
		return ErrorResponse(err)
	}
	if err := s.keyStore.Remove(resolvedName); err != nil {
		return ErrorResponse(err)
	}
	s.afterKeyMutation("key_removed")
	return OkResponse(map[string]string{"resolved_name": resolvedName})
}

type renameArgs struct {
	OldName string `json:"old_name"`
	NewName string `json:"new_name"`
}

func (s *Server) handleRename(raw json.RawMessage) Response {
	var a renameArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return ErrorResponse(fmt.Errorf("invalid args: %w", err))
	}

	resolvedOldName, err := s.resolveKeyName(a.OldName)
	if err != nil {
		return ErrorResponse(err)
	}
	if err := s.keyStore.Rename(resolvedOldName, a.NewName); err != nil {
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

func (s *Server) handleExport(raw json.RawMessage) Response {
	s.refreshForRead("export")

	var a exportArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return ErrorResponse(fmt.Errorf("invalid args: %w", err))
	}

	resolvedName, err := s.resolveKeyName(a.Name)
	if err != nil {
		return ErrorResponse(err)
	}
	pub, err := s.keyStore.Export(resolvedName)
	if err != nil {
		return ErrorResponse(err)
	}
	return OkResponse(map[string]string{
		"public_key":    pub,
		"resolved_name": resolvedName,
	})
}

type viewArgs struct {
	Name string `json:"name"`
	Full bool   `json:"full"`
}

func (s *Server) handleView(raw json.RawMessage) Response {
	s.refreshForRead("view")

	var a viewArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return ErrorResponse(fmt.Errorf("invalid args: %w", err))
	}

	resolvedName, err := s.resolveKeyName(a.Name)
	if err != nil {
		return ErrorResponse(err)
	}

	key, ok := s.keyStore.Get(resolvedName)
	if !ok {
		return ErrorResponse(fmt.Errorf("key %q not found", resolvedName))
	}

	if a.Full {
		if s.authBroker == nil || !s.authBroker.CanViewFull() {
			return ErrorResponse(fmt.Errorf("sensitive private-key access requires authentication"))
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
		out["private_key"] = string(key.PrivateKey)
	}

	return OkResponse(out)
}

type exportAllArgs struct {
	Token string `json:"token"`
}

func (s *Server) handleExportAll(raw json.RawMessage) Response {
	s.refreshForRead("export_all")

	if s.authBroker == nil {
		return ErrorResponse(fmt.Errorf("sensitive auth broker unavailable"))
	}

	var a exportAllArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return ErrorResponse(fmt.Errorf("invalid args: %w", err))
	}
	if a.Token == "" || !s.authBroker.ConsumeExportToken(a.Token) {
		return ErrorResponse(fmt.Errorf("sensitive export requires fresh authentication"))
	}

	if err := s.vault.DecryptAllPrivateKeys(); err != nil {
		return ErrorResponse(fmt.Errorf("decrypting keys: %w", err))
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

	keys := s.keyStore.List()
	exported := make([]exportedKey, 0, len(keys))
	for _, k := range keys {
		privPEM := string(k.PrivateKey)
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

func (s *Server) handleSyncTrigger(raw json.RawMessage) Response {
	var a syncTriggerArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return ErrorResponse(fmt.Errorf("invalid args: %w", err))
	}

	if a.ServerURL == "" || a.Token == "" {
		return ErrorResponse(fmt.Errorf("server_url and token required"))
	}

	if s.syncBus != nil {
		if err := s.syncBus.ForceSync(context.Background(), "manual_sync"); err != nil {
			return ErrorResponse(fmt.Errorf("sync failed: %w", err))
		}
		state := s.syncBus.SnapshotState()
		return OkResponse(map[string]any{"version": state.LastKnownServerVersion})
	}

	blob, err := s.vault.ExportForSync()
	if err != nil {
		return ErrorResponse(fmt.Errorf("exporting vault: %w", err))
	}

	client := forgedsync.NewClient(a.ServerURL, a.Token, "")

	status, err := client.Status()
	if err != nil {
		return ErrorResponse(fmt.Errorf("checking sync status: %w", err))
	}

	var expectedVersion int64
	if status.HasVault {
		expectedVersion = status.Version
	}

	protectedKey := base64.StdEncoding.EncodeToString(s.vault.ProtectedKeyBytes())
	result, err := client.Push(blob, s.vault.KDFParams(), protectedKey, expectedVersion)
	if err != nil {
		return ErrorResponse(fmt.Errorf("sync push: %w", err))
	}

	return OkResponse(map[string]any{"version": result.Version})
}

func (s *Server) handleSyncLink(raw json.RawMessage) Response {
	var a SyncLinkArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return ErrorResponse(fmt.Errorf("invalid args: %w", err))
	}
	if a.ServerURL == "" || a.Token == "" || a.UserID == "" {
		return ErrorResponse(fmt.Errorf("server_url, token, and user_id required"))
	}
	if s.syncLink == nil {
		return ErrorResponse(fmt.Errorf("sync link handler unavailable"))
	}
	if err := s.syncLink(a); err != nil {
		return ErrorResponse(err)
	}
	return OkResponse(nil)
}

type sensitiveAuthArgs struct {
	Action string `json:"action"`
}

type sensitivePasswordArgs struct {
	Action   string `json:"action"`
	Password string `json:"password"`
}

func (s *Server) handleSensitiveAuth(raw json.RawMessage) Response {
	if s.authBroker == nil {
		return ErrorResponse(fmt.Errorf("sensitive auth broker unavailable"))
	}

	var a sensitiveAuthArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return ErrorResponse(fmt.Errorf("invalid args: %w", err))
	}

	action, err := sensitiveauth.ParseAction(a.Action)
	if err != nil {
		return ErrorResponse(err)
	}

	result, err := s.authBroker.Authorize(context.Background(), action)
	if err != nil {
		return ErrorResponse(err)
	}
	return OkResponse(result)
}

func (s *Server) handleSensitivePassword(raw json.RawMessage) Response {
	if s.authBroker == nil {
		return ErrorResponse(fmt.Errorf("sensitive auth broker unavailable"))
	}

	var a sensitivePasswordArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return ErrorResponse(fmt.Errorf("invalid args: %w", err))
	}

	action, err := sensitiveauth.ParseAction(a.Action)
	if err != nil {
		return ErrorResponse(err)
	}

	password := []byte(a.Password)
	defer func() {
		for i := range password {
			password[i] = 0
		}
	}()

	result, err := s.authBroker.AuthorizeWithPassword(action, password)
	if err != nil {
		return ErrorResponse(err)
	}
	return OkResponse(result)
}

func (s *Server) handleSensitiveLock() Response {
	if s.authBroker != nil {
		s.authBroker.Invalidate("manual_lock")
	}
	return OkResponse(nil)
}

func (s *Server) handleStatus() Response {
	status := map[string]any{
		"pid":       os.Getpid(),
		"key_count": len(s.keyStore.List()),
	}

	if s.syncBus != nil {
		syncState := s.syncBus.SnapshotState()
		status["sync"] = map[string]any{
			"device_id":                 syncState.DeviceID,
			"dirty":                     syncState.Dirty,
			"last_error":                syncState.LastError,
			"last_known_server_version": syncState.LastKnownServerVersion,
			"last_successful_pull_at":   syncState.LastSuccessfulPullAt,
			"last_successful_push_at":   syncState.LastSuccessfulPushAt,
			"linked":                    syncState.LinkedUserID != "" && syncState.ServerURL != "",
			"linked_user_id":            syncState.LinkedUserID,
			"server_url":                syncState.ServerURL,
			"syncing":                   syncState.Syncing,
		}
	}

	return OkResponse(status)
}

func (s *Server) refreshForRead(reason string) {
	if s.syncBus == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.syncBus.ForegroundRead(ctx, reason); err != nil {
		s.logger.Debug("foreground sync refresh failed", "reason", reason, "error", err)
	}
	if s.onReadSync != nil {
		s.onReadSync()
	}
}

func (s *Server) afterKeyMutation(reason string) {
	if s.syncBus != nil {
		s.syncBus.LocalMutation(reason)
	}
	if s.onKeyChange != nil {
		s.onKeyChange()
	}
}

func (s *Server) resolveKeyName(input string) (string, error) {
	return s.keyStore.ResolveName(input)
}
