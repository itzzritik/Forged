package config

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func uninstallTestPaths(t *testing.T) Paths {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	return Paths{ConfigDir: filepath.Join(home, ".config", "forged"), DataDir: filepath.Join(home, ".config", "forged", "data")}
}

func TestRemoveSSHIntegration(t *testing.T) {
	cases := map[string]struct {
		forgedLines func(Paths) string
		eol         string
	}{
		"enabled":  {func(p Paths) string { return sshIncludeComment + "\n" + includeLine(p.SSHManagedConfig()) }, "\n"},
		"disabled": {func(p Paths) string { return sshIncludeComment + "\n# " + includeLine(p.SSHManagedConfig()) }, "\n"},
		"crlf":     {func(p Paths) string { return sshIncludeComment + "\n" + includeLine(p.SSHManagedConfig()) }, "\r\n"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			paths := uninstallTestPaths(t)
			user := "Host a\n  User x\n"
			content := strings.ReplaceAll(tc.forgedLines(paths)+"\n\n"+user, "\n", tc.eol)
			if err := os.WriteFile(paths.SSHUserConfig(), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(paths.LegacySSHManagedDir(), 0o700); err != nil {
				t.Fatal(err)
			}

			if err := RemoveSSHIntegration(paths); err != nil {
				t.Fatal(err)
			}

			data, err := os.ReadFile(paths.SSHUserConfig())
			if err != nil {
				t.Fatal(err)
			}
			if want := strings.ReplaceAll(user, "\n", tc.eol); string(data) != want {
				t.Fatalf("~/.ssh/config = %q, want %q", data, want)
			}
			for _, leftover := range []string{paths.SSHUserConfig() + ".forged-backup", paths.LegacySSHManagedDir()} {
				if _, err := os.Stat(leftover); !os.IsNotExist(err) {
					t.Errorf("%s still exists", leftover)
				}
			}
		})
	}
}

func gitConfigSet(t *testing.T, key, value string) {
	t.Helper()
	if out, err := exec.Command("git", "config", "--global", key, value).CombinedOutput(); err != nil {
		t.Fatalf("git config %s: %v\n%s", key, err, out)
	}
}

func gitConfigGet(key string) (string, bool) {
	out, err := exec.Command("git", "config", "--global", "--get", key).Output()
	return strings.TrimSpace(string(out)), err == nil
}

func TestRemoveGitSigning(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("needs git")
	}

	t.Run("restores the saved setup", func(t *testing.T) {
		paths := uninstallTestPaths(t)
		backup, _ := json.Marshal(map[string]string{"commit.gpgsign": "true", "gpg.format": "openpgp"})
		if err := os.MkdirAll(paths.ConfigDir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(paths.GitSigningBackupFile(), backup, 0o600); err != nil {
			t.Fatal(err)
		}
		gitConfigSet(t, "gpg.ssh.program", "/x/forged-sign")
		gitConfigSet(t, "gpg.format", "ssh")
		gitConfigSet(t, "user.signingkey", "ssh-ed25519 AAAA")
		gitConfigSet(t, "commit.gpgsign", "true")

		if err := RemoveGitSigning(paths); err != nil {
			t.Fatal(err)
		}
		if v, _ := gitConfigGet("gpg.format"); v != "openpgp" {
			t.Errorf("gpg.format = %q, want openpgp", v)
		}
		for _, key := range []string{"gpg.ssh.program", "user.signingkey"} {
			if v, set := gitConfigGet(key); set {
				t.Errorf("%s = %q, want unset", key, v)
			}
		}
	})

	t.Run("no saved setup unsets forged signing", func(t *testing.T) {
		paths := uninstallTestPaths(t)
		gitConfigSet(t, "gpg.ssh.program", "/x/forged-sign")
		gitConfigSet(t, "commit.gpgsign", "true")

		if err := RemoveGitSigning(paths); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"gpg.ssh.program", "commit.gpgsign"} {
			if v, set := gitConfigGet(key); set {
				t.Errorf("%s = %q, want unset", key, v)
			}
		}
	})

	t.Run("leaves another signing program alone", func(t *testing.T) {
		paths := uninstallTestPaths(t)
		gitConfigSet(t, "gpg.ssh.program", "/usr/bin/ssh-keygen")
		gitConfigSet(t, "commit.gpgsign", "true")

		if err := RemoveGitSigning(paths); err != nil {
			t.Fatal(err)
		}
		if v, _ := gitConfigGet("gpg.ssh.program"); v != "/usr/bin/ssh-keygen" {
			t.Errorf("gpg.ssh.program = %q, want it untouched", v)
		}
		if v, _ := gitConfigGet("commit.gpgsign"); v != "true" {
			t.Errorf("commit.gpgsign = %q, want it untouched", v)
		}
	})
}
