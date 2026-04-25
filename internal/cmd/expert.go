package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/luuuc/council/internal/config"
	"github.com/luuuc/council/internal/expert"
	"github.com/luuuc/council/internal/sync"
	"github.com/spf13/cobra"
)

var listJSON bool
var addYes bool
var addInterview bool
var addFrom string
var addNoSync bool

func init() {
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(removeCmd)

	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output in JSON format")
	addCmd.Flags().BoolVarP(&addYes, "yes", "y", false, "Skip confirmation prompts")
	addCmd.Flags().BoolVar(&addInterview, "interview", false, "AI-assisted persona creation")
	addCmd.Flags().StringVar(&addFrom, "from", "", "Fork from existing persona ID")
	addCmd.Flags().BoolVar(&addNoSync, "no-sync", false, "Skip automatic sync after adding")
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all council members",
	Long:  `Shows all experts currently in the council with their ID and focus area.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !config.Exists() {
			return fmt.Errorf("council not initialized: run 'council init' first")
		}

		result, err := expert.ListWithWarnings()
		if err != nil {
			return err
		}

		// JSON output mode
		if listJSON {
			data, err := expert.MarshalExpertsJSON(result.Experts)
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Println(string(data))
			return nil
		}

		// Display any warnings about files that couldn't be loaded
		for _, warning := range result.Warnings {
			fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)
		}

		if len(result.Experts) == 0 {
			fmt.Println("No experts in the council yet.")
			fmt.Println()
			fmt.Println("Add experts with:")
			fmt.Println("  council add \"Name\"    Add from curated library or create custom")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "ID\tNAME\tFOCUS")
		for _, e := range result.Experts {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", e.ID, e.Name, e.Focus)
		}
		_ = w.Flush()

		return nil
	},
}

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show expert details",
	Long:  `Displays the full details of an expert including their philosophy and principles.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !config.Exists() {
			return fmt.Errorf("council not initialized: run 'council init' first")
		}

		e, err := expert.Load(args[0])
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("expert '%s' not found - run 'council list' to see available experts", args[0])
			}
			return err
		}

		fmt.Printf("ID:    %s\n", e.ID)
		fmt.Printf("Name:  %s\n", e.Name)
		fmt.Printf("Focus: %s\n", e.Focus)

		if len(e.Influences) > 0 {
			fmt.Println("\nInfluences:")
			for _, inf := range e.Influences {
				fmt.Printf("  - %s\n", inf)
			}
		}

		if e.Backstory != "" {
			fmt.Printf("\nBackstory:\n  %s\n", strings.TrimSpace(e.Backstory))
		}

		if e.Philosophy != "" {
			fmt.Printf("\nPhilosophy:\n  %s\n", strings.TrimSpace(e.Philosophy))
		}

		if len(e.Principles) > 0 {
			fmt.Println("\nPrinciples:")
			for _, p := range e.Principles {
				fmt.Printf("  - %s\n", p)
			}
		}

		if len(e.RedFlags) > 0 {
			fmt.Println("\nRed Flags:")
			for _, r := range e.RedFlags {
				fmt.Printf("  - %s\n", r)
			}
		}

		if len(e.Tensions) > 0 {
			fmt.Println("\nTensions:")
			for _, t := range e.Tensions {
				fmt.Printf("  vs %s — %s\n", t.Expert, t.Topic)
				fmt.Printf("    %s: %s\n", e.Name, t.Position)
				fmt.Printf("    %s: %s\n", t.Expert, t.Counterpoint)
			}
		}

		fmt.Printf("\nFile: %s\n", e.Path())

		return nil
	},
}

var addCmd = &cobra.Command{
	Use:   "add [name]",
	Short: "Add expert to council (from library, custom, --interview, or --from)",
	Long: `Adds an expert to your council.

If the name matches a curated expert from the library, adds it directly.
If no match is found, guides you through creating a custom expert.

Modes:
  council add "Ada Redgrave"    # Found in library - adds directly
  council add "My CTO"          # Not found - creates custom persona
  council add --interview       # AI-assisted persona creation
  council add --from ada-redgrave  # Fork existing persona as starting point`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !config.Exists() {
			return fmt.Errorf("council not initialized: run 'council start' first")
		}

		// Interview mode - AI-assisted creation
		if addInterview {
			if !isInteractive() {
				return fmt.Errorf("--interview requires an interactive terminal")
			}
			return runAddInterview()
		}

		// Fork mode - copy existing persona
		if addFrom != "" {
			if !isInteractive() {
				return fmt.Errorf("--from requires an interactive terminal")
			}
			return runAddFork(addFrom)
		}

		// Standard add mode - requires a name argument
		if len(args) == 0 {
			return fmt.Errorf("requires a persona name argument\n\nUsage:\n  council add \"Name\"         Add from library or create custom\n  council add --interview    AI-assisted creation\n  council add --from ID      Fork existing persona")
		}

		name := args[0]

		// Try curated lookup first
		if persona := LookupPersona(name); persona != nil {
			if expert.Exists(persona.ID) {
				return fmt.Errorf("expert '%s' already exists", persona.ID)
			}
			if err := persona.Save(); err != nil {
				return err
			}
			fmt.Printf("Added %s (%s)\n", persona.Name, persona.ID)
			fmt.Printf("File: %s\n", persona.Path())
			runAutoSync(addNoSync, nil)
			return nil
		}

		// Not found - try suggestion
		if suggestion, distance := SuggestSimilar(name); suggestion != nil {
			// Auto-accept with --yes flag, or prompt for confirmation in interactive mode
			shouldAdd := addYes
			if !shouldAdd && isInteractive() && distance <= 2 {
				shouldAdd = Confirm(fmt.Sprintf("Did you mean %q?", suggestion.Name))
			}

			if shouldAdd {
				if expert.Exists(suggestion.ID) {
					return fmt.Errorf("expert '%s' already exists", suggestion.ID)
				}
				if err := suggestion.Save(); err != nil {
					return err
				}
				fmt.Printf("Added %s (%s)\n", suggestion.Name, suggestion.ID)
				fmt.Printf("File: %s\n", suggestion.Path())
				runAutoSync(addNoSync, nil)
				return nil
			}
		}

		// No match found - trigger creation flow
		if !isInteractive() {
			return fmt.Errorf("persona %q not found in curated library\n\n"+
				"To create a custom expert interactively, run without piping:\n  council add %q\n\n"+
				"Or browse available personas:\n  council personas", name, name)
		}

		fmt.Printf("'%s' not found in curated library. Let's create a custom persona.\n\n", name)
		return runAddCreationFlow(name)
	},
}

