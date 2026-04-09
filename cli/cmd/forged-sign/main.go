package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/hiddeco/sshsig"
	"github.com/itzzritik/forged/cli/internal/config"
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
		default:
			if !strings.HasPrefix(args[i], "-") {
				bufferFile = args[i]
			}
		}
	}

	if operation != "sign" {
		fmt.Fprintf(os.Stderr, "forged-sign: unsupported operation: %s\n", operation)
		os.Exit(1)
	}

	if err := signFile(keyFile, bufferFile, namespace); err != nil {
		fmt.Fprintf(os.Stderr, "forged-sign: %v\n", err)
		os.Exit(1)
	}
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

	socketPath := config.DefaultPaths().AgentSocket()
	conn, err := net.DialTimeout("unix", socketPath, 2*time.Second)
	if err != nil {
		return fmt.Errorf("cannot connect to forged agent at %s: %w", socketPath, err)
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

	sigFile := bufferFile + ".sig"
	return os.WriteFile(sigFile, sshsig.Armor(sig), 0600)
}
