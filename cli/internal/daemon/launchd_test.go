//go:build darwin

package daemon

import (
	"strings"
	"testing"
)

func TestRenderLaunchdPlistEscapesMasterPassword(t *testing.T) {
	t.Parallel()

	raw, err := renderLaunchdPlist(launchdTemplateData{
		Label:          launchdLabel,
		Binary:         "/tmp/forged",
		LogFile:        "/tmp/forged.log",
		MasterPassword: `abc<&>"'xyz`,
	})
	if err != nil {
		t.Fatalf("renderLaunchdPlist() error = %v", err)
	}

	out := string(raw)
	if strings.Contains(out, `abc<&>"'xyz`) {
		t.Fatalf("password was not escaped: %s", out)
	}
	if !strings.Contains(out, "<key>FORGED_MASTER_PASSWORD</key>") {
		t.Fatalf("missing master password key: %s", out)
	}
}
