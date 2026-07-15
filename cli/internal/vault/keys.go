package vault

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/itzzritik/forged/cli/internal/keytypes"
	"github.com/itzzritik/forged/cli/internal/platform"
	"golang.org/x/crypto/ssh"
)

var ErrKeyNotFound = errors.New("Key not found in vault")

type KeyStore struct {
	vault *Vault
}

func NewKeyStore(v *Vault) *KeyStore {
	return &KeyStore{vault: v}
}

func (ks *KeyStore) List() []Key {
	ks.vault.mu.RLock()
	defer ks.vault.mu.RUnlock()

	out := make([]Key, len(ks.vault.data.Keys))
	copy(out, ks.vault.data.Keys)
	for i := range out {
		out[i].Type = keytypes.Normalize(out[i].Type)
	}
	return out
}

func (ks *KeyStore) Get(name string) (Key, bool) {
	ks.vault.mu.RLock()
	defer ks.vault.mu.RUnlock()

	for _, k := range ks.vault.data.Keys {
		if k.Name == name {
			k.Type = keytypes.Normalize(k.Type)
			return k, true
		}
	}
	return Key{}, false
}

func (ks *KeyStore) ResolveName(input string) (string, error) {
	ks.vault.mu.RLock()
	defer ks.vault.mu.RUnlock()

	normalized := normalizeKeyName(input)
	if normalized == "" {
		return "", &KeyNameResolveError{Query: input}
	}

	matches := rankNameMatches(ks.vault.data.Keys, normalized)
	if len(matches) == 0 {
		suggestions, more := cappedSuggestions(suggestNameMatches(ks.vault.data.Keys, normalized))
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
	if err := validateKeyName(name); err != nil {
		return Key{}, err
	}
	ks.vault.mu.Lock()
	defer ks.vault.mu.Unlock()

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
		DeviceOrigin:        ks.vault.data.Metadata.DeviceID,
	}

	originalVersionVector := cloneVersionVector(ks.vault.data.VersionVector)
	storedKey := key
	storedKey.PrivateKey = nil
	ks.vault.data.Keys = append(ks.vault.data.Keys, storedKey)
	ks.bumpVersionVector()
	if err := ks.vault.saveLocked(); err != nil {
		ks.vault.data.Keys = ks.vault.data.Keys[:len(ks.vault.data.Keys)-1]
		ks.vault.data.VersionVector = originalVersionVector
		return Key{}, fmt.Errorf("Saving vault: %w", err)
	}
	for i := range privateKeyBytes {
		privateKeyBytes[i] = 0
	}
	key.PrivateKey = nil

	return key, nil
}

