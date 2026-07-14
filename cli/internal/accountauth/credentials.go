package accountauth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/platform"
)

var ErrLoginRequired = errors.New("log-in required")

// refreshMu serializes the load → refresh → save sequence in EnsureFresh so
// two callers in the same process never present the same refresh token to the
// server. The server has a 30s grace window for honest replays, but
// serializing here removes most of the race entirely.
var refreshMu sync.Mutex

// memCreds holds the most-recent refresh result in memory. If a successful
// rotation fails to persist (disk full, keychain locked, OS suspended
// mid-write), the new tokens still survive in this process: subsequent
// EnsureFresh calls use the in-memory copy instead of re-presenting the
// already-rotated disk token. The cache lives for the lifetime of the daemon
// process; on restart we fall back to disk, accepting one possible re-login
// in the very rare crash-during-save case.
type cachedCredentials struct {
	Path        string
	Credentials Credentials
	UpdatedAt   time.Time
}

var memCreds atomic.Pointer[cachedCredentials]

// refresh401RetryDelay is the pause before retrying a 401 once. Pairs with
// the server-side RefreshGracePeriod: if the first call's response was lost
// in flight, the server has already rotated and cached the new pair under
// our presented secret; the retry hits the cache and we get the new tokens
// back instead of being family-revoked.
const refresh401RetryDelay = 350 * time.Millisecond

const (
	accountIdentityVersion = 2
	credentialSlotA        = "-v2-a"
	credentialSlotB        = "-v2-b"
)

type Credentials struct {
	ServerURL        string    `json:"server_url"`
	Token            string    `json:"token,omitempty"`
	AccessToken      string    `json:"access_token,omitempty"`
	AccessExpiresAt  time.Time `json:"access_expires_at,omitempty"`
	RefreshToken     string    `json:"refresh_token,omitempty"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at,omitempty"`
	UserID           string    `json:"user_id"`
	Email            string    `json:"email"`
	Name             string    `json:"name,omitempty"`
	ChangeID         string    `json:"-"`
}

type accountMetadata struct {
	Version           int       `json:"version"`
	ServerURL         string    `json:"server_url"`
	CredentialID      string    `json:"credential_id"`
	CredentialBackend string    `json:"credential_backend"`
	AccessExpiresAt   time.Time `json:"access_expires_at,omitempty"`
	RefreshExpiresAt  time.Time `json:"refresh_expires_at,omitempty"`
	UserID            string    `json:"user_id"`
	Email             string    `json:"email"`
	Name              string    `json:"name,omitempty"`
	ChangeID          string    `json:"change_id,omitempty"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type credentialSecret struct {
	Version      int    `json:"version"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ServerURL    string `json:"server_url,omitempty"`
	UserID       string `json:"user_id,omitempty"`
}

func CredentialsPath(paths config.Paths) string {
	return paths.CredentialsFile()
}

func Load(paths config.Paths) (Credentials, error) {
	var creds Credentials
	err := WithCredentialsLock(paths, func() error {
		var loadErr error
		creds, loadErr = LoadLocked(paths)
		return loadErr
	})
	return creds, err
}

// LoadLocked reads credentials while the caller holds WithCredentialsLock.
func LoadLocked(paths config.Paths) (Credentials, error) {
	return loadCredentials(paths)
}

