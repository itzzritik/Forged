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

	authURL := fmt.Sprintf(ipc.DefaultWebApp + "/login?callback=http://localhost:%d/callback", port)

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
	accent := "#ea580c"
	accentGlow := "rgba(234,88,12,0.15)"
	dotColor := "#10b981"
	icon := `<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="white" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>`
	headerLabel := "Session // Authenticated"
	headerRight := "CLI"
	note := "You can close this tab and return to your terminal."

	if isError {
		accent = "#ef4444"
		accentGlow = "rgba(239,68,68,0.15)"
		dotColor = "#ef4444"
		icon = `<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="white" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>`
		headerLabel = "Session // Error"
		headerRight = "Failed"
		note = `Try running <span style="background:#000;border:1px solid #27272a;padding:3px 10px;font-size:11px;color:#ea580c">forged login</span> again.`
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>%s - Forged</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{background:#000;color:#e4e4e7;font-family:ui-monospace,SFMono-Regular,Menlo,monospace;min-height:100vh;display:flex;flex-direction:column;align-items:center;justify-content:center;padding:24px}
.wrap{width:100%%;max-width:440px}
.glow{position:relative;display:flex;justify-content:center;margin-bottom:32px}
.glow::before{content:'';position:absolute;inset:0;background:%s;filter:blur(24px);transform:scale(1.5)}
.icon-box{position:relative;width:56px;height:56px;background:#000;border:1px solid #27272a;display:flex;align-items:center;justify-content:center}
h1{font-size:1.75rem;font-weight:700;letter-spacing:-0.03em;text-align:center;margin-bottom:8px}
.sub{text-align:center;font-size:13px;color:#a1a1aa;letter-spacing:0.04em;margin-bottom:32px}
.card{border:1px solid #27272a;background:#050505;overflow:hidden}
.card-header{border-bottom:1px solid #27272a;background:#030303;padding:0 24px;height:40px;display:flex;align-items:center;justify-content:space-between}
.card-header .left{display:flex;align-items:center;gap:12px}
.card-header .dot{width:6px;height:6px;border-radius:50%%;background:%s;box-shadow:0 0 8px %s;animation:pulse 2s infinite}
.card-header .label{font-size:10px;letter-spacing:0.15em;color:#a1a1aa;text-transform:uppercase}
.card-header .right{font-size:9px;letter-spacing:0.15em;color:#3f3f46;text-transform:uppercase}
@keyframes pulse{0%%,100%%{opacity:1}50%%{opacity:0.4}}
.card-body{padding:32px 24px;text-align:center}
.email{color:%s;font-size:14px;font-weight:600;margin-bottom:24px}
.sep{display:flex;align-items:center;gap:16px;margin-bottom:24px}
.sep .line{flex:1;height:1px;background:#27272a}
.sep .text{font-size:9px;color:#3f3f46;text-transform:uppercase;letter-spacing:0.15em}
.note{color:#3f3f46;font-size:11px;line-height:1.8}
.badges{display:flex;align-items:center;justify-content:center;gap:24px;margin-top:32px}
.badges span{font-size:9px;letter-spacing:0.15em;color:#27272a;text-transform:uppercase}
.badges .dot{width:4px;height:4px;background:#27272a}
</style></head>
<body>
<div class="wrap">
<div class="glow"><div class="icon-box">%s</div></div>
<h1>%s</h1>
<p class="sub">%s</p>
<div class="card">
<div class="card-header">
<div class="left"><span class="dot"></span><span class="label">%s</span></div>
<span class="right">%s</span>
</div>
<div class="card-body">
<p class="email">%s</p>
<div class="sep"><div class="line"></div><span class="text">info</span><div class="line"></div></div>
<p class="note">%s</p>
</div>
</div>
<div class="badges">
<span>E2E Encrypted</span><span class="dot"></span>
<span>Zero Knowledge</span><span class="dot"></span>
<span>Open Source</span>
</div>
</div>
</body></html>`,
		title, accentGlow, dotColor, dotColor, accent, icon, title, detail, headerLabel, headerRight, detail, note)
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
