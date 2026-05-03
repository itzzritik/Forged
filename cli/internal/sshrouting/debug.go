package sshrouting

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/vault"
	"golang.org/x/crypto/ssh"
)

type DebugSnapshot struct {
	Routes          []DebugRoute          `json:"routes"`
	RuntimeAttempts []DebugRuntimeAttempt `json:"runtime_attempts"`
	PublicHints     []DebugPublicHint     `json:"public_hints"`
}

type DebugRoute struct {
	Target        string              `json:"target"`
	Kind          string              `json:"kind,omitempty"`
	Host          string              `json:"host,omitempty"`
	User          string              `json:"user,omitempty"`
	Port          int                 `json:"port,omitempty"`
	Owner         string              `json:"owner,omitempty"`
	Repo          string              `json:"repo,omitempty"`
	Fingerprint   string              `json:"fingerprint,omitempty"`
	KeyName       string              `json:"key_name,omitempty"`
	KeyRef        string              `json:"key_ref,omitempty"`
	ProvenBy      string              `json:"proven_by,omitempty"`
	Operation     string              `json:"operation,omitempty"`
	SuccessCount  int                 `json:"success_count,omitempty"`
	LastSuccessAt *time.Time          `json:"last_success_at,omitempty"`
	Updated       time.Time           `json:"updated"`
	Attempts      []DebugRouteAttempt `json:"attempts,omitempty"`
}

type DebugRouteAttempt struct {
	Fingerprint string    `json:"fingerprint,omitempty"`
	KeyName     string    `json:"key_name,omitempty"`
	KeyRef      string    `json:"key_ref,omitempty"`
	AttemptedAt time.Time `json:"attempted_at"`
}

type DebugRuntimeAttempt struct {
	Token         string               `json:"token"`
	Path          string               `json:"path"`
	Updated       time.Time            `json:"updated"`
	AgeSeconds    int64                `json:"age_seconds"`
	IdentityFiles []DebugIdentityFile  `json:"identity_files,omitempty"`
	Clients       []DebugRuntimeClient `json:"clients,omitempty"`
}

type DebugRuntimeClient struct {
	ClientPID  int      `json:"client_pid,omitempty"`
	Target     string   `json:"target,omitempty"`
	Operation  string   `json:"operation,omitempty"`
	Candidates []string `json:"candidates,omitempty"`
	HadExact   bool     `json:"had_exact,omitempty"`
}

