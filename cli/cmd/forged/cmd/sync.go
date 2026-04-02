package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Create cloud account",
	RunE: func(cmd *cobra.Command, args []string) error {
		server, _ := cmd.Flags().GetString("server")
		email, _ := cmd.Flags().GetString("email")
		password, _ := cmd.Flags().GetString("password")

		if email == "" || password == "" {
			return fmt.Errorf("--email and --password are required")
		}

		result, err := forgedsync.Register(server, email, password)
		if err != nil {
			return err
		}

		creds := syncCredentials{
			ServerURL: server,
			Token:     result.Token,
			UserID:    result.UserID,
			Email:     email,
		}
		if err := saveCredentials(creds); err != nil {
			return fmt.Errorf("saving credentials: %w", err)
		}

		fmt.Printf("Registered as %s\n", email)
		return nil
	},
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with cloud server",
	RunE: func(cmd *cobra.Command, args []string) error {
		server, _ := cmd.Flags().GetString("server")
		email, _ := cmd.Flags().GetString("email")
		password, _ := cmd.Flags().GetString("password")

		if email == "" || password == "" {
			return fmt.Errorf("--email and --password are required")
		}

		result, err := forgedsync.Login(server, email, password)
		if err != nil {
			return err
		}

		creds := syncCredentials{
			ServerURL: server,
			Token:     result.Token,
			UserID:    result.UserID,
			Email:     email,
		}
		if err := saveCredentials(creds); err != nil {
			return fmt.Errorf("saving credentials: %w", err)
		}

		fmt.Printf("Logged in as %s\n", email)
		return nil
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

func init() {
	defaultServer := "https://forged-api.ritik.me"

	registerCmd.Flags().String("server", defaultServer, "sync server URL")
	registerCmd.Flags().String("email", "", "account email")
	registerCmd.Flags().String("password", "", "account password")

	loginCmd.Flags().String("server", defaultServer, "sync server URL")
	loginCmd.Flags().String("email", "", "account email")
	loginCmd.Flags().String("password", "", "account password")

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
