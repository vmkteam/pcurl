package pcurl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseHeaderSource(t *testing.T) {
	tests := []struct {
		raw    string
		name   string
		source HeaderSource
		ref    string
	}{
		{"Accept: application/json", "Accept", SourcePlaintext, "application/json"},
		{"Authorization: keychain:github/authorization", "Authorization", SourceKeychain, "github/authorization"},
		{"X-Api-Key: env:AWS_API_KEY", "X-Api-Key", SourceEnv, "AWS_API_KEY"},
		{"Authorization: Bearer plaintext-token", "Authorization", SourcePlaintext, "Bearer plaintext-token"},
	}
	for _, tt := range tests {
		ps := ParseHeaderSource(tt.raw)
		assert.Equal(t, tt.name, ps.Name, "Name for %q", tt.raw)
		assert.Equal(t, tt.source, ps.Source, "Source for %q", tt.raw)
		assert.Equal(t, tt.ref, ps.Ref, "Ref for %q", tt.raw)
	}
}

func TestConfigFindProfileByHost(t *testing.T) {
	c := &Config{
		Profiles: map[string]*Profile{
			"github": {MatchHosts: []string{"api.github.com"}},
			"stripe": {MatchHosts: []string{"api.stripe.com", "files.stripe.com"}},
		},
	}

	assert.Equal(t, "github", c.FindProfileByHost("api.github.com"))
	assert.Equal(t, "stripe", c.FindProfileByHost("files.stripe.com"))
	assert.Empty(t, c.FindProfileByHost("unknown.com"))
}

func TestCheckPermissions(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "test.toml")
	require.NoError(t, os.WriteFile(tmp, []byte("test"), 0644))

	require.Error(t, checkPermissions(tmp), "0644 should fail")

	require.NoError(t, os.Chmod(tmp, 0600))
	assert.NoError(t, checkPermissions(tmp), "0600 should pass")
}

func TestConfigManager_SaveLoad(t *testing.T) {
	cm := NewConfigManagerWithDir(t.TempDir())

	c := &Config{
		Profiles: map[string]*Profile{
			"github": {
				Description: "GitHub API",
				MatchHosts:  []string{"api.github.com"},
				Headers: []string{
					"Accept: application/vnd.github+json",
					"Authorization: keychain:github/authorization",
				},
			},
		},
	}

	require.NoError(t, cm.Save(c))

	loaded, err := cm.Load()
	require.NoError(t, err)

	p := loaded.FindProfile("github")
	require.NotNil(t, p)
	assert.Equal(t, "GitHub API", p.Description)
	assert.Equal(t, []string{"api.github.com"}, p.MatchHosts)
	assert.Len(t, p.Headers, 2)
}