func loadCredentials(paths config.Paths) (Credentials, error) {
	metadata, err := readMetadata(CredentialsPath(paths))
	if err != nil {
		return Credentials{}, err
	}
	requireIdentity := metadata.Version >= accountIdentityVersion
	if _, err := os.Stat(accountIdentityMarkerPath(paths)); err == nil {
		requireIdentity = true
	} else if !os.IsNotExist(err) {
		return Credentials{}, fmt.Errorf("Reading account identity marker: %w", err)
	}
	if requireIdentity && metadata.Version < accountIdentityVersion {
		return Credentials{}, fmt.Errorf("Account credentials use an outdated identity format: %w", ErrCredentialStoreBroken)
	}
	if cached := memCreds.Load(); cached != nil && cached.Path == CredentialsPath(paths) && credsAreFresher(*cached, metadata) {
		return cached.Credentials, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	secret, err := storeForBackend(paths, metadata.CredentialBackend).Load(ctx, metadata.CredentialID)
	if errors.Is(err, ErrCredentialSecretNotFound) {
		return Credentials{}, ErrLoginRequired
	}
	if err != nil {
		return Credentials{}, fmt.Errorf("Loading account secret: %w", err)
	}
	if requireIdentity {
		if secret.Version < accountIdentityVersion || secret.ServerURL == "" || secret.UserID == "" {
			return Credentials{}, fmt.Errorf("Account credentials use an outdated identity format: %w", ErrCredentialStoreBroken)
		}
	}
	if secret.ServerURL != "" && strings.TrimRight(strings.TrimSpace(secret.ServerURL), "/") != strings.TrimRight(metadata.ServerURL, "/") {
		return Credentials{}, fmt.Errorf("Account secret does not match account metadata: %w", ErrCredentialStoreBroken)
	}
	if secret.UserID != "" && strings.TrimSpace(secret.UserID) != metadata.UserID {
		return Credentials{}, fmt.Errorf("Account secret does not match account metadata: %w", ErrCredentialStoreBroken)
	}

	creds := Credentials{
		ServerURL:        metadata.ServerURL,
		Token:            secret.AccessToken,
		AccessToken:      secret.AccessToken,
		AccessExpiresAt:  metadata.AccessExpiresAt,
		RefreshToken:     secret.RefreshToken,
		RefreshExpiresAt: metadata.RefreshExpiresAt,
		UserID:           metadata.UserID,
		Email:            metadata.Email,
		Name:             metadata.Name,
		ChangeID:         metadata.ChangeID,
	}
	normalizeCredentials(&creds)
	return creds, nil
}

// credsAreFresher reports whether the in-memory cache's refresh token is
// newer than what's on disk. Used to ensure a save-failed cache entry isn't
// discarded by a stale disk read.
func credsAreFresher(cached cachedCredentials, metadata accountMetadata) bool {
	creds := cached.Credentials
	if strings.TrimSpace(creds.RefreshToken) == "" {
		return false
	}
	if strings.TrimRight(creds.ServerURL, "/") != strings.TrimRight(metadata.ServerURL, "/") || creds.UserID != metadata.UserID {
		return false
	}
	if creds.ChangeID != metadata.ChangeID {
		return false
	}
	return cached.UpdatedAt.After(metadata.UpdatedAt) || creds.RefreshExpiresAt.After(metadata.RefreshExpiresAt)
}

func Save(paths config.Paths, creds Credentials) error {
	return WithCredentialsLock(paths, func() error {
		return SaveLocked(paths, creds)
	})
}

// SaveLocked writes credentials while the caller holds WithCredentialsLock.
func SaveLocked(paths config.Paths, creds Credentials) error {
	return saveCredentials(paths, creds)
}

func ValidateCredentials(creds Credentials) error {
	normalizeCredentials(&creds)
	return validateCredentialsForSave(creds)
}

func saveCredentials(paths config.Paths, creds Credentials) error {
	normalizeCredentials(&creds)
	if err := validateCredentialsForSave(creds); err != nil {
		return err
	}

	credentialID, oldMetadata, err := credentialIDForSave(paths)
	if err != nil {
		return err
	}

	secret := credentialSecret{
		Version:      accountIdentityVersion,
		AccessToken:  creds.AccessToken,
		RefreshToken: creds.RefreshToken,
		ServerURL:    creds.ServerURL,
		UserID:       creds.UserID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	store := preferredCredentialStore(ctx, paths)
	if err := store.Save(ctx, credentialID, secret); err != nil && store.Backend() != backendEncryptedFile {
		store = newFileCredentialStore(paths)
		if fallbackErr := store.Save(ctx, credentialID, secret); fallbackErr != nil {
			return fmt.Errorf("Saving account secret: %w", errors.Join(err, fallbackErr))
		}
	} else if err != nil {
		return fmt.Errorf("Saving account secret: %w", err)
	}

	metadata := accountMetadata{
		Version:           accountIdentityVersion,
		ServerURL:         creds.ServerURL,
		CredentialID:      credentialID,
		CredentialBackend: store.Backend(),
		AccessExpiresAt:   creds.AccessExpiresAt,
		RefreshExpiresAt:  creds.RefreshExpiresAt,
		UserID:            creds.UserID,
		Email:             creds.Email,
		Name:              creds.Name,
		ChangeID:          creds.ChangeID,
		UpdatedAt:         time.Now().UTC(),
	}
	_, markerErr := os.Stat(accountIdentityMarkerPath(paths))
	markerExisted := markerErr == nil
	if markerErr != nil && !os.IsNotExist(markerErr) {
		return fmt.Errorf("Reading account identity marker: %w", markerErr)
	}
	if err := writePrivateFile(accountIdentityMarkerPath(paths), []byte("2\n")); err != nil {
		return fmt.Errorf("Writing account identity marker: %w", err)
	}
	if err := writeMetadata(CredentialsPath(paths), metadata); err != nil {
		if !markerExisted {
			_ = os.Remove(accountIdentityMarkerPath(paths))
		}
		return err
	}

	if oldMetadata != nil && (oldMetadata.CredentialID != metadata.CredentialID || oldMetadata.CredentialBackend != metadata.CredentialBackend) {
		_ = storeForBackend(paths, oldMetadata.CredentialBackend).Delete(ctx, oldMetadata.CredentialID)
	}
	cacheCredentials(paths, creds, metadata.UpdatedAt)
	_ = os.Remove(paths.LegacyCredentialsFile())
	return nil
}

func Delete(paths config.Paths) error {
	return WithCredentialsLock(paths, func() error {
		return DeleteLocked(paths)
	})
}

// DeleteLocked removes credentials while the caller holds WithCredentialsLock.
func DeleteLocked(paths config.Paths) error {
	return deleteCredentials(paths)
}

func deleteCredentials(paths config.Paths) error {
	var metadata *accountMetadata
	if current, err := readMetadata(CredentialsPath(paths)); err == nil {
		metadata = &current
	} else if !os.IsNotExist(err) && !errors.Is(err, ErrLoginRequired) {
		slog.Warn("account metadata is unreadable; removing it and known credential slots", "error", err)
	}

	if err := os.Remove(CredentialsPath(paths)); err != nil && !os.IsNotExist(err) {
		return err
	}
	clearCachedCredentials(paths)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ids := credentialIDsForDelete(paths, metadata)
	stores := []credentialStore{newFileCredentialStore(paths)}
	if metadata != nil {
		stores = append(stores, storeForBackend(paths, metadata.CredentialBackend))
	}
	if platformStore := newPlatformCredentialStore(paths); platformStore != nil && platformStore.Available(ctx) {
		stores = append(stores, platformStore)
	}
	var cleanupErr error
	for _, store := range stores {
		for _, id := range ids {
			if err := store.Delete(ctx, id); err != nil &&
				!errors.Is(err, ErrCredentialSecretNotFound) &&
				!errors.Is(err, ErrCredentialStoreUnavailable) {
				cleanupErr = errors.Join(cleanupErr, err)
			}
		}
	}

	for _, path := range []string{paths.LegacyCredentialsFile(), paths.AccountSecretKeyFile()} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}
	if cleanupErr != nil {
		slog.Error("deleting retired account secrets failed", "error", cleanupErr)
	}
	return nil
}

func EnsureFresh(ctx context.Context, paths config.Paths) (Credentials, error) {
	refreshMu.Lock()
	defer refreshMu.Unlock()

	var refreshed Credentials
	err := WithCredentialsLock(paths, func() error {
		var err error
		refreshed, err = ensureFreshLocked(ctx, paths)
		return err
	})
	return refreshed, err
}

func ensureFreshLocked(ctx context.Context, paths config.Paths) (Credentials, error) {

	creds, err := loadCredentials(paths)
	if err != nil {
		return Credentials{}, err
	}
	if !NeedsRefresh(creds, time.Now()) {
		return creds, nil
	}
	if creds.RefreshToken == "" || (!creds.RefreshExpiresAt.IsZero() && time.Now().After(creds.RefreshExpiresAt)) {
		return Credentials{}, ErrLoginRequired
	}

	refreshed, err := refresh(ctx, creds)
	if errors.Is(err, ErrLoginRequired) {
		// One retry: a 401 here can mean an in-flight rotation whose
		// response we lost. The server keeps a short grace window for
		// the same presented secret. If we're in that window, the
		// retry returns the freshly-rotated pair instead of staying
		// stuck.
		time.Sleep(refresh401RetryDelay)
		refreshed, err = refresh(ctx, creds)
	}
	if err != nil {
		return Credentials{}, err
	}

	// Publish to the in-memory cache FIRST so a failed disk save doesn't
	// leave the next caller presenting the now-revoked refresh token. The
	// server has already committed the rotation by this point; the new
	// tokens are the only ones that work.
	cacheCredentials(paths, refreshed, time.Now().UTC())

	if err := saveCredentials(paths, refreshed); err != nil {
		// Don't return the error — we have the new tokens in memory.
		// Returning here would make the caller treat this as a failure
		// even though sync can proceed. Log loudly so the issue is
		// visible to anyone reading the daemon log.
		slog.Error("saving refreshed credentials failed; running from in-memory tokens",
			"error", err)
	}
	return refreshed, nil
}

func NeedsRefresh(creds Credentials, now time.Time) bool {
	if creds.AccessToken == "" && creds.Token == "" {
		return true
	}
	if creds.AccessExpiresAt.IsZero() {
		return false
	}
	return !creds.AccessExpiresAt.After(now.Add(time.Minute))
}

func CurrentToken(creds Credentials) string {
	if strings.TrimSpace(creds.AccessToken) != "" {
		return strings.TrimSpace(creds.AccessToken)
	}
	return strings.TrimSpace(creds.Token)
}

func RevokeRemoteSession(ctx context.Context, creds Credentials) error {
	if strings.TrimSpace(creds.ServerURL) == "" || strings.TrimSpace(creds.RefreshToken) == "" {
		return nil
	}
	body, err := json.Marshal(map[string]string{"refresh_token": creds.RefreshToken})
	if err != nil {
		return fmt.Errorf("Encoding logout request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(creds.ServerURL, "/")+"/api/v1/auth/logout", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("Creating logout request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("Revoking remote session: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("Revoking remote session failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func normalizeCredentials(creds *Credentials) {
	if creds == nil {
		return
	}
	creds.ServerURL = strings.TrimSpace(creds.ServerURL)
	creds.Email = strings.TrimSpace(creds.Email)
	creds.UserID = strings.TrimSpace(creds.UserID)
	creds.Name = strings.TrimSpace(creds.Name)
	creds.RefreshToken = strings.TrimSpace(creds.RefreshToken)
	creds.ChangeID = strings.TrimSpace(creds.ChangeID)

	if token := strings.TrimSpace(creds.AccessToken); token != "" {
		creds.AccessToken = token
		creds.Token = token
	} else {
		creds.Token = strings.TrimSpace(creds.Token)
		creds.AccessToken = creds.Token
	}

	if creds.Name == "" {
		if name := decodeAccountName(creds.AccessToken); name != "" {
			creds.Name = name
		} else {
			creds.Name = fallbackAccountName(creds.Email)
		}
	}
}

func validateCredentialsForSave(creds Credentials) error {
	if creds.ServerURL == "" {
		return fmt.Errorf("Saving account credentials: server URL is required")
	}
	if creds.UserID == "" {
		return fmt.Errorf("Saving account credentials: user ID is required")
	}
	if CurrentToken(creds) == "" {
		return fmt.Errorf("Saving account credentials: access token is required")
	}
	return nil
}

func cacheCredentials(paths config.Paths, creds Credentials, updatedAt time.Time) {
	memCreds.Store(&cachedCredentials{
		Path:        CredentialsPath(paths),
		Credentials: creds,
		UpdatedAt:   updatedAt,
	})
}

func clearCachedCredentials(paths config.Paths) {
	cached := memCreds.Load()
	if cached != nil && cached.Path == CredentialsPath(paths) {
		memCreds.CompareAndSwap(cached, nil)
	}
}

func readMetadata(path string) (accountMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return accountMetadata{}, err
	}

	var metadata accountMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return accountMetadata{}, fmt.Errorf("Parsing account metadata: %w", err)
	}
	metadata.ServerURL = strings.TrimSpace(metadata.ServerURL)
	metadata.CredentialID = strings.TrimSpace(metadata.CredentialID)
	metadata.CredentialBackend = strings.TrimSpace(metadata.CredentialBackend)
	metadata.UserID = strings.TrimSpace(metadata.UserID)
	metadata.Email = strings.TrimSpace(metadata.Email)
	metadata.Name = strings.TrimSpace(metadata.Name)
	metadata.ChangeID = strings.TrimSpace(metadata.ChangeID)
	if metadata.ServerURL == "" || metadata.CredentialID == "" || metadata.UserID == "" {
		return accountMetadata{}, ErrLoginRequired
	}
	return metadata, nil
}

func writeMetadata(path string, metadata accountMetadata) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("Creating account directory: %w", err)
	}

	body, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("Serializing account metadata: %w", err)
	}
	return writePrivateFile(path, body)
}

