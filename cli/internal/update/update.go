package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/itzzritik/forged/cli/internal/buildinfo"
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/platform"
)

const RegistryURL = "https://registry.npmjs.org/forgedssh/latest"

var release = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)$`)

// Dev, pre-release and malformed versions never count, so a junk registry reply can't prompt anyone.
func Available(current, latest string) bool {
	c, l := parse(current), parse(latest)
	if c == nil || l == nil {
		return false
	}
	for i := range c {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

func parse(version string) []int {
	m := release.FindStringSubmatch(version)
	if m == nil {
		return nil
	}
	parts := make([]int, 3)
	for i := range parts {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return nil
		}
		parts[i] = n
	}
	return parts
}

func Latest(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, RegistryURL, nil)
	if err != nil {
		return "", fmt.Errorf("building registry request: %w", err)
	}
	req.Header.Set("User-Agent", "forged/"+buildinfo.Version)
	req.Header.Set("Accept", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("asking the npm registry: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("npm registry answered %s", res.Status)
	}
	var body struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&body); err != nil {
		return "", fmt.Errorf("reading the npm registry reply: %w", err)
	}
	if parse(body.Version) == nil {
		return "", fmt.Errorf("npm registry sent an invalid version")
	}
	return body.Version, nil
}

func Load(paths config.Paths) string {
	data, err := os.ReadFile(paths.LatestVersionFile())
	if err != nil {
		return ""
	}
	version := strings.TrimSpace(string(data))
	if parse(version) == nil {
		return ""
	}
	return version
}

func Save(paths config.Paths, version string) error {
	if parse(version) == nil {
		return fmt.Errorf("saving latest version: invalid version %q", version)
	}
	if err := os.MkdirAll(paths.ConfigDir, 0o700); err != nil {
		return fmt.Errorf("saving latest version: %w", err)
	}
	if err := platform.WriteFileDurable(paths.LatestVersionFile(), []byte(version+"\n"), 0o600); err != nil {
		return fmt.Errorf("saving latest version: %w", err)
	}
	return nil
}
