package launcher

import "github.com/itzzritik/forged/cli/internal/readiness"

type ActionID string

const (
	ActionSetupVault ActionID = "setup-vault"
	ActionGenerate   ActionID = "generate"
	ActionImport     ActionID = "import"
	ActionSigning    ActionID = "signing"
	ActionLogin      ActionID = "login"
)

type MenuItem struct {
	ID    ActionID
	Label string
}

func BuildMenu(snapshot readiness.Snapshot) []MenuItem {
	if snapshot.State == readiness.StateUninitialized {
		return []MenuItem{
			{ID: ActionSetupVault, Label: "Set up a new vault"},
			{ID: ActionLogin, Label: "Sign in to an existing Forged account"},
		}
	}

	generateLabel := "Generate your first key"
	if snapshot.KeyCount > 0 {
		generateLabel = "Generate a new key"
	}

	items := []MenuItem{
		{ID: ActionGenerate, Label: generateLabel},
		{ID: ActionImport, Label: "Import keys"},
		{ID: ActionSigning, Label: "Use Forged for Git signing"},
	}

	if snapshot.LoggedIn {
		return items
	}
	if snapshot.KeyCount == 0 {
		return append(items, MenuItem{ID: ActionLogin, Label: "Sign in to sync across devices"})
	}

	return append([]MenuItem{{ID: ActionLogin, Label: "Sign in to sync across devices"}}, items...)
}
