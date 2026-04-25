package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/luuuc/council/internal/adapter"
	"github.com/luuuc/council/internal/config"
	"github.com/luuuc/council/internal/detect"
	"github.com/luuuc/council/internal/expert"
	"github.com/luuuc/council/internal/pack"
	"github.com/luuuc/council/internal/sync"
	"github.com/spf13/cobra"
)

const (
	maxStackExperts = 4 // Maximum stack-specific experts to add
	maxTotalExperts = 7 // Maximum total experts in auto-selection
)

func init() {
	rootCmd.AddCommand(startCmd)
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "First-time setup (zero-config)",
	Long: `Sets up your council with sensible defaults. One command, zero decisions.

What it does:
  1. Creates .council/ directory
  2. Detects your project's stack
  3. Adds 5 experts based on your stack
  4. Syncs to your AI tool

If you already have a council, use 'council add' to add more experts.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runStart()
	},
}

func runStart() error {
	// Check if .council/ already exists
	if config.Exists() {
		return fmt.Errorf(".council/ already exists\n\nTo modify your council:\n  council add <name>    Add an expert\n  council remove <id>   Remove an expert\n  council list          View your council")
	}

	// Step 1: Detect AI tool
	tool, err := detectTool()
	if err != nil {
		return err
	}

	// Step 2: Create directory structure (same as init)
	dirs := []string{
		config.CouncilDir,
		config.Path(config.ExpertsDir),
		config.Path(config.CommandsDir),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}

	// Create config with detected tool
	cfg := config.Default()
	cfg.Tool = tool

	// Detect review backend using config's detection logic
	backend, provider, model := cfg.DetectBackend()
	cfg.AI.Backend = backend
	cfg.AI.Provider = provider
	cfg.AI.Model = model
	printBackendDetection(backend, provider)
	if err := cfg.Save(); err != nil {
		return err
	}

	// Create .gitkeep files
	for _, subdir := range []string{config.ExpertsDir, config.CommandsDir} {
		path := config.Path(subdir, ".gitkeep")
		if err := os.WriteFile(path, []byte(""), 0644); err != nil {
			return fmt.Errorf("failed to create .gitkeep: %w", err)
		}
	}

	// Get adapter for display name
	a, _ := adapter.Get(tool)
	displayName := tool
	if a != nil {
		displayName = a.DisplayName()
	}
	fmt.Printf("✓ Detected: %s\n", displayName)

	// Step 3: Detect project stack
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	d, err := detect.Scan(dir)
	if err != nil {
		return fmt.Errorf("detection failed: %w", err)
	}

	fmt.Printf("✓ Detected: %s\n", d.Summary())

	// Step 4: Select experts based on detected stack
	experts := selectExperts(d)
	if len(experts) == 0 {
		// Fallback to generalists if detection returned nothing useful
		experts = selectGeneralists()
	}

	// Step 5: Add selected experts
	var added []*expert.Expert
	for _, e := range experts {
		if err := e.Save(); err != nil {
			fmt.Printf("  Warning: failed to add %s: %v\n", e.Name, err)
			continue
		}
		added = append(added, e)
	}

	if len(added) == 0 {
		return fmt.Errorf("failed to add any experts")
	}

	// Print added experts
	var names []string
	for _, e := range added {
		names = append(names, e.Name)
	}
	fmt.Printf("✓ Added %d experts: %s\n", len(added), joinNames(names))

	// Step 6: Sync to AI tool
	if err := sync.SyncAll(cfg, sync.Options{}); err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	fmt.Println()
	fmt.Println("Your council is ready. Try: /council <topic>")

	return nil
}

// detectTool determines which AI tool to use (auto-detect, single tool, or first of multiple)
func detectTool() (string, error) {
	detected := adapter.Detect()

	switch len(detected) {
	case 0:
		return "generic", nil
	case 1:
		return detected[0].Name(), nil
	default:
		// Multiple tools - use first one (deterministic order)
		return detected[0].Name(), nil
	}
}

// printBackendDetection prints transparent output about the detected review backend.
func printBackendDetection(backend, provider string) {
	// Build detection status
	var parts []string

	// Report what was found in the environment
	for _, cmd := range config.KnownAICLIs {
		if _, err := exec.LookPath(cmd); err == nil {
			parts = append(parts, fmt.Sprintf("%s CLI ✓", cmd))
			break
		}
	}
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		parts = append(parts, "ANTHROPIC_API_KEY ✓")
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		parts = append(parts, "OPENAI_API_KEY ✓")
	}

	if len(parts) > 0 {
		fmt.Printf("✓ Detected: %s\n", strings.Join(parts, ", "))
	}

	// Report the decision
	switch backend {
	case "cli":
		fmt.Println("  Using: cli backend (preferred — zero-config)")
	case "api":
		fmt.Printf("  Using: api backend (provider: %s)\n", provider)
	default:
		fmt.Println("  No AI CLI or API key detected — configure ai.backend in .council/config.yaml (options: anthropic, openai, ollama)")
	}
}

// selectExperts picks experts based on detected stack.
// It first checks for a matching built-in pack; if none matches, falls back to the suggestion bank.
func selectExperts(d *detect.Detection) []*expert.Expert {
	// Try built-in packs first — curated rosters for known stacks
	if experts := selectFromPack(d); len(experts) > 0 {
		return experts
	}

	// Fallback: build a council from the suggestion bank
	return selectFromSuggestionBank(d)
}

// selectFromPack checks if a built-in pack matches the detected stack.
func selectFromPack(d *detect.Detection) []*expert.Expert {
	categories := mapDetectionToCategories(d)
	builtins := pack.Builtins()

	// Use the first matching pack (framework packs take priority — they appear later in categories)
	var matchedPack *pack.Pack
	for i := len(categories) - 1; i >= 0; i-- {
		if p, ok := builtins[categories[i]]; ok {
			matchedPack = p
			break
		}
	}

	if matchedPack == nil {
		return nil
	}

	var selected []*expert.Expert
	for _, m := range matchedPack.Members {
		if e := findExpertByID(m.ID); e != nil {
			selected = append(selected, e)
		}
	}

	return selected
}

// selectFromSuggestionBank builds a council from the suggestion bank categories + core generals.
func selectFromSuggestionBank(d *detect.Detection) []*expert.Expert {
	var selected []*expert.Expert
	seen := make(map[string]bool)

	categories := mapDetectionToCategories(d)

	// Add stack-specific experts: first pass picks the primary (first) from each category
	for _, cat := range categories {
		if len(selected) >= maxStackExperts {
			break
		}
		if experts, ok := loadSuggestionBank()[cat]; ok && len(experts) > 0 {
			e := &experts[0]
			if !seen[e.ID] {
				selected = append(selected, expertFromSuggestion(e))
				seen[e.ID] = true
			}
		}
	}

	// Second pass: fill remaining stack slots from categories (deeper bench)
	for _, cat := range categories {
		if len(selected) >= maxStackExperts {
			break
		}
		if experts, ok := loadSuggestionBank()[cat]; ok {
			for i := 1; i < len(experts) && len(selected) < maxStackExperts; i++ {
				e := &experts[i]
				if !seen[e.ID] {
					selected = append(selected, expertFromSuggestion(e))
					seen[e.ID] = true
				}
			}
		}
	}

	// Always add core experts from the general category
	if generals, ok := loadSuggestionBank()["general"]; ok {
		for i := range generals {
			if len(selected) >= maxTotalExperts {
				break
			}
			if generals[i].Core && !seen[generals[i].ID] {
				selected = append(selected, expertFromSuggestion(&generals[i]))
				seen[generals[i].ID] = true
			}
		}
	}

	return selected
}

// selectGeneralists returns default generalists when detection finds nothing
func selectGeneralists() []*expert.Expert {
	var selected []*expert.Expert
	ids := []string{"ada-redgrave", "elara-nygaard", "marcus-torrent", "nadia-kowalski", "iris-vance"}

	for _, id := range ids {
		if len(selected) >= maxTotalExperts {
			break
		}
		if e := findExpertByID(id); e != nil {
			selected = append(selected, e)
		}
	}

	return selected
}

// mapDetectionToCategories maps detected stack to suggestion bank categories
func mapDetectionToCategories(d *detect.Detection) []string {
	var categories []string

	// Collect detected framework names for context-aware suppression
	frameworkNames := make(map[string]bool)
	for _, fw := range d.Frameworks {
		frameworkNames[fw.Name] = true
	}

	// Map languages
	for _, lang := range d.Languages {
		switch lang.Name {
		case "Go":
			categories = append(categories, "go")
		case "Ruby":
			categories = append(categories, "ruby")
		case "Python":
			categories = append(categories, "python")
		case "JavaScript", "TypeScript":
			// In Rails projects, JS/TS is Stimulus/Turbo — covered by Rails experts
			if !frameworkNames["Rails"] {
				categories = append(categories, "javascript")
			}
		case "Rust":
			categories = append(categories, "rust")
		case "Elixir":
			categories = append(categories, "elixir")
		case "Java", "Kotlin":
			categories = append(categories, "java")
		case "C#":
			categories = append(categories, "dotnet")
		case "Swift":
			categories = append(categories, "swift")
		}
	}

	// Map frameworks
	for _, fw := range d.Frameworks {
		switch fw.Name {
		case "Rails":
			categories = append(categories, "rails")
		case "Phoenix":
			categories = append(categories, "elixir")
		case "Django", "Flask", "FastAPI":
			categories = append(categories, "python")
		case "React", "Vue", "Next.js":
			categories = append(categories, "frontend")
		case "Express":
			categories = append(categories, "javascript")
		}
	}

	// Map testing to add testing expert
	if len(d.Testing) > 0 {
		categories = append(categories, "testing")
	}

	return categories
}

// findExpertByID searches the suggestion bank for an expert by ID.
// Callers pass composite IDs directly — legacy alias resolution
// happens at the public API boundary (expert.Load, expert.LookupPersona).
func findExpertByID(id string) *expert.Expert {
	for _, experts := range loadSuggestionBank() {
		for i := range experts {
			if experts[i].ID == id {
				return expertFromSuggestion(&experts[i])
			}
		}
	}
	return nil
}

func expertFromSuggestion(e *expert.Expert) *expert.Expert {
	copy := *e
	return &copy
}

// joinNames joins names with commas
func joinNames(names []string) string {
	return strings.Join(names, ", ")
}
