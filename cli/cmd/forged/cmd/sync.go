package cmd

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/daemon"
	"github.com/itzzritik/forged/cli/internal/ipc"
	forgedsync "github.com/itzzritik/forged/cli/internal/sync"
	"github.com/spf13/cobra"
)

type syncCredentials struct {
	ServerURL string `json:"server_url"`
	Token     string `json:"token"`
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
}

func credentialsPath() string {
	return filepath.Join(config.DefaultPaths().ConfigDir, "credentials.json")
}

func loadCredentials() (syncCredentials, error) {
	data, err := os.ReadFile(credentialsPath())
	if err != nil {
		return syncCredentials{}, fmt.Errorf("not logged in. Run: forged login")
	}
	var creds syncCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return syncCredentials{}, fmt.Errorf("corrupted credentials file")
	}
	return creds, nil
}

func saveCredentials(creds syncCredentials) error {
	path := credentialsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(creds, "", "  ")
	return os.WriteFile(path, data, 0600)
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with cloud server",
	RunE: func(cmd *cobra.Command, args []string) error {
		server, _ := cmd.Flags().GetString("server")

		result, err := oauthLogin(server)
		if err != nil {
			return err
		}

		creds := syncCredentials{
			ServerURL: server,
			Token:     result.Token,
			UserID:    result.UserID,
			Email:     result.Email,
		}
		if err := saveCredentials(creds); err != nil {
			return fmt.Errorf("saving credentials: %w", err)
		}

		if _, running := daemon.IsRunning(config.DefaultPaths()); running {
			if _, err := ctlClient().Call(ipc.CmdSyncLink, ipc.SyncLinkArgs{
				ServerURL: creds.ServerURL,
				Token:     creds.Token,
				UserID:    creds.UserID,
			}); err != nil {
				return fmt.Errorf("linking running daemon: %w", err)
			}
		}

		fmt.Printf("Logged in as %s\n", result.Email)
		return nil
	},
}

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Create cloud account (same as login)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return loginCmd.RunE(cmd, args)
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear cloud credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		os.Remove(credentialsPath())
		fmt.Println("Logged out")
		return nil
	},
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync keys with cloud",
	RunE: func(cmd *cobra.Command, args []string) error {
		creds, err := loadCredentials()
		if err != nil {
			return err
		}

		fmt.Println("Syncing vault...")
		resp, err := ctlClient().Call(ipc.CmdSyncTrigger, map[string]string{
			"server_url": creds.ServerURL,
			"token":      creds.Token,
		})
		if err != nil {
			return err
		}

		if jsonOutput {
			return printOutput(json.RawMessage(resp.Data))
		}

		var result struct {
			Version int64 `json:"version"`
		}
		if err := json.Unmarshal(resp.Data, &result); err != nil {
			return fmt.Errorf("parsing sync response: %w", err)
		}
		fmt.Printf("Sync complete (version %d)\n", result.Version)
		return nil
	},
}

type oauthResult struct {
	Token  string `json:"token"`
	UserID string `json:"user_id"`
	Email  string `json:"email"`
}

func oauthLogin(server string) (oauthResult, error) {
	code, err := randomHex(16)
	if err != nil {
		return oauthResult{}, fmt.Errorf("generating code: %w", err)
	}
	verification, err := randomHex(2)
	if err != nil {
		return oauthResult{}, fmt.Errorf("generating verification: %w", err)
	}
	verificationDisplay := fmt.Sprintf("FORGE-%s", strings.ToUpper(verification))

	body, _ := json.Marshal(map[string]string{"code": code, "verification": verification})
	resp, err := http.Post(server+"/api/v1/auth/sessions", "application/json", bytes.NewReader(body))
	if err != nil {
		return oauthResult{}, fmt.Errorf("could not reach server: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return oauthResult{}, fmt.Errorf("could not create auth session (status %d)", resp.StatusCode)
	}

	authURL := ipc.DefaultWebApp + "/login?code=" + code

	fmt.Println("Opening browser to login...")
	fmt.Printf("Verification code: %s\n", verificationDisplay)
	openBrowser(authURL)
	fmt.Printf("If browser didn't open, visit:\n  %s\n\n", authURL)
	fmt.Println("Waiting for authentication...")

	pollURL := server + "/api/v1/auth/sessions/" + code
	deadline := time.Now().Add(5 * time.Minute)
	interval := 2 * time.Second

	for time.Now().Before(deadline) {
		time.Sleep(interval)

		resp, err := http.Get(pollURL)
		if err != nil {
			interval = min(interval*2, 10*time.Second)
			continue
		}

		var result struct {
			Status string `json:"status"`
			Token  string `json:"token"`
			UserID string `json:"user_id"`
			Email  string `json:"email"`
			Error  string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()

		interval = 2 * time.Second

		switch result.Status {
		case "complete":
			return oauthResult{Token: result.Token, UserID: result.UserID, Email: result.Email}, nil
		case "error":
			return oauthResult{}, fmt.Errorf("authentication failed: %s", result.Error)
		case "pending":
			continue
		}
	}

	return oauthResult{}, fmt.Errorf("login timed out after 5 minutes")
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return
	}
	if err := cmd.Start(); err == nil {
		go cmd.Wait()
	}
}

func init() {
	defaultServer := ipc.DefaultAPIServer

	loginCmd.Flags().String("server", defaultServer, "sync server URL")
	registerCmd.Flags().String("server", defaultServer, "sync server URL")

	syncCmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show sync status",
		RunE: func(cmd *cobra.Command, args []string) error {
			creds, err := loadCredentials()
			if err != nil {
				return err
			}

			client := forgedsync.NewClient(creds.ServerURL, creds.Token, "")
			status, err := client.Status()
			if err != nil {
				return err
			}

			if jsonOutput {
				return printOutput(status)
			}

			if !status.HasVault {
				fmt.Println("Sync: no vault on server")
				return nil
			}
			fmt.Printf("Sync: version %d, last update %s\n", status.Version, status.UpdatedAt)
			return nil
		},
	})
}
