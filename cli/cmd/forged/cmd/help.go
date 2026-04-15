package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

type helpEntry struct {
	Name        string
	Description string
}

type helpSection struct {
	Title   string
	Entries []helpEntry
}

func configureHelp(root *cobra.Command) {
	root.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		renderRootHelp(cmd.OutOrStdout())
	})
}

func configureGroupHelp(cmd *cobra.Command, purpose string, examples []string) {
	cmd.SetHelpFunc(func(current *cobra.Command, _ []string) {
		renderGroupHelp(current.OutOrStdout(), current, purpose, examples)
	})
}

func renderRootHelp(w io.Writer) {
	sections := []helpSection{
		{
			Title: "Get Started",
			Entries: []helpEntry{
				{Name: "forged", Description: "open Forged"},
				{Name: "import", Description: "bring keys into Forged"},
				{Name: "export", Description: "export your vault"},
			},
		},
		{
			Title: "Account",
			Entries: []helpEntry{
				{Name: "login", Description: "sign in or create an account"},
				{Name: "logout", Description: "remove account access from this machine"},
				{Name: "sync", Description: "refresh with the cloud"},
			},
		},
		{Title: "Keys", Entries: []helpEntry{{Name: "key", Description: "manage keys"}}},
		{Title: "Vault", Entries: []helpEntry{{Name: "vault", Description: "manage vault access"}}},
		{Title: "Agent", Entries: []helpEntry{{Name: "agent", Description: "use Forged for SSH and Git signing"}}},
		{Title: "Recovery", Entries: []helpEntry{{Name: "doctor", Description: "repair this machine"}}},
	}

	fmt.Fprintln(w, "Forged manages SSH keys, vault access, and Git signing.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  forged [flags]")
	fmt.Fprintln(w, "  forged [command]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Run forged with no arguments to open the interactive launcher.")
	fmt.Fprintln(w)
	writeHelpSections(w, sections)

	fmt.Fprintln(w, "Examples:")
	for _, example := range []string{
		"  forged",
		"  forged import",
		"  forged key",
		"  forged key generate",
		"  forged vault change-password",
		"  forged agent signing",
		"  forged doctor --fix",
	} {
		fmt.Fprintln(w, example)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  -h, --help      help for forged")
	fmt.Fprintln(w, "  -v, --version   print version information")
}

func renderGroupHelp(w io.Writer, cmd *cobra.Command, purpose string, examples []string) {
	fmt.Fprintf(w, "%s\n\n", purpose)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintf(w, "  %s [command]\n", cmd.CommandPath())
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Run without a subcommand to open the interactive manager.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	for _, child := range cmd.Commands() {
		if child.Hidden {
			continue
		}
		fmt.Fprintf(w, "  %-20s %s\n", child.Use, child.Short)
	}

	if len(examples) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Examples:")
		for _, example := range examples {
			fmt.Fprintf(w, "  %s\n", example)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "Flags:\n  -h, --help   help for %s\n", strings.ReplaceAll(cmd.CommandPath(), "forged ", ""))
}

func writeHelpSections(w io.Writer, sections []helpSection) {
	for _, section := range sections {
		fmt.Fprintf(w, "%s:\n", section.Title)
		for _, entry := range section.Entries {
			fmt.Fprintf(w, "  %-24s %s\n", entry.Name, entry.Description)
		}
		fmt.Fprintln(w)
	}
}
