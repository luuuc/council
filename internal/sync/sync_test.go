package sync

import (
	"os"
	"strings"
	"testing"

	"github.com/luuuc/council-cli/internal/adapter"
	"github.com/luuuc/council-cli/internal/config"
	"github.com/luuuc/council-cli/internal/expert"
	"github.com/luuuc/council-cli/internal/fs"
	"github.com/luuuc/council-cli/internal/pack"
)

func TestGenerateCouncilCommand(t *testing.T) {
	experts := []*expert.Expert{
		{
			ID:    "kent-beck",
			Name:  "Kent Beck",
			Focus: "Test-driven development",
		},
		{
			ID:    "dhh",
			Name:  "DHH",
			Focus: "Rails and productivity",
		},
	}

	claude, _ := adapter.Get("claude")
	result := generateCouncilCommand(claude, experts, nil)

	// Check for key elements
	if !strings.Contains(result, "Code Review Council") {
		t.Error("generateCouncilCommand() missing title")
	}
	if !strings.Contains(result, "$ARGUMENTS") {
		t.Error("generateCouncilCommand() missing $ARGUMENTS placeholder")
	}
	if !strings.Contains(result, "Kent Beck") {
		t.Error("generateCouncilCommand() missing first expert name")
	}
	if !strings.Contains(result, "DHH") {
		t.Error("generateCouncilCommand() missing second expert name")
	}
	if !strings.Contains(result, "Test-driven development") {
		t.Error("generateCouncilCommand() missing first expert focus")
	}
}

func TestGenerateCouncilCommand_EmptyExperts(t *testing.T) {
	// Test with empty expert list - should not panic
	experts := []*expert.Expert{}

	claude, _ := adapter.Get("claude")
	result := generateCouncilCommand(claude, experts, nil)

	// Should still have the header and instructions
	if !strings.Contains(result, "Code Review Council") {
		t.Error("generateCouncilCommand() should have title even with empty experts")
	}
	if !strings.Contains(result, "Instructions") {
		t.Error("generateCouncilCommand() should have instructions even with empty experts")
	}
}

func TestGenerateCouncilCommand_SpecialCharacters(t *testing.T) {
	// Test with special characters that might cause template issues
	experts := []*expert.Expert{
		{
			ID:    "special",
			Name:  "Expert with <html> & \"quotes\"",
			Focus: "Testing {{templates}} and $variables",
		},
	}

	claude, _ := adapter.Get("claude")
	result := generateCouncilCommand(claude, experts, nil)

	// Should not panic and should contain the special characters
	if !strings.Contains(result, "<html>") {
		t.Error("generateCouncilCommand() should preserve special characters")
	}
}

func TestGenericGenerateAgentsMd(t *testing.T) {
	experts := []*expert.Expert{
		{
			ID:         "expert-1",
			Name:       "Expert One",
			Focus:      "Focus one",
			Philosophy: "Philosophy here.",
			Principles: []string{"Principle 1"},
		},
	}

	generic := &adapter.Generic{}
	result := generic.GenerateAgentsMd(experts)

	if !strings.Contains(result, "AGENTS.md") {
		t.Error("GenerateAgentsMd() missing AGENTS.md header")
	}
	if !strings.Contains(result, "Expert One") {
		t.Error("GenerateAgentsMd() missing expert name")
	}
	if !strings.Contains(result, "expert-1") {
		t.Error("GenerateAgentsMd() missing expert ID")
	}
	if !strings.Contains(result, "Philosophy here.") {
		t.Error("GenerateAgentsMd() missing philosophy")
	}
}

