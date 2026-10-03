package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luuuc/council/internal/config"
	"github.com/luuuc/council/internal/expert"
)

func TestGeneratePublishReadme(t *testing.T) {
	experts := []*expert.Expert{
		{ID: "my-cto", Name: "My CTO", Focus: "Backend architecture"},
		{ID: "security-expert", Name: "Security Expert", Focus: "Application security"},
	}

	readme := generatePublishReadme(experts)

	// Check key sections exist
	if !strings.Contains(readme, "# Council Personas") {
		t.Error("README should contain title")
	}
	if !strings.Contains(readme, "## Install") {
		t.Error("README should contain install section")
	}
	if !strings.Contains(readme, "## Personas") {
		t.Error("README should contain personas section")
	}
	if !strings.Contains(readme, "[My CTO]") {
		t.Error("README should list My CTO")
	}
	if !strings.Contains(readme, "[Security Expert]") {
		t.Error("README should list Security Expert")
	}
	if !strings.Contains(readme, "council install") {
		t.Error("README should contain install command")
	}
}

func TestRunPublish(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Initialize council
	if err := os.MkdirAll(filepath.Join(tmpDir, config.CouncilDir, config.ExpertsDir), 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	cfg := config.Default()
	if err := cfg.Save(); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Create a custom expert
	customExpert := &expert.Expert{
		ID:    "my-custom-expert",
		Name:  "My Custom Expert",
		Focus: "Custom focus area",
	}
	if err := customExpert.Save(); err != nil {
		t.Fatalf("failed to save custom expert: %v", err)
	}

	t.Run("publish custom only", func(t *testing.T) {
		// Clean up output dir
		_ = os.RemoveAll("council-personas")

		err := runPublish()
		if err != nil {
			t.Fatalf("runPublish() error = %v", err)
		}

		// Verify output directory exists
		if _, err := os.Stat("council-personas"); os.IsNotExist(err) {
			t.Error("expected council-personas/ to be created")
		}

		// Verify README exists
		if _, err := os.Stat("council-personas/README.md"); os.IsNotExist(err) {
			t.Error("expected README.md to be created")
		}

		// Verify expert file exists
		if _, err := os.Stat("council-personas/my-custom-expert.md"); os.IsNotExist(err) {
			t.Error("expected expert file to be created")
		}
	})
}

func TestRunPublishEmpty(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Initialize council with no experts
	if err := os.MkdirAll(filepath.Join(tmpDir, config.CouncilDir, config.ExpertsDir), 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	cfg := config.Default()
	if err := cfg.Save(); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	err := runPublish()
	if err == nil {
		t.Error("expected error when no experts to publish")
	}
}
