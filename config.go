package pith

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Config is pith's set-and-forget user configuration: a default AI backend and
// stored API keys, so you hand them to pith once instead of exporting env vars
// per shell. Plain JSON in the OS user-config dir — portable, no registry, no
// vendor anything. Explicit flags always beat the config; env vars beat stored
// keys. The deterministic ops never read it.
type Config struct {
	API   string            `json:"api,omitempty"`   // default --api (preset or URL)
	Model string            `json:"model,omitempty"` // default --model
	Agent string            `json:"agent,omitempty"` // default --agent
	Cmd   string            `json:"cmd,omitempty"`   // default --cmd
	Keys  map[string]string `json:"keys,omitempty"`  // API keys by env-var name
}

// ConfigPath returns the config file location: <user-config-dir>/pith/config.json
// (%APPDATA%\pith on Windows, ~/.config/pith on Linux, ~/Library/Application
// Support/pith on macOS).
func ConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "pith", "config.json"), nil
}

// LoadConfig reads the config file. A missing file is not an error — it just
// yields the zero Config (no defaults, no keys).
func LoadConfig() (Config, error) {
	var c Config
	p, err := ConfigPath()
	if err != nil {
		return c, err
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return c, fmt.Errorf("%s: %v", p, err)
	}
	return c, nil
}

// Save writes the config file, creating the directory if needed. The file is
// user-only (0600) since it can hold API keys.
func (c Config) Save() error {
	p, err := ConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, append(data, '\n'), 0o600)
}

// Key returns the stored API key for an env-var name ("" if none).
func (c Config) Key(envName string) string {
	return c.Keys[envName]
}

// Set assigns a config field by name. Backend fields (api, model, agent, cmd)
// are matched case-insensitively; anything else is treated as an API-key name
// and stored in Keys (conventionally an env-var name like OPENROUTER_API_KEY).
func (c *Config) Set(name, value string) {
	switch strings.ToLower(name) {
	case "api":
		c.API = value
	case "model":
		c.Model = value
	case "agent":
		c.Agent = value
	case "cmd":
		c.Cmd = value
	default:
		if c.Keys == nil {
			c.Keys = map[string]string{}
		}
		c.Keys[name] = value
	}
}

// Unset clears a config field by name (same matching as [Config.Set]).
func (c *Config) Unset(name string) {
	switch strings.ToLower(name) {
	case "api":
		c.API = ""
	case "model":
		c.Model = ""
	case "agent":
		c.Agent = ""
	case "cmd":
		c.Cmd = ""
	default:
		delete(c.Keys, name)
	}
}

// Render writes a human view of the config with keys masked (first 8 chars).
func (c Config) Render(w *strings.Builder) {
	if c.API != "" {
		fmt.Fprintf(w, "api    %s\n", c.API)
	}
	if c.Model != "" {
		fmt.Fprintf(w, "model  %s\n", c.Model)
	}
	if c.Agent != "" {
		fmt.Fprintf(w, "agent  %s\n", c.Agent)
	}
	if c.Cmd != "" {
		fmt.Fprintf(w, "cmd    %s\n", c.Cmd)
	}
	names := make([]string, 0, len(c.Keys))
	for n := range c.Keys {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(w, "%s = %s\n", n, mask(c.Keys[n]))
	}
	if w.Len() == 0 {
		w.WriteString("(empty)\n")
	}
}

// mask hides all but the first 8 characters of a secret.
func mask(s string) string {
	if len(s) <= 8 {
		return "********"
	}
	return s[:8] + strings.Repeat("*", 8)
}