func TestSyncToAdapterClaude(t *testing.T) {
	// Create a temp directory for testing
	tmpDir, err := os.MkdirTemp("", "council-sync-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Change to temp directory
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	// Create council structure with an expert
	_ = os.MkdirAll(config.Path(config.ExpertsDir), 0755)
	testExpert := &expert.Expert{
		ID:    "test",
		Name:  "Test Expert",
		Focus: "Testing",
	}
	_ = testExpert.Save()

	claude, _ := adapter.Get("claude")
	experts := []*expert.Expert{testExpert}

	// Test dry run
	err = syncToAdapter(claude, experts, nil, Options{DryRun: true})
	if err != nil {
		t.Errorf("syncToAdapter() dry run error = %v", err)
	}

	// Verify nothing was created in dry run
	if _, err := os.Stat(".claude/agents"); !os.IsNotExist(err) {
		t.Error("syncToAdapter() dry run should not create directories")
	}

	// Test actual sync
	err = syncToAdapter(claude, experts, nil, Options{DryRun: false})
	if err != nil {
		t.Errorf("syncToAdapter() error = %v", err)
	}

	// Verify agent file was created
	agentPath := ".claude/agents/test.md"
	if _, err := os.Stat(agentPath); os.IsNotExist(err) {
		t.Errorf("syncToAdapter() did not create agent file at %s", agentPath)
	}

	// Verify council command was created
	commandPath := ".claude/commands/council.md"
	if _, err := os.Stat(commandPath); os.IsNotExist(err) {
		t.Errorf("syncToAdapter() did not create council command at %s", commandPath)
	}

	// Read and verify council command content
	content, _ := os.ReadFile(commandPath)
	if !strings.Contains(string(content), "Test Expert") {
		t.Error("Council command should contain expert name")
	}
}

func TestSyncToAdapterGeneric(t *testing.T) {
	// Create a temp directory for testing
	tmpDir, err := os.MkdirTemp("", "council-sync-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Change to temp directory
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	generic, _ := adapter.Get("generic")
	experts := []*expert.Expert{
		{
			ID:    "test",
			Name:  "Test Expert",
			Focus: "Testing",
		},
	}

	err = syncToAdapter(generic, experts, nil, Options{DryRun: false})
	if err != nil {
		t.Errorf("syncToAdapter() error = %v", err)
	}

	if _, err := os.Stat("AGENTS.md"); os.IsNotExist(err) {
		t.Error("syncToAdapter() should create AGENTS.md for generic")
	}

	content, _ := os.ReadFile("AGENTS.md")
	if !strings.Contains(string(content), "Test Expert") {
		t.Error("AGENTS.md should contain expert name")
	}
}

func TestSyncAllNoExperts(t *testing.T) {
	// Create a temp directory for testing
	tmpDir, err := os.MkdirTemp("", "council-sync-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Change to temp directory
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	// Create empty council structure
	_ = os.MkdirAll(config.Path(config.ExpertsDir), 0755)

	cfg := config.Default()

	err = SyncAll(cfg, Options{DryRun: false})
	if err == nil {
		t.Error("SyncAll() should error when no experts exist")
	}
}

func TestAdaptersRegistry(t *testing.T) {
	// Verify all expected adapters are registered
	expectedAdapters := []string{"claude", "generic", "opencode"}

	for _, name := range expectedAdapters {
		a, ok := adapter.Get(name)
		if !ok {
			t.Errorf("Adapter %s not found in registry", name)
			continue
		}
		if a.Name() == "" {
			t.Errorf("Adapter %s has empty Name()", name)
		}
		if a.DisplayName() == "" {
			t.Errorf("Adapter %s has empty DisplayName()", name)
		}
	}
}

func TestFileExistsAndDirExists(t *testing.T) {
	// Create a temp directory for testing
	tmpDir, err := os.MkdirTemp("", "council-sync-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Change to temp directory
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	// Test non-existent
	if fs.FileExists("nonexistent.txt") {
		t.Error("fs.FileExists() should return false for non-existent file")
	}
	if fs.DirExists("nonexistent") {
		t.Error("fs.DirExists() should return false for non-existent directory")
	}

	// Create a file and directory
	_ = os.WriteFile("test.txt", []byte("content"), 0644)
	_ = os.MkdirAll("testdir", 0755)

	if !fs.FileExists("test.txt") {
		t.Error("fs.FileExists() should return true for existing file")
	}
	if !fs.DirExists("testdir") {
		t.Error("fs.DirExists() should return true for existing directory")
	}

	// File should not be detected as directory
	if fs.DirExists("test.txt") {
		t.Error("fs.DirExists() should return false for file")
	}
}

func TestClaudeFormatAgent(t *testing.T) {
	// Create a temp directory for testing
	tmpDir, err := os.MkdirTemp("", "council-sync-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Change to temp directory
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	// Create council structure with an expert
	_ = os.MkdirAll(config.Path(config.ExpertsDir), 0755)
	testExpert := &expert.Expert{
		ID:    "test",
		Name:  "Test Expert",
		Focus: "Testing",
		Body:  "# Test Expert\n\nCustom body content.",
	}
	_ = testExpert.Save()

	// Test Claude adapter FormatAgent reads from disk
	claude, _ := adapter.Get("claude")
	result := claude.FormatAgent(testExpert)

	if !strings.Contains(result, "Test Expert") {
		t.Error("FormatAgent() should contain expert name")
	}
	if !strings.Contains(result, "Custom body content") {
		t.Error("FormatAgent() should contain custom body")
	}
}

func TestSyncToAdapterOpenCode(t *testing.T) {
	// Create a temp directory for testing
	tmpDir, err := os.MkdirTemp("", "council-sync-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Change to temp directory
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	opencode, _ := adapter.Get("opencode")
	experts := []*expert.Expert{
		{
			ID:         "test",
			Name:       "Test Expert",
			Focus:      "Testing",
			Philosophy: "Test philosophy.",
			Principles: []string{"Principle 1"},
			RedFlags:   []string{"Red flag 1"},
		},
	}

	// Test dry run
	err = syncToAdapter(opencode, experts, nil, Options{DryRun: true})
	if err != nil {
		t.Errorf("syncToAdapter() dry run error = %v", err)
	}

	// Verify nothing was created in dry run
	if _, err := os.Stat(".opencode/agents"); !os.IsNotExist(err) {
		t.Error("syncToAdapter() dry run should not create directories")
	}

	// Test actual sync
	err = syncToAdapter(opencode, experts, nil, Options{DryRun: false})
	if err != nil {
		t.Errorf("syncToAdapter() error = %v", err)
	}

	// Verify agent file was created (new path: .opencode/agents/)
	agentPath := ".opencode/agents/test.md"
	if _, err := os.Stat(agentPath); os.IsNotExist(err) {
		t.Errorf("syncToAdapter() did not create agent file at %s", agentPath)
	}

	// Read and verify content
	content, _ := os.ReadFile(agentPath)
	contentStr := string(content)

	// Verify OpenCode-specific frontmatter format
	if !strings.Contains(contentStr, "description: Testing") {
		t.Error("OpenCode agent should have description in frontmatter")
	}
	if !strings.Contains(contentStr, "mode: subagent") {
		t.Error("OpenCode agent should have mode: subagent in frontmatter")
	}
	if !strings.Contains(contentStr, "Test Expert") {
		t.Error("OpenCode agent should contain expert name")
	}
	if !strings.Contains(contentStr, "Test philosophy.") {
		t.Error("OpenCode agent should contain philosophy")
	}
	if !strings.Contains(contentStr, "Principle 1") {
		t.Error("OpenCode agent should contain principles")
	}
	if !strings.Contains(contentStr, "Red flag 1") {
		t.Error("OpenCode agent should contain red flags")
	}
}

func TestOpenCodeFormatAgent(t *testing.T) {
	e := &expert.Expert{
		ID:         "kent-beck",
		Name:       "Kent Beck",
		Focus:      "TDD and clean code",
		Philosophy: "Test-driven development leads to better design.",
		Principles: []string{"Red-green-refactor", "Simple design"},
		RedFlags:   []string{"No tests", "Complex mocking"},
	}

	opencode, _ := adapter.Get("opencode")
	result := opencode.FormatAgent(e)

	// Verify frontmatter structure
	if !strings.HasPrefix(result, "---\n") {
		t.Error("FormatAgent() should start with YAML frontmatter delimiter")
	}
	if !strings.Contains(result, "description: TDD and clean code") {
		t.Error("FormatAgent() should have description matching focus")
	}
	if !strings.Contains(result, "mode: subagent") {
		t.Error("FormatAgent() should have mode: subagent")
	}

	// Verify body content
	if !strings.Contains(result, "# Kent Beck") {
		t.Error("FormatAgent() should have expert name as heading")
	}
	if !strings.Contains(result, "channeling Kent Beck") {
		t.Error("FormatAgent() should have channeling intro")
	}
	if !strings.Contains(result, "## Philosophy") {
		t.Error("FormatAgent() should have Philosophy section")
	}
	if !strings.Contains(result, "## Principles") {
		t.Error("FormatAgent() should have Principles section")
	}
	if !strings.Contains(result, "## Red Flags") {
		t.Error("FormatAgent() should have Red Flags section")
	}
	if !strings.Contains(result, "## Review Style") {
		t.Error("FormatAgent() should have Review Style section")
	}
}

func TestSyncTargetUnknown(t *testing.T) {
	cfg := config.Default()
	err := SyncTarget("unknown-target", cfg, Options{DryRun: false})
	if err == nil {
		t.Error("SyncTarget() should error for unknown target")
	}
	if !strings.Contains(err.Error(), "unknown target") {
		t.Errorf("Error should mention 'unknown target', got: %v", err)
	}
}

func TestAllCleanPaths(t *testing.T) {
	paths := AllCleanPaths()
	if len(paths) == 0 {
		t.Error("AllCleanPaths() should return paths")
	}

	// Should include AGENTS.md
	found := false
	for _, p := range paths {
		if p == "AGENTS.md" {
			found = true
			break
		}
	}
	if !found {
		t.Error("AllCleanPaths() should include AGENTS.md")
	}
}

func TestDetectTargets(t *testing.T) {
	// Create a temp directory for testing
	tmpDir, err := os.MkdirTemp("", "council-sync-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Change to temp directory
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	// With no directories, should return generic
	targets := DetectTargets()
	if len(targets) != 1 || targets[0] != "generic" {
		t.Errorf("DetectTargets() with no dirs should return [generic], got %v", targets)
	}

	// Create .claude directory
	_ = os.MkdirAll(".claude", 0755)
	targets = DetectTargets()
	found := false
	for _, t := range targets {
		if t == "claude" {
			found = true
		}
	}
	if !found {
		t.Errorf("DetectTargets() should detect claude, got %v", targets)
	}
}

func TestDetectTargets_ReturnsDeterministicOrder(t *testing.T) {
	// Create a temp directory for testing
	tmpDir, err := os.MkdirTemp("", "council-sync-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Change to temp directory
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	// Create both directories
	_ = os.MkdirAll(".claude", 0755)
	_ = os.MkdirAll(".opencode", 0755)

	// Run multiple times to verify deterministic order
	var firstResult []string
	for i := 0; i < 5; i++ {
		targets := DetectTargets()
		if i == 0 {
			firstResult = targets
		} else {
			for j, target := range targets {
				if j < len(firstResult) && target != firstResult[j] {
					t.Errorf("DetectTargets() not deterministic: iteration %d got %v, want %v", i, targets, firstResult)
					break
				}
			}
		}
	}
}

func TestGenerateCouncilCommand_TemplateContent(t *testing.T) {
	experts := []*expert.Expert{
		{ID: "test", Name: "Test Expert", Focus: "Testing"},
	}

	claude, _ := adapter.Get("claude")
	result := generateCouncilCommand(claude, experts, nil)

	// Verify template was executed (contains expert data)
	if !strings.Contains(result, "Test Expert") {
		t.Error("generateCouncilCommand() should contain expert name from template")
	}
	if !strings.Contains(result, "Testing") {
		t.Error("generateCouncilCommand() should contain expert focus from template")
	}
	if !strings.Contains(result, "Council Members") {
		t.Error("generateCouncilCommand() should contain Council Members section")
	}
}

func TestGenerateCouncilCommand_WithPacks(t *testing.T) {
	experts := []*expert.Expert{
		{ID: "test", Name: "Test Expert", Focus: "Testing"},
	}
	packs := []*pack.Pack{
		{Name: "go", Description: "Go review council", Members: []pack.Member{
			{ID: "rob-pike"},
			{ID: "dave-cheney"},
		}},
		{Name: "rails", Description: "Rails review council", Members: []pack.Member{
			{ID: "dhh", Blocking: true},
		}},
	}

	claude, _ := adapter.Get("claude")
	result := generateCouncilCommand(claude, experts, packs)

	if !strings.Contains(result, "Available Packs") {
		t.Error("generateCouncilCommand() with packs should contain Available Packs section")
	}
	if !strings.Contains(result, "--pack") {
		t.Error("generateCouncilCommand() with packs should mention --pack flag")
	}
	if !strings.Contains(result, "**go**") {
		t.Error("generateCouncilCommand() should list go pack")
	}
	if !strings.Contains(result, "**rails**") {
		t.Error("generateCouncilCommand() should list rails pack")
	}
	if !strings.Contains(result, "2 members") {
		t.Error("generateCouncilCommand() should show member count for go pack")
	}
}

func TestGenerateCouncilCommand_NoPacks(t *testing.T) {
	experts := []*expert.Expert{
		{ID: "test", Name: "Test Expert", Focus: "Testing"},
	}

	claude, _ := adapter.Get("claude")
	result := generateCouncilCommand(claude, experts, nil)

	// With nil packs, should not contain packs section
	if strings.Contains(result, "Available Packs") {
		t.Error("generateCouncilCommand() with nil packs should not contain Available Packs section")
	}
}
