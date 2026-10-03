// Package sync writes expert configurations to AI tool directories.
package sync

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/luuuc/council/internal/adapter"
	"github.com/luuuc/council/internal/config"
	"github.com/luuuc/council/internal/expert"
	"github.com/luuuc/council/internal/install"
	"github.com/luuuc/council/internal/pack"
)

// Pre-compiled template for council command generation
var councilCommandTemplate = template.Must(template.New("council").Parse(adapter.CouncilCommandTemplate()))

// councilTemplateData is the data passed to the council command template.
type councilTemplateData struct {
	Experts []*expert.Expert
	Packs   []*pack.Pack
}

// Options configures sync behavior
type Options struct {
	DryRun bool // Show what would be done without making changes
	Clean  bool // Remove stale files not in current config
}

// AllCleanPaths returns all paths that should be cleaned across all adapters
func AllCleanPaths() []string {
	var paths []string
	for _, a := range adapter.All() {
		p := a.Paths()
		if p.Agents != "." {
			paths = append(paths, p.Agents)
		}
		if p.Commands != "." && p.Commands != p.Agents {
			paths = append(paths, p.Commands)
		}
		paths = append(paths, p.Deprecated...)
	}
	// Add AGENTS.md for generic
	paths = append(paths, "AGENTS.md")
	return paths
}

// SyncAll syncs to the configured tool (or detects and saves if missing)
func SyncAll(cfg *config.Config, opts Options) error {
	// Load all experts
	allExperts, err := loadAllExperts()
	if err != nil {
		return err
	}

	// An empty council still gets its commands, so the AI can assemble it.
	// Load all packs
	allPacks, err := pack.ListAll()
	if err != nil {
		fmt.Printf("Warning: could not load packs: %v\n", err)
	}

	// Determine which adapter(s) to sync to
	adapters, err := resolveAdapters(cfg)
	if err != nil {
		return err
	}

	// Sync to each adapter
	for _, a := range adapters {
		fmt.Printf("Syncing to %s...\n", a.DisplayName())
		if err := syncToAdapter(a, allExperts, allPacks, opts); err != nil {
			return fmt.Errorf("failed to sync to %s: %w", a.Name(), err)
		}

		// Check for deprecated paths and warn
		checkDeprecatedPaths(a, opts)
	}

	return nil
}

// resolveAdapters determines which adapters to sync to based on config
func resolveAdapters(cfg *config.Config) ([]adapter.Adapter, error) {
	var adapters []adapter.Adapter

	// If targets explicitly set, use those
	if len(cfg.Targets) > 0 {
		for _, name := range cfg.Targets {
			a, ok := adapter.Get(name)
			if !ok {
				fmt.Printf("Warning: unknown target '%s', skipping\n", name)
				continue
			}
			adapters = append(adapters, a)
		}
		return adapters, nil
	}

	// Use configured tool
	if cfg.Tool != "" {
		a, ok := adapter.Get(cfg.Tool)
		if !ok {
			return nil, fmt.Errorf("unknown tool '%s' in config - valid tools: claude, opencode, generic", cfg.Tool)
		}
		return []adapter.Adapter{a}, nil
	}

	// Tool not configured - auto-detect and save
	detected := adapter.Detect()
	switch len(detected) {
	case 0:
		// Fall back to generic
		a, _ := adapter.Get("generic")
		fmt.Println("No AI tool detected, using generic (AGENTS.md)")
		cfg.Tool = "generic"
		if err := cfg.Save(); err != nil {
			fmt.Printf("Warning: could not save config: %v\n", err)
		}
		return []adapter.Adapter{a}, nil

	case 1:
		// Single tool detected
		a := detected[0]
		fmt.Printf("Detected: %s\n", a.DisplayName())
		cfg.Tool = a.Name()
		if err := cfg.Save(); err != nil {
			fmt.Printf("Warning: could not save config: %v\n", err)
		}
		return []adapter.Adapter{a}, nil

	default:
		// Multiple tools - use first one and warn
		a := detected[0]
		var names []string
		for _, d := range detected {
			names = append(names, d.Name())
		}
		fmt.Printf("Multiple tools detected (%s), using %s\n", strings.Join(names, ", "), a.DisplayName())
		fmt.Println("Set 'tool:' in .council/config.yaml to choose a different default")
		cfg.Tool = a.Name()
		if err := cfg.Save(); err != nil {
			fmt.Printf("Warning: could not save config: %v\n", err)
		}
		return []adapter.Adapter{a}, nil
	}
}

func syncToAdapter(a adapter.Adapter, experts []*expert.Expert, packs []*pack.Pack, opts Options) error {
	paths := a.Paths()
	templates := a.Templates()

	// Special case for generic - writes single AGENTS.md file
	if a.Name() == "generic" {
		generic := a.(*adapter.Generic)
		return writeFile("AGENTS.md", generic.GenerateAgentsMd(experts), opts.DryRun)
	}

	// Create agents directory
	if paths.Agents != "." && !opts.DryRun {
		if err := os.MkdirAll(paths.Agents, 0755); err != nil {
			return err
		}
	}

	// Sync each expert as an agent file
	for _, e := range experts {
		filename := adapter.AgentFilename(e)
		path := filepath.Join(paths.Agents, filename)
		if err := writeFile(path, a.FormatAgent(e), opts.DryRun); err != nil {
			return err
		}
	}

	// Create commands directory (if different from agents)
	if paths.Commands != "." && paths.Commands != paths.Agents && !opts.DryRun {
		if err := os.MkdirAll(paths.Commands, 0755); err != nil {
			return err
		}
	}

	// Create /council command (dynamic content based on experts and packs)
	councilContent := generateCouncilCommand(a, experts, packs)
	if councilContent != "" {
		path := filepath.Join(paths.Commands, "council.md")
		if err := writeFile(path, councilContent, opts.DryRun); err != nil {
			return err
		}
	}

	// Create other commands from adapter templates
	for name, tmpl := range templates.Commands {
		content := a.FormatCommand(name, commandDescription(name), tmpl)
		if content == "" {
			continue
		}
		path := filepath.Join(paths.Commands, name+".md")
		if err := writeFile(path, content, opts.DryRun); err != nil {
			return err
		}
	}

	// Clean up stale files if requested
	if opts.Clean {
		if err := cleanStaleAgents(paths.Agents, experts, templates.Commands, opts.DryRun); err != nil {
			return err
		}
	}

	return nil
}

