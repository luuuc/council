package adapter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luuuc/council/internal/expert"
)

// Test helper to create a temp directory and change to it
func setupTempDir(t *testing.T) (string, func()) {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	tmpDir, err := os.MkdirTemp("", "adapter-test-*")
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

// Registry tests

func TestRegister_AddsToRegistry(t *testing.T) {
	// Adapters are registered via init(), so we just verify they exist
	if len(All()) == 0 {
		t.Error("expected adapters to be registered via init()")
	}
}

func TestGet_ReturnsAdapter(t *testing.T) {
	tests := []struct {
		name    string
		display string
	}{
		{"claude", "Claude Code"},
		{"opencode", "OpenCode"},
		{"generic", "Generic (AGENTS.md)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, ok := Get(tt.name)
			if !ok {
				t.Errorf("Get(%q) returned false, want true", tt.name)
				return
			}
			if a.Name() != tt.name {
				t.Errorf("Name() = %q, want %q", a.Name(), tt.name)
			}
			if a.DisplayName() != tt.display {
				t.Errorf("DisplayName() = %q, want %q", a.DisplayName(), tt.display)
			}
		})
	}
}

func TestGet_ReturnsFalseForUnknown(t *testing.T) {
	a, ok := Get("nonexistent")
	if ok {
		t.Error("Get(nonexistent) returned true, want false")
	}
	if a != nil {
		t.Error("Get(nonexistent) returned non-nil adapter, want nil")
	}
}

func TestAll_ReturnsAllAdapters(t *testing.T) {
	all := All()
	if len(all) < 3 {
		t.Errorf("All() returned %d adapters, want at least 3", len(all))
	}

	// Verify expected adapters exist
	expected := []string{"claude", "opencode", "generic"}
	for _, name := range expected {
		if _, ok := all[name]; !ok {
			t.Errorf("All() missing adapter %q", name)
		}
	}
}

func TestNames_ReturnsAllNames(t *testing.T) {
	names := Names()
	if len(names) < 3 {
		t.Errorf("Names() returned %d names, want at least 3", len(names))
	}
}

func TestDetect_ReturnsMatchingAdapters(t *testing.T) {
	tmpDir, cleanup := setupTempDir(t)
	defer cleanup()

	// No directories - should return empty (generic is excluded from detection)
	detected := Detect()
	if len(detected) != 0 {
		t.Errorf("Detect() with no dirs returned %d adapters, want 0", len(detected))
	}

	// Create .claude directory
	if err := os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755); err != nil {
		t.Fatal(err)
	}

	detected = Detect()
	if len(detected) != 1 {
		t.Errorf("Detect() with .claude/ returned %d adapters, want 1", len(detected))
	}
	if detected[0].Name() != "claude" {
		t.Errorf("Detect() returned %q, want claude", detected[0].Name())
	}

	// Create .opencode directory too
	if err := os.MkdirAll(filepath.Join(tmpDir, ".opencode"), 0755); err != nil {
		t.Fatal(err)
	}

	detected = Detect()
	if len(detected) != 2 {
		t.Errorf("Detect() with .claude/ and .opencode/ returned %d adapters, want 2", len(detected))
	}
}

// Claude adapter tests

func TestClaude_Detect_TrueWhenClaudeDirExists(t *testing.T) {
	tmpDir, cleanup := setupTempDir(t)
	defer cleanup()

	claude, _ := Get("claude")

	// Before creating directory
	if claude.Detect() {
		t.Error("Detect() returned true before .claude/ exists")
	}

	// After creating directory
	if err := os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755); err != nil {
		t.Fatal(err)
	}
	if !claude.Detect() {
		t.Error("Detect() returned false after .claude/ exists")
	}
}

func TestClaude_Paths(t *testing.T) {
	claude, _ := Get("claude")
	paths := claude.Paths()

	if paths.Agents != ".claude/agents" {
		t.Errorf("Agents = %q, want .claude/agents", paths.Agents)
	}
	if paths.Commands != ".claude/commands" {
		t.Errorf("Commands = %q, want .claude/commands", paths.Commands)
	}
	if len(paths.Deprecated) != 0 {
		t.Errorf("Deprecated = %v, want empty", paths.Deprecated)
	}
}

