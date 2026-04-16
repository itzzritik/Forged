package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/ipc"
	forgedsync "github.com/itzzritik/forged/cli/internal/sync"
	"github.com/itzzritik/forged/cli/internal/tui"
	"github.com/spf13/cobra"
)

type syncCredentials = actions.AccountCredentials

func credentialsPath() string {
	return actions.CredentialsPath(config.DefaultPaths())
}

func loadCredentials() (syncCredentials, error) {
	return actions.LoadCredentials(config.DefaultPaths())
}

func saveCredentials(creds syncCredentials) error {
	return actions.SaveCredentials(config.DefaultPaths(), creds)
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with cloud server",
	RunE: func(cmd *cobra.Command, args []string) error {
		server, _ := cmd.Flags().GetString("server")

		if !jsonOutput && isInteractiveTerminal() {
			intent := tui.ResolveCommand([]string{"login"}, args).WithParam("server", server)
			return runInteractiveIntent(intent)
		}

		session, err := actions.BeginLogin(server, actions.OpenBrowser)
		if err != nil {
			return err
		}

		fmt.Println("Opening browser to login...")
		fmt.Printf("Verification code: %s\n", session.VerificationCode)
		fmt.Printf("Login URL:\n  %s\n\n", session.URL)
		fmt.Println("Waiting for authentication...")

		creds, err := session.Wait(cmd.Context())
		if err != nil {
			return err
		}
		if err := saveCredentials(creds); err != nil {
			return fmt.Errorf("saving credentials: %w", err)
		}

		fmt.Printf("Logged in as %s\n", creds.Email)
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
		if err := actions.ClearCredentials(config.DefaultPaths()); err != nil {
			return err
		}
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
