package curlparse

import "strings"

// SecretHeaderPrefixes are case-insensitive prefixes for secret headers.
var SecretHeaderPrefixes = []string{
	"authorization",
	"proxy-authorization",
	"x-api-key", "api-key",
	"x-auth",
	"x-access-token",
	"cookie",
	"x-csrf-token", "x-xsrf-token",
}

// BrowserNoiseHeaders are headers added by browsers, not needed for API calls.
var BrowserNoiseHeaders = []string{
	"sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform",
	"sec-fetch-dest", "sec-fetch-mode", "sec-fetch-site",
	"sentry-trace", "baggage",
	"priority",
}

// SecretCookiePatterns are substrings that indicate a secret cookie name.
var SecretCookiePatterns = []string{
	"session", "sess", "sid", "token",
	"auth", "csrf", "xsrf", "jwt",
	"login", "password", "key", "secret",
}

// IsSecretHeader returns true if name matches any SecretHeaderPrefixes (case-insensitive prefix).
func IsSecretHeader(name string) bool {
	lower := strings.ToLower(name)
	for _, prefix := range SecretHeaderPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// IsBrowserNoise returns true if name matches BrowserNoiseHeaders (case-insensitive prefix).
func IsBrowserNoise(name string) bool {
	lower := strings.ToLower(name)
	for _, noise := range BrowserNoiseHeaders {
		if strings.HasPrefix(lower, noise) {
			return true
		}
	}
	return false
}

// IsSecretCookie returns true if cookie name matches SecretCookiePatterns or value looks like a token.
func IsSecretCookie(name, value string) bool {
	lower := strings.ToLower(name)
	for _, pat := range SecretCookiePatterns {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	if strings.HasPrefix(value, "eyJ") || len(value) > 32 {
		return true
	}
	return false
}

// OptionalHeaders are browser-specific headers that are usually not needed for API calls.
var OptionalHeaders = []string{
	"user-agent",
	"referer",
	"origin",
	"accept-language",
	"accept-encoding",
}

// IsOptionalHeader returns true if name matches OptionalHeaders (case-insensitive).
func IsOptionalHeader(name string) bool {
	lower := strings.ToLower(name)
	for _, opt := range OptionalHeaders {
		if lower == opt {
			return true
		}
	}
	return false
}

// MaskValue returns a masked version of a secret value for display.
func MaskValue(v string) string {
	if len(v) <= 8 {
		return "****"
	}
	return v[:4] + "..." + v[len(v)-4:]
}
