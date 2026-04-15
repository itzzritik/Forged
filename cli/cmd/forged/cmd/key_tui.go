package cmd

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/itzzritik/forged/cli/internal/ipc"
	"github.com/spf13/cobra"
)

func shouldLaunchKeyManager(args []string) bool {
	return len(args) == 0 && !jsonOutput && isInteractiveTerminal()
}

func runKeyManager(cmd *cobra.Command) error {
	items := keyManagerItems()
	for {
		selected, cancelled, err := runManagerSelectionProgram("Keys", items)
		if err != nil {
			return err
		}
		if cancelled || selected < 0 || selected >= len(items) {
			return nil
		}

		err = items[selected].Run()
		if errors.Is(err, errManagerContinue) {
			continue
		}
		return err
	}
}

func keyManagerItems() []managerItem {
	return []managerItem{
		{
			Label: "Generate a new key",
			Run: func() error {
				return generateCmd.RunE(generateCmd, nil)
			},
		},
		{
			Label: "List keys",
			Run: func() error {
				return listCmd.RunE(listCmd, nil)
			},
		},
		{
			Label: "View a key",
			Run:   runInteractiveKeyView,
		},
		{
			Label: "Edit a key name",
			Run:   runInteractiveKeyRename,
		},
		{
			Label: "Delete a key",
			Run:   runInteractiveKeyDelete,
		},
		{
			Label: "Import keys",
			Run:   runImportFromKeyManager,
		},
		{
			Label: "Export keys",
			Run: func() error {
				return exportCmd.RunE(exportCmd, nil)
			},
		},
	}
}

func runImportFromKeyManager() error {
	if err := runImportTUI(importCmd, "", "", true); err != nil {
		return err
	}
	return errManagerContinue
}

type keySelection struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Fingerprint string `json:"fingerprint"`
}

func runInteractiveKeyView() error {
	selected, ok, err := selectKeyFromVault("View Key")
	if err != nil || !ok {
		return err
	}
	return viewKey(selected.Name, false)
}

func runInteractiveKeyRename() error {
	selected, ok, err := selectKeyFromVault("Rename Key")
	if err != nil || !ok {
		return err
	}

	newName, ok, err := runTextPromptProgram(
		"Rename Key",
		fmt.Sprintf("Enter a new name for %s.", selected.Name),
		"New name",
		selected.Name,
	)
	if err != nil || !ok {
		return err
	}

	return renameCmd.RunE(renameCmd, []string{selected.Name, newName})
}

func runInteractiveKeyDelete() error {
	selected, ok, err := selectKeyFromVault("Delete Key")
	if err != nil || !ok {
		return err
	}

	confirmed, err := runConfirmProgram(
		"Delete Key",
		fmt.Sprintf("Delete %s? This cannot be undone.", selected.Name),
		fmt.Sprintf("Delete %s", selected.Name),
	)
	if err != nil || !confirmed {
		return err
	}

	return removeCmd.RunE(removeCmd, []string{selected.Name})
}

func selectKeyFromVault(title string) (keySelection, bool, error) {
	keys, err := listKeySelections()
	if err != nil {
		return keySelection{}, false, err
	}
	if len(keys) == 0 {
		fmt.Println("No keys in vault")
		return keySelection{}, false, nil
	}

	var selected keySelection
	items := make([]managerItem, 0, len(keys))
	for _, key := range keys {
		key := key
		items = append(items, managerItem{
			Label: formatKeySelectionLabel(key),
			Run: func() error {
				selected = key
				return nil
			},
		})
	}

	if err := runManagerProgram(title, items); err != nil {
		return keySelection{}, false, err
	}
	if selected.Name == "" {
		return keySelection{}, false, nil
	}
	return selected, true, nil
}

func listKeySelections() ([]keySelection, error) {
	resp, err := ctlClient().Call(ipc.CmdList, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Keys []keySelection `json:"keys"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return result.Keys, nil
}

func formatKeySelectionLabel(key keySelection) string {
	return fmt.Sprintf("%s  %s  %s", key.Name, key.Type, key.Fingerprint)
}
