package hostmatch

import (
	"regexp"
	"strings"
)

func Match(pattern string, host string) bool {
	if pattern == "" || host == "" {
		return false
	}

	if strings.HasPrefix(pattern, "~") {
		return matchRegex(pattern[1:], host)
	}

	if strings.Contains(pattern, "*") {
		return matchWildcard(pattern, host)
	}

	return strings.EqualFold(pattern, host)
}

func matchWildcard(pattern, host string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 0 {
		return false
	}

	remaining := strings.ToLower(host)
	for i, part := range parts {
		part = strings.ToLower(part)
		if part == "" {
			continue
		}
		idx := strings.Index(remaining, part)
		if idx < 0 {
			return false
		}
		if i == 0 && idx != 0 {
			return false
		}
		remaining = remaining[idx+len(part):]
	}

	if !strings.HasSuffix(pattern, "*") && remaining != "" {
		return false
	}

	return true
}

func matchRegex(pattern, host string) bool {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(host)
}

func ClassifyPattern(pattern string) string {
	if strings.HasPrefix(pattern, "~") {
		return "regex"
	}
	if strings.Contains(pattern, "*") {
		return "wildcard"
	}
	return "exact"
}
