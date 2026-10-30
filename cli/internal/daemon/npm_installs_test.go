package daemon

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/itzzritik/forged/cli/internal/config"
)

func TestNPMRoot(t *testing.T) {
	cases := map[string]string{
		"/usr/local/lib/node_modules/forgedssh":                                                "/usr/local/lib/node_modules",
		"/x/lib/node_modules/@getforged/cli/node_modules/forgedssh":                            "/x/lib/node_modules",
		"/h/Library/pnpm/global/5/node_modules/.pnpm/forgedssh@0.1.42/node_modules/forgedssh": "/h/Library/pnpm/global/5/node_modules",
	}
	for in, want := range cases {
		got, ok := NPMRoot(filepath.FromSlash(in))
		if !ok || got != filepath.FromSlash(want) {
			t.Errorf("NPMRoot(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	if got, ok := NPMRoot(filepath.FromSlash("/opt/forged/bin")); ok {
		t.Errorf("NPMRoot without node_modules = %q, want no match", got)
	}
}

func TestRecordNPMInstallDedupes(t *testing.T) {
	paths := config.Paths{ConfigDir: t.TempDir()}
	dir := filepath.FromSlash("/g/node_modules/forgedssh")
	for range 2 {
		if err := RecordNPMInstall(paths, dir); err != nil {
			t.Fatal(err)
		}
	}
	roots, err := loadNPMInstalls(paths)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{filepath.FromSlash("/g/node_modules")}; !slices.Equal(roots, want) {
		t.Fatalf("roots = %q, want %q", roots, want)
	}
}