func credentialIDForSave(paths config.Paths) (string, *accountMetadata, error) {
	var oldMetadata *accountMetadata
	if metadata, err := readMetadata(CredentialsPath(paths)); err == nil {
		oldMetadata = &metadata
	} else if !os.IsNotExist(err) && !errors.Is(err, ErrLoginRequired) {
		return "", nil, err
	}

	installID, err := config.LoadOrCreateInstallID(paths)
	if err != nil {
		return "", nil, err
	}
	baseID := "account-" + installID
	nextID := baseID + credentialSlotA
	if oldMetadata != nil && oldMetadata.CredentialID == nextID {
		nextID = baseID + credentialSlotB
	}
	return nextID, oldMetadata, nil
}

func accountIdentityMarkerPath(paths config.Paths) string {
	return CredentialsPath(paths) + ".identity-v2"
}

func WithCredentialsLock(paths config.Paths, fn func() error) error {
	if err := os.MkdirAll(paths.AuthDir(), 0o700); err != nil {
		return fmt.Errorf("Creating account directory: %w", err)
	}
	f, err := os.OpenFile(CredentialsPath(paths)+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("Opening account lock: %w", err)
	}
	defer f.Close()
	if err := platform.LockFileWait(f); err != nil {
		return fmt.Errorf("Locking account credentials: %w", err)
	}
	defer platform.UnlockFile(f)
	return fn()
}

