package vault

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/itzzritik/forged/cli/internal/keytypes"
	"github.com/itzzritik/forged/cli/internal/platform"
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
	for i := range out {
		out[i].Type = keytypes.Normalize(out[i].Type)
	}
	return out
}

func (ks *KeyStore) Get(name string) (Key, bool) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	for _, k := range ks.vault.Data.Keys {
		if k.Name == name {
			k.Type = keytypes.Normalize(k.Type)
			return k, true
		}
	}
	return Key{}, false
}

func (ks *KeyStore) ResolveName(input string) (string, error) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	normalized := normalizeKeyName(input)
	if normalized == "" {
		return "", &KeyNameResolveError{Query: input}
	}

	matches := rankNameMatches(ks.vault.Data.Keys, normalized)
	if len(matches) == 0 {
		suggestions, more := cappedSuggestions(suggestNameMatches(ks.vault.Data.Keys, normalized))
		return "", &KeyNameResolveError{
			Query:       input,
			Suggestions: suggestions,
			More:        more,
			Ambiguous:   false,
		}
	}

	bestKind := matches[0].kind
	best := make([]nameMatch, 0, len(matches))
	for _, match := range matches {
		if match.kind != bestKind {
			break
		}
		best = append(best, match)
	}

	if len(best) == 1 {
		return best[0].key.Name, nil
	}

	suggestions, more := cappedSuggestions(best)
	return "", &KeyNameResolveError{
		Query:       input,
		Suggestions: suggestions,
		More:        more,
		Ambiguous:   true,
	}
}

func (ks *KeyStore) Generate(name, comment string) (Key, error) {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	if ks.nameExists(name) {
		return Key{}, fmt.Errorf("Key %q already exists", name)
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Key{}, fmt.Errorf("Generating Ed25519 key: %w", err)
	}

	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return Key{}, fmt.Errorf("Converting public key: %w", err)
	}

	pemBlock, err := ssh.MarshalPrivateKey(priv, comment)
	if err != nil {
		return Key{}, fmt.Errorf("Marshaling private key: %w", err)
	}

	privateKeyBytes := pem.EncodeToMemory(pemBlock)

	cipherKey := make([]byte, KeySize)
	if _, err := rand.Read(cipherKey); err != nil {
		return Key{}, fmt.Errorf("Generating cipher key: %w", err)
	}

	encPriv, err := EncryptCombined(cipherKey, privateKeyBytes)
	if err != nil {
		for i := range cipherKey {
			cipherKey[i] = 0
		}
		return Key{}, fmt.Errorf("Encrypting private key: %w", err)
	}

	encCK, err := EncryptCombined(ks.vault.key, cipherKey)
	if err != nil {
		for i := range cipherKey {
			cipherKey[i] = 0
		}
		return Key{}, fmt.Errorf("Encrypting cipher key: %w", err)
	}

	for i := range cipherKey {
		cipherKey[i] = 0
	}

	now := time.Now().UTC()
	key := Key{
		ID:                  uuid.NewString(),
		Name:                name,
		Type:                keytypes.FromSSHPublicKeyType(sshPub.Type()),
		PublicKey:           strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub))),
		EncryptedPrivateKey: base64.StdEncoding.EncodeToString(encPriv),
		EncryptedCipherKey:  base64.StdEncoding.EncodeToString(encCK),
		Comment:             comment,
		Fingerprint:         ssh.FingerprintSHA256(sshPub),
		CreatedAt:           now,
		UpdatedAt:           now,
		Tags:                []string{},
		Version:             1,
		DeviceOrigin:        ks.vault.DeviceID(),
	}

	originalVersionVector := cloneVersionVector(ks.vault.Data.VersionVector)
	storedKey := key
	storedKey.PrivateKey = nil
	ks.vault.Data.Keys = append(ks.vault.Data.Keys, storedKey)
	ks.bumpVersionVector()
	if err := ks.vault.Save(); err != nil {
		ks.vault.Data.Keys = ks.vault.Data.Keys[:len(ks.vault.Data.Keys)-1]
		ks.vault.Data.VersionVector = originalVersionVector
		return Key{}, fmt.Errorf("Saving vault: %w", err)
	}
	for i := range privateKeyBytes {
		privateKeyBytes[i] = 0
	}
	key.PrivateKey = nil

	return key, nil
}

