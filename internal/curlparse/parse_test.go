package curlparse

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_Simple(t *testing.T) {
	r := Parse([]string{
		"https://api.github.com/user",
		"-H", "Authorization: Bearer ghp_xxx",
		"-H", "Accept: application/json",
	})
	assert.Equal(t, "https://api.github.com/user", r.URL)
	assert.Equal(t, "api.github.com", r.Host)
	require.Len(t, r.Headers, 2)
	assert.True(t, r.Headers[0].Secret, "Authorization should be secret")
	assert.False(t, r.Headers[1].Secret, "Accept should not be secret")
}

func TestParse_StripsCurlWord(t *testing.T) {
	r := Parse([]string{"curl", "https://example.com", "-H", "Accept: text/html"})
	assert.Equal(t, "https://example.com", r.URL)
}

func TestParse_Cookies(t *testing.T) {
	r := Parse([]string{
		"https://example.com",
		"-b", "session=abc123; theme=dark; msAuthToken=eyJhbG",
	})
	require.Len(t, r.Cookies, 3)
	assert.True(t, r.Cookies[0].Secret, "session should be secret")
	assert.False(t, r.Cookies[1].Secret, "theme should not be secret")
	assert.True(t, r.Cookies[2].Secret, "msAuthToken should be secret")
}

func TestParse_CookieHeader(t *testing.T) {
	r := Parse([]string{
		"https://example.com",
		"-H", "Cookie: sid=abc; pref=1",
	})
	assert.Empty(t, r.Headers, "Cookie header should be parsed into cookies")
	assert.Len(t, r.Cookies, 2)
}

func TestParse_BasicAuth(t *testing.T) {
	r := Parse([]string{"https://example.com", "-u", "admin:secret"})
	require.Len(t, r.Headers, 1)
	assert.Equal(t, "Authorization", r.Headers[0].Name)
	assert.True(t, r.Headers[0].Secret)
}

func TestParse_PassthroughArgs(t *testing.T) {
	r := Parse([]string{
		"https://example.com",
		"-X", "POST",
		"-d", `{"key":"value"}`,
		"-v",
	})
	assert.Len(t, r.Args, 5)
}

func TestParse_EmptyValueHeader(t *testing.T) {
	// curl syntax: -H 'Header;' means send header with empty value
	r := Parse([]string{
		"https://example.com",
		"-H", "authorization2;",
		"-H", "Accept: text/html",
	})
	require.Len(t, r.Headers, 2)
	assert.Equal(t, "authorization2", r.Headers[0].Name)
	assert.Empty(t, r.Headers[0].Value)
	assert.True(t, r.Headers[0].Secret, "authorization2 should be secret")
}

func TestCleanHeaders(t *testing.T) {
	r := &Result{
		Headers: []Header{
			{Name: "Accept", Value: "application/json"},
			{Name: "sec-ch-ua", Value: `"Chromium"`, Noise: true},
			{Name: "Authorization", Value: "Bearer xxx", Secret: true},
			{Name: "sec-fetch-mode", Value: "cors", Noise: true},
		},
	}
	CleanHeaders(r)
	require.Len(t, r.Headers, 2)
	assert.Equal(t, "Accept", r.Headers[0].Name)
	assert.Equal(t, "Authorization", r.Headers[1].Name)
}