func WithCredentials(paths config.Paths, fn func(Credentials) error) error {
	return WithCredentialsLock(paths, func() error {
		creds, err := loadCredentials(paths)
		if err != nil {
			return err
		}
		return fn(creds)
	})
}

func credentialIDsForDelete(paths config.Paths, metadata *accountMetadata) []string {
	baseID := ""
	ids := make([]string, 0, 4)
	if metadata != nil {
		ids = append(ids, metadata.CredentialID)
		baseID = strings.TrimSuffix(strings.TrimSuffix(metadata.CredentialID, credentialSlotA), credentialSlotB)
	}
	if data, err := os.ReadFile(paths.InstallIDFile()); err == nil {
		if installID := strings.TrimSpace(string(data)); installID != "" {
			baseID = "account-" + installID
		}
	}
	if baseID == "" {
		return ids
	}
	for _, id := range []string{baseID, baseID + credentialSlotA, baseID + credentialSlotB} {
		seen := false
		for _, existing := range ids {
			if existing == id {
				seen = true
				break
			}
		}
		if !seen {
			ids = append(ids, id)
		}
	}
	return ids
}

func credentialSecretPath(legacyPath, credentialID string) string {
	slot := ""
	switch {
	case strings.HasSuffix(credentialID, credentialSlotA):
		slot = "-a"
	case strings.HasSuffix(credentialID, credentialSlotB):
		slot = "-b"
	default:
		return legacyPath
	}
	ext := filepath.Ext(legacyPath)
	return strings.TrimSuffix(legacyPath, ext) + slot + ext
}

