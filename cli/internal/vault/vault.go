package vault

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/itzzritik/forged/cli/internal/platform"
)

type Vault struct {
	mu           sync.RWMutex
	path         string
	lockFile     *os.File
	kdf          KDFParams
	key          []byte // Symmetric Key (random, decrypted from Protected Symmetric Key)
	protectedKey [ProtectedKeySize]byte
	data         VaultData
	closed       bool
}

type VaultData struct {
	Keys          []Key            `json:"keys"`
	Metadata      Metadata         `json:"metadata"`
	VersionVector map[string]int64 `json:"version_vector"`
	Tombstones    []Tombstone      `json:"tombstones"`
	KeyGeneration int              `json:"key_generation"`
	SSH           SSHData          `json:"ssh,omitempty"`
}

type SSHData struct {
	Routes     map[string]SSHRoute `json:"routes,omitempty"`
	Tombstones []SSHRouteTombstone `json:"tombstones,omitempty"`
}

type SSHRoute struct {
	Key           string               `json:"key,omitempty"`
	Updated       time.Time            `json:"updated"`
	ProvenBy      string               `json:"proven_by,omitempty"`
	Operation     string               `json:"operation,omitempty"`
	SuccessCount  int                  `json:"success_count,omitempty"`
	LastSuccessAt *time.Time           `json:"last_success_at,omitempty"`
	Attempts      map[string]time.Time `json:"attempts,omitempty"`
}

type SSHRouteTombstone struct {
	Target          string    `json:"target"`
	DeletedAt       time.Time `json:"deleted_at"`
	DeletedByDevice string    `json:"deleted_by_device,omitempty"`
}

