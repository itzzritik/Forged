package actions

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/daemon"
	"github.com/itzzritik/forged/cli/internal/ipc"
	"github.com/itzzritik/forged/cli/internal/keytypes"
	"github.com/itzzritik/forged/cli/internal/sensitiveauth"
	"github.com/itzzritik/forged/cli/internal/vault"
)

type KeySummary struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Fingerprint string `json:"fingerprint"`
	Comment     string `json:"comment,omitempty"`
}

type KeyDetail struct {
	ResolvedName string `json:"resolved_name"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	Fingerprint  string `json:"fingerprint"`
	PublicKey    string `json:"public_key"`
	PrivateKey   string `json:"private_key,omitempty"`
	Comment      string `json:"comment,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
	UpdatedAt    string `json:"updated_at,omitempty"`
	LastUsedAt   string `json:"last_used_at,omitempty"`
	Version      int    `json:"version,omitempty"`
	DeviceOrigin string `json:"device_origin,omitempty"`
	GitSigning   bool   `json:"git_signing,omitempty"`
}

type RenameResult struct {
	OldName string
	NewName string
}

type GenerateResult struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Fingerprint string `json:"fingerprint"`
	PublicKey   string `json:"public_key"`
	Comment     string `json:"comment,omitempty"`
}

type SensitiveAuthRequiredError struct {
	Prompt string
}

func (e *SensitiveAuthRequiredError) Error() string {
	if strings.TrimSpace(e.Prompt) != "" {
		return e.Prompt
	}
	return "sensitive authentication requires a password"
}

func IsSensitiveAuthRequired(err error) bool {
	var target *SensitiveAuthRequiredError
	return errors.As(err, &target)
}

type KeyQueryResolution struct {
	Exact   *KeySummary
	Matches []KeySummary
}

func ListKeys(paths config.Paths) ([]KeySummary, error) {
	resp, err := ipc.NewClient(paths.CtlSocket()).Call(ipc.CmdList, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Keys []KeySummary `json:"keys"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("parsing key list: %w", err)
	}
	for i := range result.Keys {
		result.Keys[i].Type = keytypes.Normalize(result.Keys[i].Type)
	}
	return result.Keys, nil
}

func ListLocalKeys(paths config.Paths) ([]KeySummary, error) {
	password, err := daemon.ReadInstalledServicePassword()
	if err != nil {
		return nil, err
	}

	v, err := vault.OpenReadOnly(paths.VaultFile(), []byte(password))
	if err != nil {
		return nil, err
	}
	defer v.Close()

	keys := vault.NewKeyStore(v).List()
	out := make([]KeySummary, len(keys))
	for i, key := range keys {
		out[i] = KeySummary{
			Name:        key.Name,
			Type:        keytypes.Normalize(key.Type),
			Fingerprint: key.Fingerprint,
			Comment:     key.Comment,
		}
	}
	return out, nil
}

func ViewKey(paths config.Paths, name string) (KeyDetail, error) {
	return viewKey(paths, name, false)
}

func ViewFullKey(paths config.Paths, name string, password []byte) (KeyDetail, error) {
	if _, err := authorizeSensitiveResult(paths, sensitiveauth.ActionView, password); err != nil {
		return KeyDetail{}, err
	}
	return viewKey(paths, name, true)
}

func ExportPublicKey(paths config.Paths, name string) (GenerateResult, error) {
	resp, err := ipc.NewClient(paths.CtlSocket()).Call(ipc.CmdExport, map[string]any{
		"name": name,
	})
	if err != nil {
		return GenerateResult{}, err
	}

	var result struct {
		PublicKey    string `json:"public_key"`
		ResolvedName string `json:"resolved_name"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return GenerateResult{}, fmt.Errorf("parsing key export: %w", err)
	}

	detail, err := ViewKey(paths, result.ResolvedName)
	if err != nil {
		return GenerateResult{}, err
	}

	return GenerateResult{
		Name:        detail.Name,
		Type:        detail.Type,
		Fingerprint: detail.Fingerprint,
		PublicKey:   result.PublicKey,
		Comment:     detail.Comment,
	}, nil
}

