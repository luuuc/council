// Package config manages council configuration stored in .council/config.yaml.
package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	CouncilDir  = ".council"
	ConfigFile  = "config.yaml"
	ExpertsDir  = "experts"
	CommandsDir = "commands"
	PacksDir    = "packs"
)

// Config represents the council configuration
type Config struct {
	Version int      `yaml:"version"`
	Tool    string   `yaml:"tool,omitempty"` // Primary tool: "claude", "opencode", "generic"
	AI      AIConfig `yaml:"ai"`
	Targets []string `yaml:"targets,omitempty"` // Optional: override sync targets
}

// AIConfig holds AI configuration for reviews.
type AIConfig struct {
	Command     string   `yaml:"command,omitempty"`
	Args        []string `yaml:"args,omitempty"`
	Backend     string   `yaml:"backend,omitempty"`     // "cli" or "api"
	Provider    string   `yaml:"provider,omitempty"`     // "anthropic", "openai", "ollama"
	Model       string   `yaml:"model,omitempty"`        // e.g. "claude-sonnet-4-6", "gpt-4o"
	Timeout     int      `yaml:"timeout"`
	Concurrency int      `yaml:"concurrency,omitempty"`
}

// ValidBackends is the set of recognized backend values.
var ValidBackends = []string{"cli", "api"}

// ValidProviders is the set of recognized API provider values.
var ValidProviders = []string{"anthropic", "openai", "ollama", "github"}

// ProviderEnvKeys maps providers to their expected environment variable.
var ProviderEnvKeys = map[string]string{
	"anthropic": "ANTHROPIC_API_KEY",
	"openai":    "OPENAI_API_KEY",
	"github":    "GITHUB_TOKEN",
}

// DefaultModels maps providers to their default model.
var DefaultModels = map[string]string{
	"anthropic": "claude-sonnet-4-6",
	"openai":    "gpt-4o",
	"github":    "openai/gpt-4.1-mini",
}

// Default returns a default configuration
// AI.Command and Targets are intentionally empty - detected at runtime or set by installer
func Default() *Config {
	return &Config{
		Version: 1,
		AI: AIConfig{
			Timeout: 120,
		},
	}
}

// KnownAICLIs is the list of AI CLIs to detect, in order of preference
var KnownAICLIs = []string{"claude", "opencode", "aichat", "llm"}

// DetectAICommand returns the configured AI command, or detects one if not set.
// Returns empty string with nil error when backend is "api" and no CLI is needed.
func (c *Config) DetectAICommand() (string, error) {
	if c.AI.Command != "" {
		return c.AI.Command, nil
	}

	// If explicitly configured for API backend, no CLI needed
	if c.AI.Backend == "api" {
		return "", nil
	}

	// Try known CLIs in order of preference
	for _, cmd := range KnownAICLIs {
		if _, err := exec.LookPath(cmd); err == nil {
			return cmd, nil
		}
	}

	// If no CLI found, check for API keys before giving up
	if c.AI.Backend == "" {
		for _, provider := range []string{"anthropic", "openai", "github"} {
			if os.Getenv(ProviderEnvKeys[provider]) != "" {
				return "", nil // API key available, no CLI needed
			}
		}
	}

	return "", fmt.Errorf("no AI command configured and none detected\n\nInstall claude, opencode, aichat, or llm, set an API key (ANTHROPIC_API_KEY or OPENAI_API_KEY), or set ai.command in .council/config.yaml")
}

// DetectBackend determines the backend and provider to use.
// Returns (backend, provider, model) based on config and environment.
func (c *Config) DetectBackend() (string, string, string) {
	// Explicit config takes precedence
	if c.AI.Backend != "" {
		model := c.AI.Model
		if c.AI.Backend == "api" && c.AI.Provider != "" && model == "" {
			model = DefaultModels[c.AI.Provider]
		}
		return c.AI.Backend, c.AI.Provider, model
	}

	// Auto-detect: prefer CLI, then API key
	for _, cmd := range KnownAICLIs {
		if _, err := exec.LookPath(cmd); err == nil {
			return "cli", "", ""
		}
	}

	// No CLI found — check for API keys (github last: free tier fallback)
	for _, p := range []string{"anthropic", "openai", "github"} {
		if os.Getenv(ProviderEnvKeys[p]) != "" {
			model := DefaultModels[p]
			if c.AI.Model != "" {
				model = c.AI.Model
			}
			return "api", p, model
		}
	}

	// Nothing detected
	return "", "", ""
}

// Path returns the full path to a council file or directory
func Path(parts ...string) string {
	all := append([]string{CouncilDir}, parts...)
	return filepath.Join(all...)
}

// Exists checks if the council directory exists
func Exists() bool {
	info, err := os.Stat(CouncilDir)
	return err == nil && info.IsDir()
}

// Load loads the configuration from .council/config.yaml
func Load() (*Config, error) {
	path := Path(ConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := Default()
			cfg.applyDefaults()
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Apply defaults for missing values
	cfg.applyDefaults()

	return &cfg, nil
}

// applyDefaults fills in missing configuration with sensible defaults
// Note: AI.Command and Targets are NOT defaulted here - they are detected at runtime
func (c *Config) applyDefaults() {
	defaults := Default()

	// If no timeout specified, use default
	if c.AI.Timeout == 0 {
		c.AI.Timeout = defaults.AI.Timeout
	}

	// If no concurrency specified, use default
	if c.AI.Concurrency == 0 {
		c.AI.Concurrency = 4
	}
}

// Save saves the configuration to .council/config.yaml
func (c *Config) Save() error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(Path(ConfigFile), data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// ValidTools is the list of valid tool values
var ValidTools = []string{"claude", "opencode", "generic"}

// ValidateTool checks if the tool name is valid
func ValidateTool(tool string) error {
	if tool == "" {
		return nil // Empty is valid (will be detected)
	}
	for _, valid := range ValidTools {
		if tool == valid {
			return nil
		}
	}
	return fmt.Errorf("invalid tool '%s': must be one of: claude, opencode, generic", tool)
}
