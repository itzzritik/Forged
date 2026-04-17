package actions

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/ipc"
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
	return result.Keys, nil
}

func ViewKey(paths config.Paths, name string) (KeyDetail, error) {
	resp, err := ipc.NewClient(paths.CtlSocket()).Call(ipc.CmdView, map[string]any{
		"name": name,
		"full": false,
	})
	if err != nil {
		return KeyDetail{}, err
	}

	var result KeyDetail
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return KeyDetail{}, fmt.Errorf("parsing key detail: %w", err)
	}
	return result, nil
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
