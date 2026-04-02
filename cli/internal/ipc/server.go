package ipc

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"

	"github.com/forgedkeys/forged/cli/internal/activity"
	"github.com/forgedkeys/forged/cli/internal/vault"
)

type Server struct {
	socketPath  string
	keyStore    *vault.KeyStore
	activityLog *activity.ActivityLog
	listener    net.Listener
	logger      *slog.Logger
	wg          sync.WaitGroup
}

func NewServer(socketPath string, ks *vault.KeyStore, al *activity.ActivityLog, logger *slog.Logger) *Server {
	return &Server{
		socketPath:  socketPath,
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
	var req Request
	if err := ReadMessage(conn, &req); err != nil {
		return
	}

	s.logger.Debug("ipc request", "command", req.Command)

	resp := s.dispatch(req)
	WriteMessage(conn, resp)
}

func (s *Server) dispatch(req Request) Response {
	switch req.Command {
	case "list":
		return s.handleList()
	case "add":
		return s.handleAdd(req.Args)
	case "generate":
		return s.handleGenerate(req.Args)
	case "remove":
		return s.handleRemove(req.Args)
	case "rename":
		return s.handleRename(req.Args)
	case "export":
		return s.handleExport(req.Args)
	case "host":
		return s.handleHost(req.Args)
	case "unhost":
		return s.handleUnhost(req.Args)
	case "hosts":
		return s.handleHosts()
	case "activity":
		return s.handleActivity(req.Args)
	case "status":
		return s.handleStatus()
	default:
		return ErrorResponse(fmt.Errorf("unknown command: %s", req.Command))
	}
}

func (s *Server) handleList() Response {
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
	if err := s.keyStore.Remove(a.Name); err != nil {
		return ErrorResponse(err)
	}
	return OkResponse(nil)
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
	if err := s.keyStore.Rename(a.OldName, a.NewName); err != nil {
		return ErrorResponse(err)
	}
	return OkResponse(nil)
}

type exportArgs struct {
	Name string `json:"name"`
}

func (s *Server) handleExport(raw json.RawMessage) Response {
	var a exportArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return ErrorResponse(fmt.Errorf("invalid args: %w", err))
	}
	pub, err := s.keyStore.Export(a.Name)
	if err != nil {
		return ErrorResponse(err)
	}
	return OkResponse(map[string]string{"public_key": pub})
}

type hostArgs struct {
	KeyName  string   `json:"key_name"`
	Patterns []string `json:"patterns"`
}

func (s *Server) handleHost(raw json.RawMessage) Response {
	var a hostArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return ErrorResponse(fmt.Errorf("invalid args: %w", err))
	}
	for _, p := range a.Patterns {
		if err := s.keyStore.AddHostRule(a.KeyName, p); err != nil {
			return ErrorResponse(err)
		}
	}
	return OkResponse(nil)
}

type unhostArgs struct {
	KeyName string `json:"key_name"`
	Pattern string `json:"pattern"`
}

func (s *Server) handleUnhost(raw json.RawMessage) Response {
	var a unhostArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return ErrorResponse(fmt.Errorf("invalid args: %w", err))
	}
	if err := s.keyStore.RemoveHostRule(a.KeyName, a.Pattern); err != nil {
		return ErrorResponse(err)
	}
	return OkResponse(nil)
}

func (s *Server) handleHosts() Response {
	keys := s.keyStore.List()
	type mapping struct {
		KeyName  string           `json:"key_name"`
		Rules    []vault.HostRule `json:"rules"`
	}
	var mappings []mapping
	for _, k := range keys {
		if len(k.HostRules) > 0 {
			mappings = append(mappings, mapping{KeyName: k.Name, Rules: k.HostRules})
		}
	}
	return OkResponse(map[string]any{"mappings": mappings})
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

func (s *Server) handleStatus() Response {
	keys := s.keyStore.List()
	return OkResponse(map[string]any{
		"pid":       os.Getpid(),
		"key_count": len(keys),
	})
}