func (ks *KeyStore) Add(name string, privateKeyBytes []byte, comment string) (Key, error) {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	if ks.nameExists(name) {
		return Key{}, fmt.Errorf("Key %q already exists", name)
	}

	normalized, err := NormalizePrivateKeyToOpenSSH(privateKeyBytes, comment)
	if err != nil {
		return Key{}, err
	}

	cipherKey := make([]byte, KeySize)
	if _, err := rand.Read(cipherKey); err != nil {
		return Key{}, fmt.Errorf("Generating cipher key: %w", err)
	}

	encPriv, err := EncryptCombined(cipherKey, normalized.Bytes)
	if err != nil {
		for i := range cipherKey {
			cipherKey[i] = 0
		}
		return Key{}, fmt.Errorf("Encrypting private key: %w", err)
	}

	encCK, err := EncryptCombined(ks.vault.key, cipherKey)
	if err != nil {
		for i := range cipherKey {
			cipherKey[i] = 0
		}
		return Key{}, fmt.Errorf("Encrypting cipher key: %w", err)
	}

	for i := range cipherKey {
		cipherKey[i] = 0
	}

	now := time.Now().UTC()
	key := Key{
		ID:                  uuid.NewString(),
		Name:                name,
		Type:                keytypes.Normalize(normalized.Type),
		PublicKey:           normalized.PublicKey,
		EncryptedPrivateKey: base64.StdEncoding.EncodeToString(encPriv),
		EncryptedCipherKey:  base64.StdEncoding.EncodeToString(encCK),
		Comment:             comment,
		Fingerprint:         normalized.Fingerprint,
		CreatedAt:           now,
		UpdatedAt:           now,
		Tags:                []string{},
		Version:             1,
		DeviceOrigin:        ks.vault.DeviceID(),
	}

	originalVersionVector := cloneVersionVector(ks.vault.Data.VersionVector)
	storedKey := key
	storedKey.PrivateKey = nil
	ks.vault.Data.Keys = append(ks.vault.Data.Keys, storedKey)
	ks.bumpVersionVector()
	if err := ks.vault.Save(); err != nil {
		ks.vault.Data.Keys = ks.vault.Data.Keys[:len(ks.vault.Data.Keys)-1]
		ks.vault.Data.VersionVector = originalVersionVector
		return Key{}, fmt.Errorf("Saving vault: %w", err)
	}
	for i := range normalized.Bytes {
		normalized.Bytes[i] = 0
	}
	key.PrivateKey = nil

	return key, nil
}

func (ks *KeyStore) AddFromFile(name, path, comment string) (Key, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Key{}, fmt.Errorf("Reading key file: %w", err)
	}
	return ks.Add(name, data, comment)
}

func (ks *KeyStore) Remove(name string) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	idx := ks.indexOf(name)
	if idx < 0 {
		return fmt.Errorf("Key %q not found", name)
	}

	originalVersionVector := cloneVersionVector(ks.vault.Data.VersionVector)
	originalTombstones := cloneTombstones(ks.vault.Data.Tombstones)
	removed := ks.vault.Data.Keys[idx]
	ks.vault.Data.Keys = append(ks.vault.Data.Keys[:idx], ks.vault.Data.Keys[idx+1:]...)
	now := time.Now().UTC()
	ks.upsertTombstone(removed.ID, now)
	ks.bumpVersionVector()

	if err := ks.vault.Save(); err != nil {
		ks.vault.Data.Keys = append(ks.vault.Data.Keys[:idx], append([]Key{removed}, ks.vault.Data.Keys[idx:]...)...)
		ks.vault.Data.Tombstones = originalTombstones
		ks.vault.Data.VersionVector = originalVersionVector
		return fmt.Errorf("Saving vault: %w", err)
	}

	return nil
}

