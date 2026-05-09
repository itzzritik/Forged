package sshrouting

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	publicHintTTL          = 5 * time.Minute
	routeSnippetTTL        = 5 * time.Minute
	routeIdentitySlotCount = 6
)

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
	return atomicWriteFile(filepath.Join(dir, attempt+".conf"), []byte(content), 0o600)
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

func routeIdentitySlotName(slot int) string {
	return fmt.Sprintf("k%d.pub", slot)
}

func CleanupRouteRuntime(dir string, cutoff time.Time) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if !info.ModTime().Before(cutoff) {
			continue
		}
		if entry.IsDir() {
			_ = os.RemoveAll(path)
			continue
		}
		if strings.HasSuffix(entry.Name(), ".conf") {
			_ = os.Remove(path)
		}
	}
	return nil
}

func RemoveRouteSnippet(dir, attempt string) {
	if validateAttemptToken(attempt) != nil {
		return
	}
	_ = os.Remove(filepath.Join(dir, attempt+".conf"))
	_ = os.RemoveAll(filepath.Join(dir, attempt))
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

func validateAttemptToken(token string) error {
	if token == "" {
		return fmt.Errorf("Empty SSH route attempt token")
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
