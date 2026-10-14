package vault

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// A password changed on another device arrives as the server's wrap. It is only staged here and
// adopted once a typed password opens it and decrypts this vault, so a bad wrap can never lock
// anyone out.
type pendingWrap struct {
	KDF          KDFParams `json:"kdf"`
	ProtectedKey []byte    `json:"protected_key"`
}

func pendingWrapPath(vaultPath string) string { return vaultPath + ".pending-wrap" }

func rekeyPendingPath(vaultPath string) string { return vaultPath + ".rekey-pending" }

// SetRekeyPending records whether this device's own wrap still has to reach the server.
func SetRekeyPending(vaultPath string, pending bool) error {
	if pending {
		return atomicWrite(rekeyPendingPath(vaultPath), nil)
	}
	if err := os.Remove(rekeyPendingPath(vaultPath)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clearing rekey marker: %w", err)
	}
	return nil
}

func RekeyPending(vaultPath string) bool {
	_, err := os.Stat(rekeyPendingPath(vaultPath))
	return err == nil
}

// StagePendingWrap keeps the server's wrap when it differs from this vault's own, unless this
// device's newer wrap is still waiting to be uploaded.
func (v *Vault) StagePendingWrap(kdf KDFParams, protectedKey []byte) error {
	v.mu.RLock()
	same := v.kdf == kdf && bytes.Equal(v.protectedKey[:], protectedKey)
	path := v.path
	v.mu.RUnlock()

	if same {
		if err := os.Remove(pendingWrapPath(path)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("clearing pending wrap: %w", err)
		}
		return nil
	}
	if RekeyPending(path) || len(protectedKey) != ProtectedKeySize {
		return nil
	}
	data, err := json.Marshal(pendingWrap{KDF: kdf, ProtectedKey: protectedKey})
	if err != nil {
		return fmt.Errorf("encoding pending wrap: %w", err)
	}
	return atomicWrite(pendingWrapPath(path), data)
}

// pendingWrapKey returns the vault key when the password opens the staged wrap and that key
// really decrypts this vault.
func pendingWrapKey(vaultPath string, password []byte, header Header, ciphertext []byte) ([]byte, *pendingWrap) {
	data, err := os.ReadFile(pendingWrapPath(vaultPath))
	if err != nil {
		return nil, nil
	}
	var pending pendingWrap
	if json.Unmarshal(data, &pending) != nil || len(pending.ProtectedKey) != ProtectedKeySize {
		return nil, nil
	}
	masterKey, err := DeriveKey(password, pending.KDF)
	if err != nil {
		return nil, nil
	}
	defer zeroBytes(masterKey)
	stretchedKey, err := DeriveStretchedKey(masterKey)
	if err != nil {
		return nil, nil
	}
	defer zeroBytes(stretchedKey)
	key, err := DecryptCombined(stretchedKey, pending.ProtectedKey)
	if err != nil {
		return nil, nil
	}
	plaintext, err := Decrypt(key, header.Nonce[:], ciphertext)
	if err != nil {
		zeroBytes(key)
		return nil, nil
	}
	zeroBytes(plaintext)
	return key, &pending
}

func (v *Vault) adoptPendingWrap(pending *pendingWrap) {
	v.kdf = pending.KDF
	copy(v.protectedKey[:], pending.ProtectedKey)
	v.adoptedWrap = true
}

// PendingWrapAdopted reports that this open used a password changed on another device.
func (v *Vault) PendingWrapAdopted() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.adoptedWrap
}
