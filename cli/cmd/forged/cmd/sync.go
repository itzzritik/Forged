package cmd

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/itzzritik/forged/cli/internal/config"
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

		resp, ipcErr := ctlClient().Call("sync-trigger", map[string]string{
			"server_url": creds.ServerURL,
			"token":      creds.Token,
		})
		if ipcErr != nil {
			return ipcErr
		}

		if jsonOutput {
			return printOutput(json.RawMessage(resp.Data))
		}

		fmt.Println("Sync complete")
		return nil
	},
}

type oauthResult struct {
	Token  string `json:"token"`
	UserID string `json:"user_id"`
	Email  string `json:"email"`
}

func oauthLogin(server string) (oauthResult, error) {
	resultCh := make(chan oauthResult, 1)
	errCh := make(chan error, 1)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return oauthResult{}, fmt.Errorf("starting local server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		userID := r.URL.Query().Get("user_id")
		email := r.URL.Query().Get("email")
		errMsg := r.URL.Query().Get("error")

		if errMsg != "" {
			w.Write([]byte("Authentication failed: " + errMsg + "\nYou can close this tab."))
			errCh <- fmt.Errorf("auth failed: %s", errMsg)
			return
		}

		w.Write([]byte("Logged in as " + email + "\nYou can close this tab."))
		resultCh <- oauthResult{Token: token, UserID: userID, Email: email}
	})

	srv := &http.Server{Handler: mux}
	go srv.Serve(listener)
	defer srv.Close()

	authURL := fmt.Sprintf("https://forged.ritik.me/login?callback=http://localhost:%d/callback", port)

	fmt.Println("Opening browser to login...")
	openBrowser(authURL)
	fmt.Printf("If browser didn't open, visit:\n  %s\n\n", authURL)
	fmt.Println("Waiting for authentication...")

	select {
	case result := <-resultCh:
		return result, nil
	case err := <-errCh:
		return oauthResult{}, err
	}
}

func openBrowser(url string) {
	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", url).Start()
	case "linux":
		exec.Command("xdg-open", url).Start()
	case "windows":
		exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	}
}

func init() {
	defaultServer := "https://forged-api.ritik.me"

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