type Key struct {
	ID                  string     `json:"id"`
	Name                string     `json:"name"`
	Type                string     `json:"type"`
	PublicKey           string     `json:"public_key"`
	EncryptedPrivateKey string     `json:"encrypted_private_key"`
	EncryptedCipherKey  string     `json:"encrypted_cipher_key"`
	PrivateKey          []byte     `json:"-"`
	Comment             string     `json:"comment"`
	Fingerprint         string     `json:"fingerprint"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	LastUsedAt          *time.Time `json:"last_used_at,omitempty"`
	Tags                []string   `json:"tags"`
	GitSigning          bool       `json:"git_signing"`
	Version             int        `json:"version"`
	DeviceOrigin        string     `json:"device_origin"`
}

type Metadata struct {
	CreatedAt  time.Time `json:"created_at"`
	DeviceID   string    `json:"device_id"`
	DeviceName string    `json:"device_name"`
}

type Tombstone struct {
	KeyID           string    `json:"key_id"`
	DeletedAt       time.Time `json:"deleted_at"`
	DeletedByDevice string    `json:"deleted_by_device"`
}

func Create(path string, password []byte) (*Vault, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("Creating vault directory: %w", err)
	}

	kdf := DefaultKDFParams()

	masterKey, err := DeriveKey(password, kdf)
	if err != nil {
		return nil, fmt.Errorf("Deriving master key: %w", err)
	}

	stretchedKey, err := DeriveStretchedKey(masterKey)
	if err != nil {
		for i := range masterKey {
			masterKey[i] = 0
		}
		return nil, fmt.Errorf("Deriving stretched key: %w", err)
	}

	// Zero the master key -- no longer needed
	for i := range masterKey {
		masterKey[i] = 0
	}

	symmetricKey := make([]byte, KeySize)
	if _, err := rand.Read(symmetricKey); err != nil {
		for i := range stretchedKey {
			stretchedKey[i] = 0
		}
		return nil, fmt.Errorf("Generating symmetric key: %w", err)
	}

	protectedKeyData, err := EncryptCombined(stretchedKey, symmetricKey)
	if err != nil {
		for i := range stretchedKey {
			stretchedKey[i] = 0
		}
		for i := range symmetricKey {
			symmetricKey[i] = 0
		}
		return nil, fmt.Errorf("Encrypting symmetric key: %w", err)
	}

	// Zero the stretched key -- no longer needed
	for i := range stretchedKey {
		stretchedKey[i] = 0
	}

	var protectedKey [ProtectedKeySize]byte
	copy(protectedKey[:], protectedKeyData)

	v := &Vault{
		path:         path,
		kdf:          kdf,
		key:          symmetricKey,
		protectedKey: protectedKey,
		data: VaultData{
			Keys: []Key{},
			Metadata: Metadata{
				CreatedAt: time.Now().UTC(),
			},
			VersionVector: map[string]int64{},
			Tombstones:    []Tombstone{},
			KeyGeneration: 1,
			SSH: SSHData{
				Routes: map[string]SSHRoute{},
			},
		},
	}

	if err := v.acquireLock(); err != nil {
		return nil, err
	}

	if err := v.Save(); err != nil {
		v.Close()
		return nil, err
	}

	return v, nil
}

func Open(path string, password []byte) (*Vault, error) {
	lockFile, err := acquireVaultLock(path)
	if err != nil {
		return nil, err
	}

	v, err := openVault(path, password)
	if err != nil {
		releaseVaultLock(lockFile)
		return nil, err
	}
	v.lockFile = lockFile
	return v, nil
}

func OpenReadOnly(path string, password []byte) (*Vault, error) {
	return openVault(path, password)
}

func OpenReadOnlyWithSymmetricKey(path string, symmetricKey []byte) (*Vault, error) {
	return openVaultWithSymmetricKey(path, symmetricKey)
}

func OpenWithSymmetricKey(path string, symmetricKey []byte) (*Vault, error) {
	lockFile, err := acquireVaultLock(path)
	if err != nil {
		return nil, err
	}

	v, err := openVaultWithSymmetricKey(path, symmetricKey)
	if err != nil {
		releaseVaultLock(lockFile)
		return nil, err
	}
	v.lockFile = lockFile
	return v, nil
}

func openVault(path string, password []byte) (*Vault, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Reading vault: %w", err)
	}

	header, ciphertext, err := UnmarshalVault(data)
	if err != nil {
		return nil, err
	}

	masterKey, err := DeriveKey(password, header.KDF)
	if err != nil {
		return nil, fmt.Errorf("Deriving master key: %w", err)
	}

	stretchedKey, err := DeriveStretchedKey(masterKey)
	if err != nil {
		for i := range masterKey {
			masterKey[i] = 0
		}
		return nil, fmt.Errorf("Deriving stretched key: %w", err)
	}

	symmetricKey, err := DecryptCombined(stretchedKey, header.ProtectedKey[:])
	if err != nil {
		for i := range masterKey {
			masterKey[i] = 0
		}
		for i := range stretchedKey {
			stretchedKey[i] = 0
		}
		return nil, fmt.Errorf("Decrypting protected key: %w", err)
	}

	// Zero master key and stretched key -- no longer needed
	for i := range masterKey {
		masterKey[i] = 0
	}
	for i := range stretchedKey {
		stretchedKey[i] = 0
	}

	plaintext, err := Decrypt(symmetricKey, header.Nonce[:], ciphertext)
	if err != nil {
		for i := range symmetricKey {
			symmetricKey[i] = 0
		}
		return nil, err
	}

	var vd VaultData
	if err := json.Unmarshal(plaintext, &vd); err != nil {
		for i := range symmetricKey {
			symmetricKey[i] = 0
		}
		return nil, fmt.Errorf("Parsing vault data: %w", err)
	}
	normalizeVaultKeyTypes(&vd)

	v := &Vault{
		path:         path,
		kdf:          header.KDF,
		key:          symmetricKey,
		protectedKey: header.ProtectedKey,
		data:         vd,
	}

	return v, nil
}

func openVaultWithSymmetricKey(path string, symmetricKey []byte) (*Vault, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Reading vault: %w", err)
	}

	header, ciphertext, err := UnmarshalVault(data)
	if err != nil {
		return nil, err
	}

	workingKey := append([]byte(nil), symmetricKey...)
	plaintext, err := Decrypt(workingKey, header.Nonce[:], ciphertext)
	if err != nil {
		for i := range workingKey {
			workingKey[i] = 0
		}
		return nil, err
	}

	var vd VaultData
	if err := json.Unmarshal(plaintext, &vd); err != nil {
		for i := range workingKey {
			workingKey[i] = 0
		}
		return nil, fmt.Errorf("Parsing vault data: %w", err)
	}
	normalizeVaultKeyTypes(&vd)

	return &Vault{
		path:         path,
		kdf:          header.KDF,
		key:          workingKey,
		protectedKey: header.ProtectedKey,
		data:         vd,
	}, nil
}

func (v *Vault) Save() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.saveLocked()
}

func (v *Vault) saveLocked() error {
	return v.saveDataLocked(&v.data)
}

func (v *Vault) saveDataLocked(data *VaultData) error {
	if err := v.ensureOpenLocked(); err != nil {
		return err
	}

	normalizeVaultKeyTypes(data)
	plaintext, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("Serializing vault: %w", err)
	}

	nonce, ciphertext, err := Encrypt(v.key, plaintext)
	if err != nil {
		return err
	}

	var nonceArr [NonceSize]byte
	copy(nonceArr[:], nonce)

	header := Header{
		Version:      CurrentVersion,
		KDF:          v.kdf,
		ProtectedKey: v.protectedKey,
		Nonce:        nonceArr,
	}

	raw := MarshalVault(header, ciphertext)
	return atomicWrite(v.path, raw)
}

func (v *Vault) Close() {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.closed {
		return
	}
	v.closed = true
	for i := range v.key {
		v.key[i] = 0
	}
	v.key = nil
	v.releaseLock()
}

func (v *Vault) Path() string {
	return v.path
}

func (v *Vault) Key() []byte {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return append([]byte(nil), v.key...)
}

func (v *Vault) KDFParams() KDFParams {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.kdf
}

func (v *Vault) ProtectedKeyBytes() []byte {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return append([]byte(nil), v.protectedKey[:]...)
}

func (v *Vault) DeviceID() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.data.Metadata.DeviceID
}

func (v *Vault) ChangePassword(newPassword []byte) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if err := v.ensureOpenLocked(); err != nil {
		return err
	}

	newKDF := DefaultKDFParams()

	newMasterKey, err := DeriveKey(newPassword, newKDF)
	if err != nil {
		return fmt.Errorf("Deriving new master key: %w", err)
	}

	newStretchedKey, err := DeriveStretchedKey(newMasterKey)
	if err != nil {
		for i := range newMasterKey {
			newMasterKey[i] = 0
		}
		return fmt.Errorf("Deriving new stretched key: %w", err)
	}

	newProtectedKey, err := EncryptCombined(newStretchedKey, v.key)
	if err != nil {
		for i := range newMasterKey {
			newMasterKey[i] = 0
		}
		for i := range newStretchedKey {
			newStretchedKey[i] = 0
		}
		return fmt.Errorf("Encrypting new protected key: %w", err)
	}

	for i := range newMasterKey {
		newMasterKey[i] = 0
	}
	for i := range newStretchedKey {
		newStretchedKey[i] = 0
	}

	var protectedKeyArr [ProtectedKeySize]byte
	copy(protectedKeyArr[:], newProtectedKey)

	v.kdf = newKDF
	v.protectedKey = protectedKeyArr

	return v.saveLocked()
}

func (v *Vault) ExportForSync() ([]byte, KDFParams, []byte, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if err := v.ensureOpenLocked(); err != nil {
		return nil, KDFParams{}, nil, err
	}

	plaintext, err := json.Marshal(v.data)
	if err != nil {
		return nil, KDFParams{}, nil, fmt.Errorf("Serializing vault: %w", err)
	}
	blob, err := EncryptCombined(v.key, plaintext)
	if err != nil {
		return nil, KDFParams{}, nil, err
	}
	protectedKey := append([]byte(nil), v.protectedKey[:]...)
	return blob, v.kdf, protectedKey, nil
}

func (v *Vault) DecryptSyncBlob(data []byte) ([]byte, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if err := v.ensureOpenLocked(); err != nil {
		return nil, err
	}
	return DecryptCombined(v.key, data)
}

func (v *Vault) UpdateData(update func(*VaultData) error) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if err := v.ensureOpenLocked(); err != nil {
		return err
	}

	next, err := cloneVaultData(v.data)
	if err != nil {
		return err
	}
	if err := update(&next); err != nil {
		return err
	}
	if err := v.saveDataLocked(&next); err != nil {
		return err
	}
	v.data = next
	return nil
}

func (v *Vault) ImportFromSync(data []byte) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if err := v.ensureOpenLocked(); err != nil {
		return err
	}

	plaintext, err := DecryptCombined(v.key, data)
	if err != nil {
		return err
	}
	var vd VaultData
	if err := json.Unmarshal(plaintext, &vd); err != nil {
		return fmt.Errorf("Parsing synced vault: %w", err)
	}
	normalizeVaultKeyTypes(&vd)
	if err := v.saveDataLocked(&vd); err != nil {
		return err
	}
	v.data = vd
	return nil
}

func (v *Vault) ensureOpenLocked() error {
	if v.closed || len(v.key) != KeySize {
		return fmt.Errorf("Vault is closed")
	}
	return nil
}

func cloneVaultData(data VaultData) (VaultData, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return VaultData{}, fmt.Errorf("Cloning vault data: %w", err)
	}
	var cloned VaultData
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return VaultData{}, fmt.Errorf("Cloning vault data: %w", err)
	}
	return cloned, nil
}

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".vault-*.tmp")
	if err != nil {
		return fmt.Errorf("Creating temp file: %w", err)
	}
	tmpPath := tmp.Name()

	defer func() {
		tmp.Close()
		os.Remove(tmpPath)
	}()

	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("Writing temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("Syncing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("Closing temp file: %w", err)
	}

	if err := os.Chmod(tmpPath, 0600); err != nil {
		return fmt.Errorf("Setting permissions: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("Renaming vault file: %w", err)
	}

	return nil
}

func (v *Vault) acquireLock() error {
	f, err := acquireVaultLock(v.path)
	if err != nil {
		return err
	}
	v.lockFile = f
	return nil
}

func acquireVaultLock(path string) (*os.File, error) {
	lockPath := path + ".lock"
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("Opening lock file: %w", err)
	}

	if err := platform.LockFile(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("Vault is locked by another process")
	}

	return f, nil
}

func (v *Vault) releaseLock() {
	if v.lockFile == nil {
		return
	}
	releaseVaultLock(v.lockFile)
	v.lockFile = nil
}

func releaseVaultLock(f *os.File) {
	platform.UnlockFile(f)
	f.Close()
}