func generateCouncilCommand(a adapter.Adapter, experts []*expert.Expert, packs []*pack.Pack) string {
	data := councilTemplateData{Experts: experts, Packs: packs}
	var buf bytes.Buffer
	if err := councilCommandTemplate.Execute(&buf, data); err != nil {
		// Fallback to simple format if template fails
		return "# Council\n\nConvene the council on: $ARGUMENTS\n"
	}
	body := buf.String()

	// Format according to adapter's command format
	return a.FormatCommand("council", "Convene the council to review code, a plan, or a decision", body)
}

func commandDescription(name string) string {
	descriptions := map[string]string{
		"council-assemble": "Assemble the council: your AI proposes members, you choose",
		"council-add":      "Add a member: a person, a role, or a customer",
		"council-remove":   "Remove expert from council",
	}
	if desc, ok := descriptions[name]; ok {
		return desc
	}
	return name
}

func checkDeprecatedPaths(a adapter.Adapter, opts Options) {
	paths := a.Paths()
	for _, deprecated := range paths.Deprecated {
		if _, err := os.Stat(deprecated); err == nil {
			if opts.Clean {
				// Remove deprecated path
				if !opts.DryRun {
					if err := os.RemoveAll(deprecated); err != nil {
						fmt.Printf("  Warning: could not remove deprecated %s: %v\n", deprecated, err)
					} else {
						fmt.Printf("  Removed deprecated: %s\n", deprecated)
					}
				} else {
					fmt.Printf("  Would remove deprecated: %s\n", deprecated)
				}
			} else {
				fmt.Printf("  Warning: deprecated path exists: %s\n", deprecated)
				fmt.Printf("    Run 'council sync --clean' to remove\n")
			}
		}
	}
}

// loadAllExperts loads experts from all sources: installed and project
func loadAllExperts() ([]*expert.Expert, error) {
	var allExperts []*expert.Expert

	// Load installed experts (from cloned repositories)
	// Errors here are non-fatal (user may not have installed councils)
	installedExperts, err := install.ListInstalledExperts()
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("Warning: could not load installed experts: %v\n", err)
		}
	} else {
		allExperts = append(allExperts, installedExperts...)
	}

	// Load project council experts - this is required
	projectExperts, err := expert.List()
	if err != nil {
		return nil, err
	}
	allExperts = append(allExperts, projectExperts...)

	return allExperts, nil
}

// writeFile writes content to path, or prints what would be written in dry-run mode
func writeFile(path, content string, dryRun bool) error {
	if dryRun {
		fmt.Printf("  Would create: %s\n", path)
		return nil
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return err
	}
	fmt.Printf("  Created: %s\n", path)
	return nil
}

// removeFile removes a file if it exists, or prints what would be removed in dry-run mode
func removeFile(path string, dryRun bool) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // File doesn't exist, nothing to do
	}
	if dryRun {
		fmt.Printf("  Would remove: %s\n", path)
		return nil
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	fmt.Printf("  Removed: %s\n", path)
	return nil
}

func cleanStaleAgents(agentsDir string, experts []*expert.Expert, commandFiles map[string]string, dryRun bool) error {
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	// Build set of current expert filenames
	currentFiles := make(map[string]bool)
	for _, e := range experts {
		currentFiles[adapter.AgentFilename(e)] = true
	}

	// Build set of command file names to exclude
	commandSet := make(map[string]bool)
	for name := range commandFiles {
		commandSet[name+".md"] = true
	}
	commandSet["council.md"] = true // Always exclude council command

	// Remove files for experts that no longer exist
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		// Skip command files
		if commandSet[entry.Name()] {
			continue
		}
		// Skip current expert files
		if currentFiles[entry.Name()] {
			continue
		}
		path := filepath.Join(agentsDir, entry.Name())
		if err := removeFile(path, dryRun); err != nil {
			return err
		}
	}

	return nil
}

// SyncTarget syncs to a specific target by name
func SyncTarget(targetName string, cfg *config.Config, opts Options) error {
	a, ok := adapter.Get(targetName)
	if !ok {
		return fmt.Errorf("unknown target '%s' - valid targets: claude, opencode, generic", targetName)
	}

	allExperts, err := loadAllExperts()
	if err != nil {
		return err
	}

	// An empty council still gets its commands, so the AI can assemble it.
	allPacks, err := pack.ListAll()
	if err != nil {
		fmt.Printf("Warning: could not load packs: %v\n", err)
	}

	fmt.Printf("Syncing to %s...\n", a.DisplayName())
	if err := syncToAdapter(a, allExperts, allPacks, opts); err != nil {
		return fmt.Errorf("failed to sync to %s: %w", targetName, err)
	}

	checkDeprecatedPaths(a, opts)
	return nil
}

// DetectTargets returns target names that have existing config directories
// This is for backward compatibility with existing code
func DetectTargets() []string {
	detected := adapter.Detect()
	if len(detected) == 0 {
		return []string{"generic"}
	}
	var names []string
	for _, a := range detected {
		names = append(names, a.Name())
	}
	return names
}
