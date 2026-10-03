package cmd

import (
	"os"
	"path/filepath"
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

func TestInitAddsDefaultMember(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if err := initCouncil(false, "generic"); err != nil {
		t.Fatalf("init: %v", err)
	}

	e, err := expert.Load("luc-perussault-diallo")
	if err != nil {
		t.Fatalf("new councils should start with the author's persona: %v", err)
	}
	if e.Name != "Virtual Luc Perussault-Diallo" || e.Body == "" {
		t.Errorf("default member not saved as a full persona: %+v", e)
	}

	// It can be removed like any other member.
	if err := expert.Delete("luc-perussault-diallo"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if expert.Exists("luc-perussault-diallo") {
		t.Error("default member should be removable")
	}
}

// writePersona writes a persona file for council add.
func writePersona(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAddCmd_Person(t *testing.T) {
	testInTempDir(t, func(t *testing.T, dir string) {
		path := writePersona(t, dir, "jane.md", `Here is the persona:
---
name: Jane Doe
focus: Test-driven development
sources:
  - "A talk on TDD"
principles:
  - Write the test first
inferred:
  - Prefers small commits
---`)
		addNoSync = true
		t.Cleanup(func() { addNoSync = false })

		if err := addCmd.RunE(addCmd, []string{path}); err != nil {
			t.Fatalf("add: %v", err)
		}
		e, err := expert.Load("jane-doe")
		if err != nil {
			t.Fatalf("persona not saved: %v", err)
		}
		if e.Name != "Virtual Jane Doe" || e.Kind != expert.KindPerson {
			t.Errorf("got %q kind %q", e.Name, e.Kind)
		}
		if !strings.Contains(e.Body, "Not affiliated with or endorsed by Jane Doe") || !strings.Contains(e.Body, "## Inferred From Their Work") {
			t.Errorf("body should carry the disclaimer and inferred positions:\n%s", e.Body)
		}

		// Adding the same person again is refused.
		err = addCmd.RunE(addCmd, []string{path})
		if err == nil || !strings.Contains(err.Error(), "already on the council") {
			t.Errorf("duplicate add should be refused, got %v", err)
		}
	})
}

func TestAddCmd_RejectsPersonWithoutSources(t *testing.T) {
	testInTempDir(t, func(t *testing.T, dir string) {
		path := writePersona(t, dir, "jane.md", "name: Jane Doe\nfocus: Testing\nprinciples:\n  - Test first\n")
		err := addCmd.RunE(addCmd, []string{path})
		if err == nil || !strings.Contains(err.Error(), "public sources") {
			t.Fatalf("expected a sources error, got %v", err)
		}
		if expert.Exists("jane-doe") {
			t.Error("rejected persona should not be saved")
		}
	})
}

func TestAddCmd_NameInsteadOfFile(t *testing.T) {
	testInTempDir(t, func(t *testing.T, dir string) {
		err := addCmd.RunE(addCmd, []string{"Jane Doe"})
		if err == nil || !strings.Contains(err.Error(), "takes a persona file, not a name") || !strings.Contains(err.Error(), "council assemble") {
			t.Errorf("a name should get a pointer to the AI flow, got %v", err)
		}
	})
}
