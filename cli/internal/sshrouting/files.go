package sshrouting

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/platform"
	"golang.org/x/crypto/ssh"
)

const (
	publicHintTTL          = 5 * time.Minute
	routeSnippetTTL        = 5 * time.Minute
	routeIdentitySlotCount = 6
	routeStateFilename     = ".route-state.json"
	routeStateMaxBytes     = 1 << 20
	routeStateMaxAttempts  = 256
)

type routeStateFile struct {
	Version  int                 `json:"version"`
	Attempts []routeStateAttempt `json:"attempts"`
}

type routeStateAttempt struct {
	Token      string                   `json:"token"`
	Client     platform.ProcessInstance `json:"client"`
	Candidates []string                 `json:"candidates,omitempty"`
	Created    time.Time                `json:"created"`
	ExpiresAt  time.Time                `json:"expires_at"`
}

type routeAttemptCandidate struct {
	key     string
	attempt Attempt
}

func candidatesForRouteAttempts(attempts []routeAttemptCandidate) []string {
	sort.Slice(attempts, func(i, j int) bool {
		if !attempts[i].attempt.Created.Equal(attempts[j].attempt.Created) {
			return attempts[i].attempt.Created.Before(attempts[j].attempt.Created)
		}
		if attempts[i].attempt.ClientPID != attempts[j].attempt.ClientPID {
			return attempts[i].attempt.ClientPID < attempts[j].attempt.ClientPID
		}
		return attempts[i].key < attempts[j].key
	})
	seen := map[string]struct{}{}
	out := make([]string, 0, routeIdentitySlotCount)
	for candidateIndex := 0; len(out) < routeIdentitySlotCount; candidateIndex++ {
		progressed := false
		for _, item := range attempts {
			if candidateIndex >= len(item.attempt.Candidates) {
				continue
			}
			progressed = true
			fingerprint := item.attempt.Candidates[candidateIndex]
			if _, ok := seen[fingerprint]; ok {
				continue
			}
			seen[fingerprint] = struct{}{}
			out = append(out, fingerprint)
			if len(out) == routeIdentitySlotCount {
				return out
			}
		}
		if !progressed {
			break
		}
	}
	return out
}

func SyncPublicHintFiles(dir string, refs []KeyRef, now time.Time) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("Creating SSH key hint directory: %w", err)
	}

	keep := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		if strings.TrimSpace(ref.PublicKey) == "" || strings.TrimSpace(ref.Path) == "" {
			continue
		}
		keep[ref.Path] = struct{}{}
		if err := atomicWriteFile(ref.Path, []byte(strings.TrimSpace(ref.PublicKey)+"\n"), 0o600); err != nil {
			return fmt.Errorf("Writing SSH key hint %s: %w", ref.Ref, err)
		}
	}

	return cleanupUnknownHintFiles(dir, keep, now.Add(-publicHintTTL))
}

func WriteRouteSnippet(dir, attempt string, refs []KeyRef) error {
	if err := validateAttemptToken(attempt); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("Creating SSH route runtime directory: %w", err)
	}
	if err := RemoveRouteReady(dir, attempt); err != nil {
		return err
	}

	if err := writeRouteIdentitySlots(dir, attempt, refs); err != nil {
		return err
	}

	var lines []string
	lines = append(lines, "# Forged SSH route attempt")
	lines = append(lines, "IdentitiesOnly yes")
	for _, ref := range refs {
		if strings.TrimSpace(ref.Path) == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("IdentityFile %q", ref.Path))
	}
	content := strings.Join(lines, "\n") + "\n"
	if err := atomicWriteFile(filepath.Join(dir, attempt+".conf"), []byte(content), 0o600); err != nil {
		return err
	}
	return atomicWriteFile(routeReadyPath(dir, attempt), []byte("ready\n"), 0o600)
}

