package vault

import "github.com/itzzritik/forged/cli/internal/keytypes"

func normalizeVaultKeyTypes(data *VaultData) {
	if data == nil {
		return
	}
	for i := range data.Keys {
		data.Keys[i].Type = keytypes.Normalize(data.Keys[i].Type)
	}
}
