package pcurl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config represents the top-level profiles.toml structure.
type Config struct {
	Profiles map[string]*Profile `toml:"Profiles"`
}

// Profile represents a single named credential profile.
type Profile struct {
	Description string   `toml:"Description,omitempty"`
	MatchHosts  []string `toml:"MatchHosts"`
	Headers     []string `toml:"Headers"`
}

// HeaderSource describes where a header value comes from.
type HeaderSource int

const (
	SourcePlaintext HeaderSource = iota
	SourceKeychain
	SourceEnv
)

// ParsedSource is the result of parsing a single header line from TOML.
type ParsedSource struct {
	Name   string
	Source HeaderSource
	Ref    string
}

const (
	keychainPrefix = "keychain:"
	envPrefix      = "env:"
)

// ParseHeaderSource splits "Authorization: keychain:gh/auth" into components.
func ParseHeaderSource(raw string) ParsedSource {
	name, value, ok := strings.Cut(raw, ": ")
	if !ok {
		return ParsedSource{Name: raw, Source: SourcePlaintext}
	}

	switch {
	case strings.HasPrefix(value, keychainPrefix):
		return ParsedSource{Name: name, Source: SourceKeychain, Ref: strings.TrimPrefix(value, keychainPrefix)}
	case strings.HasPrefix(value, envPrefix):
		return ParsedSource{Name: name, Source: SourceEnv, Ref: strings.TrimPrefix(value, envPrefix)}
	default:
		return ParsedSource{Name: name, Source: SourcePlaintext, Ref: value}
	}
}

func (c *Config) FindProfile(name string) *Profile {
	return c.Profiles[name]
}

// FindProfileByHost returns the profile name whose MatchHosts contains host.
func (c *Config) FindProfileByHost(host string) string {
	for name, p := range c.Profiles {
		for _, h := range p.MatchHosts {
			if strings.EqualFold(h, host) {
				return name
			}
		}
	}
	return ""
}

// ConfigManager owns config file path and load/save operations.
type ConfigManager struct {
	dir  string
	path string
}

func NewConfigManager() *ConfigManager {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".config", "pcurl")
	return &ConfigManager{
		dir:  dir,
		path: filepath.Join(dir, "profiles.toml"),
	}
}

// NewConfigManagerWithDir creates a ConfigManager with a custom directory (for tests).
func NewConfigManagerWithDir(dir string) *ConfigManager {
	return &ConfigManager{
		dir:  dir,
		path: filepath.Join(dir, "profiles.toml"),
	}
}

func (cm *ConfigManager) Dir() string  { return cm.dir }
func (cm *ConfigManager) Path() string { return cm.path }

// EnsureDir creates the config directory with 0700 permissions.
func (cm *ConfigManager) EnsureDir() error {
	return os.MkdirAll(cm.dir, 0700)
}

// Load reads and parses profiles.toml. Returns empty config if file does not exist.
func (cm *ConfigManager) Load() (*Config, error) {
	if _, err := os.Stat(cm.path); os.IsNotExist(err) {
		return &Config{Profiles: make(map[string]*Profile)}, nil
	}

	if err := checkPermissions(cm.path); err != nil {
		return nil, err
	}

	var c Config
	if _, err := toml.DecodeFile(cm.path, &c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", cm.path, err)
	}
	if c.Profiles == nil {
		c.Profiles = make(map[string]*Profile)
	}
	return &c, nil
}

// Save writes config to profiles.toml with 0600 permissions.
func (cm *ConfigManager) Save(c *Config) error {
	if err := cm.EnsureDir(); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	f, err := os.OpenFile(cm.path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("open %s: %w", cm.path, err)
	}
	defer f.Close()

	enc := toml.NewEncoder(f)
	enc.Indent = ""
	return enc.Encode(c)
}

func checkPermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	perm := info.Mode().Perm()
	if perm&0077 != 0 {
		return fmt.Errorf("%s has permissions %04o, want 0600; fix with: chmod 600 %s", path, perm, path)
	}
	return nil
}
