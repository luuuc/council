package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParse_ValidExpert(t *testing.T) {
	data := []byte(`---
id: ada-redgrave
name: Ada Redgrave
focus: Testing and TDD
---

# Ada Redgrave

Expert in testing.`)

	expert, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if expert.ID != "ada-redgrave" {
		t.Errorf("ID = %q, want ada-redgrave", expert.ID)
	}
	if expert.Name != "Ada Redgrave" {
		t.Errorf("Name = %q, want Ada Redgrave", expert.Name)
	}
	if expert.Focus != "Testing and TDD" {
		t.Errorf("Focus = %q, want Testing and TDD", expert.Focus)
	}
	if expert.Body != "# Ada Redgrave\n\nExpert in testing." {
		t.Errorf("Body = %q, unexpected content", expert.Body)
	}
}

func TestParse_MissingFrontmatter(t *testing.T) {
	data := []byte(`# No frontmatter

Just markdown content.`)

	_, err := Parse(data)
	if err == nil {
		t.Error("Parse() should return error for missing frontmatter")
	}
}

func TestParse_UnclosedFrontmatter(t *testing.T) {
	data := []byte(`---
id: test
name: Test

Missing closing delimiter.`)

	_, err := Parse(data)
	if err == nil {
		t.Error("Parse() should return error for unclosed frontmatter")
	}
}

func TestParse_MinimalFrontmatter(t *testing.T) {
	data := []byte(`---
id: test
name: Test
---

Content.`)

	expert, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if expert.ID != "test" {
		t.Errorf("ID = %q, want test", expert.ID)
	}
}

func TestLoadFile_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.md")

	content := `---
id: test-expert
name: Test Expert
focus: Testing
---

# Test Expert

Content here.`

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	expert, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}

	if expert.ID != "test-expert" {
		t.Errorf("ID = %q, want test-expert", expert.ID)
	}
}

func TestLoadFile_NonexistentFile(t *testing.T) {
	_, err := LoadFile("/nonexistent/path/expert.md")
	if err == nil {
		t.Error("LoadFile() should return error for nonexistent file")
	}
}

func TestListExpertsInDir_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	experts, err := ListExpertsInDir(tmpDir, "test")
	if err != nil {
		t.Fatalf("ListExpertsInDir() error = %v", err)
	}
	if len(experts) != 0 {
		t.Errorf("ListExpertsInDir() returned %d experts, want 0", len(experts))
	}
}

func TestListExpertsInDir_NonexistentDir(t *testing.T) {
	experts, err := ListExpertsInDir("/nonexistent/path", "test")
	if err != nil {
		t.Fatalf("ListExpertsInDir() error = %v", err)
	}
	if len(experts) != 0 {
		t.Errorf("ListExpertsInDir() returned %d experts, want 0", len(experts))
	}
}

func TestListExpertsInDir_WithExperts(t *testing.T) {
	tmpDir := t.TempDir()

	// Create valid expert file
	expert1 := `---
id: expert-1
name: Expert One
focus: Focus 1
---

Content 1.`
	if err := os.WriteFile(filepath.Join(tmpDir, "expert-1.md"), []byte(expert1), 0644); err != nil {
		t.Fatal(err)
	}

	// Create another valid expert file
	expert2 := `---
id: expert-2
name: Expert Two
focus: Focus 2
---

Content 2.`
	if err := os.WriteFile(filepath.Join(tmpDir, "expert-2.md"), []byte(expert2), 0644); err != nil {
		t.Fatal(err)
	}

	// Create README.md (should be skipped)
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Readme"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create subdirectory (should be skipped)
	if err := os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create non-markdown file (should be skipped)
	if err := os.WriteFile(filepath.Join(tmpDir, "notes.txt"), []byte("notes"), 0644); err != nil {
		t.Fatal(err)
	}

	experts, err := ListExpertsInDir(tmpDir, "test-source")
	if err != nil {
		t.Fatalf("ListExpertsInDir() error = %v", err)
	}

	if len(experts) != 2 {
		t.Errorf("ListExpertsInDir() returned %d experts, want 2", len(experts))
	}

	// Verify source is set
	for _, e := range experts {
		if e.Source != "test-source" {
			t.Errorf("Expert %s source = %q, want test-source", e.ID, e.Source)
		}
	}
}

func TestListExpertsInDir_SkipsInvalidFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create valid expert file
	valid := `---
id: valid
name: Valid Expert
focus: Testing
---

Content.`
	if err := os.WriteFile(filepath.Join(tmpDir, "valid.md"), []byte(valid), 0644); err != nil {
		t.Fatal(err)
	}

	// Create invalid expert file (no frontmatter)
	invalid := `# No frontmatter

Just content.`
	if err := os.WriteFile(filepath.Join(tmpDir, "invalid.md"), []byte(invalid), 0644); err != nil {
		t.Fatal(err)
	}

	experts, err := ListExpertsInDir(tmpDir, "test")
	if err != nil {
		t.Fatalf("ListExpertsInDir() error = %v", err)
	}

	if len(experts) != 1 {
		t.Errorf("ListExpertsInDir() returned %d experts, want 1 (should skip invalid)", len(experts))
	}

	if experts[0].ID != "valid" {
		t.Errorf("Expected valid expert, got %s", experts[0].ID)
	}
}
