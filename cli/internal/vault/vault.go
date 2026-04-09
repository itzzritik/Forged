package vault

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/itzzritik/forged/cli/internal/platform"
)

type Vault struct {
	path     string
	lockFile *os.File
	kdf      KDFParams
	key      []byte
	Data     VaultData
}

type VaultData struct {
	Keys           []Key              `json:"keys"`
	Metadata       Metadata           `json:"metadata"`
	VersionVector  map[string]int64   `json:"version_vector"`
	Tombstones     []Tombstone        `json:"tombstones"`
	KeyGeneration  int                `json:"key_generation"`
}

type Key struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Type         string     `json:"type"`
	PublicKey    string     `json:"public_key"`
	PrivateKey   []byte     `json:"private_key"`
	Comment      string     `json:"comment"`
	Fingerprint  string     `json:"fingerprint"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
	Tags         []string   `json:"tags"`
	HostRules    []HostRule `json:"host_rules"`
	GitSigning   bool       `json:"git_signing"`
	Version      int        `json:"version"`
	DeviceOrigin string     `json:"device_origin"`
}

type HostRule struct {
	Match string `json:"match"`
	Type  string `json:"type"`
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
		return nil, fmt.Errorf("creating vault directory: %w", err)
	}

	kdf := DefaultKDFParams()
	key := DeriveKey(password, kdf)

	v := &Vault{
		path: path,
		kdf:  kdf,
		key:  key,
		Data: VaultData{
			Keys:          []Key{},
			Metadata:      Metadata{CreatedAt: time.Now().UTC()},
			VersionVector: map[string]int64{},
			Tombstones:    []Tombstone{},
			KeyGeneration: 1,
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
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading vault: %w", err)
	}

	header, ciphertext, err := UnmarshalVault(data)
	if err != nil {
		return nil, err
	}

	key := DeriveKey(password, header.KDF)

	plaintext, err := Decrypt(key, header.Nonce[:], ciphertext)
	if err != nil {
		return nil, err
	}

	var vd VaultData
	if err := json.Unmarshal(plaintext, &vd); err != nil {
		return nil, fmt.Errorf("parsing vault data: %w", err)
	}

	v := &Vault{
		path: path,
		kdf:  header.KDF,
		key:  key,
		Data: vd,
	}

	if err := v.acquireLock(); err != nil {
		return nil, err
	}

	return v, nil
}

func (v *Vault) Save() error {
	plaintext, err := json.Marshal(v.Data)
	if err != nil {
		return fmt.Errorf("serializing vault: %w", err)
	}

	nonce, ciphertext, err := Encrypt(v.key, plaintext)
	if err != nil {
		return err
	}

	var nonceArr [NonceSize]byte
	copy(nonceArr[:], nonce)

	header := Header{
		Version: CurrentVersion,
		KDF:     v.kdf,
		Nonce:   nonceArr,
	}

	raw := MarshalVault(header, ciphertext)
	return atomicWrite(v.path, raw)
}

func (v *Vault) Close() {
	for i := range v.key {
		v.key[i] = 0
	}
	v.releaseLock()
}

func (v *Vault) Path() string {
	return v.path
}

func (v *Vault) Key() []byte {
	return v.key
}

func (v *Vault) ExportForSync() ([]byte, error) {
	plaintext, err := json.Marshal(v.Data)
	if err != nil {
		return nil, fmt.Errorf("serializing vault: %w", err)
	}
	return EncryptForSync(v.key, plaintext)
}

func (v *Vault) ImportFromSync(data []byte) error {
	plaintext, err := DecryptFromSync(v.key, data)
	if err != nil {
		return err
	}
	var vd VaultData
	if err := json.Unmarshal(plaintext, &vd); err != nil {
		return fmt.Errorf("parsing synced vault: %w", err)
	}
	v.Data = vd
	return v.Save()
}

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".vault-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()

	defer func() {
		tmp.Close()
		os.Remove(tmpPath)
	}()

	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("syncing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Chmod(tmpPath, 0600); err != nil {
		return fmt.Errorf("setting permissions: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming vault file: %w", err)
	}

	return nil
}

func (v *Vault) acquireLock() error {
	lockPath := v.path + ".lock"
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("opening lock file: %w", err)
	}

	if err := platform.LockFile(f); err != nil {
		f.Close()
		return fmt.Errorf("vault is locked by another process")
	}

	v.lockFile = f
	return nil
}

func (v *Vault) releaseLock() {
	if v.lockFile == nil {
		return
	}
	platform.UnlockFile(v.lockFile)
	name := v.lockFile.Name()
	v.lockFile.Close()
	os.Remove(name)
	v.lockFile = nil
}
