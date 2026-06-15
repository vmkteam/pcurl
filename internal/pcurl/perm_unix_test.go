//go:build !windows

package pcurl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckPermissions(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "test.toml")
	require.NoError(t, os.WriteFile(tmp, []byte("test"), 0644))

	require.Error(t, checkPermissions(tmp), "0644 should fail")

	require.NoError(t, os.Chmod(tmp, 0600))
	assert.NoError(t, checkPermissions(tmp), "0600 should pass")
}
