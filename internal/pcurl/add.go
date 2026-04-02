package pcurl

import (
	"fmt"
	"strings"

	"github.com/vmkteam/pcurl/internal/curlparse"
	"github.com/vmkteam/pcurl/internal/keyring"
)

// AddOptions controls pcurl add behavior.
type AddOptions struct {
	Name  string
	Test  bool
	Force bool
	Raw   bool
}

func confirmExisting(profileName, host string, c *Config, prompt *Prompter, force bool) (existing *Profile, declined bool, err error) {
	existing = c.FindProfile(profileName)
	if existing == nil {
		if name := c.FindProfileByHost(host); name != "" {
			existing = c.FindProfile(name)
		}
	}

	if existing != nil && !force {
		ok, promptErr := prompt.ConfirmUpdate(profileName)
		if promptErr != nil {
			return nil, false, promptErr
		}
		if !ok {
			return existing, true, nil
		}
	}

	return existing, false, nil
}

func pickHeaders(parsed *curlparse.Result, opts AddOptions, prompt *Prompter) error {
	if opts.Force || opts.Raw {
		for i := range parsed.Headers {
			parsed.Headers[i].Selected = true
		}
		return nil
	}
	return prompt.PickHeaders(parsed.Headers)
}

func processHeaders(
	parsed *curlparse.Result, profileName string, opts AddOptions, prompt *Prompter, kc keyring.Keyring,
) (headers, secretResults []string, err error) {
	for _, h := range parsed.Headers {
		if !h.Selected {
			continue
		}
		if !h.Secret {
			headers = append(headers, h.Name+": "+h.Value)
			continue
		}

		var choice StorageChoice
		if opts.Force {
			choice = StoreKeychain
		} else {
			choice, err = prompt.Storage(h.Name, curlparse.MaskValue(h.Value))
			if err != nil {
				return nil, nil, err
			}
		}

		keychainKey := profileName + "/" + strings.ToLower(h.Name)
		hdr, result := applyStorage(choice, h.Name, h.Value, keychainKey)
		if hdr != "" {
			headers = append(headers, hdr)
		}
		if result != "" {
			secretResults = append(secretResults, result)
		}

		if choice == StoreKeychain {
			if setErr := kc.Set(keychainKey, h.Value); setErr != nil {
				return nil, nil, fmt.Errorf("store %q in keychain: %w", h.Name, setErr)
			}
		}
	}
	return headers, secretResults, nil
}

func processCookies(
	parsed *curlparse.Result, profileName string, opts AddOptions, prompt *Prompter, kc keyring.Keyring,
) (headers, secretResults []string, err error) {
	if len(parsed.Cookies) == 0 {
		return nil, nil, nil
	}

	if !opts.Raw {
		if pickErr := prompt.PickCookies(parsed.Cookies); pickErr != nil {
			return nil, nil, fmt.Errorf("cookie picker: %w", pickErr)
		}
	} else {
		for i := range parsed.Cookies {
			parsed.Cookies[i].Selected = true
		}
	}

	var selectedParts []string
	for _, ck := range parsed.Cookies {
		if ck.Selected {
			selectedParts = append(selectedParts, ck.Name+"="+ck.Value)
		}
	}
	if len(selectedParts) == 0 {
		return nil, nil, nil
	}

	cookieValue := strings.Join(selectedParts, "; ")
	var cookieChoice StorageChoice
	if opts.Force {
		cookieChoice = StoreKeychain
	} else {
		cookieChoice, err = prompt.CookieStorage()
		if err != nil {
			return nil, nil, err
		}
	}

	keychainKey := profileName + "/cookie"
	switch cookieChoice {
	case StoreKeychain:
		if setErr := kc.Set(keychainKey, cookieValue); setErr != nil {
			return nil, nil, fmt.Errorf("store cookies in keychain: %w", setErr)
		}
		headers = append(headers, "Cookie: keychain:"+keychainKey)
		secretResults = append(secretResults, fmt.Sprintf("Cookie (%d selected) → keychain:%s", len(selectedParts), keychainKey))
	case StoreConfig:
		headers = append(headers, "Cookie: "+cookieValue)
		secretResults = append(secretResults, fmt.Sprintf("Cookie (%d selected) → config (⚠ plaintext)", len(selectedParts)))
	case StoreSkip:
		// cookies skipped
	}

	return headers, secretResults, nil
}

func applyStorage(choice StorageChoice, name, value, keychainKey string) (header, result string) {
	switch choice {
	case StoreKeychain:
		return name + ": keychain:" + keychainKey, fmt.Sprintf("%s → keychain:%s", name, keychainKey)
	case StoreConfig:
		return name + ": " + value, fmt.Sprintf("%s → config (⚠ plaintext)", name)
	case StoreSkip:
		return "", fmt.Sprintf("%s → skipped", name)
	}
	return "", ""
}

func printResult(out *Output, isUpdate bool, profileName, rawURL string, headers, secretResults []string) {
	action := "Added"
	if isUpdate {
		action = "Updated"
	}

	var plainHeaders []string
	for _, h := range headers {
		ps := ParseHeaderSource(h)
		if ps.Source == SourcePlaintext && !curlparse.IsSecretHeader(ps.Name) {
			plainHeaders = append(plainHeaders, ps.Name)
		}
	}

	out.Empty()
	out.Addf("%s profile %q:", action, profileName)
	if len(plainHeaders) > 0 {
		out.Addf("  Headers (plaintext): %s", strings.Join(plainHeaders, ", "))
	}
	for _, s := range secretResults {
		out.Addf("  Secrets: %s", s)
	}
	out.Addf("Use: pcurl @%s %s", profileName, rawURL)
}
