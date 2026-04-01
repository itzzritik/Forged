package cmd

import "github.com/spf13/cobra"

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync keys with cloud",
	RunE:  notImplemented("sync"),
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with cloud server",
	RunE:  notImplemented("login"),
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear cloud credentials",
	RunE:  notImplemented("logout"),
}

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Create cloud account",
	RunE:  notImplemented("register"),
}
