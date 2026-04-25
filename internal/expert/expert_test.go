package expert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luuuc/council/internal/config"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *Expert
		wantErr bool
	}{
		{
			name: "valid expert with all fields",
			input: `---
id: kent-beck
name: Kent Beck
focus: Test-driven development and simple design
philosophy: Write tests first, then code.
principles:
  - Red-green-refactor
  - Simple design over complexity
red_flags:
  - Tests written after code
  - Over-engineering
---

# Kent Beck

Expert in testing.`,
			want: &Expert{
				ID:         "kent-beck",
				Name:       "Kent Beck",
				Focus:      "Test-driven development and simple design",
				Philosophy: "Write tests first, then code.",
				Principles: []string{"Red-green-refactor", "Simple design over complexity"},
				RedFlags:   []string{"Tests written after code", "Over-engineering"},
				Body:       "# Kent Beck\n\nExpert in testing.",
			},
			wantErr: false,
		},
		{
			name: "minimal expert",
			input: `---
id: minimal
name: Minimal Expert
focus: Minimal focus
---

Body content.`,
			want: &Expert{
				ID:    "minimal",
				Name:  "Minimal Expert",
				Focus: "Minimal focus",
				Body:  "Body content.",
			},
			wantErr: false,
		},
		{
			name: "expert with influences and backstory",
			input: `---
id: ada-redgrave
name: Ada Redgrave
focus: Test-driven development and incremental design
influences:
  - "Kent Beck — TDD, red-green-refactor"
  - "Michael Feathers — Working with legacy code"
backstory: |
  Former embedded systems engineer who moved to web development.
philosophy: Untested code is a liability.
principles:
  - Red-green-refactor
red_flags:
  - Code without tests
---

# Ada Redgrave`,
			want: &Expert{
				ID:         "ada-redgrave",
				Name:       "Ada Redgrave",
				Focus:      "Test-driven development and incremental design",
				Influences: []string{"Kent Beck — TDD, red-green-refactor", "Michael Feathers — Working with legacy code"},
				Backstory:  "Former embedded systems engineer who moved to web development.\n",
				Philosophy: "Untested code is a liability.",
				Principles: []string{"Red-green-refactor"},
				RedFlags:   []string{"Code without tests"},
				Body:       "# Ada Redgrave",
			},
			wantErr: false,
		},
		{
			name: "expert with tensions",
			input: `---
id: kent-beck
name: Kent Beck
focus: Test-driven development
tensions:
  - expert: dhh
    topic: test depth
    position: Comprehensive unit tests for every class
    counterpoint: System tests that cover user flows, minimal unit tests
  - expert: rob-pike
    topic: abstraction
    position: Extract when the pattern emerges three times
    counterpoint: A little copying is better than a little dependency
---

# Kent Beck`,
			want: &Expert{
				ID:    "kent-beck",
				Name:  "Kent Beck",
				Focus: "Test-driven development",
				Tensions: []Tension{
					{
						Expert:       "dhh",
						Topic:        "test depth",
						Position:     "Comprehensive unit tests for every class",
						Counterpoint: "System tests that cover user flows, minimal unit tests",
					},
					{
						Expert:       "rob-pike",
						Topic:        "abstraction",
						Position:     "Extract when the pattern emerges three times",
						Counterpoint: "A little copying is better than a little dependency",
					},
				},
				Body: "# Kent Beck",
			},
			wantErr: false,
		},
		{
			name:    "missing frontmatter",
			input:   "No frontmatter here",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid frontmatter format",
			input:   "---\nno closing delimiter",
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid yaml in frontmatter",
			input: `---
id: [invalid yaml
name: broken
---

Body.`,
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if got.ID != tt.want.ID {
				t.Errorf("Parse() ID = %v, want %v", got.ID, tt.want.ID)
			}
			if got.Name != tt.want.Name {
				t.Errorf("Parse() Name = %v, want %v", got.Name, tt.want.Name)
			}
			if got.Focus != tt.want.Focus {
				t.Errorf("Parse() Focus = %v, want %v", got.Focus, tt.want.Focus)
			}
			if got.Philosophy != tt.want.Philosophy {
				t.Errorf("Parse() Philosophy = %v, want %v", got.Philosophy, tt.want.Philosophy)
			}
			if got.Backstory != tt.want.Backstory {
				t.Errorf("Parse() Backstory = %q, want %q", got.Backstory, tt.want.Backstory)
			}
			if len(got.Influences) != len(tt.want.Influences) {
				t.Errorf("Parse() Influences len = %v, want %v", len(got.Influences), len(tt.want.Influences))
			} else {
				for i, inf := range got.Influences {
					if inf != tt.want.Influences[i] {
						t.Errorf("Parse() Influences[%d] = %q, want %q", i, inf, tt.want.Influences[i])
					}
				}
			}
			if got.Body != tt.want.Body {
				t.Errorf("Parse() Body = %v, want %v", got.Body, tt.want.Body)
			}
			if len(got.Principles) != len(tt.want.Principles) {
				t.Errorf("Parse() Principles len = %v, want %v", len(got.Principles), len(tt.want.Principles))
			}
			if len(got.RedFlags) != len(tt.want.RedFlags) {
				t.Errorf("Parse() RedFlags len = %v, want %v", len(got.RedFlags), len(tt.want.RedFlags))
			}
			if len(got.Tensions) != len(tt.want.Tensions) {
				t.Errorf("Parse() Tensions len = %v, want %v", len(got.Tensions), len(tt.want.Tensions))
			} else {
				for i, tension := range got.Tensions {
					if tension != tt.want.Tensions[i] {
						t.Errorf("Parse() Tensions[%d] = %v, want %v", i, tension, tt.want.Tensions[i])
					}
				}
			}
		})
	}
}

