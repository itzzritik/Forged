package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: forged-sign <namespace>")
		os.Exit(1)
	}

	socketPath := os.Getenv("SSH_AUTH_SOCK")
	if socketPath == "" {
		socketPath = defaultSocketPath()
	}

	conn, err := net.DialTimeout("unix", socketPath, 2*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "forged-sign: cannot connect to agent at %s: %v\n", socketPath, err)
		os.Exit(1)
	}
	defer conn.Close()

	agentClient := agent.NewClient(conn)

	signers, err := agentClient.Signers()
	if err != nil {
		fmt.Fprintf(os.Stderr, "forged-sign: getting signers: %v\n", err)
		os.Exit(1)
	}

	if len(signers) == 0 {
		fmt.Fprintln(os.Stderr, "forged-sign: no keys available in agent")
		os.Exit(1)
	}

	// Find the signing key (first one, or the one matching git's requested key)
	signer := signers[0]

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "forged-sign: reading stdin: %v\n", err)
		os.Exit(1)
	}

	sig, err := signer.Sign(nil, data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "forged-sign: signing: %v\n", err)
		os.Exit(1)
	}

	// Output in SSH signature format
	serialized := ssh.Marshal(sig)
	os.Stdout.Write(serialized)
}

func defaultSocketPath() string {
	home, _ := os.UserHomeDir()
	return home + "/.forged/agent.sock"
}
