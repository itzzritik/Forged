//go:build e2e

package e2e

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"

	"golang.org/x/crypto/ssh"
)

// sshServer accepts public-key logins for an allow-listed key and answers
// every exec request, so a real ssh client must sign through the agent.
type sshServer struct {
	ln         net.Listener
	mu         sync.Mutex
	authorized []ssh.PublicKey
	commands   []string
}

func startSSHServer(t *testing.T) *sshServer {
	t.Helper()
	_, hostPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	hostKey, err := ssh.NewSignerFromKey(hostPriv)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &sshServer{ln: ln}
	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			s.mu.Lock()
			defer s.mu.Unlock()
			for _, allowed := range s.authorized {
				if bytes.Equal(allowed.Marshal(), key.Marshal()) {
					return &ssh.Permissions{}, nil
				}
			}
			return nil, fmt.Errorf("unknown key")
		},
	}
	cfg.AddHostKey(hostKey)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.serve(conn, cfg)
		}
	}()
	t.Cleanup(func() { ln.Close() })
	return s
}

func (s *sshServer) Port() string {
	return strconv.Itoa(s.ln.Addr().(*net.TCPAddr).Port)
}

func (s *sshServer) Authorize(t *testing.T, authorizedKey string) {
	t.Helper()
	key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(authorizedKey))
	if err != nil {
		t.Fatalf("parsing authorized key: %v", err)
	}
	s.mu.Lock()
	s.authorized = append(s.authorized, key)
	s.mu.Unlock()
}

func (s *sshServer) serve(conn net.Conn, cfg *ssh.ServerConfig) {
	defer conn.Close()
	sc, chans, reqs, err := ssh.NewServerConn(conn, cfg)
	if err != nil {
		return
	}
	defer sc.Close()
	go ssh.DiscardRequests(reqs)
	for ch := range chans {
		if ch.ChannelType() != "session" {
			_ = ch.Reject(ssh.UnknownChannelType, "only sessions")
			continue
		}
		channel, requests, err := ch.Accept()
		if err != nil {
			continue
		}
		go func() {
			defer channel.Close()
			for req := range requests {
				if req.Type != "exec" {
					_ = req.Reply(false, nil)
					continue
				}
				var payload struct{ Command string }
				_ = ssh.Unmarshal(req.Payload, &payload)
				_ = req.Reply(true, nil)
				s.mu.Lock()
				s.commands = append(s.commands, payload.Command)
				s.mu.Unlock()
				if strings.HasPrefix(payload.Command, "git-upload-pack") {
					// An empty ref advertisement: enough for git ls-remote.
					_, _ = channel.Write([]byte("0000"))
				} else {
					fmt.Fprintf(channel, "forged-e2e-ok user=%s cmd=%s\n", sc.User(), payload.Command)
				}
				_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
				return
			}
		}()
	}
}

func (s *sshServer) Commands() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.commands...)
}
