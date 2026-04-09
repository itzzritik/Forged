package vault

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

type KeyStore struct {
	mu    sync.RWMutex
	vault *Vault
}

func NewKeyStore(v *Vault) *KeyStore {
	return &KeyStore{vault: v}
}

func (ks *KeyStore) List() []Key {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	out := make([]Key, len(ks.vault.Data.Keys))
	copy(out, ks.vault.Data.Keys)
	return out
}

func (ks *KeyStore) Get(name string) (Key, bool) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	for _, k := range ks.vault.Data.Keys {
		if k.Name == name {
			return k, true
		}
	}
	return Key{}, false
}

func (ks *KeyStore) Generate(name, comment string) (Key, error) {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	if ks.nameExists(name) {
		return Key{}, fmt.Errorf("key %q already exists", name)
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Key{}, fmt.Errorf("generating ed25519 key: %w", err)
	}

	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return Key{}, fmt.Errorf("converting public key: %w", err)
	}

	pemBlock, err := ssh.MarshalPrivateKey(priv, comment)
	if err != nil {
		return Key{}, fmt.Errorf("marshaling private key: %w", err)
	}

	now := time.Now().UTC()
	key := Key{
		ID:          uuid.NewString(),
		Name:        name,
		Type:        sshPub.Type(),
		PublicKey:    strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub))),
		PrivateKey:  pem.EncodeToMemory(pemBlock),
		Comment:     comment,
		Fingerprint: ssh.FingerprintSHA256(sshPub),
		CreatedAt:   now,
		UpdatedAt:   now,
		Tags:        []string{},
		HostRules:   []HostRule{},
		Version:     1,
	}

	ks.vault.Data.Keys = append(ks.vault.Data.Keys, key)
	if err := ks.vault.Save(); err != nil {
		ks.vault.Data.Keys = ks.vault.Data.Keys[:len(ks.vault.Data.Keys)-1]
		return Key{}, fmt.Errorf("saving vault: %w", err)
	}

	return key, nil
}

func (ks *KeyStore) Add(name string, privateKeyBytes []byte, comment string) (Key, error) {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	if ks.nameExists(name) {
		return Key{}, fmt.Errorf("key %q already exists", name)
	}

	signer, err := ssh.ParsePrivateKey(privateKeyBytes)
	if err != nil {
		return Key{}, fmt.Errorf("parsing private key: %w", err)
	}

	sshPub := signer.PublicKey()

	now := time.Now().UTC()
	key := Key{
		ID:          uuid.NewString(),
		Name:        name,
		Type:        sshPub.Type(),
		PublicKey:    strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub))),
		PrivateKey:  privateKeyBytes,
		Comment:     comment,
		Fingerprint: ssh.FingerprintSHA256(sshPub),
		CreatedAt:   now,
		UpdatedAt:   now,
		Tags:        []string{},
		HostRules:   []HostRule{},
		Version:     1,
	}

	ks.vault.Data.Keys = append(ks.vault.Data.Keys, key)
	if err := ks.vault.Save(); err != nil {
		ks.vault.Data.Keys = ks.vault.Data.Keys[:len(ks.vault.Data.Keys)-1]
		return Key{}, fmt.Errorf("saving vault: %w", err)
	}

	return key, nil
}

func (ks *KeyStore) AddFromFile(name, path, comment string) (Key, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Key{}, fmt.Errorf("reading key file: %w", err)
	}
	return ks.Add(name, data, comment)
}

func (ks *KeyStore) Remove(name string) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	idx := ks.indexOf(name)
	if idx < 0 {
		return fmt.Errorf("key %q not found", name)
	}

	removed := ks.vault.Data.Keys[idx]
	ks.vault.Data.Keys = append(ks.vault.Data.Keys[:idx], ks.vault.Data.Keys[idx+1:]...)

	if err := ks.vault.Save(); err != nil {
		ks.vault.Data.Keys = append(ks.vault.Data.Keys[:idx], append([]Key{removed}, ks.vault.Data.Keys[idx:]...)...)
		return fmt.Errorf("saving vault: %w", err)
	}

	return nil
}

