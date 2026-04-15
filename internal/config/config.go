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

// AIConfig holds AI CLI configuration
type AIConfig struct {
	Command     string   `yaml:"command"`
	Args        []string `yaml:"args,omitempty"`
	Timeout     int      `yaml:"timeout"`
	Concurrency int      `yaml:"concurrency,omitempty"`
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

// DetectAICommand returns the configured AI command, or detects one if not set
func (c *Config) DetectAICommand() (string, error) {
	if c.AI.Command != "" {
		return c.AI.Command, nil
	}

	// Try known CLIs in order of preference
	for _, cmd := range KnownAICLIs {
		if _, err := exec.LookPath(cmd); err == nil {
			return cmd, nil
		}
	}

	return "", fmt.Errorf("no AI command configured and none detected\n\nInstall claude, opencode, aichat, or llm, or set ai.command in .council/config.yaml")
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
			return nil, fmt.Errorf("council not initialized: run 'council init' first")
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