var removeCmd = &cobra.Command{
	Use:   "remove <id>",
	Short: "Remove an expert from the council",
	Long:  `Removes an expert from the council.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !config.Exists() {
			return fmt.Errorf("council not initialized: run 'council init' first")
		}

		id := args[0]

		e, err := expert.Load(id)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("expert '%s' not found - run 'council list' to see available experts", id)
			}
			return err
		}

		if !Confirm(fmt.Sprintf("Remove %s from the council?", e.Name)) {
			fmt.Println("Cancelled.")
			return nil
		}

		if err := expert.Delete(id); err != nil {
			return err
		}

		fmt.Printf("Removed %s\n", e.Name)

		return nil
	},
}

// runAddCreationFlow guides the user through creating a custom expert
// for the project council (.council/experts/).
func runAddCreationFlow(name string) error {
	reader := bufio.NewReader(os.Stdin)

	// Generate ID from name
	id := expert.ToID(name)

	// Check if expert already exists
	if expert.Exists(id) {
		return fmt.Errorf("expert '%s' already exists", id)
	}

	// Focus (required)
	fmt.Print("Focus (one-line description of their expertise): ")
	focus, _ := reader.ReadString('\n')
	focus = trimNewline(focus)
	if focus == "" {
		return fmt.Errorf("focus is required")
	}

	// Philosophy (optional)
	fmt.Print("Philosophy (optional, press Enter to skip): ")
	philosophy, _ := reader.ReadString('\n')
	philosophy = trimNewline(philosophy)

	// Create expert
	e := &expert.Expert{
		ID:         id,
		Name:       name,
		Focus:      focus,
		Philosophy: philosophy,
	}

	// Save to project council
	if err := e.Save(); err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("Created %s (%s)\n", e.Name, e.ID)
	fmt.Printf("File: %s\n", e.Path())
	runAutoSync(addNoSync, nil)

	return nil
}

// trimNewline removes trailing newline characters from a string
func trimNewline(s string) string {
	return strings.TrimRight(s, "\r\n")
}

// runAutoSync runs sync after adding an expert.
// Pass skipSync=true to skip (for batch operations).
// Pass cfg=nil to auto-load config, or pass existing config to avoid redundant load.
func runAutoSync(skipSync bool, cfg *config.Config) {
	if skipSync {
		return
	}

	var err error
	if cfg == nil {
		cfg, err = config.Load()
		if err != nil {
			fmt.Printf("Warning: could not load config for sync: %v\n", err)
			return
		}
	}

	fmt.Println()
	if err := sync.SyncAll(cfg, sync.Options{}); err != nil {
		fmt.Printf("Warning: sync failed: %v\n", err)
		fmt.Println("Run 'council sync' to retry.")
	}
}

// runAddFork creates a new expert based on an existing one.
func runAddFork(fromID string) error {
	// Try to load from project council first
	var source *expert.Expert
	var err error

	source, err = expert.Load(fromID)
	if err != nil {
		// Try to find in curated library
		source = LookupPersona(fromID)
		if source == nil {
			return fmt.Errorf("expert '%s' not found in project council or curated library\n\nBrowse available personas with: council personas", fromID)
		}
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("Fork '%s' as starting point\n", source.Name)
	fmt.Println()

	// Prompt for new name
	fmt.Printf("New name: [%s (Custom)] ", source.Name)
	nameInput, _ := reader.ReadString('\n')
	nameInput = trimNewline(nameInput)
	if nameInput == "" {
		nameInput = source.Name + " (Custom)"
	}

	// Generate and prompt for ID
	suggestedID := expert.ToID(nameInput)
	fmt.Printf("New ID: [%s] ", suggestedID)
	idInput, _ := reader.ReadString('\n')
	idInput = trimNewline(idInput)
	if idInput == "" {
		idInput = suggestedID
	}

	if expert.Exists(idInput) {
		return fmt.Errorf("expert '%s' already exists", idInput)
	}

	// Create new expert based on source
	e := &expert.Expert{
		ID:         idInput,
		Name:       nameInput,
		Focus:      source.Focus,
		Category:   "custom",
		Priority:   source.Priority,
		Philosophy: source.Philosophy,
		Principles: source.Principles,
		RedFlags:   source.RedFlags,
		Tensions:   source.Tensions,
		Triggers:   source.Triggers,
	}

	// Save to project council
	if err := e.Save(); err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("Created %s (forked from %s)\n", e.Name, source.Name)
	fmt.Printf("File: %s\n", e.Path())

	// Offer to edit
	fmt.Println()
	if Confirm("Open in editor to customize?") {
		if err := openInEditor(e.Path()); err != nil {
			return err
		}
	}

	runAutoSync(addNoSync, nil)
	return nil
}
