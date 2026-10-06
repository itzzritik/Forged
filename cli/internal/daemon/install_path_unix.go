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
func EnsureOnPath() (bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, err
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

	targets := []struct {
		file   string
		block  string
		create bool
	}{
		{filepath.Join(zdot, ".zshrc"), posixBlock, shell == "zsh"},
		{filepath.Join(home, ".bashrc"), posixBlock, shell == "bash"},
		{filepath.Join(home, ".bash_profile"), posixBlock, false},
		{filepath.Join(home, ".profile"), posixBlock, shell != "zsh" && shell != "bash" && shell != "fish"},
		{filepath.Join(xdgConfig, "fish", "conf.d", "forged.fish"), fishBlock, fishErr == nil},
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
		if strings.Contains(string(data), target.block) {
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
