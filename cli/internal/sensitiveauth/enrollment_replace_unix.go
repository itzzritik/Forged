//go:build !windows

package sensitiveauth

import "os"

func replaceLocalEnrollmentFile(source, target string) error {
	return os.Rename(source, target)
}
