package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/hiddeco/sshsig"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/keytypes"
	"github.com/itzzritik/forged/cli/internal/platform"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "forged-sign: No arguments provided")
		os.Exit(1)
	}
	if !isSignOperation(args) {
		delegateSSHKeygen(args)
		return
	}

	var namespace, keyFile, bufferFile string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-Y":
			if i+1 < len(args) {
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

	if err := signFile(keyFile, bufferFile, namespace); err != nil {
		fmt.Fprintf(os.Stderr, "forged-sign: %v\n", err)
		os.Exit(1)
	}
}

func isSignOperation(args []string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "-Y" {
			return args[i+1] == "sign"
		}
	}
	return false
}

func delegateSSHKeygen(args []string) {
	cmd := exec.Command("ssh-keygen", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if exitCode := exitErr.ExitCode(); exitCode > 0 {
				os.Exit(exitCode)
			}
		}
		fmt.Fprintf(os.Stderr, "forged-sign: running ssh-keygen: %v\n", err)
		os.Exit(1)
	}
}

func signFile(keyFile, bufferFile, namespace string) error {
	if bufferFile == "" {
		return fmt.Errorf("No buffer file specified")
	}
	if namespace == "" {
		namespace = "git"
	}

	var signingPubKey ssh.PublicKey
	if keyFile != "" {
		keyData, err := os.ReadFile(keyFile)
		if err != nil {
			return fmt.Errorf("Reading key file: %w", err)
		}
		pub, _, _, _, err := ssh.ParseAuthorizedKey(keyData)
		if err != nil {
			return fmt.Errorf("Parsing public key: %w", err)
		}
		if !keytypes.SupportsSSHSigning(pub.Type()) {
			return fmt.Errorf("SSH key type %q is not supported for Forged signing; choose an RSA, ECDSA, or Ed25519 key", pub.Type())
		}
		signingPubKey = pub
	}

	data, err := os.ReadFile(bufferFile)
	if err != nil {
		return fmt.Errorf("Reading buffer file: %w", err)
	}
	defer clear(data)

	paths := config.DefaultPaths()
	if err := actions.AuthorizeExternalUse(paths); err != nil {
		return err
	}

	socketPath := paths.AgentSocket()
	conn, err := platform.Dial(socketPath, 2*time.Second)
	if err != nil {
		return fmt.Errorf("Cannot connect to Forged agent at %s: %w", socketPath, err)
	}
	defer conn.Close()

	agentClient := agent.NewClient(conn)

	signers, err := agentClient.Signers()
	if err != nil {
		return fmt.Errorf("Getting signers: %w", err)
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
			return fmt.Errorf("Signing key not found in agent")
		}
	} else {
		if len(signers) == 0 {
			return fmt.Errorf("No keys available in agent")
		}
		signer = signers[0]
	}

	sig, err := sshsig.Sign(bytes.NewReader(data), signer, sshsig.HashSHA512, namespace)
	if err != nil {
		return fmt.Errorf("Signing: %w", err)
	}

	sigFile := bufferFile + ".sig"
	return os.WriteFile(sigFile, sshsig.Armor(sig), 0600)
}