func writeRouteIdentitySlots(dir, attempt string, refs []KeyRef) error {
	attemptDir := filepath.Join(dir, attempt)
	if err := os.MkdirAll(attemptDir, 0o700); err != nil {
		return fmt.Errorf("Creating SSH route identity slots: %w", err)
	}

	for slot := 1; slot <= routeIdentitySlotCount; slot++ {
		path := routeIdentitySlotPath(dir, attempt, slot)
		index := slot - 1
		if index >= len(refs) || strings.TrimSpace(refs[index].PublicKey) == "" {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("Removing stale SSH route identity slot: %w", err)
			}
			continue
		}
		content := strings.TrimSpace(refs[index].PublicKey) + "\n"
		if err := atomicWriteFile(path, []byte(content), 0o600); err != nil {
			return fmt.Errorf("Writing SSH route identity slot: %w", err)
		}
	}
	return nil
}

func routeIdentitySlotPattern(dir string, slot int) string {
	return filepath.Join(dir, "%C", routeIdentitySlotName(slot))
}

func routeIdentitySlotPath(dir, attempt string, slot int) string {
	return filepath.Join(dir, attempt, routeIdentitySlotName(slot))
}

func routeReadyPath(dir, attempt string) string {
	return filepath.Join(dir, attempt+".ready")
}

func routeIdentitySlotName(slot int) string {
	return fmt.Sprintf("k%d.pub", slot)
}

func RemoveRouteSnippet(dir, attempt string) error {
	if validateAttemptToken(attempt) != nil {
		return nil
	}
	if err := RemoveRouteReady(dir, attempt); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(dir, attempt+".conf")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing SSH route snippet: %w", err)
	}
	if err := os.RemoveAll(filepath.Join(dir, attempt)); err != nil {
		return fmt.Errorf("removing SSH route identity slots: %w", err)
	}
	return nil
}

func RemoveRouteReady(dir, attempt string) error {
	if validateAttemptToken(attempt) != nil {
		return nil
	}
	if err := os.Remove(routeReadyPath(dir, attempt)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing SSH route ready marker: %w", err)
	}
	return nil
}

func cleanupUnknownHintFiles(dir string, keep map[string]struct{}, cutoff time.Time) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "k_") || !strings.HasSuffix(entry.Name(), ".pub") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if _, ok := keep[path]; ok {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(path)
		}
	}
	return nil
}

func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func (s *Service) writeRouteStateLocked() error {
	attempts := make([]Attempt, 0, len(s.attempts))
	for _, attempt := range s.attempts {
		attempts = append(attempts, attempt)
	}
	sort.Slice(attempts, func(i, j int) bool {
		if attempts[i].Token != attempts[j].Token {
			return attempts[i].Token < attempts[j].Token
		}
		return attempts[i].ClientPID < attempts[j].ClientPID
	})

	state := routeStateFile{Version: 1, Attempts: make([]routeStateAttempt, 0, len(attempts))}
	for _, attempt := range attempts {
		state.Attempts = append(state.Attempts, routeStateAttempt{
			Token:      attempt.Token,
			Client:     attempt.Process,
			Candidates: append([]string(nil), attempt.Candidates...),
			Created:    attempt.Created,
			ExpiresAt:  attempt.ExpiresAt,
		})
	}

	path := filepath.Join(s.paths.SSHRouteRuntimeDir(), routeStateFilename)
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encoding SSH route state: %w", err)
	}
	if len(data) > routeStateMaxBytes {
		return fmt.Errorf("SSH route state exceeds %d bytes", routeStateMaxBytes)
	}
	if err := atomicWriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing SSH route state: %w", err)
	}
	return nil
}

