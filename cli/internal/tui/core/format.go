package core

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/itzzritik/forged/cli/internal/config"
)

const day = 24 * time.Hour

func Ago(rfc3339 string, now time.Time) string {
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil || t.IsZero() {
		return "never"
	}
	switch d := now.Sub(t); {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d/time.Minute))
	case d < day:
		return Plural(int(d/time.Hour), "hour") + " ago"
	case d < 2*day:
		return "yesterday"
	case d < 30*day:
		return fmt.Sprintf("%d days ago", int(d/day))
	case d < 365*day:
		return Plural(int(d/(30*day)), "month") + " ago"
	default:
		return Plural(int(d/(365*day)), "year") + " ago"
	}
}

func Date(rfc3339 string) string {
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil || t.IsZero() {
		return ""
	}
	return t.Local().Format("2 Jan 2006")
}

func FallbackName(email string) string {
	local := strings.TrimSpace(email)
	if at := strings.Index(local, "@"); at > 0 {
		local = local[:at]
	}
	words := strings.FieldsFunc(local, func(r rune) bool { return unicode.IsSpace(r) || strings.ContainsRune("._-", r) })
	for i, w := range words {
		runes := []rune(strings.ToLower(w))
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

func Plural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return fmt.Sprintf("%d %ss", n, unit)
}

func IntervalLabel(v string) string {
	switch config.NormalizeMasterPasswordInterval(v) {
	case config.MasterPasswordInterval15Days:
		return "15 days"
	case config.MasterPasswordInterval30Days:
		return "1 month"
	}
	return "7 days"
}
