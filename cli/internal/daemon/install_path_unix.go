//go:build !windows

package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const pathComment = "# Forged CLI"

// EnsureOnPath appends the install folder's bin to the user's shell PATH.
// Appended, so a package manager's forged still wins while it exists.
type pathTarget struct {
	file   string
	block  string
	create bool
}

func pathTargets() ([]pathTarget, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	bin := filepath.Join(InstallDir(), "bin")
	sh := "'" + strings.ReplaceAll(bin, "'", `'\''`) + "'"
	fish := "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(bin) + "'"
	posixBlock := fmt.Sprintf("%s\ncase \":$PATH:\" in *:%s:*) ;; *) export PATH=\"$PATH:\"%s ;; esac\n", pathComment, sh, sh)
	fishBlock := fmt.Sprintf("%s\ncontains -- %s $PATH; or set -gx PATH $PATH %s\n", pathComment, fish, fish)

	zdot := os.Getenv("ZDOTDIR")
	if zdot == "" {
		zdot = home
	}
	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if !filepath.IsAbs(xdgConfig) {
		xdgConfig = filepath.Join(home, ".config")
	}
	shell := filepath.Base(os.Getenv("SHELL"))
	_, fishErr := os.Stat(filepath.Join(xdgConfig, "fish"))

	return []pathTarget{
		{filepath.Join(zdot, ".zshrc"), posixBlock, shell == "zsh"},
		{filepath.Join(home, ".bashrc"), posixBlock, shell == "bash"},
		{filepath.Join(home, ".bash_profile"), posixBlock, false},
		{filepath.Join(home, ".profile"), posixBlock, shell != "zsh" && shell != "bash" && shell != "fish"},
		{filepath.Join(xdgConfig, "fish", "conf.d", "forged.fish"), fishBlock, fishErr == nil},
	}, nil
}

func EnsureOnPath() (bool, error) {
	targets, err := pathTargets()
	if err != nil {
		return false, err
	}
	changed := false
	var errs []error
	for _, target := range targets {
		data, err := os.ReadFile(target.file)
		if errors.Is(err, os.ErrNotExist) && !target.create {
			continue
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, fmt.Errorf("reading %s: %w", target.file, err))
			continue
		}
		if hasPathBlock(data) {
			continue
		}
		if err := appendShellBlock(target.file, data, target.block); err != nil {
			errs = append(errs, fmt.Errorf("updating %s: %w", target.file, err))
			continue
		}
		changed = true
	}
	return changed, errors.Join(errs...)
}

// RemoveFromPath takes the block EnsureOnPath added out of every shell rc file.
// It writes in place, so a symlinked rc file (dotfiles) stays a link.
func RemoveFromPath() error {
	targets, err := pathTargets()
	if err != nil {
		return err
	}
	var errs []error
	for _, target := range targets {
		data, err := os.ReadFile(target.file)
		if errors.Is(err, os.ErrNotExist) || (err == nil && !hasPathBlock(data)) {
			continue
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("reading %s: %w", target.file, err))
			continue
		}
		cleaned := removePathBlock(string(data))
		if filepath.Base(target.file) == "forged.fish" && strings.TrimSpace(cleaned) == "" {
			err = os.Remove(target.file)
		} else {
			err = os.WriteFile(target.file, []byte(cleaned), 0o644)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("updating %s: %w", target.file, err))
		}
	}
	return errors.Join(errs...)
}

// removePathBlock drops each marker, the PATH line after it, and the blank
// line appendShellBlock put before it.
func removePathBlock(content string) string {
	lines := strings.Split(content, "\n")
	kept := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != pathComment {
			kept = append(kept, lines[i])
			continue
		}
		if n := len(kept); n > 0 && strings.TrimSpace(kept[n-1]) == "" {
			kept = kept[:n-1]
		}
		i++
	}
	return strings.Join(kept, "\n")
}

// hasPathBlock matches the marker line, not the exact block: a CRLF file, a reformatted block or
// another install path must not add one more block on every launch.
func hasPathBlock(data []byte) bool {
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == pathComment {
			return true
		}
	}
	return false
}

func appendShellBlock(path string, existing []byte, block string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if len(existing) > 0 {
		block = "\n" + block
		if existing[len(existing)-1] != '\n' {
			block = "\n" + block
		}
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = file.WriteString(block)
	return errors.Join(err, file.Close())
}
