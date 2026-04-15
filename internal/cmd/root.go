// Package cmd implements the council CLI commands using cobra.
package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/luuuc/council-cli/internal/adapter"
	"github.com/luuuc/council-cli/internal/config"
	"github.com/luuuc/council-cli/internal/sync"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
)

var rootCmd = &cobra.Command{
	Use:   "council",
	Short: "Expert councils for AI coding assistants",
	Long: `council-cli helps you create an expert council for AI coding assistants.

The council pattern establishes quality standards through expert personas
that represent excellence in specific domains - Rob Pike for Go clarity,
Kent Beck for testing, Dieter Rams for design simplicity.

Quick start:
  council start          Zero-config setup (creates council, adds experts, syncs)
  council add "Name"     Add expert from library or create custom
  council sync           Sync council to AI tool configs`,
}

func Execute() error {
	return rootCmd.Execute()
}

var initClean bool
var initTool string
var versionJSON bool

func init() {
	rootCmd.Version = fmt.Sprintf("%s (%s)", version, commit)
	rootCmd.SetVersionTemplate("council {{.Version}}\n")
	rootCmd.AddCommand(versionCmd)
	versionCmd.Flags().BoolVar(&versionJSON, "json", false, "Output version information as JSON")
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVar(&initClean, "clean", false, "Remove existing council and synced files before initializing")
	initCmd.Flags().StringVar(&initTool, "tool", "", "Primary AI tool: claude, opencode, generic")
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		if versionJSON {
			_ = json.NewEncoder(os.Stdout).Encode(map[string]string{
				"version": version,
				"commit":  commit,
			})
			return
		}
		fmt.Printf("council %s (%s)\n", version, commit)
	},
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new .council directory",
	Long: `Creates the .council/ directory structure in the current project.

Tool detection:
  - If only one AI tool is detected (e.g., .claude/ exists), it's used automatically
  - If multiple tools are detected, you'll be prompted to choose
  - If no tool is detected, use --tool to specify one

Examples:
  council init              Auto-detect tool
  council init --tool=claude   Force Claude Code
  council init --tool=generic  Use AGENTS.md fallback`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return initCouncil(initClean, initTool)
	},
}

// cleanExisting removes existing council directory and synced files
func cleanExisting() error {
	// Remove .council/ directory
	if err := os.RemoveAll(config.CouncilDir); err != nil {
		return fmt.Errorf("failed to remove .council/: %w", err)
	}
	fmt.Println("Removed .council/")

	// Remove synced files from all targets (derived from registry)
	for _, path := range sync.AllCleanPaths() {
		if _, err := os.Stat(path); err == nil {
			if err := os.RemoveAll(path); err != nil {
				fmt.Printf("Warning: could not remove %s: %v\n", path, err)
			} else {
				fmt.Printf("Removed %s\n", path)
			}
		}
	}

	return nil
}

func initCouncil(clean bool, toolFlag string) error {
	// Handle existing installation
	if config.Exists() {
		if !clean {
			return fmt.Errorf(".council/ already exists (use --clean to remove and reinitialize)")
		}
		// Clean existing council and synced files
		if err := cleanExisting(); err != nil {
			return fmt.Errorf("failed to clean existing setup: %w", err)
		}
	}

	// Determine the tool to use
	tool, err := detectOrSelectTool(toolFlag)
	if err != nil {
		return err
	}

	// Create directory structure
	dirs := []string{
		config.CouncilDir,
		config.Path(config.ExpertsDir),
		config.Path(config.CommandsDir),
		config.Path(config.PacksDir),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}

	// Create config with detected tool
	cfg := config.Default()
	cfg.Tool = tool
	if err := cfg.Save(); err != nil {
		return err
	}

	// Create .gitkeep files
	for _, subdir := range []string{config.ExpertsDir, config.CommandsDir, config.PacksDir} {
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

	fmt.Printf("Initialized .council/ directory for %s\n", displayName)
	fmt.Println("")
	fmt.Println("Next steps:")
	fmt.Println("  council add \"Name\"     Add experts from library or create custom")
	fmt.Println("  council sync           Sync to AI tool configs")

	return nil
}

// detectOrSelectTool determines which tool to use based on flag, detection, or user input
func detectOrSelectTool(toolFlag string) (string, error) {
	// If explicit tool provided, validate and use it
	if toolFlag != "" {
		if err := config.ValidateTool(toolFlag); err != nil {
			return "", err
		}
		a, ok := adapter.Get(toolFlag)
		if !ok {
			return "", fmt.Errorf("unknown tool '%s'", toolFlag)
		}
		fmt.Printf("Using: %s\n", a.DisplayName())
		return toolFlag, nil
	}

	// Detect tools
	detected := adapter.Detect()

	switch len(detected) {
	case 0:
		// No tool detected - require explicit flag
		return "", fmt.Errorf("no AI tool detected\n\nSpecify a tool with:\n  council init --tool=claude\n  council init --tool=opencode\n  council init --tool=generic")

	case 1:
		// Single tool detected - use it automatically
		tool := detected[0]
		fmt.Printf("Detected: %s\n", tool.DisplayName())
		return tool.Name(), nil

	default:
		// Multiple tools detected - prompt user
		return promptForTool(detected)
	}
}

// promptForTool asks the user to select from multiple detected tools
func promptForTool(detected []adapter.Adapter) (string, error) {
	fmt.Print("Multiple AI tools detected:\n")
	for i, a := range detected {
		fmt.Printf("  %d. %s\n", i+1, a.DisplayName())
	}
	fmt.Print("\nSelect primary tool (1-")
	fmt.Printf("%d): ", len(detected))

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(input)

	// Parse as number
	var idx int
	if _, err := fmt.Sscanf(input, "%d", &idx); err != nil || idx < 1 || idx > len(detected) {
		return "", fmt.Errorf("invalid selection '%s': enter a number 1-%d", input, len(detected))
	}

	selected := detected[idx-1]
	fmt.Printf("Selected: %s\n", selected.DisplayName())
	return selected.Name(), nil
}
