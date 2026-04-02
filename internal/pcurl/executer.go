package pcurl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/vmkteam/pcurl/internal/curlparse"
	"github.com/vmkteam/pcurl/internal/keyring"
)

// Executer holds shared dependencies and provides all pcurl commands.
type Executer struct {
	CM      *ConfigManager
	Keyring keyring.Keyring
}

// Exec resolves a profile's headers, then executes curl.
// Looks for @profile in args. If not found, checks MatchHosts and warns.
func (e *Executer) Exec(args []string, stderr io.Writer) (int, error) {
	c, err := e.CM.Load()
	if err != nil {
		return 1, err
	}

	profileName, curlArgs := extractProfile(args)

	if profileName == "" {
		if host := extractHost(curlArgs); host != "" {
			if match := c.FindProfileByHost(host); match != "" {
				fmt.Fprintf(stderr, "pcurl: profile %q matches %s, use: pcurl @%s %s\n",
					match, host, match, strings.Join(curlArgs, " "))
			}
		}
		return runCurl(nil, curlArgs)
	}

	p := c.FindProfile(profileName)
	if p == nil {
		return 1, fmt.Errorf("profile %q not found", profileName)
	}

	resolved, err := resolveHeaders(p, e.Keyring)
	if err != nil {
		return 1, err
	}

	secretArgs, publicArgs := buildCurlArgs(resolved, curlArgs)
	return runCurl(secretArgs, publicArgs)
}

// Add parses a curl command, classifies headers, prompts for storage,
// saves secrets, writes profile, and updates agent rules.
func (e *Executer) Add(args []string, opts AddOptions, w io.Writer, in io.Reader, interactive bool) error {
	parsed := curlparse.Parse(args)
	if parsed.URL == "" {
		return errors.New("no URL found in arguments")
	}

	c, err := e.CM.Load()
	if err != nil {
		return err
	}

	out := &Output{}
	prompt := NewPrompter(in, w, interactive)

	if !opts.Raw {
		before := len(parsed.Headers)
		curlparse.CleanHeaders(parsed)
		if cleaned := before - len(parsed.Headers); cleaned > 0 {
			out.Addf("Cleaned %d browser headers (use --raw to keep)", cleaned)
		}
	}

	profileName := opts.Name
	if profileName == "" {
		profileName = parsed.Host
	}
	if profileName == "" {
		return errors.New("cannot determine profile name from URL")
	}

	existing, declined, confirmErr := confirmExisting(profileName, parsed.Host, c, prompt, opts.Force)
	if confirmErr != nil {
		return confirmErr
	}
	if declined {
		fmt.Fprintln(w, "Cancelled")
		return nil
	}

	if pickErr := pickHeaders(parsed, opts, prompt); pickErr != nil {
		return pickErr
	}

	headers, secretResults, err := processHeaders(parsed, profileName, opts, prompt, e.Keyring)
	if err != nil {
		return err
	}

	cookieHeaders, cookieResults, err := processCookies(parsed, profileName, opts, prompt, e.Keyring)
	if err != nil {
		return err
	}
	headers = append(headers, cookieHeaders...)
	secretResults = append(secretResults, cookieResults...)

	profile := &Profile{
		MatchHosts: []string{parsed.Host},
		Headers:    headers,
	}
	if existing != nil {
		profile.Description = existing.Description
	}

	c.Profiles[profileName] = profile
	if saveErr := e.CM.Save(c); saveErr != nil {
		return fmt.Errorf("save config: %w", saveErr)
	}

	if rulesErr := e.updateRules(c); rulesErr != nil {
		out.Addf("Warning: could not update agent rules: %v", rulesErr)
	}

	printResult(out, existing != nil, profileName, parsed.URL, headers, secretResults)
	out.Print(w)

	if opts.Test {
		return e.runTest(profileName, parsed.URL, w)
	}

	return nil
}

// List prints all profiles to w.
func (e *Executer) List(w io.Writer) error {
	c, err := e.CM.Load()
	if err != nil {
		return err
	}
	return printList(c, w)
}

// Show prints profile details with masked secret values.
func (e *Executer) Show(name string, w io.Writer) error {
	c, err := e.CM.Load()
	if err != nil {
		return err
	}
	return printShow(c, name, e.Keyring, w)
}

// Delete removes a profile from config and its keychain entries.
func (e *Executer) Delete(name string, w io.Writer) error {
	c, err := e.CM.Load()
	if err != nil {
		return err
	}
	return deleteProfile(c, name, e.Keyring, e.CM, w, func(cfg *Config) error {
		return e.updateRules(cfg)
	})
}

// Edit opens profiles.toml in $EDITOR.
func (e *Executer) Edit() error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.CommandContext(context.Background(), editor, e.CM.Path())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Install creates config directory and writes agent rules.
func (e *Executer) Install() error {
	if err := e.CM.EnsureDir(); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	c, err := e.CM.Load()
	if err != nil {
		return err
	}

	if len(c.Profiles) == 0 {
		if saveErr := e.CM.Save(c); saveErr != nil {
			return saveErr
		}
		fmt.Printf("✓ Created %s\n", e.CM.Path())
	}

	targets := DetectTargets()
	if err := writeRules(c, targets); err != nil {
		return err
	}

	fmt.Println("Done. Add your first profile: pcurl add <curl command>")
	return nil
}

