package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/luuuc/council/internal/config"
	"github.com/luuuc/council/internal/expert"
	"github.com/luuuc/council/internal/sync"
	"github.com/spf13/cobra"
)

var listJSON bool
var addNoSync bool

func init() {
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(removeCmd)

	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output in JSON format")
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

		if e.Kind != "" {
			fmt.Printf("Kind:  %s\n", e.Kind)
		}
		if e.Disclaimer != "" {
			fmt.Printf("\n%s\n", e.Disclaimer)
		}

		if len(e.Sources) > 0 {
			fmt.Println("\nSources:")
			for _, src := range e.Sources {
				fmt.Printf("  - %s\n", src)
			}
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

		if len(e.Inferred) > 0 {
			fmt.Println("\nInferred (not stated by them):")
			for _, p := range e.Inferred {
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
	Use:   "add <file | ->",
	Short: "Add a member from a persona file your AI wrote",
	Long: `Adds a member to the council from a persona file: markdown with YAML
frontmatter, or bare YAML, read from a file or from stdin ("-").

Council doesn't research or invent people. Your AI writes the persona (run
'council assemble' for the format and rules), and Council checks it,
applies the naming rules, and saves it:

  person    a real person, named "Virtual {Name}"; needs public sources;
            Council adds a no-affiliation disclaimer
  role      a role such as SRE or security engineer
  customer  a type of user, named "Customer: {label}"; needs evidence

Examples:
  council add persona.md
  council add - <<'EOF'
  name: Jane Doe
  kind: person
  focus: Test-driven development
  sources:
    - "A talk on TDD"
  principles:
    - Write the test first
  EOF`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !config.Exists() {
			return fmt.Errorf("council not initialized: run 'council init' first")
		}
		return runAdd(args[0])
	},
}

// runAdd validates a persona file (or stdin) and saves it as a member.
func runAdd(arg string) error {
	var data []byte
	var err error
	if arg == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(arg)
		if os.IsNotExist(err) && !strings.ContainsAny(arg, "/.") {
			return fmt.Errorf("council add takes a persona file, not a name.\n\n"+
				"Council doesn't research people. Ask your AI to build the persona,\n"+
				"e.g. /council-add %s (it follows 'council assemble'), then save it with:\n"+
				"  council add persona.md   or   council add - < persona.md", arg)
		}
	}
	if err != nil {
		return fmt.Errorf("reading persona: %w", err)
	}

	e, err := expert.ParseLoose(string(data))
	if err != nil {
		return fmt.Errorf("could not read the persona: %w", err)
	}
	if err := expert.Prepare(e); err != nil {
		return err
	}
	if expert.Exists(e.ID) {
		return fmt.Errorf("%s (%s) is already on the council: remove it first with 'council remove %s' to replace it", e.Name, e.ID, e.ID)
	}
	if err := e.Save(); err != nil {
		return err
	}

	fmt.Printf("Added %s (%s)\n", e.Name, e.ID)
	fmt.Printf("File: %s\n", e.Path())
	runAutoSync(addNoSync, nil)
	return nil
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

		// Ask only in a terminal; an AI tool removing a member has already asked.
		if isTerminal(os.Stdin) && !Confirm(fmt.Sprintf("Remove %s from the council?", e.Name)) {
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