func TestSave(t *testing.T) {
	// Create a temp directory for testing
	tmpDir, err := os.MkdirTemp("", "council-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Change to temp directory and set up council structure
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	// Create the council directory structure
	_ = os.MkdirAll(config.Path(config.ExpertsDir), 0755)

	tests := []struct {
		name    string
		expert  *Expert
		wantErr bool
	}{
		{
			name: "save expert with all fields",
			expert: &Expert{
				ID:         "diego-valdez",
				Name:       "Diego Valdez",
				Focus:      "Rails and convention over configuration",
				Philosophy: "Optimize for programmer happiness.",
				Principles: []string{"Convention over configuration", "DRY"},
				RedFlags:   []string{"Too much abstraction"},
			},
			wantErr: false,
		},
		{
			name: "save expert with influences and backstory",
			expert: &Expert{
				ID:         "composite",
				Name:       "Ada Redgrave",
				Focus:      "Test-driven development",
				Influences: []string{"Kent Beck — TDD", "Michael Feathers — Legacy code"},
				Backstory:  "Former embedded systems engineer.",
				Philosophy: "Untested code is a liability.",
				Principles: []string{"Red-green-refactor"},
				RedFlags:   []string{"Code without tests"},
			},
			wantErr: false,
		},
		{
			name: "save minimal expert",
			expert: &Expert{
				ID:    "minimal",
				Name:  "Minimal",
				Focus: "Testing",
			},
			wantErr: false,
		},
		{
			name: "save expert with tensions",
			expert: &Expert{
				ID:    "tensions-test",
				Name:  "Tensions Test",
				Focus: "Testing tensions round-trip",
				Tensions: []Tension{
					{
						Expert:       "other-expert",
						Topic:        "test philosophy",
						Position:     "Unit tests are essential",
						Counterpoint: "Integration tests are sufficient",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "save expert with custom body",
			expert: &Expert{
				ID:    "custom",
				Name:  "Custom",
				Focus: "Custom focus",
				Body:  "# Custom Body\n\nThis is a custom body.",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.expert.Save()
			if (err != nil) != tt.wantErr {
				t.Errorf("Save() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Verify file was created
			path := tt.expert.Path()
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Errorf("Save() did not create file at %s", path)
				return
			}

			// Load it back and verify
			loaded, err := Load(tt.expert.ID)
			if err != nil {
				t.Errorf("Failed to load saved expert: %v", err)
				return
			}

			if loaded.ID != tt.expert.ID {
				t.Errorf("Loaded ID = %v, want %v", loaded.ID, tt.expert.ID)
			}
			if loaded.Name != tt.expert.Name {
				t.Errorf("Loaded Name = %v, want %v", loaded.Name, tt.expert.Name)
			}
			if loaded.Focus != tt.expert.Focus {
				t.Errorf("Loaded Focus = %v, want %v", loaded.Focus, tt.expert.Focus)
			}
			if len(loaded.Tensions) != len(tt.expert.Tensions) {
				t.Errorf("Loaded Tensions len = %v, want %v", len(loaded.Tensions), len(tt.expert.Tensions))
			} else {
				for i, tension := range loaded.Tensions {
					if tension != tt.expert.Tensions[i] {
						t.Errorf("Loaded Tensions[%d] = %v, want %v", i, tension, tt.expert.Tensions[i])
					}
				}
			}
		})
	}
}

func TestToID(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"Kent Beck", "kent-beck"},
		{"DHH", "dhh"},
		{"José Valim", "jos-valim"},
		{"Rob Pike", "rob-pike"},
		{"Already-Kebab", "already-kebab"},
		{"  Spaces  Around  ", "spaces-around"},
		{"Special!@#$%Characters", "special-characters"},
		{"123Numbers", "123numbers"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToID(tt.name); got != tt.want {
				t.Errorf("ToID(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestParseAIResponse(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCount int
		wantErr   bool
	}{
		{
			name: "yaml in code block",
			input: `Here are the experts:

` + "```yaml" + `
experts:
  - id: kent-beck
    name: Kent Beck
    focus: TDD
  - id: dhh
    name: DHH
    focus: Rails
` + "```" + `

That's all!`,
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "yaml in generic code block",
			input: "```\nexperts:\n  - id: test\n    name: Test\n    focus: Testing\n```",
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "plain yaml",
			input: `experts:
  - id: plain
    name: Plain Expert
    focus: Plain focus`,
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:      "invalid yaml",
			input:     "not: [valid: yaml",
			wantCount: 0,
			wantErr:   true,
		},
		{
			name:      "empty experts",
			input:     "experts: []",
			wantCount: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAIResponse([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseAIResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != tt.wantCount {
				t.Errorf("ParseAIResponse() returned %d experts, want %d", len(got), tt.wantCount)
			}
		})
	}
}

func TestListWithWarnings(t *testing.T) {
	// Create a temp directory for testing
	tmpDir, err := os.MkdirTemp("", "council-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Change to temp directory
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	// Create the council directory structure
	expertsDir := config.Path(config.ExpertsDir)
	_ = os.MkdirAll(expertsDir, 0755)

	// Create a valid expert file
	validExpert := &Expert{
		ID:    "valid",
		Name:  "Valid Expert",
		Focus: "Testing",
	}
	_ = validExpert.Save()

	// Create an invalid expert file
	invalidPath := filepath.Join(expertsDir, "invalid.md")
	_ = os.WriteFile(invalidPath, []byte("not valid frontmatter"), 0644)

	result, err := ListWithWarnings()
	if err != nil {
		t.Fatalf("ListWithWarnings() error = %v", err)
	}

	if len(result.Experts) != 1 {
		t.Errorf("ListWithWarnings() got %d experts, want 1", len(result.Experts))
	}

	if len(result.Warnings) != 1 {
		t.Errorf("ListWithWarnings() got %d warnings, want 1", len(result.Warnings))
	}

	if len(result.Experts) > 0 && result.Experts[0].ID != "valid" {
		t.Errorf("ListWithWarnings() expert ID = %v, want valid", result.Experts[0].ID)
	}
}

func TestExistsAndDelete(t *testing.T) {
	// Create a temp directory for testing
	tmpDir, err := os.MkdirTemp("", "council-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Change to temp directory
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	// Create the council directory structure
	_ = os.MkdirAll(config.Path(config.ExpertsDir), 0755)

	// Test that non-existent expert doesn't exist
	if Exists("nonexistent") {
		t.Error("Exists() returned true for non-existent expert")
	}

	// Create an expert
	expert := &Expert{
		ID:    "test",
		Name:  "Test Expert",
		Focus: "Testing",
	}
	_ = expert.Save()

	// Test that it now exists
	if !Exists("test") {
		t.Error("Exists() returned false for existing expert")
	}

	// Delete it
	err = Delete("test")
	if err != nil {
		t.Errorf("Delete() error = %v", err)
	}

	// Test that it no longer exists
	if Exists("test") {
		t.Error("Exists() returned true after deletion")
	}

	// Test deleting non-existent expert
	err = Delete("nonexistent")
	if err == nil {
		t.Error("Delete() should error for non-existent expert")
	}
}

func TestGenerateBody(t *testing.T) {
	expert := &Expert{
		ID:         "test",
		Name:       "Test Expert",
		Focus:      "Testing methodology",
		Philosophy: "Test everything.",
		Principles: []string{"Write tests first", "Keep tests simple"},
		RedFlags:   []string{"No tests", "Flaky tests"},
	}

	body := expert.generateBody()

	if !strings.Contains(body, "Test Expert") {
		t.Error("generateBody() missing expert name")
	}
	if !strings.Contains(body, "Testing methodology") {
		t.Error("generateBody() missing focus")
	}
	if !strings.Contains(body, "Test everything.") {
		t.Error("generateBody() missing philosophy")
	}
	if !strings.Contains(body, "Write tests first") {
		t.Error("generateBody() missing principle")
	}
	if !strings.Contains(body, "No tests") {
		t.Error("generateBody() missing red flag")
	}
	if strings.Contains(body, "channeling") {
		t.Error("generateBody() should not contain 'channeling'")
	}
	if !strings.Contains(body, "You are Test Expert") {
		t.Error("generateBody() should contain 'You are <Name>' identity line")
	}
}

func TestGenerateBody_WithBackstory(t *testing.T) {
	e := &Expert{
		ID:        "ada-redgrave",
		Name:      "Ada Redgrave",
		Focus:     "Test-driven development",
		Backstory: "Former embedded systems engineer who moved to web development.",
	}

	body := e.generateBody()

	if !strings.Contains(body, "You are Ada Redgrave") {
		t.Error("generateBody() should use 'You are' identity, not 'channeling'")
	}
	if !strings.Contains(body, "Former embedded systems engineer") {
		t.Error("generateBody() missing backstory content")
	}
}

func TestGenerateBody_WithTensions(t *testing.T) {
	e := &Expert{
		ID:    "test-tensions",
		Name:  "Test Expert",
		Focus: "Testing",
		Tensions: []Tension{
			{
				Expert:       "other-expert",
				Topic:        "test depth",
				Position:     "Unit tests for every class",
				Counterpoint: "System tests are sufficient",
			},
		},
	}

	body := e.generateBody()

	if !strings.Contains(body, "## Tensions") {
		t.Error("generateBody() missing Tensions section")
	}
	if !strings.Contains(body, "other-expert") {
		t.Error("generateBody() missing tension expert name")
	}
	if !strings.Contains(body, "test depth") {
		t.Error("generateBody() missing tension topic")
	}
	if !strings.Contains(body, "Unit tests for every class") {
		t.Error("generateBody() missing tension position")
	}
}

func TestGenerateBody_MinimalExpert(t *testing.T) {
	expert := &Expert{
		ID:    "minimal",
		Name:  "Minimal",
		Focus: "Minimal focus",
	}

	body := expert.generateBody()

	if body == "" {
		t.Error("generateBody() should not return empty string for minimal expert")
	}
	if !strings.Contains(body, "Minimal") {
		t.Error("generateBody() should contain expert name")
	}
	if !strings.Contains(body, "Minimal focus") {
		t.Error("generateBody() should contain focus")
	}
	if strings.Contains(body, "## Philosophy") {
		t.Error("generateBody() should not contain Philosophy section when empty")
	}
	if strings.Contains(body, "channeling") {
		t.Error("generateBody() should not contain 'channeling'")
	}
}

func TestGenerateBody_EmptyStrings(t *testing.T) {
	// Test with empty strings - should handle gracefully
	expert := &Expert{
		ID:         "",
		Name:       "",
		Focus:      "",
		Philosophy: "",
		Principles: []string{},
		RedFlags:   []string{},
	}

	body := expert.generateBody()

	// Should not panic and should produce some output
	if body == "" {
		t.Error("generateBody() should not return empty string")
	}
	// The template should still have the Review Style section
	if !strings.Contains(body, "Review Style") {
		t.Error("generateBody() should contain Review Style section")
	}
}

func TestGenerateBody_SpecialCharacters(t *testing.T) {
	// Test with special characters that might cause template issues
	expert := &Expert{
		ID:         "special",
		Name:       "Expert with <html> & \"quotes\"",
		Focus:      "Testing {{templates}}",
		Philosophy: "Philosophy with `backticks` and $variables",
		Principles: []string{"Principle with {{braces}}"},
		RedFlags:   []string{"Red flag with <angle> brackets"},
	}

	body := expert.generateBody()

	// Should not panic and should contain the special characters as-is
	// (text/template does not escape by default)
	if !strings.Contains(body, "<html>") {
		t.Error("generateBody() should preserve HTML-like content")
	}
	if !strings.Contains(body, "{{templates}}") {
		t.Error("generateBody() should preserve template-like content as literal text")
	}
}

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *Expert
		wantErr bool
	}{
		{
			name: "valid frontmatter",
			input: `id: kent-beck
name: Kent Beck
focus: TDD expert
philosophy: Write tests first.
principles:
  - Red-green-refactor
red_flags:
  - No tests`,
			want: &Expert{
				ID:         "kent-beck",
				Name:       "Kent Beck",
				Focus:      "TDD expert",
				Philosophy: "Write tests first.",
				Principles: []string{"Red-green-refactor"},
				RedFlags:   []string{"No tests"},
			},
			wantErr: false,
		},
		{
			name: "with category and priority",
			input: `id: custom-expert
name: Custom
focus: Custom focus
category: custom
priority: high`,
			want: &Expert{
				ID:       "custom-expert",
				Name:     "Custom",
				Focus:    "Custom focus",
				Category: "custom",
				Priority: "high",
			},
			wantErr: false,
		},
		{
			name:    "invalid yaml",
			input:   "id: [broken",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFrontmatter([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFrontmatter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if got.ID != tt.want.ID {
				t.Errorf("ParseFrontmatter() ID = %v, want %v", got.ID, tt.want.ID)
			}
			if got.Name != tt.want.Name {
				t.Errorf("ParseFrontmatter() Name = %v, want %v", got.Name, tt.want.Name)
			}
			if got.Category != tt.want.Category {
				t.Errorf("ParseFrontmatter() Category = %v, want %v", got.Category, tt.want.Category)
			}
			if got.Priority != tt.want.Priority {
				t.Errorf("ParseFrontmatter() Priority = %v, want %v", got.Priority, tt.want.Priority)
			}
		})
	}
}

func TestApplyDefaults(t *testing.T) {
	tests := []struct {
		name         string
		expert       *Expert
		wantCategory string
		wantPriority string
	}{
		{
			name:         "empty fields get defaults",
			expert:       &Expert{ID: "test"},
			wantCategory: "custom",
			wantPriority: "normal",
		},
		{
			name:         "existing values preserved",
			expert:       &Expert{ID: "test", Category: "rails", Priority: "high"},
			wantCategory: "rails",
			wantPriority: "high",
		},
		{
			name:         "partial defaults",
			expert:       &Expert{ID: "test", Category: "go"},
			wantCategory: "go",
			wantPriority: "normal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.expert.ApplyDefaults()
			if tt.expert.Category != tt.wantCategory {
				t.Errorf("ApplyDefaults() Category = %v, want %v", tt.expert.Category, tt.wantCategory)
			}
			if tt.expert.Priority != tt.wantPriority {
				t.Errorf("ApplyDefaults() Priority = %v, want %v", tt.expert.Priority, tt.wantPriority)
			}
		})
	}
}

func TestSaveToPath(t *testing.T) {
	// Create a temp directory for testing
	tmpDir, err := os.MkdirTemp("", "council-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	expert := &Expert{
		ID:         "test-save-path",
		Name:       "Test Save Path",
		Focus:      "Testing SaveToPath",
		Philosophy: "Test all paths.",
		Principles: []string{"Test first"},
	}

	path := filepath.Join(tmpDir, "experts", "test.md")

	err = SaveToPath(expert, path)
	if err != nil {
		t.Fatalf("SaveToPath() error = %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("SaveToPath() did not create file at %s", path)
		return
	}

	// Load it back and verify
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read saved file: %v", err)
	}

	loaded, err := Parse(data)
	if err != nil {
		t.Fatalf("Failed to parse saved file: %v", err)
	}

	if loaded.ID != expert.ID {
		t.Errorf("SaveToPath() ID = %v, want %v", loaded.ID, expert.ID)
	}
	if loaded.Name != expert.Name {
		t.Errorf("SaveToPath() Name = %v, want %v", loaded.Name, expert.Name)
	}
}

func TestSaveToPath_RoundtripValidation(t *testing.T) {
	// Create a temp directory for testing
	tmpDir, err := os.MkdirTemp("", "council-test-roundtrip-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Test that saving and loading produces consistent data
	expert := &Expert{
		ID:         "roundtrip-test",
		Name:       "Roundtrip Test",
		Focus:      "Testing roundtrip validation",
		Philosophy: "Validate all saves.",
		Principles: []string{"Test", "Verify", "Repeat"},
		RedFlags:   []string{"Untested saves"},
		Category:   "test",
		Priority:   "high",
	}

	path := filepath.Join(tmpDir, "experts", "roundtrip.md")

	err = SaveToPath(expert, path)
	if err != nil {
		t.Fatalf("SaveToPath() error = %v", err)
	}

	// Verify the file exists (roundtrip validation passed)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("SaveToPath() file was removed, likely due to roundtrip failure")
	}

	// Load and verify all fields match
	loaded, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}

	if loaded.ID != expert.ID {
		t.Errorf("ID = %q, want %q", loaded.ID, expert.ID)
	}
	if loaded.Name != expert.Name {
		t.Errorf("Name = %q, want %q", loaded.Name, expert.Name)
	}
	if loaded.Focus != expert.Focus {
		t.Errorf("Focus = %q, want %q", loaded.Focus, expert.Focus)
	}
	if loaded.Philosophy != expert.Philosophy {
		t.Errorf("Philosophy = %q, want %q", loaded.Philosophy, expert.Philosophy)
	}
	if len(loaded.Principles) != len(expert.Principles) {
		t.Errorf("Principles len = %d, want %d", len(loaded.Principles), len(expert.Principles))
	}
}

func TestMarshalExpertsJSON(t *testing.T) {
	experts := []*Expert{
		{
			ID:         "test-json",
			Name:       "Test JSON",
			Focus:      "Testing JSON conversion",
			Philosophy: "JSON all the things.",
			Principles: []string{"Serialize", "Deserialize"},
			RedFlags:   []string{"Invalid JSON"},
			Category:   "testing",
			Priority:   "high",
			// Internal fields should NOT be in JSON output
			Core:     true,
			Triggers: []string{"test"},
			Body:     "# Body content",
			Source:   "custom",
		},
	}

	data, err := MarshalExpertsJSON(experts)
	if err != nil {
		t.Fatalf("MarshalExpertsJSON() error = %v", err)
	}

	jsonStr := string(data)

	// Verify included fields
	if !strings.Contains(jsonStr, `"id": "test-json"`) {
		t.Error("JSON should contain id field")
	}
	if !strings.Contains(jsonStr, `"name": "Test JSON"`) {
		t.Error("JSON should contain name field")
	}
	if !strings.Contains(jsonStr, `"category": "testing"`) {
		t.Error("JSON should contain category field")
	}
	if !strings.Contains(jsonStr, `"red_flags"`) {
		t.Error("JSON should contain red_flags field")
	}

	// Verify excluded fields (internal metadata)
	if strings.Contains(jsonStr, `"core"`) {
		t.Error("JSON should NOT contain core field")
	}
	if strings.Contains(jsonStr, `"triggers"`) {
		t.Error("JSON should NOT contain triggers field")
	}
	if strings.Contains(jsonStr, `"body"`) && strings.Contains(jsonStr, "Body content") {
		t.Error("JSON should NOT contain body field")
	}
	if strings.Contains(jsonStr, `"source"`) {
		t.Error("JSON should NOT contain source field")
	}

	// Verify tensions omitted when empty
	if strings.Contains(jsonStr, `"tensions"`) {
		t.Error("JSON should NOT contain tensions field when empty")
	}
}

func TestMarshalExpertsJSON_WithTensions(t *testing.T) {
	experts := []*Expert{
		{
			ID:    "test-tensions",
			Name:  "Test Tensions",
			Focus: "Testing",
			Tensions: []Tension{
				{
					Expert:       "other",
					Topic:        "testing",
					Position:     "Unit tests",
					Counterpoint: "Integration tests",
				},
			},
		},
	}

	data, err := MarshalExpertsJSON(experts)
	if err != nil {
		t.Fatalf("MarshalExpertsJSON() error = %v", err)
	}

	jsonStr := string(data)

	if !strings.Contains(jsonStr, `"tensions"`) {
		t.Error("JSON should contain tensions field")
	}
	if !strings.Contains(jsonStr, `"expert": "other"`) {
		t.Error("JSON should contain tension expert")
	}
	if !strings.Contains(jsonStr, `"topic": "testing"`) {
		t.Error("JSON should contain tension topic")
	}
}