func (ks *KeyStore) Add(name string, privateKeyBytes []byte, comment string) (Key, error) {
	if err := validateKeyName(name); err != nil {
		return Key{}, err
	}
	ks.vault.mu.Lock()
	defer ks.vault.mu.Unlock()

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
		DeviceOrigin:        ks.vault.data.Metadata.DeviceID,
	}

	originalVersionVector := cloneVersionVector(ks.vault.data.VersionVector)
	storedKey := key
	storedKey.PrivateKey = nil
	ks.vault.data.Keys = append(ks.vault.data.Keys, storedKey)
	ks.bumpVersionVector()
	if err := ks.vault.saveLocked(); err != nil {
		ks.vault.data.Keys = ks.vault.data.Keys[:len(ks.vault.data.Keys)-1]
		ks.vault.data.VersionVector = originalVersionVector
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

func (ks *KeyStore) Remove(name string, expectedFingerprint string) error {
	ks.vault.mu.Lock()
	defer ks.vault.mu.Unlock()

	idx := ks.indexOf(name)
	if idx < 0 {
		return fmt.Errorf("Key %q not found", name)
	}
	if expected := strings.TrimSpace(expectedFingerprint); expected != "" && strings.TrimSpace(ks.vault.data.Keys[idx].Fingerprint) != expected {
		return fmt.Errorf("Key %q no longer matches the reviewed key. Go back and choose it again.", name)
	}

	originalVersionVector := cloneVersionVector(ks.vault.data.VersionVector)
	originalTombstones := cloneTombstones(ks.vault.data.Tombstones)
	removed := ks.vault.data.Keys[idx]
	ks.vault.data.Keys = append(ks.vault.data.Keys[:idx], ks.vault.data.Keys[idx+1:]...)
	now := time.Now().UTC()
	ks.upsertTombstone(removed.ID, now)
	ks.bumpVersionVector()

	if err := ks.vault.saveLocked(); err != nil {
		ks.vault.data.Keys = append(ks.vault.data.Keys[:idx], append([]Key{removed}, ks.vault.data.Keys[idx:]...)...)
		ks.vault.data.Tombstones = originalTombstones
		ks.vault.data.VersionVector = originalVersionVector
		return fmt.Errorf("Saving vault: %w", err)
	}

	return nil
}

func (ks *KeyStore) Rename(oldName, newName string) error {
	if err := validateKeyName(newName); err != nil {
		return err
	}
	ks.vault.mu.Lock()
	defer ks.vault.mu.Unlock()

	if ks.nameExists(newName) {
		return fmt.Errorf("Key %q already exists", newName)
	}

	idx := ks.indexOf(oldName)
	if idx < 0 {
		return fmt.Errorf("Key %q not found", oldName)
	}

	original := cloneKey(ks.vault.data.Keys[idx])
	originalVersionVector := cloneVersionVector(ks.vault.data.VersionVector)
	ks.vault.data.Keys[idx].Name = newName
	ks.vault.data.Keys[idx].UpdatedAt = time.Now().UTC()
	ks.vault.data.Keys[idx].Version++
	ks.bumpVersionVector()

	if err := ks.vault.saveLocked(); err != nil {
		ks.vault.data.Keys[idx] = original
		ks.vault.data.VersionVector = originalVersionVector
		return fmt.Errorf("Saving vault: %w", err)
	}

	return nil
}

func (ks *KeyStore) Export(name string) (string, error) {
	ks.vault.mu.RLock()
	defer ks.vault.mu.RUnlock()

	for _, k := range ks.vault.data.Keys {
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
	ks.vault.mu.RLock()
	defer ks.vault.mu.RUnlock()

	idx := ks.indexOf(name)
	if idx < 0 {
		return nil, fmt.Errorf("Key %q not found", name)
	}
	return ks.decryptPrivateKeyLocked(&ks.vault.data.Keys[idx])
}

func (ks *KeyStore) RecordUsage(name string) {
	ks.vault.mu.Lock()
	defer ks.vault.mu.Unlock()
	if ks.vault.closed {
		return
	}

	idx := ks.indexOf(name)
	if idx < 0 {
		return
	}
	now := time.Now().UTC()
	ks.vault.data.Keys[idx].LastUsedAt = &now
}

func (ks *KeyStore) SetGitSigning(keyName string, enabled bool) error {
	ks.vault.mu.Lock()
	defer ks.vault.mu.Unlock()

	idx := ks.indexOf(keyName)
	if idx < 0 {
		return fmt.Errorf("Key %q not found", keyName)
	}

	originalKeys := cloneKeys(ks.vault.data.Keys)
	originalVersionVector := cloneVersionVector(ks.vault.data.VersionVector)
	now := time.Now().UTC()
	changed := false
	if enabled {
		for i := range ks.vault.data.Keys {
			if i != idx && ks.vault.data.Keys[i].GitSigning {
				ks.vault.data.Keys[i].GitSigning = false
				ks.vault.data.Keys[i].UpdatedAt = now
				ks.vault.data.Keys[i].Version++
				changed = true
			}
		}
	}

	if ks.vault.data.Keys[idx].GitSigning != enabled {
		ks.vault.data.Keys[idx].GitSigning = enabled
		ks.vault.data.Keys[idx].UpdatedAt = now
		ks.vault.data.Keys[idx].Version++
		changed = true
	}

	if !changed {
		return nil
	}

	ks.bumpVersionVector()
	if err := ks.vault.saveLocked(); err != nil {
		ks.vault.data.Keys = originalKeys
		ks.vault.data.VersionVector = originalVersionVector
		return err
	}
	return nil
}

func (ks *KeyStore) GetGitSigningKey() (Key, bool) {
	ks.vault.mu.RLock()
	defer ks.vault.mu.RUnlock()

	for _, k := range ks.vault.data.Keys {
		if k.GitSigning {
			return k, true
		}
	}
	return Key{}, false
}

func (ks *KeyStore) SignerByPublicKey(pub ssh.PublicKey) (ssh.Signer, string, string, error) {
	ks.vault.mu.RLock()
	defer ks.vault.mu.RUnlock()

	if ks.vault == nil {
		return nil, "", "", fmt.Errorf("Vault is locked")
	}

	requested, err := ssh.ParsePublicKey(pub.Marshal())
	if err != nil {
		return nil, "", "", fmt.Errorf("Parsing requested public key: %w", err)
	}
	if !keytypes.SupportsSSHSigning(requested.Type()) {
		return nil, "", "", fmt.Errorf("Unsupported public key type %q", requested.Type())
	}

	wanted := requested.Marshal()
	for i := range ks.vault.data.Keys {
		key := &ks.vault.data.Keys[i]
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
		if !keytypes.SupportsSSHSigning(signer.PublicKey().Type()) {
			return nil, "", "", fmt.Errorf("Unsupported private key type %q", signer.PublicKey().Type())
		}
		return signer, key.Name, key.Fingerprint, nil
	}
	return nil, "", "", ErrKeyNotFound
}

func (ks *KeyStore) Signers() ([]ssh.Signer, error) {
	ks.vault.mu.RLock()
	defer ks.vault.mu.RUnlock()

	if ks.vault == nil {
		return nil, fmt.Errorf("Vault is locked")
	}

	signers := make([]ssh.Signer, 0, len(ks.vault.data.Keys))
	for i := range ks.vault.data.Keys {
		publicKey, err := parseAuthorizedPublicKey(ks.vault.data.Keys[i].PublicKey)
		if err == nil && !keytypes.SupportsSSHSigning(publicKey.Type()) {
			continue
		}
		privateKey, err := ks.decryptPrivateKeyLocked(&ks.vault.data.Keys[i])
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
			return nil, fmt.Errorf("Parsing private key for %s: %w", ks.vault.data.Keys[i].Name, err)
		}
		if !keytypes.SupportsSSHSigning(signer.PublicKey().Type()) {
			continue
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
	for i, k := range ks.vault.data.Keys {
		if k.Name == name {
			return i
		}
	}
	return -1
}

func validateKeyName(name string) error {
	if strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return fmt.Errorf("Key name contains control characters")
	}
	return nil
}

func (ks *KeyStore) bumpVersionVector() {
	deviceID := ks.vault.data.Metadata.DeviceID
	if deviceID == "" {
		return
	}

	if ks.vault.data.VersionVector == nil {
		ks.vault.data.VersionVector = map[string]int64{}
	}
	ks.vault.data.VersionVector[deviceID]++
}

func (ks *KeyStore) upsertTombstone(keyID string, deletedAt time.Time) {
	tombstone := Tombstone{
		KeyID:           keyID,
		DeletedAt:       deletedAt,
		DeletedByDevice: ks.vault.data.Metadata.DeviceID,
	}

	for i := range ks.vault.data.Tombstones {
		if ks.vault.data.Tombstones[i].KeyID != keyID {
			continue
		}
		if deletedAt.After(ks.vault.data.Tombstones[i].DeletedAt) {
			ks.vault.data.Tombstones[i] = tombstone
		}
		return
	}

	ks.vault.data.Tombstones = append(ks.vault.data.Tombstones, tombstone)
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
