package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luuuc/council/internal/config"
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

func TestAddCmd_Success(t *testing.T) {
	testInTempDir(t, func(t *testing.T, dir string) {
		// Run addCmd with a known persona
		err := addCmd.RunE(addCmd, []string{"The Go Purist"})
		if err != nil {
			t.Fatalf("addCmd failed: %v", err)
		}

		// Verify file was created
		expertPath := config.Path(config.ExpertsDir, "sable-okoro.md")
		if _, err := os.Stat(expertPath); os.IsNotExist(err) {
			t.Errorf("expert file not created at %s", expertPath)
		}

		// Verify file content
		data, err := os.ReadFile(expertPath)
		if err != nil {
			t.Fatalf("failed to read expert file: %v", err)
		}

		content := string(data)
		if !strings.Contains(content, "The Go Purist") {
			t.Error("expert file does not contain 'The Go Purist'")
		}
		if !strings.Contains(content, "id: sable-okoro") {
			t.Error("expert file does not contain 'id: sable-okoro'")
		}
	})
}

func TestAddCmd_DuplicateExpert(t *testing.T) {
	testInTempDir(t, func(t *testing.T, dir string) {
		// Add expert first time
		err := addCmd.RunE(addCmd, []string{"The Go Purist"})
		if err != nil {
			t.Fatalf("first addCmd failed: %v", err)
		}

		// Try to add again
		err = addCmd.RunE(addCmd, []string{"The Go Purist"})
		if err == nil {
			t.Fatal("expected error for duplicate expert, got nil")
		}

		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("error message should contain 'already exists', got: %v", err)
		}
	})
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

func TestAddCmd_FuzzySuggestion(t *testing.T) {
	testInTempDir(t, func(t *testing.T, dir string) {
		// Test with a typo that should suggest The Go Purist
		err := addCmd.RunE(addCmd, []string{"The Go Puris"})

		if err == nil {
			// Expert was added (interactive mode with auto-confirm)
			expertPath := config.Path(config.ExpertsDir, "sable-okoro.md")
			if _, statErr := os.Stat(expertPath); os.IsNotExist(statErr) {
				t.Errorf("expert should have been added at %s", expertPath)
			}
		} else {
			// Non-interactive mode - should have suggestion
			errMsg := err.Error()
			if !strings.Contains(errMsg, "Did you mean") && !strings.Contains(errMsg, "The Go Purist") {
				t.Errorf("error should contain suggestion, got: %v", err)
			}
		}
	})
}

func TestAddCmd_FirstNameMatch(t *testing.T) {
	testInTempDir(t, func(t *testing.T, dir string) {
		// Test first-name lookup — only works for non-"The" names
		// (all composite personas are "The X", so only "Luc" has a unique first name)
		err := addCmd.RunE(addCmd, []string{"Luc"})
		if err != nil {
			t.Fatalf("addCmd failed: %v", err)
		}

		expertPath := config.Path(config.ExpertsDir, "luc-perussault-diallo.md")
		if _, err := os.Stat(expertPath); os.IsNotExist(err) {
			t.Errorf("expert file not created at %s", expertPath)
		}
	})
}

func TestAddCmd_CaseInsensitive(t *testing.T) {
	testInTempDir(t, func(t *testing.T, dir string) {
		// Test case insensitive lookup
		err := addCmd.RunE(addCmd, []string{"THE GO PURIST"})
		if err != nil {
			t.Fatalf("addCmd failed: %v", err)
		}

		// Verify file was created with correct ID
		expertPath := config.Path(config.ExpertsDir, "sable-okoro.md")
		if _, err := os.Stat(expertPath); os.IsNotExist(err) {
			t.Errorf("expert file not created at %s", expertPath)
		}
	})
}

func TestAddCmd_IDFormat(t *testing.T) {
	testInTempDir(t, func(t *testing.T, dir string) {
		// Test using ID format directly
		err := addCmd.RunE(addCmd, []string{"ada-redgrave"})
		if err != nil {
			t.Fatalf("addCmd failed: %v", err)
		}

		expertPath := config.Path(config.ExpertsDir, "ada-redgrave.md")
		if _, err := os.Stat(expertPath); os.IsNotExist(err) {
			t.Errorf("expert file not created at %s", expertPath)
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
	err = addCmd.RunE(addCmd, []string{"The Go Purist"})
	if err == nil {
		t.Fatal("expected error when council not initialized, got nil")
	}

	if !strings.Contains(err.Error(), "council not initialized") {
		t.Errorf("error should mention 'council not initialized', got: %v", err)
	}
}

func TestAddCmd_YesFlag(t *testing.T) {
	testInTempDir(t, func(t *testing.T, dir string) {
		// Set --yes flag
		addYes = true
		defer func() { addYes = false }()

		// Test with a typo - should auto-accept suggestion with --yes
		err := addCmd.RunE(addCmd, []string{"The Go Puris"})
		if err != nil {
			t.Fatalf("addCmd with --yes failed: %v", err)
		}

		// Verify The Go Purist was added
		expertPath := config.Path(config.ExpertsDir, "sable-okoro.md")
		if _, err := os.Stat(expertPath); os.IsNotExist(err) {
			t.Errorf("expert file not created at %s", expertPath)
		}
	})
}

func TestAddCmd_YesFlagDuplicate(t *testing.T) {
	testInTempDir(t, func(t *testing.T, dir string) {
		// Add expert first
		err := addCmd.RunE(addCmd, []string{"The Go Purist"})
		if err != nil {
			t.Fatalf("first addCmd failed: %v", err)
		}

		// Set --yes flag and try with typo
		addYes = true
		defer func() { addYes = false }()

		err = addCmd.RunE(addCmd, []string{"The Go Puris"})
		if err == nil {
			t.Fatal("expected error for duplicate expert, got nil")
		}

		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("error message should contain 'already exists', got: %v", err)
		}
	})
}

func TestListExperts(t *testing.T) {
	testInTempDir(t, func(t *testing.T, dir string) {
		// Add two experts
		if err := addCmd.RunE(addCmd, []string{"The Go Purist"}); err != nil {
			t.Fatalf("failed to add The Go Purist: %v", err)
		}
		if err := addCmd.RunE(addCmd, []string{"The TDD Advocate"}); err != nil {
			t.Fatalf("failed to add The TDD Advocate: %v", err)
		}

		// Verify both files exist
		files, err := filepath.Glob(config.Path(config.ExpertsDir, "*.md"))
		if err != nil {
			t.Fatalf("failed to glob expert files: %v", err)
		}

		if len(files) != 2 {
			t.Errorf("expected 2 expert files, got %d", len(files))
		}
	})
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
