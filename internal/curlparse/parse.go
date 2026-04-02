package curlparse

import (
	"net/url"
	"strings"
)

// Result holds the result of parsing a curl command line.
type Result struct {
	URL     string
	Host    string
	Headers []Header
	Cookies []Cookie
	Args    []string // remaining curl args (-X, -d, -o, -v, etc.)
}

// Header is a single -H value classified by type.
type Header struct {
	Name     string
	Value    string
	Secret   bool
	Noise    bool
	Selected bool
}

// Cookie is a single cookie from -b or -H Cookie:.
type Cookie struct {
	Name     string
	Value    string
	Secret   bool
	Selected bool
}

// Parse parses curl command-line arguments into structured form.
// Leading "curl" word is stripped if present.
func Parse(args []string) *Result {
	if len(args) > 0 && args[0] == "curl" {
		args = args[1:]
	}

	r := &Result{}
	i := 0
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "-H" || arg == "--header":
			i++
			if i < len(args) {
				r.addHeader(args[i])
			}
		case arg == "-b" || arg == "--cookie":
			i++
			if i < len(args) {
				r.addCookies(args[i])
			}
		case arg == "-u" || arg == "--user":
			i++
			if i < len(args) {
				r.Headers = append(r.Headers, Header{
					Name:   "Authorization",
					Value:  "Basic " + args[i],
					Secret: true,
				})
			}
		case !strings.HasPrefix(arg, "-") && r.URL == "":
			r.URL = arg
		default:
			r.Args = append(r.Args, arg)
			if hasValue(arg) && i+1 < len(args) {
				i++
				r.Args = append(r.Args, args[i])
			}
		}
		i++
	}

	if r.URL != "" {
		if u, err := url.Parse(r.URL); err == nil {
			r.Host = u.Hostname()
		}
	}

	return r
}

// CleanHeaders removes browser noise headers.
func CleanHeaders(r *Result) {
	clean := r.Headers[:0]
	for _, h := range r.Headers {
		if !h.Noise {
			clean = append(clean, h)
		}
	}
	r.Headers = clean
}

func (r *Result) addHeader(raw string) {
	// curl syntax: -H 'Header;' means send header with empty value
	if strings.HasSuffix(raw, ";") && !strings.Contains(raw, ":") {
		raw = strings.TrimSuffix(raw, ";") + ":"
	}

	name, value, ok := strings.Cut(raw, ": ")
	if !ok {
		name, value, _ = strings.Cut(raw, ":")
	}
	name = strings.TrimSpace(name)
	value = strings.TrimSpace(value)

	if strings.EqualFold(name, "cookie") {
		r.addCookies(value)
		return
	}

	secret := IsSecretHeader(name)
	r.Headers = append(r.Headers, Header{
		Name:     name,
		Value:    value,
		Secret:   secret,
		Noise:    IsBrowserNoise(name),
		Selected: secret || !IsOptionalHeader(name),
	})
}

func (r *Result) addCookies(raw string) {
	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, value, _ := strings.Cut(part, "=")
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		secret := IsSecretCookie(name, value)
		r.Cookies = append(r.Cookies, Cookie{
			Name:     name,
			Value:    value,
			Secret:   secret,
			Selected: secret,
		})
	}
}

func hasValue(flag string) bool {
	switch flag {
	case "-X", "--request",
		"-d", "--data", "--data-raw", "--data-binary", "--data-urlencode",
		"-o", "--output",
		"-w", "--write-out",
		"-A", "--user-agent",
		"-e", "--referer",
		"-T", "--upload-file",
		"--connect-timeout", "--max-time",
		"--retry", "--retry-delay",
		"-x", "--proxy":
		return true
	}
	return false
}
