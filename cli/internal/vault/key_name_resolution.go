package vault

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

const maxNameSuggestions = 5

type nameMatchKind int

const (
	nameMatchExact nameMatchKind = iota
	nameMatchCaseInsensitiveExact
	nameMatchPrefix
	nameMatchToken
	nameMatchSubsequence
)

type nameMatch struct {
	key   Key
	kind  nameMatchKind
	score int
}

type KeyNameResolveError struct {
	Query       string
	Suggestions []string
	More        bool
	Ambiguous   bool
}

func (e *KeyNameResolveError) Error() string {
	if e.Ambiguous {
		return formatAmbiguousKeyError(e.Query, e.Suggestions, e.More)
	}
	return formatNotFoundKeyError(e.Query, e.Suggestions, e.More)
}

func formatAmbiguousKeyError(query string, suggestions []string, more bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Multiple keys match %q:\n", query)
	for _, suggestion := range suggestions {
		fmt.Fprintf(&b, "  - %s\n", suggestion)
	}
	if more {
		b.WriteString("  ...more\n")
	}
	b.WriteString("\nUse a more specific name.")
	return b.String()
}

func formatNotFoundKeyError(query string, suggestions []string, more bool) string {
	if len(suggestions) == 0 {
		return fmt.Sprintf("key %q not found", query)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Key %q not found.\n\nDid you mean:\n", query)
	for _, suggestion := range suggestions {
		fmt.Fprintf(&b, "  - %s\n", suggestion)
	}
	if more {
		b.WriteString("  ...more\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func normalizeKeyName(input string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(input)), " ")
}

func foldKeyName(input string) string {
	return strings.ToLower(normalizeKeyName(input))
}

func keyNameTokens(input string) []string {
	normalized := foldKeyName(input)
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

func rankNameMatches(keys []Key, query string) []nameMatch {
	normalized := normalizeKeyName(query)
	if normalized == "" {
		return nil
	}

	folded := strings.ToLower(normalized)
	queryTokens := keyNameTokens(query)
	var matches []nameMatch

	for _, key := range keys {
		name := normalizeKeyName(key.Name)
		foldedName := strings.ToLower(name)

		switch {
		case name == normalized:
			matches = append(matches, nameMatch{key: key, kind: nameMatchExact, score: 0})
		case foldedName == folded:
			matches = append(matches, nameMatch{key: key, kind: nameMatchCaseInsensitiveExact, score: 0})
		case strings.HasPrefix(foldedName, folded):
			matches = append(matches, nameMatch{key: key, kind: nameMatchPrefix, score: len(name)})
		case strings.Contains(foldedName, folded) || containsAnyToken(keyNameTokens(key.Name), queryTokens):
			matches = append(matches, nameMatch{key: key, kind: nameMatchToken, score: len(name)})
		}
	}

	sortNameMatches(matches)
	return matches
}

func suggestNameMatches(keys []Key, query string) []nameMatch {
	matches := rankNameMatches(keys, query)
	if len(matches) > 0 {
		return matches
	}

	normalized := normalizeKeyName(query)
	if normalized == "" {
		return nil
	}

	folded := strings.ToLower(normalized)
	queryTokens := keyNameTokens(query)
	var suggestions []nameMatch

	for _, key := range keys {
		name := normalizeKeyName(key.Name)
		foldedName := strings.ToLower(name)

		if isSubsequence(folded, foldedName) || containsSubsequenceToken(keyNameTokens(key.Name), queryTokens) {
			suggestions = append(suggestions, nameMatch{key: key, kind: nameMatchSubsequence, score: len(name)})
		}
	}

	sortNameMatches(suggestions)
	return suggestions
}

func sortNameMatches(matches []nameMatch) {
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].kind != matches[j].kind {
			return matches[i].kind < matches[j].kind
		}
		if matches[i].score != matches[j].score {
			return matches[i].score < matches[j].score
		}
		return strings.ToLower(matches[i].key.Name) < strings.ToLower(matches[j].key.Name)
	})
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
	j := 0
	for _, r := range haystack {
		if j >= len(needleRunes) {
			break
		}
		if r == needleRunes[j] {
			j++
		}
	}
	return j == len(needleRunes)
}

func cappedSuggestions(matches []nameMatch) ([]string, bool) {
	seen := make(map[string]struct{}, len(matches))
	suggestions := make([]string, 0, maxNameSuggestions)

	for _, match := range matches {
		if _, ok := seen[match.key.Name]; ok {
			continue
		}
		seen[match.key.Name] = struct{}{}
		if len(suggestions) < maxNameSuggestions {
			suggestions = append(suggestions, match.key.Name)
		}
	}

	return suggestions, len(seen) > maxNameSuggestions
}