func GenerateKey(paths config.Paths, name, comment string) (GenerateResult, error) {
	resp, err := ipc.NewClient(paths.CtlSocket()).Call(ipc.CmdGenerate, map[string]string{
		"name":    name,
		"comment": comment,
	})
	if err != nil {
		return GenerateResult{}, err
	}

	var result GenerateResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return GenerateResult{}, fmt.Errorf("parsing generate result: %w", err)
	}
	result.Type = keytypes.Normalize(result.Type)
	result.Comment = comment
	return result, nil
}

func viewKey(paths config.Paths, name string, full bool) (KeyDetail, error) {
	resp, err := ipc.NewClient(paths.CtlSocket()).Call(ipc.CmdView, map[string]any{
		"name": name,
		"full": full,
	})
	if err != nil {
		return KeyDetail{}, err
	}

	var result KeyDetail
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return KeyDetail{}, fmt.Errorf("parsing key detail: %w", err)
	}
	result.Type = keytypes.Normalize(result.Type)
	return result, nil
}

func authorizeSensitive(paths config.Paths, action sensitiveauth.Action, password []byte) error {
	_, err := authorizeSensitiveResult(paths, action, password)
	return err
}

func authorizeSensitiveResult(paths config.Paths, action sensitiveauth.Action, password []byte) (sensitiveauth.AuthorizeResult, error) {
	client := ipc.NewClient(paths.CtlSocket())
	parseResult := func(raw json.RawMessage) (sensitiveauth.AuthorizeResult, error) {
		var result sensitiveauth.AuthorizeResult
		if err := json.Unmarshal(raw, &result); err != nil {
			return sensitiveauth.AuthorizeResult{}, fmt.Errorf("parsing auth response: %w", err)
		}
		return result, nil
	}

	if len(password) == 0 {
		resp, err := client.CallWithTimeout(ipc.CmdSensitiveAuth, map[string]string{
			"action": string(action),
		}, 5*60*1e9)
		if err != nil {
			return sensitiveauth.AuthorizeResult{}, err
		}
		result, err := parseResult(resp.Data)
		if err != nil {
			return sensitiveauth.AuthorizeResult{}, err
		}
		if result.PasswordRequired {
			return sensitiveauth.AuthorizeResult{}, &SensitiveAuthRequiredError{Prompt: result.Prompt}
		}
		return result, nil
	}

	resp, err := client.CallWithTimeout(ipc.CmdSensitivePassword, map[string]string{
		"action":   string(action),
		"password": string(password),
	}, 5*60*1e9)
	if err != nil {
		return sensitiveauth.AuthorizeResult{}, err
	}
	return parseResult(resp.Data)
}

func RenameKey(paths config.Paths, oldName, newName string) (RenameResult, error) {
	resp, err := ipc.NewClient(paths.CtlSocket()).Call(ipc.CmdRename, map[string]string{
		"old_name": oldName,
		"new_name": newName,
	})
	if err != nil {
		return RenameResult{}, err
	}

	var result struct {
		OldName string `json:"old_name"`
		NewName string `json:"new_name"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return RenameResult{}, fmt.Errorf("parsing rename result: %w", err)
	}
	return RenameResult{OldName: result.OldName, NewName: result.NewName}, nil
}

func DeleteKey(paths config.Paths, name string) (string, error) {
	resp, err := ipc.NewClient(paths.CtlSocket()).Call(ipc.CmdRemove, map[string]string{
		"name": name,
	})
	if err != nil {
		return "", err
	}

	var result struct {
		ResolvedName string `json:"resolved_name"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return "", fmt.Errorf("parsing delete result: %w", err)
	}
	if strings.TrimSpace(result.ResolvedName) == "" {
		return name, nil
	}
	return result.ResolvedName, nil
}

func ResolveKeyQuery(keys []KeySummary, query string) KeyQueryResolution {
	normalized := normalizeKeyQuery(query)
	if normalized == "" {
		return KeyQueryResolution{Matches: cloneKeySummaries(keys)}
	}

	if exact := exactKeyMatch(keys, normalized); exact != nil {
		return KeyQueryResolution{Exact: exact, Matches: []KeySummary{*exact}}
	}

	matches := rankKeyMatches(keys, normalized)
	if len(matches) == 0 {
		matches = suggestKeyMatches(keys, normalized)
	}
	return KeyQueryResolution{Matches: matches}
}

