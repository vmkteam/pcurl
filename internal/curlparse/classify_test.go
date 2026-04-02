package curlparse

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSecretHeader(t *testing.T) {
	tests := []struct {
		name   string
		secret bool
	}{
		{"Authorization", true},
		{"authorization", true},
		{"Authorization2", true},
		{"X-API-Key", true},
		{"x-api-key", true},
		{"X-Auth-Token", true},
		{"X-Auth-Custom", true},
		{"Cookie", true},
		{"X-CSRF-Token", true},
		{"Accept", false},
		{"Content-Type", false},
		{"User-Agent", false},
		{"X-Request-Id", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.secret, IsSecretHeader(tt.name), "IsSecretHeader(%q)", tt.name)
	}
}

func TestIsBrowserNoise(t *testing.T) {
	tests := []struct {
		name  string
		noise bool
	}{
		{"sec-ch-ua", true},
		{"sec-ch-ua-mobile", true},
		{"sec-fetch-dest", true},
		{"sentry-trace", true},
		{"baggage", true},
		{"priority", true},
		{"Accept", false},
		{"Authorization", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.noise, IsBrowserNoise(tt.name), "IsBrowserNoise(%q)", tt.name)
	}
}

func TestIsSecretCookie(t *testing.T) {
	tests := []struct {
		name, value string
		secret      bool
	}{
		{"session", "abc", true},
		{"msAuthToken", "xyz", true},
		{"_csrf", "tok", true},
		{"User[login]", "admin", true},
		{"User[password]", "hash", true},
		{"theme", "dark", false},
		{"locale", "en", false},
		{"isAcceptedCookies", "1", false},
		{"somekey", "eyJhbGciOiJSUzI1NiIs", true},
		{"tracker", "abcdefghijklmnopqrstuvwxyz1234567890", true},
		{"short", "abc", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.secret, IsSecretCookie(tt.name, tt.value), "IsSecretCookie(%q, %q)", tt.name, tt.value)
	}
}

func TestMaskValue(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"ghp_xxxxxxxxxxxx", "ghp_...xxxx"},
		{"short", "****"},
		{"12345678", "****"},
		{"123456789", "1234...6789"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, MaskValue(tt.in), "MaskValue(%q)", tt.in)
	}
}