type DebugIdentityFile struct {
	Path        string `json:"path"`
	Ref         string `json:"ref,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	KeyName     string `json:"key_name,omitempty"`
}

type DebugPublicHint struct {
	Ref         string    `json:"ref"`
	Path        string    `json:"path"`
	Fingerprint string    `json:"fingerprint,omitempty"`
	KeyName     string    `json:"key_name,omitempty"`
	Updated     time.Time `json:"updated"`
	Stale       bool      `json:"stale,omitempty"`
}

func (s *Service) DebugSnapshot() (DebugSnapshot, error) {
	s.mu.RLock()
	keyStore := s.keyStore
	attempts := make([]Attempt, 0, len(s.attempts))
	for _, attempt := range s.attempts {
		attempts = append(attempts, cloneAttempt(attempt))
	}
	s.mu.RUnlock()

	now := s.now()
	var keys []vault.Key
	var routes map[string]vault.SSHRoute
	if keyStore != nil {
		keys = keyStore.List()
		routes = keyStore.SSHRoutes()
	}

	refs, err := BuildKeyRefs(keys, s.paths.SSHManagedKeysDir())
	if err != nil {
		return DebugSnapshot{}, fmt.Errorf("building SSH key refs: %w", err)
	}
	refByFingerprint := KeyRefsByFingerprint(refs)
	refByPath := keyRefsByPath(refs)
	keyNameByFingerprint := keyNamesByFingerprint(keys)

	return DebugSnapshot{
		Routes:          debugRoutes(routes, refByFingerprint, keyNameByFingerprint),
		RuntimeAttempts: debugRuntimeAttempts(s.paths.SSHRouteRuntimeDir(), attempts, refByPath, now),
		PublicHints:     debugPublicHints(s.paths.SSHManagedKeysDir(), refByPath),
	}, nil
}

func (s *Service) Clear(target string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return fmt.Errorf("empty SSH route target")
	}

	s.mu.RLock()
	keyStore := s.keyStore
	s.mu.RUnlock()
	if keyStore == nil {
		return fmt.Errorf("vault is locked")
	}

	if err := keyStore.ClearSSHRoute(target, s.now()); err != nil {
		return err
	}
	s.notifyMutation("ssh_route_cleared")
	return nil
}

func (s *Service) ClearAll() error {
	s.mu.RLock()
	keyStore := s.keyStore
	s.mu.RUnlock()
	if keyStore == nil {
		return fmt.Errorf("vault is locked")
	}

	routes := keyStore.SSHRoutes()
	targets := make([]string, 0, len(routes))
	for target := range routes {
		targets = append(targets, target)
	}
	sort.Strings(targets)

	now := s.now()
	for _, target := range targets {
		if err := keyStore.ClearSSHRoute(target, now); err != nil {
			return err
		}
	}
	if len(targets) > 0 {
		s.notifyMutation("ssh_routes_cleared")
	}
	return nil
}

func debugRoutes(routes map[string]vault.SSHRoute, refs map[string]KeyRef, keyNames map[string]string) []DebugRoute {
	out := make([]DebugRoute, 0, len(routes))
	for target, route := range routes {
		ref := refs[route.Key]
		row := DebugRoute{
			Target:        target,
			Fingerprint:   route.Key,
			KeyName:       firstNonEmpty(ref.Name, keyNames[route.Key]),
			KeyRef:        ref.Ref,
			ProvenBy:      route.ProvenBy,
			Operation:     route.Operation,
			SuccessCount:  route.SuccessCount,
			LastSuccessAt: route.LastSuccessAt,
			Updated:       route.Updated,
			Attempts:      debugRouteAttempts(route.Attempts, refs, keyNames),
		}
		if parsed, err := ParseCanonicalTarget(target); err == nil {
			row.Kind = string(parsed.Kind)
			row.Host = parsed.Host
			row.User = parsed.User
			row.Port = parsed.Port
			row.Owner = parsed.Owner
			row.Repo = parsed.Repo
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Updated.Equal(out[j].Updated) {
			return out[i].Updated.After(out[j].Updated)
		}
		return out[i].Target < out[j].Target
	})
	return out
}

func debugRouteAttempts(attempts map[string]time.Time, refs map[string]KeyRef, keyNames map[string]string) []DebugRouteAttempt {
	if len(attempts) == 0 {
		return nil
	}
	out := make([]DebugRouteAttempt, 0, len(attempts))
	for fingerprint, attemptedAt := range attempts {
		ref := refs[fingerprint]
		out = append(out, DebugRouteAttempt{
			Fingerprint: fingerprint,
			KeyName:     firstNonEmpty(ref.Name, keyNames[fingerprint]),
			KeyRef:      ref.Ref,
			AttemptedAt: attemptedAt,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].AttemptedAt.Equal(out[j].AttemptedAt) {
			return out[i].AttemptedAt.After(out[j].AttemptedAt)
		}
		return out[i].Fingerprint < out[j].Fingerprint
	})
	return out
}

func debugRuntimeAttempts(dir string, attempts []Attempt, refs map[string]KeyRef, now time.Time) []DebugRuntimeAttempt {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	clientsByToken := map[string][]DebugRuntimeClient{}
	for _, attempt := range attempts {
		clientsByToken[attempt.Token] = append(clientsByToken[attempt.Token], DebugRuntimeClient{
			ClientPID:  attempt.ClientPID,
			Target:     attempt.Target.Canonical,
			Operation:  attempt.Operation.String(),
			Candidates: append([]string(nil), attempt.Candidates...),
			HadExact:   attempt.HadExact,
		})
	}

	out := make([]DebugRuntimeAttempt, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".conf") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		token := strings.TrimSuffix(entry.Name(), ".conf")
		out = append(out, DebugRuntimeAttempt{
			Token:         token,
			Path:          path,
			Updated:       info.ModTime(),
			AgeSeconds:    int64(now.Sub(info.ModTime()).Seconds()),
			IdentityFiles: debugIdentityFiles(path, refs),
			Clients:       clientsByToken[token],
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Updated.Equal(out[j].Updated) {
			return out[i].Updated.After(out[j].Updated)
		}
		return out[i].Token < out[j].Token
	})
	return out
}

func debugIdentityFiles(path string, refs map[string]KeyRef) []DebugIdentityFile {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []DebugIdentityFile
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "IdentityFile ") {
			continue
		}
		identityPath := strings.TrimSpace(strings.TrimPrefix(line, "IdentityFile "))
		if unquoted, err := strconv.Unquote(identityPath); err == nil {
			identityPath = unquoted
		} else {
			identityPath = strings.Trim(identityPath, `"'`)
		}
		ref := refs[identityPath]
		out = append(out, DebugIdentityFile{
			Path:        identityPath,
			Ref:         ref.Ref,
			Fingerprint: ref.Fingerprint,
			KeyName:     ref.Name,
		})
	}
	return out
}

func debugPublicHints(dir string, refs map[string]KeyRef) []DebugPublicHint {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := make([]DebugPublicHint, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "k_") || !strings.HasSuffix(entry.Name(), ".pub") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		ref := refs[path]
		fingerprint := ref.Fingerprint
		if fingerprint == "" {
			fingerprint = publicKeyFileFingerprint(path)
		}
		out = append(out, DebugPublicHint{
			Ref:         strings.TrimSuffix(entry.Name(), ".pub"),
			Path:        path,
			Fingerprint: fingerprint,
			KeyName:     ref.Name,
			Updated:     info.ModTime(),
			Stale:       ref.Fingerprint == "",
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Ref < out[j].Ref
	})
	return out
}

func publicKeyFileFingerprint(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	pub, _, _, _, err := ssh.ParseAuthorizedKey(data)
	if err != nil {
		return ""
	}
	return ssh.FingerprintSHA256(pub)
}

func keyRefsByPath(refs []KeyRef) map[string]KeyRef {
	out := make(map[string]KeyRef, len(refs))
	for _, ref := range refs {
		out[ref.Path] = ref
	}
	return out
}

func keyNamesByFingerprint(keys []vault.Key) map[string]string {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if strings.TrimSpace(key.Fingerprint) != "" {
			out[key.Fingerprint] = key.Name
		}
	}
	return out
}

func cloneAttempt(attempt Attempt) Attempt {
	attempt.Candidates = append([]string(nil), attempt.Candidates...)
	return attempt
}
