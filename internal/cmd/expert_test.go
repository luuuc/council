package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/luuuc/council/internal/config"
	"github.com/luuuc/council/internal/expert"
)

// testInTempDir runs a test function in a temporary directory,
// setting up a council project structure.
func testInTempDir(t *testing.T, fn func(t *testing.T, dir string)) {
	t.Helper()

	// Save current directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "council-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Change to temp directory
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Initialize council structure
	if err := os.MkdirAll(config.Path(config.ExpertsDir), 0755); err != nil {
		t.Fatalf("failed to create experts dir: %v", err)
	}
	cfg := config.Default()
	if err := cfg.Save(); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	fn(t, tmpDir)
}

func TestAddCmd_NotFound(t *testing.T) {
	testInTempDir(t, func(t *testing.T, dir string) {
		// With new behavior, unknown personas trigger creation flow
		// In interactive mode without input, it will fail on "focus is required"
		// This tests that the creation flow is triggered
		err := addCmd.RunE(addCmd, []string{"Unknown Person XYZ"})
		if err == nil {
			t.Fatal("expected error for unknown persona without focus input, got nil")
		}

		errMsg := err.Error()
		// Either we get the creation flow asking for focus (interactive)
		// or we get "not found" (non-interactive - stdin is piped/closed)
		if !strings.Contains(errMsg, "focus is required") && !strings.Contains(errMsg, "not found") {
			t.Errorf("error message should contain 'focus is required' or 'not found', got: %v", err)
		}
	})
}

func TestAddCmd_NoCouncilInit(t *testing.T) {
	// Save current directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}

	// Create temp directory WITHOUT council init
	tmpDir, err := os.MkdirTemp("", "council-test-noinit-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()
	defer func() { _ = os.Chdir(origDir) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}

	// Try to add without council init
	err = addCmd.RunE(addCmd, []string{"Virtual Cleo"})
	if err == nil {
		t.Fatal("expected error when council not initialized, got nil")
	}

	if !strings.Contains(err.Error(), "council not initialized") {
		t.Errorf("error should mention 'council not initialized', got: %v", err)
	}
}

// Note: Interactive flag tests (--interview, --from) are skipped because
// isInteractive() behavior varies by test environment. The flags are tested
// implicitly through the NoArgWithoutFlags test which verifies the error
// messages include these options.

func TestAddCmd_NoArgWithoutFlags(t *testing.T) {
	testInTempDir(t, func(t *testing.T, dir string) {
		// No argument and no flags should produce helpful error
		err := addCmd.RunE(addCmd, []string{})
		if err == nil {
			t.Fatal("expected error for add without args, got nil")
		}

		errMsg := err.Error()
		if !strings.Contains(errMsg, "requires a persona name argument") {
			t.Errorf("error should mention 'requires a persona name argument', got: %v", err)
		}
		// Should suggest alternatives
		if !strings.Contains(errMsg, "--interview") || !strings.Contains(errMsg, "--from") {
			t.Errorf("error should suggest --interview and --from alternatives, got: %v", err)
		}
	})
}

func TestTrimNewline(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "unix newline",
			input:    "hello\n",
			expected: "hello",
		},
		{
			name:     "windows newline",
			input:    "hello\r\n",
			expected: "hello",
		},
		{
			name:     "no newline",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "multiple trailing newlines",
			input:    "hello\n\n\n",
			expected: "hello",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only newlines",
			input:    "\n\r\n",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trimNewline(tt.input)
			if result != tt.expected {
				t.Errorf("trimNewline(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestAddCmd_DuplicateExpert(t *testing.T) {
	testInTempDir(t, func(t *testing.T, dir string) {
		stubAI(t, `---
name: Cleo
focus: Simple, clear code
principles:
  - Clear is better than clever
red_flags:
  - Clever code
---`)
		addYes = true
		t.Cleanup(func() { addYes = false })

		if err := addCmd.RunE(addCmd, []string{"Cleo"}); err != nil {
			t.Fatalf("first add failed: %v", err)
		}
		err := addCmd.RunE(addCmd, []string{"Cleo"})
		if err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Errorf("second add should fail with 'already exists', got %v", err)
		}
	})
}

func TestListExperts(t *testing.T) {
	testInTempDir(t, func(t *testing.T, dir string) {
		for _, e := range []*expert.Expert{
			{ID: "cleo", Name: "Virtual Cleo", Focus: "Simple code"},
			{ID: "ada", Name: "Virtual Ada", Focus: "Tests first"},
		} {
			if err := e.Save(); err != nil {
				t.Fatalf("save %s: %v", e.ID, err)
			}
		}

		list, err := expert.List()
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(list) != 2 {
			t.Errorf("expected 2 experts, got %d", len(list))
		}
	})
}
