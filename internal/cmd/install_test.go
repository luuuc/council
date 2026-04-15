package cmd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luuuc/council-cli/internal/config"
)

func TestExpandGitHubShorthand(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "https://example.com/expert.md",
			expected: "https://example.com/expert.md",
		},
		{
			input:    "http://example.com/expert.md",
			expected: "http://example.com/expert.md",
		},
		{
			input:    "user/repo/expert",
			expected: "https://raw.githubusercontent.com/user/repo/main/expert.md",
		},
		{
			input:    "user/repo/expert.md",
			expected: "https://raw.githubusercontent.com/user/repo/main/expert.md",
		},
		{
			input:    "user/repo/experts/my-cto",
			expected: "https://raw.githubusercontent.com/user/repo/main/experts/my-cto.md",
		},
		{
			input:    "invalid",
			expected: "invalid", // returns as-is, will fail on HTTP GET
		},
		// Edge cases
		{
			input:    "user/repo",
			expected: "user/repo", // only 2 parts, not a valid shorthand
		},
		{
			input:    "user-name/repo-name/path/to/expert",
			expected: "https://raw.githubusercontent.com/user-name/repo-name/main/path/to/expert.md",
		},
		{
			input:    "user/repo/expert-with-dashes",
			expected: "https://raw.githubusercontent.com/user/repo/main/expert-with-dashes.md",
		},
		{
			input:    "",
			expected: "", // empty returns as-is
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := expandGitHubShorthand(tc.input)
			if result != tc.expected {
				t.Errorf("expandGitHubShorthand(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestRunInstall(t *testing.T) {
	// Create a test server
	validPersona := `---
id: test-expert
name: Test Expert
focus: Testing code quality
---

# Test Expert

Expert in testing.
`

	invalidYAML := `no frontmatter here`

	missingFields := `---
id: test-expert
---

Missing name and focus.
`

	emptyResponse := ``

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/valid.md":
			_, _ = w.Write([]byte(validPersona))
		case "/invalid.md":
			_, _ = w.Write([]byte(invalidYAML))
		case "/missing-fields.md":
			_, _ = w.Write([]byte(missingFields))
		case "/empty.md":
			_, _ = w.Write([]byte(emptyResponse))
		case "/notfound.md":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create temp directory for testing
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

	t.Run("valid persona", func(t *testing.T) {
		// Clean up any existing expert
		_ = os.Remove(filepath.Join(tmpDir, config.CouncilDir, config.ExpertsDir, "test-expert.md"))

		err := runInstall(server.URL + "/valid.md")
		if err != nil {
			t.Fatalf("runInstall() error = %v", err)
		}

		// Verify file was created
		path := filepath.Join(tmpDir, config.CouncilDir, config.ExpertsDir, "test-expert.md")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Error("expected expert file to be created")
		}
	})

	t.Run("invalid YAML", func(t *testing.T) {
		err := runInstall(server.URL + "/invalid.md")
		if err == nil {
			t.Error("expected error for invalid YAML")
		}
	})

	t.Run("missing required fields", func(t *testing.T) {
		err := runInstall(server.URL + "/missing-fields.md")
		if err == nil {
			t.Error("expected error for missing fields")
		}
	})

	t.Run("404 response", func(t *testing.T) {
		err := runInstall(server.URL + "/notfound.md")
		if err == nil {
			t.Error("expected error for 404 response")
		}
	})

	t.Run("empty response", func(t *testing.T) {
		err := runInstall(server.URL + "/empty.md")
		if err == nil {
			t.Error("expected error for empty response")
		}
	})

	t.Run("duplicate expert", func(t *testing.T) {
		// First install should succeed
		_ = os.Remove(filepath.Join(tmpDir, config.CouncilDir, config.ExpertsDir, "test-expert.md"))
		err := runInstall(server.URL + "/valid.md")
		if err != nil {
			t.Fatalf("first install error = %v", err)
		}

		// Second install should fail
		err = runInstall(server.URL + "/valid.md")
		if err == nil {
			t.Error("expected error for duplicate expert")
		}
	})
}

func TestRunInstallNotInitialized(t *testing.T) {
	// Create temp directory without council
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	cmd := installCmd
	err := cmd.RunE(cmd, []string{"https://example.com/expert.md"})
	if err == nil {
		t.Error("expected error when council not initialized")
	}
}

func TestRunInstallTooLarge(t *testing.T) {
	// Create a server that returns a response larger than maxPersonaSize
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write more than 1MB (maxPersonaSize)
		largeContent := make([]byte, maxPersonaSize+100)
		for i := range largeContent {
			largeContent[i] = 'x'
		}
		_, _ = w.Write(largeContent)
	}))
	defer server.Close()

	// Create temp directory for testing
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

	err := runInstall(server.URL + "/large.md")
	if err == nil {
		t.Error("expected error for response exceeding size limit")
	}
	if err != nil && !strings.Contains(err.Error(), "too large") {
		t.Errorf("expected 'too large' error, got: %v", err)
	}
}
