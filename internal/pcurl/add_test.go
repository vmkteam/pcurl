package pcurl

import (
	"bytes"
	"strings"
	"testing"

	"github.com/vmkteam/pcurl/internal/keyring"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestExecuter(t *testing.T) (*Executer, *keyring.Mock) {
	t.Helper()
	cm := NewConfigManagerWithDir(t.TempDir())
	kc := keyring.NewMock()
	return &Executer{CM: cm, Keyring: kc}, kc
}

func TestAdd_SimpleHeader(t *testing.T) {
	ex, kc := newTestExecuter(t)
	var out bytes.Buffer

	err := ex.Add(
		[]string{"https://api.github.com/user", "-H", "Authorization: Bearer ghp_test_token", "-H", "Accept: application/json"},
		AddOptions{},
		&out, strings.NewReader("\nk\n"), false,
	)
	require.NoError(t, err)

	c, _ := ex.CM.Load()
	p := c.FindProfile("api.github.com")
	require.NotNil(t, p)
	assert.Equal(t, []string{"api.github.com"}, p.MatchHosts)

	val, err := kc.Get("api.github.com/authorization")
	require.NoError(t, err)
	assert.Equal(t, "Bearer ghp_test_token", val)
	assert.Contains(t, p.Headers, "Accept: application/json")
}

func TestAdd_Force(t *testing.T) {
	ex, kc := newTestExecuter(t)
	var out bytes.Buffer

	err := ex.Add(
		[]string{"https://api.example.com/data", "-H", "X-API-Key: sk-secret"},
		AddOptions{Force: true, Name: "myapi"},
		&out, strings.NewReader(""), false,
	)
	require.NoError(t, err)

	c, _ := ex.CM.Load()
	require.NotNil(t, c.FindProfile("myapi"))
	val, _ := kc.Get("myapi/x-api-key")
	assert.Equal(t, "sk-secret", val)
}

func TestAdd_StoreInConfig(t *testing.T) {
	ex, kc := newTestExecuter(t)
	var out bytes.Buffer

	err := ex.Add(
		[]string{"https://ci.example.com/api", "-H", "Authorization: Bearer ci_token"},
		AddOptions{},
		&out, strings.NewReader("c\n"), false, // no non-secret headers, no picker
	)
	require.NoError(t, err)

	c, _ := ex.CM.Load()
	p := c.FindProfile("ci.example.com")
	require.NotNil(t, p)

	_, err = kc.Get("ci.example.com/authorization")
	require.Error(t, err, "should NOT be in keychain")
	assert.Contains(t, p.Headers, "Authorization: Bearer ci_token")
}

func TestAdd_Skip(t *testing.T) {
	ex, _ := newTestExecuter(t)
	var out bytes.Buffer

	err := ex.Add(
		[]string{"https://example.com", "-H", "Authorization: Bearer skip_me", "-H", "Accept: text/html"},
		AddOptions{},
		&out, strings.NewReader("\ns\n"), false,
	)
	require.NoError(t, err)

	c, _ := ex.CM.Load()
	p := c.FindProfile("example.com")
	require.NotNil(t, p)
	assert.Len(t, p.Headers, 1, "only Accept should remain")
}

func TestAdd_UpdateExisting_Confirm(t *testing.T) {
	ex, _ := newTestExecuter(t)
	c := &Config{Profiles: map[string]*Profile{
		"api.github.com": {
			Description: "Old",
			MatchHosts:  []string{"api.github.com"},
			Headers:     []string{"Accept: old"},
		},
	}}
	require.NoError(t, ex.CM.Save(c))
	var out bytes.Buffer

	err := ex.Add(
		[]string{"https://api.github.com/user", "-H", "Authorization: Bearer new_token"},
		AddOptions{},
		&out, strings.NewReader("\n\nk\n"), false,
	)
	require.NoError(t, err)

	c, _ = ex.CM.Load()
	p := c.FindProfile("api.github.com")
	assert.Equal(t, "Old", p.Description, "description should be preserved")
}

func TestAdd_UpdateExisting_Decline(t *testing.T) {
	ex, _ := newTestExecuter(t)
	c := &Config{Profiles: map[string]*Profile{
		"example.com": {MatchHosts: []string{"example.com"}, Headers: []string{"Accept: old"}},
	}}
	require.NoError(t, ex.CM.Save(c))
	var out bytes.Buffer

	err := ex.Add(
		[]string{"https://example.com", "-H", "Authorization: Bearer new"},
		AddOptions{},
		&out, strings.NewReader("n\n"), false, // decline update, no header picker shown
	)
	require.NoError(t, err)

	c, _ = ex.CM.Load()
	p := c.FindProfile("example.com")
	assert.Equal(t, []string{"Accept: old"}, p.Headers, "headers should be unchanged")
}

func TestAdd_WithCookies_Force(t *testing.T) {
	ex, kc := newTestExecuter(t)
	var out bytes.Buffer

	err := ex.Add(
		[]string{"https://example.com", "-b", "session=abc123; theme=dark", "-H", "Accept: text/html"},
		AddOptions{Force: true, Raw: true},
		&out, strings.NewReader(""), false,
	)
	require.NoError(t, err)

	val, err := kc.Get("example.com/cookie")
	require.NoError(t, err)
	assert.Contains(t, val, "session=abc123")
}

func TestAdd_CleanBrowserHeaders(t *testing.T) {
	ex, _ := newTestExecuter(t)
	var out bytes.Buffer

	err := ex.Add(
		[]string{
			"https://example.com",
			"-H", "Accept: text/html",
			"-H", "sec-ch-ua: Chromium",
			"-H", "sec-fetch-mode: cors",
			"-H", "sentry-trace: abc123",
		},
		AddOptions{Force: true},
		&out, strings.NewReader(""), false,
	)
	require.NoError(t, err)

	c, _ := ex.CM.Load()
	p := c.FindProfile("example.com")
	assert.Len(t, p.Headers, 1, "only Accept should remain after clean")
	assert.Contains(t, out.String(), "Cleaned 3 browser headers")
}

func TestAdd_NoURL(t *testing.T) {
	ex, _ := newTestExecuter(t)
	var out bytes.Buffer

	err := ex.Add(
		[]string{"-H", "Accept: text/html"},
		AddOptions{Force: true},
		&out, strings.NewReader(""), false,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no URL")
}
