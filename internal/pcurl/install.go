package pcurl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RulesTarget describes a supported agent rules file location.
type RulesTarget struct {
	Name string
	Path string
}

// DetectTargets returns all agent rule targets found on the system.
func DetectTargets() []RulesTarget {
	home, _ := os.UserHomeDir()
	candidates := []RulesTarget{
		{Name: "claude", Path: filepath.Join(home, ".claude", "CLAUDE.md")},
		{Name: "cursor", Path: filepath.Join(home, ".cursor", "rules", "pcurl.mdc")},
		{Name: "windsurf", Path: filepath.Join(home, ".windsurf", "rules", "pcurl.md")},
	}

	var found []RulesTarget
	for _, t := range candidates {
		if info, err := os.Stat(filepath.Dir(t.Path)); err == nil && info.IsDir() {
			found = append(found, t)
		}
	}
	return found
}

const (
	rulesMarkerStart = "<!-- pcurl:start -->"
	rulesMarkerEnd   = "<!-- pcurl:end -->"
)

func writeRules(c *Config, targets []RulesTarget) error {
	rules := RenderRules(c)
	for _, t := range targets {
		if err := upsertRulesInFile(t.Path, rules); err != nil {
			return fmt.Errorf("write rules to %s: %w", t.Path, err)
		}
		fmt.Printf("✓ Updated pcurl rules in %s\n", t.Path)
	}
	return nil
}

func upsertRulesInFile(path, rules string) error {
	block := rulesMarkerStart + "\n" + rules + "\n" + rulesMarkerEnd

	content, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	s := string(content)
	startIdx := strings.Index(s, rulesMarkerStart)
	endIdx := strings.Index(s, rulesMarkerEnd)

	if startIdx != -1 && endIdx != -1 {
		s = s[:startIdx] + block + s[endIdx+len(rulesMarkerEnd):]
	} else {
		if len(s) > 0 && !strings.HasSuffix(s, "\n") {
			s += "\n"
		}
		s += "\n" + block + "\n"
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(s), 0644)
}

func removeRulesFromFile(t RulesTarget) error {
	content, err := os.ReadFile(t.Path)
	if err != nil {
		return err
	}

	s := string(content)
	startIdx := strings.Index(s, rulesMarkerStart)
	endIdx := strings.Index(s, rulesMarkerEnd)

	if startIdx == -1 || endIdx == -1 {
		return nil
	}

	before := strings.TrimRight(s[:startIdx], "\n")
	after := strings.TrimLeft(s[endIdx+len(rulesMarkerEnd):], "\n")
	result := before
	if after != "" {
		result += "\n" + after
	}

	return os.WriteFile(t.Path, []byte(result), 0644)
}

// RenderRules returns the markdown rules text including the profile list.
func RenderRules(c *Config) string {
	var b strings.Builder
	b.WriteString("## pcurl — Private Curl\n\n")
	b.WriteString("Use `pcurl @profile` instead of `curl` for authenticated HTTP requests.\n")
	b.WriteString("NEVER pass Authorization headers, API keys, or cookies directly in curl commands.\n")
	b.WriteString("NEVER run `pcurl add` — only the user creates profiles.\n")
	b.WriteString("NEVER read `~/.config/pcurl/profiles.toml` or use `security`/`secret-tool` to access secrets.\n\n")

	b.WriteString("Usage: `pcurl @profile [curl args]`\n\n")

	b.WriteString("Examples:\n")
	b.WriteString("```\n")
	b.WriteString("pcurl @api https://api.example.com/data                          # GET\n")
	b.WriteString("pcurl @api https://api.example.com/data -X POST -d '{\"key\":1}'   # POST with JSON\n")
	b.WriteString("pcurl @api https://api.example.com/data -s | jq .                # silent + pipe\n")
	b.WriteString("pcurl https://httpbin.org/get                                    # no profile = plain curl\n")
	b.WriteString("```\n\n")

	if len(c.Profiles) > 0 {
		b.WriteString("Available profiles:\n")
		for name, p := range c.Profiles {
			hosts := strings.Join(p.MatchHosts, ", ")
			desc := ""
			if p.Description != "" {
				desc = " — " + p.Description
			}
			fmt.Fprintf(&b, "- @%s (%s)%s\n", name, hosts, desc)
		}
		b.WriteString("\n")
	}

	b.WriteString("If no profile exists for the target host, ask the user:\n")
	b.WriteString("  \"Please create a pcurl profile: pcurl add <your curl command with credentials>\"\n\n")
	b.WriteString("Run `pcurl show` to list profiles or `pcurl show <name>` for details.\n")
	b.WriteString("Secrets are stored in OS keychain and never visible to you.\n")
	return b.String()
}
