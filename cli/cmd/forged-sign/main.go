package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/hiddeco/sshsig"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "forged-sign: no arguments provided")
		os.Exit(1)
	}

	var operation, namespace, keyFile, bufferFile string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-Y":
			if i+1 < len(args) {
				operation = args[i+1]
				i++
			}
		case "-n":
			if i+1 < len(args) {
				namespace = args[i+1]
				i++
			}
		case "-f":
			if i+1 < len(args) {
				keyFile = args[i+1]
				i++
			}
		case "-U":
			// Agent mode, ignored since we always use agent
		default:
			if !strings.HasPrefix(args[i], "-") {
				bufferFile = args[i]
			}
		}
	}

	if operation == "sign" {
		if err := signFile(keyFile, bufferFile, namespace); err != nil {
			fmt.Fprintf(os.Stderr, "forged-sign: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	fmt.Fprintf(os.Stderr, "forged-sign: unsupported operation: %s\n", operation)
	os.Exit(1)
}

func signFile(keyFile, bufferFile, namespace string) error {
	if bufferFile == "" {
		return fmt.Errorf("no buffer file specified")
	}
	if namespace == "" {
		namespace = "git"
	}

	data, err := os.ReadFile(bufferFile)
	if err != nil {
		return fmt.Errorf("reading buffer file: %w", err)
	}

	var signingPubKey ssh.PublicKey
	if keyFile != "" {
		keyData, err := os.ReadFile(keyFile)
		if err != nil {
			return fmt.Errorf("reading key file: %w", err)
		}
		pub, _, _, _, err := ssh.ParseAuthorizedKey(keyData)
		if err != nil {
			return fmt.Errorf("parsing public key: %w", err)
		}
		signingPubKey = pub
	}

	conn, err := net.DialTimeout("unix", forgedSocketPath(), 2*time.Second)
	if err != nil {
		return fmt.Errorf("cannot connect to forged agent: %w", err)
	}
	defer conn.Close()

	agentClient := agent.NewClient(conn)

	signers, err := agentClient.Signers()
	if err != nil {
		return fmt.Errorf("getting signers: %w", err)
	}

	var signer ssh.Signer
	if signingPubKey != nil {
		wantBlob := signingPubKey.Marshal()
		for _, s := range signers {
			if bytes.Equal(s.PublicKey().Marshal(), wantBlob) {
				signer = s
				break
			}
		}
		if signer == nil {
			return fmt.Errorf("signing key not found in agent")
		}
	} else {
		if len(signers) == 0 {
			return fmt.Errorf("no keys available in agent")
		}
		signer = signers[0]
	}

	sig, err := sshsig.Sign(bytes.NewReader(data), signer, sshsig.HashSHA512, namespace)
	if err != nil {
		return fmt.Errorf("signing: %w", err)
	}

	armored := sshsig.Armor(sig)

	sigFile := bufferFile + ".sig"
	if err := os.WriteFile(sigFile, armored, 0600); err != nil {
		return fmt.Errorf("writing signature file: %w", err)
	}

	return nil
}

func forgedSocketPath() string {
	switch runtime.GOOS {
	case "linux":
		if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
			return filepath.Join(xdg, "forged", "agent.sock")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "state", "forged", "agent.sock")
	default:
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".forged", "agent.sock")
	}
}