func TestClaude_FormatAgent_IncludesNameDescriptionTools(t *testing.T) {
	claude, _ := Get("claude")

	// Create a temp dir with a mock expert file
	tmpDir, cleanup := setupTempDir(t)
	defer cleanup()

	// Create .council/experts directory
	expertsDir := filepath.Join(tmpDir, ".council", "experts")
	if err := os.MkdirAll(expertsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create expert file
	expertContent := `---
id: ada
name: Ada
focus: Testing and TDD
---

# Ada - Testing and TDD

Expert content here.`

	if err := os.WriteFile(filepath.Join(expertsDir, "ada.md"), []byte(expertContent), 0644); err != nil {
		t.Fatal(err)
	}

	e := &expert.Expert{
		ID:    "ada",
		Name:  "Ada",
		Focus: "Testing and TDD",
	}

	result := claude.FormatAgent(e)

	// Claude adapter reads and returns the original file content
	if !strings.Contains(result, "name: Ada") {
		t.Error("FormatAgent() missing name in frontmatter")
	}
	if !strings.Contains(result, "focus: Testing and TDD") {
		t.Error("FormatAgent() missing focus in frontmatter")
	}
}

func TestClaude_FormatCommand_IncludesCorrectStructure(t *testing.T) {
	claude, _ := Get("claude")

	result := claude.FormatCommand("test-cmd", "Test description", "# Test Command\n\nBody here.")

	// Claude commands are plain markdown (no frontmatter)
	if strings.Contains(result, "---") {
		t.Error("FormatCommand() should not include frontmatter for Claude")
	}
	if !strings.Contains(result, "# Test Command") {
		t.Error("FormatCommand() missing body content")
	}
}

func TestClaude_Templates(t *testing.T) {
	claude, _ := Get("claude")
	templates := claude.Templates()

	if templates.Install == "" {
		t.Error("Templates().Install is empty")
	}
	if len(templates.Commands) == 0 {
		t.Error("Templates().Commands is empty")
	}

	// Verify expected commands exist
	expected := []string{"council-add", "council-remove"}
	for _, name := range expected {
		if _, ok := templates.Commands[name]; !ok {
			t.Errorf("Templates().Commands missing %q", name)
		}
	}

	// Claude templates should use AskUserQuestion
	if !strings.Contains(templates.Commands["council-add"], "AskUserQuestion") {
		t.Error("Claude council-add template should use AskUserQuestion")
	}
}

// OpenCode adapter tests

func TestOpenCode_Detect_TrueWhenOpencodeDirExists(t *testing.T) {
	tmpDir, cleanup := setupTempDir(t)
	defer cleanup()

	opencode, _ := Get("opencode")

	// Before creating directory
	if opencode.Detect() {
		t.Error("Detect() returned true before .opencode/ exists")
	}

	// After creating .opencode directory
	if err := os.MkdirAll(filepath.Join(tmpDir, ".opencode"), 0755); err != nil {
		t.Fatal(err)
	}
	if !opencode.Detect() {
		t.Error("Detect() returned false after .opencode/ exists")
	}
}

func TestOpenCode_Detect_TrueWhenOpencodeJsonExists(t *testing.T) {
	tmpDir, cleanup := setupTempDir(t)
	defer cleanup()

	opencode, _ := Get("opencode")

	// Create opencode.json file
	if err := os.WriteFile(filepath.Join(tmpDir, "opencode.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if !opencode.Detect() {
		t.Error("Detect() returned false after opencode.json exists")
	}
}

func TestOpenCode_Paths_ReturnsDeprecated(t *testing.T) {
	opencode, _ := Get("opencode")
	paths := opencode.Paths()

	if paths.Agents != ".opencode/agents" {
		t.Errorf("Agents = %q, want .opencode/agents", paths.Agents)
	}
	if paths.Commands != ".opencode/commands" {
		t.Errorf("Commands = %q, want .opencode/commands", paths.Commands)
	}
	if len(paths.Deprecated) != 1 || paths.Deprecated[0] != ".opencode/agent" {
		t.Errorf("Deprecated = %v, want [.opencode/agent]", paths.Deprecated)
	}
}

func TestOpenCode_FormatAgent_IncludesDescriptionModeTools(t *testing.T) {
	opencode, _ := Get("opencode")

	e := &expert.Expert{
		ID:         "ada",
		Name:       "Ada",
		Focus:      "Testing and TDD",
		Philosophy: "Tests are the first customer of your code.",
		Principles: []string{"Test first", "Red-green-refactor"},
		RedFlags:   []string{"Tests without assertions"},
	}

	result := opencode.FormatAgent(e)

	// OpenCode uses different frontmatter
	if !strings.Contains(result, "description: Testing and TDD") {
		t.Error("FormatAgent() missing description in frontmatter")
	}
	if !strings.Contains(result, "mode: subagent") {
		t.Error("FormatAgent() missing mode in frontmatter")
	}
	if !strings.Contains(result, "# Ada") {
		t.Error("FormatAgent() missing name heading")
	}
	if !strings.Contains(result, "## Philosophy") {
		t.Error("FormatAgent() missing Philosophy section")
	}
	if !strings.Contains(result, "## Principles") {
		t.Error("FormatAgent() missing Principles section")
	}
	if !strings.Contains(result, "## Red Flags") {
		t.Error("FormatAgent() missing Red Flags section")
	}
}

func TestOpenCode_FormatCommand_IncludesCorrectStructure(t *testing.T) {
	opencode, _ := Get("opencode")

	result := opencode.FormatCommand("test-cmd", "Test description", "# Test Command\n\nBody here.")

	// OpenCode commands have frontmatter
	if !strings.Contains(result, "---") {
		t.Error("FormatCommand() should include frontmatter for OpenCode")
	}
	if !strings.Contains(result, "description: Test description") {
		t.Error("FormatCommand() missing description in frontmatter")
	}
	if !strings.Contains(result, "mode: subagent") {
		t.Error("FormatCommand() missing mode in frontmatter")
	}
	if !strings.Contains(result, "# Test Command") {
		t.Error("FormatCommand() missing body content")
	}
}

func TestOpenCode_Templates(t *testing.T) {
	opencode, _ := Get("opencode")
	templates := opencode.Templates()

	if templates.Install == "" {
		t.Error("Templates().Install is empty")
	}
	if len(templates.Commands) == 0 {
		t.Error("Templates().Commands is empty")
	}

	// Verify expected commands exist
	expected := []string{"council-add", "council-remove"}
	for _, name := range expected {
		if _, ok := templates.Commands[name]; !ok {
			t.Errorf("Templates().Commands missing %q", name)
		}
	}

	// OpenCode templates should NOT use AskUserQuestion (uses text-based choices)
	if strings.Contains(templates.Commands["council-add"], "AskUserQuestion") {
		t.Error("OpenCode council-add template should not use AskUserQuestion")
	}
	// OpenCode templates should use numbered options
	if !strings.Contains(templates.Commands["council-add"], "Which option? (1/2/3)") {
		t.Error("OpenCode council-add template should use numbered options")
	}
}

// Generic adapter tests

func TestGeneric_Detect_AlwaysTrue(t *testing.T) {
	generic, _ := Get("generic")

	if !generic.Detect() {
		t.Error("Generic Detect() should always return true")
	}
}

func TestGeneric_Paths(t *testing.T) {
	generic, _ := Get("generic")
	paths := generic.Paths()

	if paths.Agents != "." {
		t.Errorf("Agents = %q, want .", paths.Agents)
	}
	if paths.Commands != "." {
		t.Errorf("Commands = %q, want .", paths.Commands)
	}
	if len(paths.Deprecated) != 0 {
		t.Errorf("Deprecated = %v, want empty", paths.Deprecated)
	}
}

func TestGeneric_FormatAgent_SimpleMarkdown(t *testing.T) {
	generic, _ := Get("generic")

	e := &expert.Expert{
		ID:         "ada",
		Name:       "Ada",
		Focus:      "Testing and TDD",
		Philosophy: "Tests are the first customer.",
		Principles: []string{"Test first"},
	}

	result := generic.FormatAgent(e)

	// Generic format is simple markdown sections
	if !strings.Contains(result, "### Ada") {
		t.Error("FormatAgent() missing name heading")
	}
	if !strings.Contains(result, "- **ID**: ada") {
		t.Error("FormatAgent() missing ID")
	}
	if !strings.Contains(result, "- **Focus**: Testing and TDD") {
		t.Error("FormatAgent() missing Focus")
	}
	if !strings.Contains(result, "Tests are the first customer.") {
		t.Error("FormatAgent() missing philosophy")
	}
}

func TestGeneric_FormatCommand_ReturnsEmpty(t *testing.T) {
	generic, _ := Get("generic")

	result := generic.FormatCommand("test", "desc", "body")
	if result != "" {
		t.Errorf("FormatCommand() = %q, want empty string", result)
	}
}

func TestGeneric_GenerateAgentsMd(t *testing.T) {
	generic := &Generic{}

	experts := []*expert.Expert{
		{ID: "ada", Name: "Ada", Focus: "Testing"},
		{ID: "finn", Name: "Finn", Focus: "OOP"},
	}

	result := generic.GenerateAgentsMd(experts)

	if !strings.Contains(result, "# AGENTS.md - Expert Council") {
		t.Error("GenerateAgentsMd() missing title")
	}
	if !strings.Contains(result, "## Council Members") {
		t.Error("GenerateAgentsMd() missing Council Members section")
	}
	if !strings.Contains(result, "### Ada") {
		t.Error("GenerateAgentsMd() missing Ada")
	}
	if !strings.Contains(result, "### Finn") {
		t.Error("GenerateAgentsMd() missing Finn")
	}
}

func TestGeneric_Templates_NoCommands(t *testing.T) {
	generic, _ := Get("generic")
	templates := generic.Templates()

	if templates.Install == "" {
		t.Error("Templates().Install is empty")
	}
	if len(templates.Commands) != 0 {
		t.Errorf("Templates().Commands = %v, want empty map", templates.Commands)
	}
}

// AgentFilename tests

func TestAgentFilename(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		id       string
		expected string
	}{
		{"project expert", "", "ada", "ada.md"},
		{"custom expert", "custom", "my-expert", "custom-my-expert.md"},
		{"installed expert", "installed:rails-council", "ben", "installed-ben.md"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &expert.Expert{
				ID:     tt.id,
				Source: tt.source,
			}
			result := AgentFilename(e)
			if result != tt.expected {
				t.Errorf("AgentFilename() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// Test that All() returns a copy that can't mutate the registry
func TestAll_ReturnsCopy(t *testing.T) {
	all := All()
	originalLen := len(all)

	// Try to mutate the returned map
	all["fake-adapter"] = nil
	delete(all, "claude")

	// Verify the registry wasn't affected
	all2 := All()
	if len(all2) != originalLen {
		t.Errorf("All() returned mutable reference: original len %d, after mutation %d", originalLen, len(all2))
	}
	if _, ok := all2["claude"]; !ok {
		t.Error("All() returned mutable reference: claude adapter was deleted from registry")
	}
}

// Test that Names() returns sorted output
func TestNames_ReturnsSorted(t *testing.T) {
	names := Names()
	if len(names) < 2 {
		t.Skip("Need at least 2 adapters to test sorting")
	}

	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("Names() not sorted: %q comes after %q", names[i], names[i-1])
		}
	}
}

// Test that Detect() returns deterministic (sorted) output
func TestDetect_ReturnsDeterministicOrder(t *testing.T) {
	tmpDir, cleanup := setupTempDir(t)
	defer cleanup()

	// Create both .claude and .opencode directories
	if err := os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, ".opencode"), 0755); err != nil {
		t.Fatal(err)
	}

	// Run detect multiple times and verify order is consistent
	var firstOrder []string
	for i := 0; i < 5; i++ {
		detected := Detect()
		var names []string
		for _, a := range detected {
			names = append(names, a.Name())
		}
		if i == 0 {
			firstOrder = names
		} else {
			if len(names) != len(firstOrder) {
				t.Errorf("Detect() returned different number of adapters: got %d, want %d", len(names), len(firstOrder))
			}
			for j, name := range names {
				if name != firstOrder[j] {
					t.Errorf("Detect() order not deterministic: iteration %d got %v, want %v", i, names, firstOrder)
					break
				}
			}
		}
	}

	// Verify claude comes before opencode (alphabetical)
	if len(firstOrder) >= 2 && firstOrder[0] != "claude" {
		t.Errorf("Detect() should return claude before opencode, got %v", firstOrder)
	}
}

// Test Claude FormatAgent fallback when file doesn't exist
func TestClaude_FormatAgent_FallbackWhenFileNotFound(t *testing.T) {
	claude, _ := Get("claude")

	// Expert with no file on disk
	e := &expert.Expert{
		ID:    "nonexistent",
		Name:  "Nonexistent Expert",
		Focus: "Testing fallback",
		Body:  "Fallback body content",
	}

	result := claude.FormatAgent(e)

	// Should use fallback format
	if !strings.Contains(result, "id: nonexistent") {
		t.Error("FormatAgent() fallback should include id")
	}
	if !strings.Contains(result, "name: Nonexistent Expert") {
		t.Error("FormatAgent() fallback should include name")
	}
	if !strings.Contains(result, "focus: Testing fallback") {
		t.Error("FormatAgent() fallback should include focus")
	}
}

// Test ResetRegistry clears the registry
func TestResetRegistry(t *testing.T) {
	// Save the current registry state
	originalCount := len(All())
	if originalCount == 0 {
		t.Skip("No adapters registered to test reset")
	}

	// Reset the registry
	ResetRegistry()

	// Verify registry is empty
	if len(All()) != 0 {
		t.Errorf("ResetRegistry() did not clear registry, got %d adapters", len(All()))
	}

	// Re-register adapters for other tests (they're registered via init())
	// We need to manually re-register since init() only runs once
	Register(&Claude{})
	Register(&OpenCode{})
	Register(&Generic{})

	// Verify adapters were re-registered
	if len(All()) != 3 {
		t.Errorf("Failed to re-register adapters after reset, got %d", len(All()))
	}
}