func writePrivateFile(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func refresh(ctx context.Context, creds Credentials) (Credentials, error) {
	body, _ := json.Marshal(map[string]string{
		"refresh_token": creds.RefreshToken,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(creds.ServerURL, "/")+"/api/v1/auth/refresh", bytes.NewReader(body))
	if err != nil {
		return Credentials{}, fmt.Errorf("Creating refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return Credentials{}, fmt.Errorf("Refreshing credentials: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return Credentials{}, ErrLoginRequired
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return Credentials{}, fmt.Errorf("Refreshing credentials failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result struct {
		AccessToken      string `json:"access_token"`
		AccessExpiresAt  string `json:"access_expires_at"`
		RefreshToken     string `json:"refresh_token"`
		RefreshExpiresAt string `json:"refresh_expires_at"`
		UserID           string `json:"user_id"`
		Email            string `json:"email"`
		Name             string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return Credentials{}, fmt.Errorf("Decoding refresh response: %w", err)
	}

	accessExpiry, err := parseOptionalTime(result.AccessExpiresAt)
	if err != nil {
		return Credentials{}, fmt.Errorf("Parsing access expiry: %w", err)
	}
	refreshExpiry, err := parseOptionalTime(result.RefreshExpiresAt)
	if err != nil {
		return Credentials{}, fmt.Errorf("Parsing refresh expiry: %w", err)
	}

	refreshed := Credentials{
		ServerURL:        creds.ServerURL,
		AccessToken:      result.AccessToken,
		Token:            result.AccessToken,
		AccessExpiresAt:  accessExpiry,
		RefreshToken:     result.RefreshToken,
		RefreshExpiresAt: refreshExpiry,
		UserID:           result.UserID,
		Email:            result.Email,
		Name:             result.Name,
		ChangeID:         creds.ChangeID,
	}
	normalizeCredentials(&refreshed)
	if refreshed.UserID == "" || refreshed.UserID != creds.UserID {
		return Credentials{}, fmt.Errorf("Refreshing credentials returned a different account identity")
	}
	if CurrentToken(refreshed) == "" || refreshed.RefreshToken == "" {
		return Credentials{}, fmt.Errorf("Refreshing credentials returned incomplete tokens")
	}
	return refreshed, nil
}

func parseOptionalTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, value)
}

func decodeAccountName(token string) string {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) < 2 {
		return ""
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}

	var claims struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	return strings.TrimSpace(claims.Name)
}

func fallbackAccountName(email string) string {
	local := strings.TrimSpace(email)
	if local == "" {
		return ""
	}
	if at := strings.Index(local, "@"); at > 0 {
		local = local[:at]
	}
	local = strings.ReplaceAll(local, ".", " ")
	local = strings.ReplaceAll(local, "_", " ")
	local = strings.ReplaceAll(local, "-", " ")
	words := strings.Fields(local)
	if len(words) == 0 {
		return ""
	}
	for index, word := range words {
		runes := []rune(strings.ToLower(word))
		if len(runes) == 0 {
			continue
		}
		runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
		words[index] = string(runes)
	}
	return strings.Join(words, " ")
}

func CodeChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
