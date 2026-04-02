package vault

import "fmt"

func (v *Vault) ChangePassword(oldPassword, newPassword []byte) error {
	testKey := DeriveKey(oldPassword, v.kdf)
	match := true
	for i := range testKey {
		if testKey[i] != v.key[i] {
			match = false
		}
	}
	if !match {
		return fmt.Errorf("incorrect current password")
	}

	newKDF := DefaultKDFParams()
	newKey := DeriveKey(newPassword, newKDF)

	oldKey := make([]byte, len(v.key))
	copy(oldKey, v.key)
	oldKDF := v.kdf

	v.kdf = newKDF
	v.key = newKey
	v.Data.KeyGeneration++

	if err := v.Save(); err != nil {
		v.kdf = oldKDF
		v.key = oldKey
		v.Data.KeyGeneration--
		return fmt.Errorf("saving vault with new password: %w", err)
	}

	for i := range oldKey {
		oldKey[i] = 0
	}

	return nil
}