func (ks *KeyStore) Rename(oldName, newName string) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	if ks.nameExists(newName) {
		return fmt.Errorf("key %q already exists", newName)
	}

	idx := ks.indexOf(oldName)
	if idx < 0 {
		return fmt.Errorf("key %q not found", oldName)
	}

	ks.vault.Data.Keys[idx].Name = newName
	ks.vault.Data.Keys[idx].UpdatedAt = time.Now().UTC()
	ks.vault.Data.Keys[idx].Version++

	if err := ks.vault.Save(); err != nil {
		ks.vault.Data.Keys[idx].Name = oldName
		return fmt.Errorf("saving vault: %w", err)
	}

	return nil
}

func (ks *KeyStore) Export(name string) (string, error) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	for _, k := range ks.vault.Data.Keys {
		if k.Name == name {
			if k.Comment != "" {
				return k.PublicKey + " " + k.Comment, nil
			}
			return k.PublicKey, nil
		}
	}
	return "", fmt.Errorf("key %q not found", name)
}

func (ks *KeyStore) RecordUsage(name string) {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	idx := ks.indexOf(name)
	if idx < 0 {
		return
	}
	now := time.Now().UTC()
	ks.vault.Data.Keys[idx].LastUsedAt = &now
}

func (ks *KeyStore) AddHostRule(keyName, pattern string) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	idx := ks.indexOf(keyName)
	if idx < 0 {
		return fmt.Errorf("key %q not found", keyName)
	}

	patternType := classifyPattern(pattern)

	for _, r := range ks.vault.Data.Keys[idx].HostRules {
		if r.Match == pattern {
			return fmt.Errorf("pattern %q already mapped to %q", pattern, keyName)
		}
	}

	ks.vault.Data.Keys[idx].HostRules = append(ks.vault.Data.Keys[idx].HostRules, HostRule{
		Match: pattern,
		Type:  patternType,
	})
	ks.vault.Data.Keys[idx].UpdatedAt = time.Now().UTC()
	ks.vault.Data.Keys[idx].Version++

	return ks.vault.Save()
}

func (ks *KeyStore) RemoveHostRule(keyName, pattern string) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	idx := ks.indexOf(keyName)
	if idx < 0 {
		return fmt.Errorf("key %q not found", keyName)
	}

	rules := ks.vault.Data.Keys[idx].HostRules
	found := false
	for i, r := range rules {
		if r.Match == pattern {
			ks.vault.Data.Keys[idx].HostRules = append(rules[:i], rules[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("pattern %q not found on key %q", pattern, keyName)
	}

	ks.vault.Data.Keys[idx].UpdatedAt = time.Now().UTC()
	ks.vault.Data.Keys[idx].Version++

	return ks.vault.Save()
}

func (ks *KeyStore) SetGitSigning(keyName string, enabled bool) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	idx := ks.indexOf(keyName)
	if idx < 0 {
		return fmt.Errorf("key %q not found", keyName)
	}

	if enabled {
		for i := range ks.vault.Data.Keys {
			ks.vault.Data.Keys[i].GitSigning = false
		}
	}

	ks.vault.Data.Keys[idx].GitSigning = enabled
	ks.vault.Data.Keys[idx].UpdatedAt = time.Now().UTC()
	ks.vault.Data.Keys[idx].Version++

	return ks.vault.Save()
}

func (ks *KeyStore) GetGitSigningKey() (Key, bool) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	for _, k := range ks.vault.Data.Keys {
		if k.GitSigning {
			return k, true
		}
	}
	return Key{}, false
}

func classifyPattern(pattern string) string {
	if strings.HasPrefix(pattern, "~") {
		return "regex"
	}
	if strings.Contains(pattern, "*") {
		return "wildcard"
	}
	return "exact"
}

func (ks *KeyStore) nameExists(name string) bool {
	return ks.indexOf(name) >= 0
}

func (ks *KeyStore) indexOf(name string) int {
	for i, k := range ks.vault.Data.Keys {
		if k.Name == name {
			return i
		}
	}
	return -1
}