func (ks *KeyStore) Rename(oldName, newName string) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	if ks.nameExists(newName) {
		return fmt.Errorf("Key %q already exists", newName)
	}

	idx := ks.indexOf(oldName)
	if idx < 0 {
		return fmt.Errorf("Key %q not found", oldName)
	}

	original := cloneKey(ks.vault.Data.Keys[idx])
	originalVersionVector := cloneVersionVector(ks.vault.Data.VersionVector)
	ks.vault.Data.Keys[idx].Name = newName
	ks.vault.Data.Keys[idx].UpdatedAt = time.Now().UTC()
	ks.vault.Data.Keys[idx].Version++
	ks.bumpVersionVector()

	if err := ks.vault.Save(); err != nil {
		ks.vault.Data.Keys[idx] = original
		ks.vault.Data.VersionVector = originalVersionVector
		return fmt.Errorf("Saving vault: %w", err)
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
	return "", fmt.Errorf("Key %q not found", name)
}

func (ks *KeyStore) PrivateKeyBytes(name string) ([]byte, error) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	idx := ks.indexOf(name)
	if idx < 0 {
		return nil, fmt.Errorf("Key %q not found", name)
	}
	return ks.decryptPrivateKeyLocked(&ks.vault.Data.Keys[idx])
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

func (ks *KeyStore) SetGitSigning(keyName string, enabled bool) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	idx := ks.indexOf(keyName)
	if idx < 0 {
		return fmt.Errorf("Key %q not found", keyName)
	}

	originalKeys := cloneKeys(ks.vault.Data.Keys)
	originalVersionVector := cloneVersionVector(ks.vault.Data.VersionVector)
	now := time.Now().UTC()
	changed := false
	if enabled {
		for i := range ks.vault.Data.Keys {
			if i != idx && ks.vault.Data.Keys[i].GitSigning {
				ks.vault.Data.Keys[i].GitSigning = false
				ks.vault.Data.Keys[i].UpdatedAt = now
				ks.vault.Data.Keys[i].Version++
				changed = true
			}
		}
	}

	if ks.vault.Data.Keys[idx].GitSigning != enabled {
		ks.vault.Data.Keys[idx].GitSigning = enabled
		ks.vault.Data.Keys[idx].UpdatedAt = now
		ks.vault.Data.Keys[idx].Version++
		changed = true
	}

	if !changed {
		return nil
	}

	ks.bumpVersionVector()
	if err := ks.vault.Save(); err != nil {
		ks.vault.Data.Keys = originalKeys
		ks.vault.Data.VersionVector = originalVersionVector
		return err
	}
	return nil
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

func (ks *KeyStore) SignerByPublicKey(pub ssh.PublicKey) (ssh.Signer, string, string, error) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	if ks.vault == nil {
		return nil, "", "", fmt.Errorf("Vault is locked")
	}

	wanted := pub.Marshal()
	for i := range ks.vault.Data.Keys {
		key := &ks.vault.Data.Keys[i]
		parsed, err := parseAuthorizedPublicKey(key.PublicKey)
		if err != nil {
			continue
		}
		if !bytes.Equal(parsed.Marshal(), wanted) {
			continue
		}

		privateKey, err := ks.decryptPrivateKeyLocked(key)
		if err != nil {
			return nil, "", "", err
		}
		_ = platform.Mlock(privateKey)
		signer, err := ssh.ParsePrivateKey(privateKey)
		for j := range privateKey {
			privateKey[j] = 0
		}
		_ = platform.Munlock(privateKey)
		if err != nil {
			return nil, "", "", fmt.Errorf("Parsing private key for %s: %w", key.Name, err)
		}
		return signer, key.Name, key.Fingerprint, nil
	}
	return nil, "", "", fmt.Errorf("Key not found in vault")
}