func (s *Service) resetRouteRuntimeLocked(discardAttempts bool) error {
	if s.startupIdentityUnavailable {
		return fmt.Errorf("daemon process identity is unavailable")
	}
	s.runtimeUntrusted = true
	s.runtimeWriteFailed = true
	if len(s.unscoped) > 0 || (!discardAttempts && len(s.attempts) > 0) {
		return fmt.Errorf("close active SSH sessions before resetting route runtime")
	}
	if err := os.RemoveAll(s.paths.SSHRouteRuntimeDir()); err != nil {
		return fmt.Errorf("removing SSH route runtime: %w", err)
	}
	if err := os.MkdirAll(s.paths.SSHRouteRuntimeDir(), 0o700); err != nil {
		return fmt.Errorf("creating SSH route runtime: %w", err)
	}
	s.attempts = map[string]Attempt{}
	s.attemptOwners = map[string]uint64{}
	s.clientAttempt = map[int]string{}
	s.attemptSeq = 0
	if err := s.writeRouteStateLocked(); err != nil {
		return err
	}
	artifacts, err := routeRuntimeArtifacts(s.paths.SSHRouteRuntimeDir())
	if err != nil {
		return fmt.Errorf("checking SSH route runtime: %w", err)
	}
	if len(artifacts) != 0 {
		return fmt.Errorf("SSH route runtime still has route artifacts")
	}
	s.runtimeUntrusted = false
	s.runtimeWriteFailed = false
	return nil
}

func loadRouteState(dir string) ([]Attempt, bool) {
	artifacts, err := routeRuntimeArtifacts(dir)
	if err != nil {
		return nil, true
	}
	path := filepath.Join(dir, routeStateFilename)
	data, err := readRouteState(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, len(artifacts) > 0
		}
		return nil, true
	}

	var state routeStateFile
	if err := json.Unmarshal(data, &state); err != nil || state.Version != 1 {
		return nil, true
	}

	if len(state.Attempts) > routeStateMaxAttempts {
		return nil, true
	}
	attempts := make([]Attempt, 0, len(state.Attempts))
	seenPIDs := map[int]struct{}{}
	for _, stored := range state.Attempts {
		attempt, err := restoreRouteAttempt(stored)
		if err != nil {
			return nil, true
		}
		if _, exists := seenPIDs[attempt.ClientPID]; exists {
			return nil, true
		}
		seenPIDs[attempt.ClientPID] = struct{}{}
		attempts = append(attempts, attempt)
	}

	if !routeRuntimeMatchesState(dir, artifacts, attempts) {
		return nil, true
	}
	return attempts, false
}

func routeRuntimeMatchesState(dir string, artifacts map[string]time.Time, attempts []Attempt) bool {
	byToken := map[string][]routeAttemptCandidate{}
	for _, attempt := range attempts {
		if len(attempt.Candidates) == 0 {
			continue
		}
		byToken[attempt.Token] = append(byToken[attempt.Token], routeAttemptCandidate{
			key:     routeAttemptKey(attempt.Token, attempt.ClientPID),
			attempt: attempt,
		})
	}
	for token, entries := range byToken {
		candidates := candidatesForRouteAttempts(entries)
		if len(candidates) == 0 || !allRouteCandidatesRepresented(entries, candidates) {
			return false
		}
		if _, ok := artifacts[token]; !ok || !routeIdentitySlotsMatch(dir, token, candidates) {
			return false
		}
		delete(artifacts, token)
	}
	return len(artifacts) == 0
}

