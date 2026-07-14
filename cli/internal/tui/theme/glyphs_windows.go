//go:build windows

package theme

import (
	"os"
	"strings"
)

func terminalSupportsUnicode() bool {
	// Go writes console output through UTF-16, but legacy console fonts still
	// lack many of the glyphs used by the TUI. Unknown Windows hosts stay ASCII.
	return os.Getenv("WT_SESSION") != "" ||
		os.Getenv("TERM_PROGRAM") != "" ||
		strings.EqualFold(os.Getenv("ConEmuANSI"), "ON")
}
