package pcurl

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/vmkteam/pcurl/internal/curlparse"
	"github.com/vmkteam/pcurl/internal/keyring"
)

func printList(c *Config, w io.Writer) error {
	if len(c.Profiles) == 0 {
		fmt.Fprintln(w, "No profiles configured. Add one: pcurl add <curl command>")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "NAME\tHOSTS\tDESCRIPTION\n")
	for name, p := range c.Profiles {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", name, strings.Join(p.MatchHosts, ", "), p.Description)
	}
	return tw.Flush()
}

func printShow(c *Config, name string, kc keyring.Keyring, w io.Writer) error {
	p := c.FindProfile(name)
	if p == nil {
		return fmt.Errorf("profile %q not found", name)
	}

	out := &Output{}
	out.Addf("Profile: %s", name)
	if p.Description != "" {
		out.Addf("Description: %s", p.Description)
	}
	out.Addf("Hosts: %s", strings.Join(p.MatchHosts, ", "))
	out.Addf("Headers:")

	for _, raw := range p.Headers {
		ps := ParseHeaderSource(raw)
		switch ps.Source {
		case SourceKeychain:
			val, err := kc.Get(ps.Ref)
			if err != nil {
				out.Addf("  %s: keychain:%s (error: %v)", ps.Name, ps.Ref, err)
			} else {
				out.Addf("  %s: keychain:%s (%s)", ps.Name, ps.Ref, curlparse.MaskValue(val))
			}
		case SourceEnv:
			val := os.Getenv(ps.Ref)
			if val == "" {
				out.Addf("  %s: env:%s (not set)", ps.Name, ps.Ref)
			} else {
				out.Addf("  %s: env:%s (%s)", ps.Name, ps.Ref, curlparse.MaskValue(val))
			}
		case SourcePlaintext:
			if curlparse.IsSecretHeader(ps.Name) {
				out.Addf("  %s: %s (⚠ plaintext secret)", ps.Name, curlparse.MaskValue(ps.Ref))
			} else {
				out.Addf("  %s: %s", ps.Name, ps.Ref)
			}
		}
	}

	out.Print(w)
	return nil
}

func deleteProfile(c *Config, name string, kc keyring.Keyring, cm *ConfigManager, w io.Writer, updateRulesFn func(*Config) error) error {
	p := c.FindProfile(name)
	if p == nil {
		return fmt.Errorf("profile %q not found", name)
	}

	keychainDeleted := 0
	for _, raw := range p.Headers {
		ps := ParseHeaderSource(raw)
		if ps.Source == SourceKeychain {
			if err := kc.Delete(ps.Ref); err != nil {
				fmt.Fprintf(w, "pcurl: warning: %v\n", err)
			} else {
				keychainDeleted++
			}
		}
	}

	delete(c.Profiles, name)
	if err := cm.Save(c); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	if err := updateRulesFn(c); err != nil {
		fmt.Fprintf(w, "pcurl: warning: could not update agent rules: %v\n", err)
	}

	out := &Output{}
	if keychainDeleted > 0 {
		out.Addf("Deleted profile %q and %d keychain entry(ies)", name, keychainDeleted)
	} else {
		out.Addf("Deleted profile %q", name)
	}
	out.Print(w)
	return nil
}
