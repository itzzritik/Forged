//go:build !windows

package config

const defaultAgentEndpoint = ""

func sshClientPath() string { return "ssh" }

func InspectGitSSH() GitSSHStatus { return GitSSHStatus{} }

func EnsureGitUsesNativeSSH() error { return nil }

func RemoveGitNativeSSH() error { return nil }
