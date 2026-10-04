// Package config manages council configuration stored in .council/config.yaml.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	CouncilDir = ".council"
	ConfigFile = "config.yaml"
	ExpertsDir = "experts"
	PacksDir   = "packs"
)

// Config represents the council configuration
type Config struct {
	Version int      `yaml:"version"`
	Tool    string   `yaml:"tool,omitempty"` // Primary tool: "claude", "opencode", "generic"
	AI      AIConfig `yaml:"ai"`
	Targets []string `yaml:"targets,omitempty"` // Optional: override sync targets
}

// AIConfig holds the model API settings for unattended reviews
// (council review --api). In an AI tool, the tool's own model answers.
// Older keys (command, args, backend, mix, timeout) are ignored.
type AIConfig struct {
	Provider string `yaml:"provider,omitempty"` // "anthropic", "openai", "ollama", "github"
	Model    string `yaml:"model,omitempty"`    // e.g. "claude-sonnet-4-6", "gpt-4o"
}

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

// Default returns a default configuration.
func Default() *Config {
	return &Config{Version: 1}
}

// DetectProvider returns the model API and model for council review --api:
// the configured provider, else the first one with a key in the
// environment (GitHub Models last: the free tier).
func (c *Config) DetectProvider() (provider, model string, err error) {
	provider = c.AI.Provider
	if provider == "" {
		for _, p := range []string{"anthropic", "openai", "github"} {
			if os.Getenv(ProviderEnvKeys[p]) != "" {
				provider = p
				break
			}
		}
	}
	if provider == "" {
		return "", "", fmt.Errorf("no model API key found: set ANTHROPIC_API_KEY, OPENAI_API_KEY, or GITHUB_TOKEN, or pass --provider")
	}
	model = c.AI.Model
	if model == "" {
		model = DefaultModels[provider]
	}
	if model == "" {
		return "", "", fmt.Errorf("provider %s needs a model: pass --model or set ai.model in .council/config.yaml", provider)
	}
	return provider, model, nil
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
			return Default(), nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
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
