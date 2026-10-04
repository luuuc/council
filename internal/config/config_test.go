package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Version != 1 {
		t.Errorf("Default().Version = %d, want 1", cfg.Version)
	}
	// Tool should be empty (detected at runtime)
	if cfg.Tool != "" {
		t.Errorf("Default().Tool = %q, want empty (detected at runtime)", cfg.Tool)
	}
	// Targets should be empty (detected at sync time)
	if len(cfg.Targets) != 0 {
		t.Errorf("Default().Targets length = %d, want 0 (detected at sync time)", len(cfg.Targets))
	}
}

func TestPath(t *testing.T) {
	tests := []struct {
		parts []string
		want  string
	}{
		{[]string{}, ".council"},
		{[]string{"config.yaml"}, filepath.Join(".council", "config.yaml")},
		{[]string{"experts", "diego-valdez.md"}, filepath.Join(".council", "experts", "diego-valdez.md")},
	}

	for _, tt := range tests {
		got := Path(tt.parts...)
		if got != tt.want {
			t.Errorf("Path(%v) = %s, want %s", tt.parts, got, tt.want)
		}
	}
}

func TestExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Should not exist initially
	if Exists() {
		t.Error("Exists() should return false when .council doesn't exist")
	}

	// Create .council directory
	if err := os.MkdirAll(CouncilDir, 0755); err != nil {
		t.Fatalf("Failed to create council dir: %v", err)
	}

	if !Exists() {
		t.Error("Exists() should return true when .council exists")
	}

	// Create a file with same name (edge case)
	_ = os.RemoveAll(CouncilDir)
	if err := os.WriteFile(CouncilDir, []byte("not a dir"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	if Exists() {
		t.Error("Exists() should return false when .council is a file, not directory")
	}
}

func TestLoadAndSave(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Load should return defaults when not initialized
	defaultCfg, err := Load()
	if err != nil {
		t.Fatalf("Load() should return defaults when not initialized, got error: %v", err)
	}
	if defaultCfg.Version != 1 {
		t.Errorf("expected default version 1, got %d", defaultCfg.Version)
	}

	// Create .council directory and save config
	if err := os.MkdirAll(CouncilDir, 0755); err != nil {
		t.Fatalf("Failed to create council dir: %v", err)
	}

	cfg := Default()
	cfg.AI.Provider = "openai"
	cfg.Targets = []string{"claude", "windsurf"}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load should succeed now
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.AI.Provider != "openai" {
		t.Errorf("Load().AI.Provider = %s, want openai", loaded.AI.Provider)
	}
	if len(loaded.Targets) != 2 {
		t.Errorf("Load().Targets length = %d, want 2", len(loaded.Targets))
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	if err := os.MkdirAll(CouncilDir, 0755); err != nil {
		t.Fatalf("Failed to create council dir: %v", err)
	}

	// Write invalid YAML
	invalidYAML := []byte("version: [invalid\n  yaml: content")
	if err := os.WriteFile(Path(ConfigFile), invalidYAML, 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	_, err = Load()
	if err == nil {
		t.Error("Load() should error on invalid YAML")
	}
}

func TestConstants(t *testing.T) {
	if CouncilDir != ".council" {
		t.Errorf("CouncilDir = %s, want .council", CouncilDir)
	}
	if ConfigFile != "config.yaml" {
		t.Errorf("ConfigFile = %s, want config.yaml", ConfigFile)
	}
	if ExpertsDir != "experts" {
		t.Errorf("ExpertsDir = %s, want experts", ExpertsDir)
	}
}

func TestValidateTool(t *testing.T) {
	tests := []struct {
		tool    string
		wantErr bool
	}{
		{"", false},         // Empty is valid (detected at runtime)
		{"claude", false},   // Valid
		{"opencode", false}, // Valid
		{"generic", false},  // Valid
		{"invalid", true},   // Invalid
		{"Claude", true},    // Case sensitive
		{"cursor", true},    // Not a valid tool
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			err := ValidateTool(tt.tool)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTool(%q) error = %v, wantErr %v", tt.tool, err, tt.wantErr)
			}
		})
	}
}

func TestDetectProvider(t *testing.T) {
	tests := []struct {
		name         string
		cfg          Config
		env          map[string]string
		wantProvider string
		wantModel    string
		wantErr      string
	}{
		{name: "configured provider and model", cfg: Config{AI: AIConfig{Provider: "anthropic", Model: "claude-opus-4-6"}}, wantProvider: "anthropic", wantModel: "claude-opus-4-6"},
		{name: "configured provider, default model", cfg: Config{AI: AIConfig{Provider: "openai"}}, wantProvider: "openai", wantModel: "gpt-4o"},
		{name: "ollama needs a model", cfg: Config{AI: AIConfig{Provider: "ollama"}}, wantErr: "needs a model"},
		{name: "key in the environment", env: map[string]string{"OPENAI_API_KEY": "k"}, wantProvider: "openai", wantModel: "gpt-4o"},
		{name: "anthropic before github", env: map[string]string{"GITHUB_TOKEN": "k", "ANTHROPIC_API_KEY": "k"}, wantProvider: "anthropic", wantModel: "claude-sonnet-4-6"},
		{name: "configured provider beats the environment", cfg: Config{AI: AIConfig{Provider: "github"}}, env: map[string]string{"ANTHROPIC_API_KEY": "k"}, wantProvider: "github", wantModel: "openai/gpt-4.1-mini"},
		{name: "nothing", wantErr: "no model API key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, k := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GITHUB_TOKEN"} {
				t.Setenv(k, tt.env[k])
			}
			provider, model, err := tt.cfg.DetectProvider()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || provider != tt.wantProvider || model != tt.wantModel {
				t.Errorf("got (%q, %q, %v), want (%q, %q)", provider, model, err, tt.wantProvider, tt.wantModel)
			}
		})
	}
}

func TestConfigToolFieldPersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	if err := os.MkdirAll(CouncilDir, 0755); err != nil {
		t.Fatalf("Failed to create council dir: %v", err)
	}

	// Save config with tool field
	cfg := Default()
	cfg.Tool = "claude"
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load and verify
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.Tool != "claude" {
		t.Errorf("Load().Tool = %q, want claude", loaded.Tool)
	}
}