// Uninstall removes agent rules. Config kept unless purge.
func (e *Executer) Uninstall(purge bool) error {
	targets := DetectTargets()
	for _, t := range targets {
		if err := removeRulesFromFile(t); err != nil {
			fmt.Fprintf(os.Stderr, "pcurl: warning: %v\n", err)
		} else {
			fmt.Printf("✓ Removed pcurl rules from %s\n", t.Path)
		}
	}

	if purge {
		if err := os.RemoveAll(e.CM.Dir()); err != nil {
			return fmt.Errorf("remove %s: %w", e.CM.Dir(), err)
		}
		fmt.Printf("✓ Removed %s\n", e.CM.Dir())
	}
	return nil
}

func (e *Executer) updateRules(c *Config) error {
	targets := DetectTargets()
	if len(targets) == 0 {
		return nil
	}
	return writeRules(c, targets)
}

func (e *Executer) runTest(profileName, rawURL string, w io.Writer) error {
	fmt.Fprintln(w, "\nTesting...")
	code, err := e.Exec([]string{"@" + profileName, rawURL}, w)
	if err != nil {
		return fmt.Errorf("test request failed: %w", err)
	}
	if code != 0 {
		fmt.Fprintf(w, "Test: exit code %d\n", code)
	} else {
		fmt.Fprintln(w, "Test: OK ✓")
	}
	return nil
}

// --- internal helpers (moved from exec.go, profile.go) ---

// ResolvedHeader is a header ready to be passed to curl.
type ResolvedHeader struct {
	Name   string
	Value  string
	Secret bool
}

func extractProfile(args []string) (string, []string) {
	for i, arg := range args {
		if strings.HasPrefix(arg, "@") && len(arg) > 1 {
			return arg[1:], append(args[:i:i], args[i+1:]...)
		}
		if arg == "--profile" && i+1 < len(args) {
			return args[i+1], append(args[:i:i], args[i+2:]...)
		}
	}
	return "", args
}

func extractHost(args []string) string {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if u, err := url.Parse(arg); err == nil && u.Host != "" {
			return u.Hostname()
		}
	}
	return ""
}

func resolveHeaders(p *Profile, kc keyring.Keyring) ([]ResolvedHeader, error) {
	resolved := make([]ResolvedHeader, 0, len(p.Headers))
	for _, raw := range p.Headers {
		ps := ParseHeaderSource(raw)
		var value string
		var secret bool

		switch ps.Source {
		case SourceKeychain:
			v, err := kc.Get(ps.Ref)
			if err != nil {
				return nil, fmt.Errorf("resolve header %q: %w", ps.Name, err)
			}
			value = v
			secret = true
		case SourceEnv:
			v := os.Getenv(ps.Ref)
			if v == "" {
				return nil, fmt.Errorf("resolve header %q: env var %q is not set", ps.Name, ps.Ref)
			}
			value = v
			secret = true
		case SourcePlaintext:
			value = ps.Ref
		}

		resolved = append(resolved, ResolvedHeader{Name: ps.Name, Value: value, Secret: secret})
	}
	return resolved, nil
}

func buildCurlArgs(resolved []ResolvedHeader, userArgs []string) (secretArgs, publicArgs []string) {
	userHeaders := make(map[string]bool)
	for i, arg := range userArgs {
		if (arg == "-H" || arg == "--header") && i+1 < len(userArgs) {
			name, _, _ := strings.Cut(userArgs[i+1], ":")
			userHeaders[strings.ToLower(strings.TrimSpace(name))] = true
		}
	}

	for _, h := range resolved {
		if userHeaders[strings.ToLower(h.Name)] {
			continue
		}
		hdr := formatHeader(h.Name, h.Value)
		if h.Secret {
			secretArgs = append(secretArgs, fmt.Sprintf("header = %q", hdr))
		} else {
			publicArgs = append(publicArgs, "-H", hdr)
		}
	}

	publicArgs = append(publicArgs, userArgs...)
	return
}

// formatHeader formats a header for curl. Empty values use "Name;" syntax.
func formatHeader(name, value string) string {
	if value == "" {
		return name + ";"
	}
	return name + ": " + value
}

func runCurl(secretArgs, publicArgs []string) (int, error) {
	ctx := context.Background()
	var cmd *exec.Cmd
	if len(secretArgs) > 0 {
		allArgs := append([]string{"--config", "-"}, publicArgs...)
		cmd = exec.CommandContext(ctx, "curl", allArgs...)
		cmd.Stdin = strings.NewReader(strings.Join(secretArgs, "\n") + "\n")
	} else {
		cmd = exec.CommandContext(ctx, "curl", publicArgs...)
		cmd.Stdin = os.Stdin
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return 1, fmt.Errorf("exec curl: %w", err)
	}
	return 0, nil
}
