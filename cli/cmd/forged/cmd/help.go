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
		if cmd != root {
			renderCommandHelp(cmd.OutOrStdout(), cmd)
			return
		}
		renderRootHelp(cmd.OutOrStdout())
	})
}

func renderRootHelp(w io.Writer) {
	sections := []helpSection{
		{
			Title: "Get Started",
			Entries: []helpEntry{
				{Name: "forged", Description: "open Forged"},
			},
		},
		{
			Title: "Info",
			Entries: []helpEntry{
				{Name: "help", Description: "show help"},
				{Name: "version", Description: "print version information"},
			},
		},
	}

	fmt.Fprintln(w, "Forged manages SSH keys, vault access, and Git signing.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  forged [flags]")
	fmt.Fprintln(w, "  forged [command]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Run forged with no arguments to open the interactive shell.")
	fmt.Fprintln(w)
	writeHelpSections(w, sections)

	fmt.Fprintln(w, "Examples:")
	for _, example := range []string{
		"  forged",
		"  forged help",
		"  forged version",
	} {
		fmt.Fprintln(w, example)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  -h, --help      help for forged")
	fmt.Fprintln(w, "  -v, --version   print version information")
}

func renderCommandHelp(w io.Writer, cmd *cobra.Command) {
	description := cmd.Long
	if description == "" {
		description = cmd.Short
	}
	if description != "" {
		fmt.Fprintln(w, description)
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "Usage:")
	fmt.Fprintf(w, "  %s\n", cmd.UseLine())

	if cmd.Example != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Examples:")
		for _, line := range strings.Split(strings.TrimSpace(cmd.Example), "\n") {
			fmt.Fprintf(w, "  %s\n", strings.TrimSpace(line))
		}
	}

	flagUsages := strings.TrimSpace(cmd.Flags().FlagUsagesWrapped(80))
	if flagUsages != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Flags:")
		fmt.Fprintln(w, indentFlagUsages(flagUsages))
	}
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

func indentFlagUsages(usages string) string {
	lines := strings.Split(usages, "\n")
	for i, line := range lines {
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n")
}
