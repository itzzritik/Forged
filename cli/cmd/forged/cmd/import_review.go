package cmd

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

type importSection string

const (
	importSectionNew        importSection = "new"
	importSectionDuplicates importSection = "duplicates"
	importSectionUpgrades   importSection = "upgrades"
	importSectionAll        importSection = "all"
)

type importSummary struct {
	AlreadyInVault      int
	CollapsedDuplicates int
	Found               int
	ReadyToImport       int
	Upgrades            int
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
			summary.ReadyToImport++
		}
		if preview.alreadyInVault {
			summary.AlreadyInVault++
		}
		if preview.converted {
			summary.Upgrades++
		}
		summary.CollapsedDuplicates += preview.collapsedDuplicates
	}
	return summary
}

func (s *importReviewState) selectedCount() int {
	return s.summary().ReadyToImport
}

func (s *importReviewState) actionLabel() string {
	count := s.selectedCount()
	switch {
	case count == 0:
		return "No keys selected"
	case count == len(s.previews):
		return "Import All Keys"
	case count == 1:
		return "Import 1 Key"
	default:
		return fmt.Sprintf("Import %d Keys", count)
	}
}

func (s *importReviewState) indexesFor(section importSection) []int {
	indexes := make([]int, 0, len(s.previews))
	for i, preview := range s.previews {
		switch section {
		case importSectionNew:
			if !preview.alreadyInVault {
				indexes = append(indexes, i)
			}
		case importSectionDuplicates:
			if preview.alreadyInVault {
				indexes = append(indexes, i)
			}
		case importSectionUpgrades:
			if preview.converted {
				indexes = append(indexes, i)
			}
		case importSectionAll:
			indexes = append(indexes, i)
		}
	}
	return indexes
}

func compactFingerprint(fingerprint string) string {
	if len(fingerprint) <= 24 {
		return fingerprint
	}
	return fingerprint[:16] + "..." + fingerprint[len(fingerprint)-10:]
}

func compactTags(preview importPreview) string {
	tags := make([]string, 0, 2)
	if preview.alreadyInVault {
		tags = append(tags, "[dup]")
	}
	if preview.converted {
		tags = append(tags, "[upgrade]")
	}
	return strings.Join(tags, " ")
}

func printImportSummary(state *importReviewState) {
	summary := state.summary()

	fmt.Println()
	fmt.Printf("  %s\n", state.sourceLabel)
	fmt.Println()
	fmt.Printf("  %d keys found\n", summary.Found)
	if summary.ReadyToImport > 0 && summary.ReadyToImport < summary.Found {
		fmt.Printf("  %d ready to import\n", summary.ReadyToImport)
	}
	if summary.AlreadyInVault > 0 {
		fmt.Printf("  %d already in vault\n", summary.AlreadyInVault)
	}
	if summary.Upgrades > 0 {
		fmt.Printf("  %d will upgrade to OpenSSH\n", summary.Upgrades)
	}
	if summary.CollapsedDuplicates > 0 {
		fmt.Printf("  %d duplicate entries were consolidated\n", summary.CollapsedDuplicates)
	}
	fmt.Println()

	if summary.ReadyToImport == 0 {
		fmt.Println("  [r] Review   [a] Import All   [q] Cancel")
		return
	}

	fmt.Printf("  [Enter] %s   [r] Review   [a] Import All   [q] Cancel\n", state.actionLabel())
}

type reviewMenuItem struct {
	key     string
	label   string
	section importSection
}

func reviewMenu(state *importReviewState) []reviewMenuItem {
	items := make([]reviewMenuItem, 0, 4)
	if count := len(state.indexesFor(importSectionNew)); count > 0 {
		items = append(items, reviewMenuItem{key: "n", label: fmt.Sprintf("New keys (%d)", count), section: importSectionNew})
	}
	if count := len(state.indexesFor(importSectionDuplicates)); count > 0 {
		items = append(items, reviewMenuItem{key: "d", label: fmt.Sprintf("Duplicates (%d)", count), section: importSectionDuplicates})
	}
	if count := len(state.indexesFor(importSectionUpgrades)); count > 0 {
		items = append(items, reviewMenuItem{key: "u", label: fmt.Sprintf("Upgrades (%d)", count), section: importSectionUpgrades})
	}
	items = append(items, reviewMenuItem{key: "l", label: fmt.Sprintf("Full list (%d)", len(state.previews)), section: importSectionAll})
	return items
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
				fmt.Println("  No keys selected.")
				continue
			}
			return doImport(selected)
		case "r":
			if err := runGroupedReview(reader, state); err != nil {
				return err
			}
		case "a":
			return doImport(selectedImportKeys(state.previews, true))
		case "q":
			fmt.Println("  Aborted.")
			return nil
		default:
			fmt.Println("  Invalid action.")
		}
	}
}

func runGroupedReview(reader *bufio.Reader, state *importReviewState) error {
	for {
		fmt.Println()
		fmt.Println("  Review import")
		fmt.Println()
		items := reviewMenu(state)
		for _, item := range items {
			fmt.Printf("  [%s] %s\n", item.key, item.label)
		}
		fmt.Println("  [b] Back")
		fmt.Println()
		fmt.Print("  Action: ")

		line, _ := reader.ReadString('\n')
		action := strings.TrimSpace(strings.ToLower(line))
		if action == "b" || action == "" {
			return nil
		}

		matched := false
		for _, item := range items {
			if action != item.key {
				continue
			}
			matched = true
			if err := runSectionReview(reader, state, item.section, item.label); err != nil {
				return err
			}
			break
		}
		if !matched {
			fmt.Println("  Invalid action.")
		}
	}
}

func printSectionRows(state *importReviewState, section importSection, label string) []int {
	indexes := state.indexesFor(section)
	fmt.Println()
	fmt.Printf("  %s\n", label)
	fmt.Println()
	for displayIndex, previewIndex := range indexes {
		preview := state.previews[previewIndex]
		marker := " "
		if preview.selected {
			marker = "x"
		}
		line := fmt.Sprintf("    %d. [%s] %-22s %-29s", displayIndex+1, marker, preview.key.Name, compactFingerprint(preview.fingerprint))
		if tags := compactTags(preview); tags != "" {
			line += " " + tags
		}
		fmt.Println(line)
	}
	return indexes
}

func runSectionReview(reader *bufio.Reader, state *importReviewState, section importSection, label string) error {
	for {
		indexes := printSectionRows(state, section, label)
		fmt.Println()
		fmt.Println("  [Enter] Done   [a] Select all shown   [u] Unselect all shown   [b] Back")
		fmt.Print("  Toggle: ")

		line, _ := reader.ReadString('\n')
		input := strings.TrimSpace(strings.ToLower(line))
		switch input {
		case "", "b":
			return nil
		case "a":
			for _, idx := range indexes {
				state.previews[idx].selected = true
			}
		case "u":
			for _, idx := range indexes {
				state.previews[idx].selected = false
			}
		default:
			if err := toggleSectionSelection(state.previews, indexes, input); err != nil {
				fmt.Printf("  %v\n", err)
			}
		}
	}
}

func toggleSectionSelection(previews []importPreview, visible []int, input string) error {
	parts := strings.Split(input, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		index, err := strconv.Atoi(part)
		if err != nil || index < 1 || index > len(visible) {
			return fmt.Errorf("invalid selection %q", part)
		}
		previewIndex := visible[index-1]
		previews[previewIndex].selected = !previews[previewIndex].selected
	}
	return nil
}