func (ks *KeyStore) Signers() ([]ssh.Signer, error) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	if ks.vault == nil {
		return nil, fmt.Errorf("Vault is locked")
	}

	signers := make([]ssh.Signer, 0, len(ks.vault.Data.Keys))
	for i := range ks.vault.Data.Keys {
		privateKey, err := ks.decryptPrivateKeyLocked(&ks.vault.Data.Keys[i])
		if err != nil {
			return nil, err
		}
		_ = platform.Mlock(privateKey)
		signer, err := ssh.ParsePrivateKey(privateKey)
		for j := range privateKey {
			privateKey[j] = 0
		}
		_ = platform.Munlock(privateKey)
		if err != nil {
			return nil, fmt.Errorf("Parsing private key for %s: %w", ks.vault.Data.Keys[i].Name, err)
		}
		signers = append(signers, signer)
	}
	return signers, nil
}

func (ks *KeyStore) nameExists(name string) bool {
	return ks.indexOf(name) >= 0
}

func (ks *KeyStore) decryptPrivateKeyLocked(key *Key) ([]byte, error) {
	if key == nil {
		return nil, fmt.Errorf("Key not found")
	}
	if ks.vault == nil {
		return nil, fmt.Errorf("Vault is locked")
	}
	if key.EncryptedCipherKey == "" || key.EncryptedPrivateKey == "" {
		return nil, fmt.Errorf("Private key is unavailable")
	}

	cipherKeyData, err := base64.StdEncoding.DecodeString(key.EncryptedCipherKey)
	if err != nil {
		return nil, fmt.Errorf("Decoding cipher key for %s: %w", key.Name, err)
	}
	cipherKey, err := DecryptCombined(ks.vault.key, cipherKeyData)
	if err != nil {
		return nil, fmt.Errorf("Decrypting cipher key for %s: %w", key.Name, err)
	}
	defer zeroBytes(cipherKey)

	privateKeyData, err := base64.StdEncoding.DecodeString(key.EncryptedPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("Decoding private key for %s: %w", key.Name, err)
	}
	privateKey, err := DecryptCombined(cipherKey, privateKeyData)
	if err != nil {
		return nil, fmt.Errorf("Decrypting private key for %s: %w", key.Name, err)
	}
	return privateKey, nil
}

func parseAuthorizedPublicKey(authorizedKey string) (ssh.PublicKey, error) {
	pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(authorizedKey))
	if err != nil {
		return nil, err
	}
	return pub, nil
}

func (ks *KeyStore) indexOf(name string) int {
	for i, k := range ks.vault.Data.Keys {
		if k.Name == name {
			return i
		}
	}
	return -1
}

func (ks *KeyStore) bumpVersionVector() {
	deviceID := ks.vault.DeviceID()
	if deviceID == "" {
		return
	}

	if ks.vault.Data.VersionVector == nil {
		ks.vault.Data.VersionVector = map[string]int64{}
	}
	ks.vault.Data.VersionVector[deviceID]++
}

func (ks *KeyStore) upsertTombstone(keyID string, deletedAt time.Time) {
	tombstone := Tombstone{
		KeyID:           keyID,
		DeletedAt:       deletedAt,
		DeletedByDevice: ks.vault.DeviceID(),
	}

	for i := range ks.vault.Data.Tombstones {
		if ks.vault.Data.Tombstones[i].KeyID != keyID {
			continue
		}
		if deletedAt.After(ks.vault.Data.Tombstones[i].DeletedAt) {
			ks.vault.Data.Tombstones[i] = tombstone
		}
		return
	}

	ks.vault.Data.Tombstones = append(ks.vault.Data.Tombstones, tombstone)
}

func cloneKeys(keys []Key) []Key {
	cloned := make([]Key, len(keys))
	for i := range keys {
		cloned[i] = cloneKey(keys[i])
	}
	return cloned
}

func cloneKey(key Key) Key {
	cloned := key
	cloned.Tags = append([]string(nil), key.Tags...)
	if key.LastUsedAt != nil {
		lastUsedAt := *key.LastUsedAt
		cloned.LastUsedAt = &lastUsedAt
	}
	return cloned
}

func cloneTombstones(tombstones []Tombstone) []Tombstone {
	cloned := make([]Tombstone, len(tombstones))
	copy(cloned, tombstones)
	return cloned
}

func cloneVersionVector(vector map[string]int64) map[string]int64 {
	cloned := make(map[string]int64, len(vector))
	for key, value := range vector {
		cloned[key] = value
	}
	return cloned
}
