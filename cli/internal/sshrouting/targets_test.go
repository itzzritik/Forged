package sshrouting

import (
	"os/exec"
	"testing"
)

func writeGitConfig(t *testing.T, dir string, key string, value string) {
	t.Helper()
	cmd := exec.Command("git", "config", key, value)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git config %s=%s: %v\n%s", key, value, err, out)
	}
}

func TestResolveGitTargetPrefersPushRemoteAndPushURL(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	writeGitConfig(t, dir, "branch.main.remote", "origin")
	writeGitConfig(t, dir, "branch.main.pushRemote", "work")
	writeGitConfig(t, dir, "remote.origin.url", "git@github.com:itzzritik/XtremeSetup.git")
	writeGitConfig(t, dir, "remote.work.url", "git@github.com:AdeptMind/dlp-ssr.git")
	writeGitConfig(t, dir, "remote.work.pushurl", "ssh://git@github.com:22/AdeptMind/dlp-web.git")

	target, err := ResolveGitTarget(dir, "main")
	if err != nil {
		t.Fatalf("resolve git target: %v", err)
	}
	if target.Canonical != "git+ssh://git@github.com:22/AdeptMind/dlp-web" {
		t.Fatalf("unexpected canonical target: %#v", target)
	}
}

func TestResolveSSHTargetBuildsCanonicalString(t *testing.T) {
	target, err := ResolveSSHTarget(PrepareInput{
		Host:         "144.24.124.129",
		OriginalHost: "prod-box",
		User:         "ubuntu",
		Port:         "22",
	})
	if err != nil {
		t.Fatalf("resolve ssh target: %v", err)
	}
	if target.Canonical != "ssh://ubuntu@144.24.124.129:22" {
		t.Fatalf("unexpected canonical target: %#v", target)
	}
}

func TestParseCanonicalTargetRoundTripsGitOwnerAndRepo(t *testing.T) {
	target, err := ParseCanonicalTarget("git+ssh://git@github.com:22/AdeptMind/dlp-ssr")
	if err != nil {
		t.Fatalf("parse canonical target: %v", err)
	}
	if target.Owner != "AdeptMind" || target.Repo != "dlp-ssr" {
		t.Fatalf("unexpected owner/repo: %#v", target)
	}
}
