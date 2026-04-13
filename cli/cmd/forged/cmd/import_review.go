package cmd

import (
	"bufio"
	"fmt"
	"strings"
)

type importSummary struct {
	Duplicates int
	Found      int
	NewKeys    int
	Upgrades   int
}

type importReviewState struct {
	sourceLabel string
	previews    []importPreview
}

func newImportReviewState(sourceLabel string, previews []importPreview) *importReviewState {
	return &importReviewState{sourceLabel: sourceLabel, previews: previews}
}

func (s *importReviewState) summary() importSummary {
	summary := importSummary{Found: len(s.previews)}
	for _, preview := range s.previews {
		if preview.selected {
			summary.NewKeys++
		}
		if preview.alreadyInVault {
			summary.Duplicates++
		}
		if preview.converted {
			summary.Upgrades++
		}
	}
	return summary
}

func mutedLine(text string) string {
	if !terminalIsInteractive() {
		return text
	}
	return "\033[2m" + text + "\033[0m"
}

func newKeysLine(count int) string {
	if count == 1 {
		return "Importing 1 new key"
	}
	return fmt.Sprintf("Importing %d new keys", count)
}

func duplicatesLine(count int) string {
	if count == 1 {
		return "Skipping 1 duplicate"
	}
	return fmt.Sprintf("Skipping %d duplicates", count)
}

func upgradesLine(count int) string {
	if count == 1 {
		return "Upgrading 1 key to OpenSSH"
	}
	return fmt.Sprintf("Upgrading %d keys to OpenSSH", count)
}

func printImportSummary(state *importReviewState) {
	summary := state.summary()

	fmt.Printf("  %s\n", state.sourceLabel)
	fmt.Printf("  %s\n", mutedLine(fmt.Sprintf("%d keys found", summary.Found)))
	fmt.Println()

	if summary.NewKeys > 0 {
		fmt.Printf("  %s\n", newKeysLine(summary.NewKeys))
	} else {
		fmt.Println("  No new keys to import")
	}
	if summary.Duplicates > 0 {
		fmt.Printf("  %s\n", duplicatesLine(summary.Duplicates))
	}
	if summary.Upgrades > 0 {
		fmt.Printf("  %s\n", upgradesLine(summary.Upgrades))
	}
	fmt.Println()

	switch {
	case summary.NewKeys == 0:
		fmt.Println("  [a] Import all   [q] Cancel")
	case summary.NewKeys == summary.Found:
		fmt.Println("  [Enter] Import All Keys   [q] Cancel")
	case summary.NewKeys == 1:
		fmt.Println("  [Enter] Import 1 New Key   [a] Import all   [q] Cancel")
	default:
		fmt.Printf("  [Enter] Import %d New Keys   [a] Import all   [q] Cancel\n", summary.NewKeys)
	}
}

func runImportReview(reader *bufio.Reader, state *importReviewState) error {
	for {
		printImportSummary(state)
		fmt.Print("\n  Action: ")

		line, _ := reader.ReadString('\n')
		action := strings.TrimSpace(strings.ToLower(line))

		switch action {
		case "":
			selected := selectedImportKeys(state.previews, false)
			if len(selected) == 0 {
				fmt.Println("  No new keys selected. Use 'a' to import all or 'q' to cancel.")
				continue
			}
			printStepSeparator()
			return doImport(selected)
		case "a":
			printStepSeparator()
			return doImport(selectedImportKeys(state.previews, true))
		case "q":
			printStepSeparator()
			fmt.Println("  Aborted.")
			return nil
		default:
			fmt.Println("  Invalid action.")
		}
	}
}