func allRouteCandidatesRepresented(entries []routeAttemptCandidate, available []string) bool {
	for _, entry := range entries {
		for _, fingerprint := range entry.attempt.Candidates {
			found := false
			for _, candidate := range available {
				if candidate == fingerprint {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	return true
}

func routeIdentitySlotsMatch(dir, attempt string, candidates []string) bool {
	ready, err := os.Stat(routeReadyPath(dir, attempt))
	if err != nil || !ready.Mode().IsRegular() {
		return false
	}
	conf, err := os.Stat(filepath.Join(dir, attempt+".conf"))
	if err != nil || !conf.Mode().IsRegular() {
		return false
	}
	slots, err := os.Stat(filepath.Join(dir, attempt))
	if err != nil || !slots.IsDir() {
		return false
	}
	for slot := 1; slot <= routeIdentitySlotCount; slot++ {
		path := routeIdentitySlotPath(dir, attempt, slot)
		index := slot - 1
		if index >= len(candidates) {
			if _, err := os.Lstat(path); err == nil || !os.IsNotExist(err) {
				return false
			}
			continue
		}
		if !routeIdentitySlotMatches(dir, attempt, slot, candidates[index]) {
			return false
		}
	}
	return true
}

func routeIdentitySlotMatches(dir, attempt string, slot int, fingerprint string) bool {
	if validateAttemptToken(attempt) != nil || slot < 1 || slot > routeIdentitySlotCount || fingerprint == "" {
		return false
	}
	ready, err := os.Stat(routeReadyPath(dir, attempt))
	if err != nil || !ready.Mode().IsRegular() {
		return false
	}
	conf, err := os.Stat(filepath.Join(dir, attempt+".conf"))
	if err != nil || !conf.Mode().IsRegular() {
		return false
	}
	slots, err := os.Stat(filepath.Join(dir, attempt))
	if err != nil || !slots.IsDir() {
		return false
	}
	data, err := os.ReadFile(routeIdentitySlotPath(dir, attempt, slot))
	if err != nil {
		return false
	}
	key, _, _, _, err := ssh.ParseAuthorizedKey(data)
	return err == nil && ssh.FingerprintSHA256(key) == fingerprint
}

func readRouteState(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, routeStateMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > routeStateMaxBytes {
		return nil, fmt.Errorf("SSH route state exceeds %d bytes", routeStateMaxBytes)
	}
	return data, nil
}

func restoreRouteAttempt(stored routeStateAttempt) (Attempt, error) {
	if err := validateAttemptToken(stored.Token); err != nil {
		return Attempt{}, err
	}
	if !stored.Client.Valid() || stored.Client.PID <= 0 {
		return Attempt{}, fmt.Errorf("invalid SSH route client process")
	}
	if stored.Created.IsZero() || stored.ExpiresAt.IsZero() || !stored.ExpiresAt.Equal(stored.Created.Add(routeSnippetTTL)) {
		return Attempt{}, fmt.Errorf("invalid SSH route expiry")
	}
	if len(stored.Candidates) > routeIdentitySlotCount {
		return Attempt{}, fmt.Errorf("too many SSH route candidates")
	}
	seen := map[string]struct{}{}
	for _, fingerprint := range stored.Candidates {
		if fingerprint == "" || len(fingerprint) > 512 {
			return Attempt{}, fmt.Errorf("invalid SSH route fingerprint")
		}
		if _, exists := seen[fingerprint]; exists {
			return Attempt{}, fmt.Errorf("duplicate SSH route fingerprint")
		}
		seen[fingerprint] = struct{}{}
	}
	return Attempt{
		Token:      stored.Token,
		ClientPID:  stored.Client.PID,
		Process:    stored.Client,
		Candidates: append([]string(nil), stored.Candidates...),
		Created:    stored.Created,
		ExpiresAt:  stored.ExpiresAt,
	}, nil
}

func routeRuntimeArtifacts(dir string) (map[string]time.Time, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return map[string]time.Time{}, nil
	}
	if err != nil {
		return nil, err
	}
	artifacts := map[string]time.Time{}
	for _, entry := range entries {
		name := entry.Name()
		token := ""
		switch {
		case entry.IsDir():
			token = name
		case strings.HasSuffix(name, ".conf"):
			token = strings.TrimSuffix(name, ".conf")
		case strings.HasSuffix(name, ".ready"):
			token = strings.TrimSuffix(name, ".ready")
		default:
			continue
		}
		if validateAttemptToken(token) != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(artifacts[token]) {
			artifacts[token] = info.ModTime()
		}
	}
	return artifacts, nil
}

func validateAttemptToken(token string) error {
	if token == "" {
		return fmt.Errorf("Empty SSH route attempt token")
	}
	if token == "." || token == ".." {
		return fmt.Errorf("SSH route attempt token cannot be a path component")
	}
	if len(token) > 256 {
		return fmt.Errorf("SSH route attempt token is too long")
	}
	for _, r := range token {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '-' || r == '.':
		default:
			return fmt.Errorf("SSH route attempt token contains unsafe character %q", r)
		}
	}
	return nil
}