func normalizeKeyQuery(input string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(input)), " ")
}

func exactKeyMatch(keys []KeySummary, normalized string) *KeySummary {
	folded := strings.ToLower(normalized)
	for _, key := range keys {
		if normalizeKeyQuery(key.Name) == normalized {
			match := key
			return &match
		}
	}
	for _, key := range keys {
		if strings.ToLower(normalizeKeyQuery(key.Name)) == folded {
			match := key
			return &match
		}
	}
	return nil
}

type keyMatchKind int

const (
	keyMatchPrefix keyMatchKind = iota
	keyMatchToken
	keyMatchSubsequence
)

type keyMatch struct {
	key   KeySummary
	kind  keyMatchKind
	score int
}

func rankKeyMatches(keys []KeySummary, query string) []KeySummary {
	folded := strings.ToLower(query)
	queryTokens := keyTokens(query)
	matches := make([]keyMatch, 0, len(keys))

	for _, key := range keys {
		name := normalizeKeyQuery(key.Name)
		foldedName := strings.ToLower(name)

		switch {
		case strings.HasPrefix(foldedName, folded):
			matches = append(matches, keyMatch{key: key, kind: keyMatchPrefix, score: len(name)})
		case strings.Contains(foldedName, folded) || containsAnyToken(keyTokens(key.Name), queryTokens):
			matches = append(matches, keyMatch{key: key, kind: keyMatchToken, score: len(name)})
		}
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].kind != matches[j].kind {
			return matches[i].kind < matches[j].kind
		}
		if matches[i].score != matches[j].score {
			return matches[i].score < matches[j].score
		}
		return strings.ToLower(matches[i].key.Name) < strings.ToLower(matches[j].key.Name)
	})

	return uniqueMatchedKeys(matches)
}

func suggestKeyMatches(keys []KeySummary, query string) []KeySummary {
	folded := strings.ToLower(query)
	queryTokens := keyTokens(query)
	matches := make([]keyMatch, 0, len(keys))

	for _, key := range keys {
		name := normalizeKeyQuery(key.Name)
		foldedName := strings.ToLower(name)
		if isSubsequence(folded, foldedName) || containsSubsequenceToken(keyTokens(key.Name), queryTokens) {
			matches = append(matches, keyMatch{key: key, kind: keyMatchSubsequence, score: len(name)})
		}
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score < matches[j].score
		}
		return strings.ToLower(matches[i].key.Name) < strings.ToLower(matches[j].key.Name)
	})

	return uniqueMatchedKeys(matches)
}

func uniqueMatchedKeys(matches []keyMatch) []KeySummary {
	seen := map[string]struct{}{}
	out := make([]KeySummary, 0, len(matches))
	for _, match := range matches {
		if _, ok := seen[match.key.Name]; ok {
			continue
		}
		seen[match.key.Name] = struct{}{}
		out = append(out, match.key)
	}
	return out
}

func cloneKeySummaries(keys []KeySummary) []KeySummary {
	out := make([]KeySummary, len(keys))
	copy(out, keys)
	return out
}

func keyTokens(input string) []string {
	normalized := strings.ToLower(normalizeKeyQuery(input))
	fields := strings.FieldsFunc(normalized, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r)
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if field != "" {
			out = append(out, field)
		}
	}
	return out
}

func containsAnyToken(nameTokens, queryTokens []string) bool {
	for _, queryToken := range queryTokens {
		for _, nameToken := range nameTokens {
			if strings.Contains(nameToken, queryToken) {
				return true
			}
		}
	}
	return false
}

func containsSubsequenceToken(nameTokens, queryTokens []string) bool {
	for _, queryToken := range queryTokens {
		for _, nameToken := range nameTokens {
			if isSubsequence(queryToken, nameToken) {
				return true
			}
		}
	}
	return false
}

func isSubsequence(needle, haystack string) bool {
	if needle == "" {
		return false
	}
	needleRunes := []rune(needle)
	index := 0
	for _, r := range haystack {
		if index >= len(needleRunes) {
			break
		}
		if r == needleRunes[index] {
			index++
		}
	}
	return index == len(needleRunes)
}
