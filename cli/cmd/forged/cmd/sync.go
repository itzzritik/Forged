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
	"time"

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

		w.Header().Set("Content-Type", "text/html")

		if errMsg != "" {
			fmt.Fprintf(w, callbackPage("Authentication Failed", errMsg, true))
			errCh <- fmt.Errorf("auth failed: %s", errMsg)
			return
		}

		fmt.Fprintf(w, callbackPage("Welcome to Forged", email, false))
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
	case <-time.After(5 * time.Minute):
		return oauthResult{}, fmt.Errorf("login timed out after 5 minutes")
	}
}

func callbackPage(title, detail string, isError bool) string {
	icon := `<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="#f59e0b" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>`
	color := "#f59e0b"
	subtitle := "Signed in as"
	note := "You can close this tab and return to your terminal."

	if isError {
		icon = `<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="#ef4444" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>`
		color = "#ef4444"
		subtitle = "Error"
		note = "Try running <code>forged login</code> again."
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>%s - Forged</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{background:#08080a;color:#e4e4e7;font-family:system-ui,-apple-system,sans-serif;min-height:100vh;display:flex;align-items:center;justify-content:center}
.card{text-align:center;max-width:400px;padding:48px 32px}
.icon{margin-bottom:24px}
h1{font-size:1.5rem;font-weight:500;margin-bottom:8px;letter-spacing:-0.02em}
.detail{color:%s;font-size:0.95rem;margin-bottom:8px}
.subtitle{color:#71717a;font-size:0.8rem;margin-bottom:4px}
.note{color:#52525b;font-size:0.8rem;margin-top:24px;line-height:1.5}
code{background:#18181b;padding:2px 6px;border-radius:4px;font-size:0.75rem;color:#a1a1aa}
.brand{color:#71717a;font-size:0.75rem;margin-top:32px;font-family:ui-monospace,monospace;letter-spacing:0.05em}
</style></head>
<body><div class="card">
<div class="icon">%s</div>
<h1>%s</h1>
<p class="subtitle">%s</p>
<p class="detail">%s</p>
<p class="note">%s</p>
<p class="brand">forged</p>
</div></body></html>`, title, color, icon, title, subtitle, detail, note)
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
