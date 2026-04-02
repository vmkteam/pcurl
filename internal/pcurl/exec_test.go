package pcurl

import (
	"bytes"
	"testing"

	"github.com/vmkteam/pcurl/internal/keyring"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractProfile(t *testing.T) {
	tests := []struct {
		args       []string
		wantName   string
		wantRemain int
	}{
		{[]string{"@github", "https://api.github.com"}, "github", 1},
		{[]string{"https://api.github.com", "@github"}, "github", 1},
		{[]string{"--profile", "github", "https://api.github.com"}, "github", 1},
		{[]string{"https://api.github.com"}, "", 1},
		{[]string{"-v", "@stripe", "https://api.stripe.com", "-s"}, "stripe", 3},
	}
	for _, tt := range tests {
		name, remaining := extractProfile(tt.args)
		assert.Equal(t, tt.wantName, name, "name for %v", tt.args)
		assert.Len(t, remaining, tt.wantRemain, "remaining for %v", tt.args)
	}
}

func TestExtractHost(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"https://api.github.com/user"}, "api.github.com"},
		{[]string{"-v", "https://example.com:8080/path"}, "example.com"},
		{[]string{"http://localhost/test"}, "localhost"},
		{[]string{"-X", "POST"}, ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, extractHost(tt.args), "host for %v", tt.args)
	}
}

func TestResolveHeaders(t *testing.T) {
	kc := keyring.NewMock()
	require.NoError(t, kc.Set("github/authorization", "Bearer ghp_real_token"))
	t.Setenv("TEST_API_KEY", "sk-test-123")

	p := &Profile{
		Headers: []string{
			"Accept: application/json",
			"Authorization: keychain:github/authorization",
			"X-Api-Key: env:TEST_API_KEY",
		},
	}

	resolved, err := resolveHeaders(p, kc)
	require.NoError(t, err)
	require.Len(t, resolved, 3)

	assert.False(t, resolved[0].Secret)
	assert.Equal(t, "application/json", resolved[0].Value)

	assert.True(t, resolved[1].Secret)
	assert.Equal(t, "Bearer ghp_real_token", resolved[1].Value)

	assert.True(t, resolved[2].Secret)
	assert.Equal(t, "sk-test-123", resolved[2].Value)
}

func TestResolveHeaders_MissingKeychain(t *testing.T) {
	kc := keyring.NewMock()
	p := &Profile{Headers: []string{"Authorization: keychain:missing/key"}}
	_, err := resolveHeaders(p, kc)
	assert.Error(t, err)
}

func TestResolveHeaders_MissingEnv(t *testing.T) {
	kc := keyring.NewMock()
	p := &Profile{Headers: []string{"X-Key: env:NONEXISTENT_VAR_12345"}}
	_, err := resolveHeaders(p, kc)
	assert.Error(t, err)
}

func TestBuildCurlArgs(t *testing.T) {
	resolved := []ResolvedHeader{
		{Name: "Accept", Value: "application/json", Secret: false},
		{Name: "Authorization", Value: "Bearer token", Secret: true},
		{Name: "X-Custom", Value: "val", Secret: false},
	}
	userArgs := []string{
		"https://api.example.com",
		"-H", "Accept: text/plain",
		"-X", "POST",
	}

	secretArgs, publicArgs := buildCurlArgs(resolved, userArgs)

	require.Len(t, secretArgs, 1)
	assert.Equal(t, `header = "Authorization: Bearer token"`, secretArgs[0])
	assert.Contains(t, publicArgs, "X-Custom: val")
}

func TestExec_NoProfile_Warning(t *testing.T) {
	ex, _ := newTestExecuter(t)
	c := &Config{Profiles: map[string]*Profile{
		"github": {MatchHosts: []string{"api.github.com"}},
	}}
	require.NoError(t, ex.CM.Save(c))

	var stderr bytes.Buffer
	args := []string{"https://api.github.com/user", "-o", "/dev/null", "-s", "-w", "%{http_code}"}
	_, _ = ex.Exec(args, &stderr)

	assert.Contains(t, stderr.String(), `profile "github" matches api.github.com`)
}
