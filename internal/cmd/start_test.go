package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luuuc/council/internal/config"
	"github.com/luuuc/council/internal/detect"
)

func TestStartCmd_FailsIfCouncilExists(t *testing.T) {
	testInTempDir(t, func(t *testing.T, dir string) {
		// Council already exists from testInTempDir setup
		err := startCmd.RunE(startCmd, []string{})
		if err == nil {
			t.Fatal("expected error when .council/ already exists, got nil")
		}

		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("error should mention 'already exists', got: %v", err)
		}
	})
}

func TestStartCmd_CreatesCouncilDirectory(t *testing.T) {
	// Create temp directory WITHOUT council init
	tmpDir, cleanup := setupTempDirNoInit(t)
	defer cleanup()

	// Create .claude directory so tool detection works
	if err := os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755); err != nil {
		t.Fatal(err)
	}

	err := runStart()
	if err != nil {
		t.Fatalf("runStart failed: %v", err)
	}

	// Verify .council/ was created
	if !config.Exists() {
		t.Error(".council/ directory was not created")
	}

	// Verify config.yaml was created
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Tool != "claude" {
		t.Errorf("expected tool 'claude', got '%s'", cfg.Tool)
	}
}

func TestStartCmd_AddsExperts(t *testing.T) {
	tmpDir, cleanup := setupTempDirNoInit(t)
	defer cleanup()

	// Create .claude directory
	if err := os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755); err != nil {
		t.Fatal(err)
	}

	err := runStart()
	if err != nil {
		t.Fatalf("runStart failed: %v", err)
	}

	// Verify experts were added
	files, err := filepath.Glob(config.Path(config.ExpertsDir, "*.md"))
	if err != nil {
		t.Fatalf("failed to glob expert files: %v", err)
	}

	if len(files) == 0 {
		t.Error("no expert files were created")
	}

	if len(files) > maxTotalExperts {
		t.Errorf("expected at most %d experts, got %d", maxTotalExperts, len(files))
	}
}

func TestSelectExperts_GoProject(t *testing.T) {
	d := &detect.Detection{
		Languages: []detect.Language{
			{Name: "Go", Percentage: 100},
		},
	}

	experts := selectExperts(d)

	if len(experts) == 0 {
		t.Fatal("expected experts to be selected for Go project")
	}

	// Should include The Go Purist for Go projects
	hasSable := false
	for _, e := range experts {
		if e.ID == "sable-okoro" {
			hasSable = true
			break
		}
	}

	if !hasSable {
		t.Error("expected The Go Purist to be selected for Go project")
	}
}

func TestSelectExperts_RailsProject(t *testing.T) {
	d := &detect.Detection{
		Languages: []detect.Language{
			{Name: "Ruby", Percentage: 100},
		},
		Frameworks: []detect.Framework{
			{Name: "Rails"},
		},
	}

	experts := selectExperts(d)

	if len(experts) == 0 {
		t.Fatal("expected experts to be selected for Rails project")
	}

	// Should include The Rails Monolith for Rails projects
	hasDiego := false
	for _, e := range experts {
		if e.ID == "diego-valdez" {
			hasDiego = true
			break
		}
	}

	if !hasDiego {
		t.Error("expected The Rails Monolith to be selected for Rails project")
	}
}

func TestSelectExperts_FallbackToGeneralists(t *testing.T) {
	// selectGeneralists is called when detection returns nothing useful
	generalists := selectGeneralists()

	if len(generalists) == 0 {
		t.Fatal("expected generalists to be selected")
	}

	// Should include The TDD Advocate as a generalist
	hasAda := false
	for _, e := range generalists {
		if e.ID == "ada-redgrave" {
			hasAda = true
			break
		}
	}

	if !hasAda {
		t.Error("expected The TDD Advocate in generalists")
	}
}

func TestSelectExperts_MaxLimit(t *testing.T) {
	d := &detect.Detection{
		Languages: []detect.Language{
			{Name: "Go", Percentage: 50},
			{Name: "Ruby", Percentage: 30},
			{Name: "Python", Percentage: 20},
		},
		Frameworks: []detect.Framework{
			{Name: "Rails"},
			{Name: "Django"},
		},
		Testing: []string{"Go testing"},
	}

	experts := selectExperts(d)

	if len(experts) > maxTotalExperts {
		t.Errorf("expected at most %d experts, got %d", maxTotalExperts, len(experts))
	}
}

func TestMapDetectionToCategories(t *testing.T) {
	d := &detect.Detection{
		Languages: []detect.Language{
			{Name: "Go", Percentage: 100},
		},
		Frameworks: []detect.Framework{
			{Name: "React"},
		},
		Testing: []string{"Go testing"},
	}

	categories := mapDetectionToCategories(d)

	// Should contain "go", "frontend", and "testing"
	expected := map[string]bool{"go": true, "frontend": true, "testing": true}
	for _, cat := range categories {
		delete(expected, cat)
	}

	if len(expected) > 0 {
		t.Errorf("missing categories: %v", expected)
	}
}

func TestDetectTool_ReturnsGenericWhenNoneDetected(t *testing.T) {
	_, cleanup := setupTempDirNoInit(t)
	defer cleanup()

	// No .claude or .opencode directories
	tool, err := detectTool()
	if err != nil {
		t.Fatalf("detectTool failed: %v", err)
	}

	if tool != "generic" {
		t.Errorf("expected 'generic' when no tool detected, got '%s'", tool)
	}
}

func TestDetectTool_ReturnsClaude(t *testing.T) {
	tmpDir, cleanup := setupTempDirNoInit(t)
	defer cleanup()

	// Create .claude directory
	if err := os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755); err != nil {
		t.Fatal(err)
	}

	tool, err := detectTool()
	if err != nil {
		t.Fatalf("detectTool failed: %v", err)
	}

	if tool != "claude" {
		t.Errorf("expected 'claude', got '%s'", tool)
	}
}

func TestJoinNames(t *testing.T) {
	tests := []struct {
		names    []string
		expected string
	}{
		{[]string{}, ""},
		{[]string{"Alice"}, "Alice"},
		{[]string{"Alice", "Bob"}, "Alice, Bob"},
		{[]string{"Alice", "Bob", "Charlie"}, "Alice, Bob, Charlie"},
	}

	for _, tt := range tests {
		result := joinNames(tt.names)
		if result != tt.expected {
			t.Errorf("joinNames(%v) = %q, want %q", tt.names, result, tt.expected)
		}
	}
}

// setupTempDirNoInit creates a temp directory without initializing council
func setupTempDirNoInit(t *testing.T) (string, func()) {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	tmpDir, err := os.MkdirTemp("", "council-test-*")
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	return tmpDir, func() {
		_ = os.Chdir(original)
		_ = os.RemoveAll(tmpDir)
	}
}
