//go:build windows

package pcurl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// On Windows the permission check is a no-op: Go cannot represent Unix mode
// bits there, so any file (writable -> 0666, read-only -> 0444) must pass.
func TestCheckPermissions(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "test.toml")
	require.NoError(t, os.WriteFile(tmp, []byte("test"), 0644))
	assert.NoError(t, checkPermissions(tmp), "writable file should pass on Windows")

	require.NoError(t, os.Chmod(tmp, 0444))
	assert.NoError(t, checkPermissions(tmp), "read-only file should pass on Windows")
}
