//go:build !windows

package pcurl

import (
	"fmt"
	"os"
)

// checkPermissions ensures profiles.toml is not readable by group or others,
// since it may contain plaintext secrets. See perm_windows.go for why this is
// a no-op on Windows.
func checkPermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	perm := info.Mode().Perm()
	if perm&0077 != 0 {
		return fmt.Errorf("%s has permissions %04o, want 0600; fix with: chmod 600 %s", path, perm, path)
	}
	return nil
}
