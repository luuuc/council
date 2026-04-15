package cmd

import (
	"cmp"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/luuuc/council-cli/internal/expert"
	"github.com/luuuc/council-cli/internal/install"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

//go:embed suggestions.yaml
var suggestionsYAML []byte

// suggestionBank holds all expert suggestions loaded from YAML (lazy loaded)
var (
	suggestionBank     map[string][]expert.Expert
	suggestionBankOnce sync.Once
)

// loadSuggestionBank lazily loads the suggestion bank on first access.
func loadSuggestionBank() map[string][]expert.Expert {
	suggestionBankOnce.Do(func() {
		if err := yaml.Unmarshal(suggestionsYAML, &suggestionBank); err != nil {
			panic(fmt.Sprintf("failed to parse suggestions.yaml: %v", err))
		}
	})
	return suggestionBank
}

var (
	personasJSON     bool
	personasCategory string
	personasSearch   string
)

func init() {
	rootCmd.AddCommand(personasCmd)
	personasCmd.AddCommand(personasInstallCmd)
	personasCmd.AddCommand(personasInstalledCmd)
	personasCmd.AddCommand(personasUpdateCmd)
	personasCmd.AddCommand(personasUninstallCmd)

	personasCmd.Flags().BoolVar(&personasJSON, "json", false, "Output as JSON")
	personasCmd.Flags().StringVar(&personasCategory, "category", "", "Filter by category (e.g., go, ruby, testing)")
	personasCmd.Flags().StringVar(&personasSearch, "search", "", "Search by name or focus")
}

// PersonaJSON is a flattened persona with category
type PersonaJSON struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Category   string   `json:"category"`
	Focus      string   `json:"focus"`
	Philosophy string   `json:"philosophy,omitempty"`
	Principles []string `json:"principles,omitempty"`
	RedFlags   []string         `json:"red_flags,omitempty"`
	Tensions   []expert.Tension `json:"tensions,omitempty"`
	Triggers   []string         `json:"triggers,omitempty"`
}

// personasCmd manages the expert persona library.
// Design decision: Repository management (install/update/uninstall) lives here
// rather than a separate "repos" command because personas are the core concept
// and repositories are just a source of personas. If this grows further,
// consider extracting to a "council repos" subcommand.
var personasCmd = &cobra.Command{
	Use:   "personas",
	Short: "List all available expert personas",
	Long: `Shows all built-in expert personas that can be added to a council.

Filtering:
  --category <name>   Filter by category (go, ruby, python, javascript, testing, etc.)
  --search <term>     Search by name or focus (case-insensitive)

Examples:
  council personas                        # List all
  council personas --category go          # Go experts
  council personas --category testing     # Testing experts
  council personas --search "security"    # Search for security-related`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var personas []PersonaJSON

		for category, experts := range loadSuggestionBank() {
			for _, e := range experts {
				personas = append(personas, PersonaJSON{
					ID:         e.ID,
					Name:       e.Name,
					Category:   category,
					Focus:      e.Focus,
					Philosophy: e.Philosophy,
					Principles: e.Principles,
					RedFlags:   e.RedFlags,
					Tensions:   e.Tensions,
					Triggers:   e.Triggers,
				})
			}
		}

		// Apply category filter
		if personasCategory != "" {
			personas = filterPersonasByCategory(personas, personasCategory)
		}

		// Apply search filter
		if personasSearch != "" {
			personas = filterPersonasBySearch(personas, personasSearch)
		}

		// Sort by category, then name for deterministic output
		slices.SortFunc(personas, func(a, b PersonaJSON) int {
			if c := cmp.Compare(a.Category, b.Category); c != 0 {
				return c
			}
			return cmp.Compare(a.Name, b.Name)
		})

		if personasJSON {
			data, err := json.MarshalIndent(personas, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		// Human-readable output
		if len(personas) == 0 {
			fmt.Println("No personas found matching your criteria.")
			return nil
		}

		fmt.Printf("Available personas (%d):\n\n", len(personas))
		for _, p := range personas {
			fmt.Printf("  %s (%s) - %s\n", p.Name, p.Category, p.Focus)
		}

		return nil
	},
}

// filterPersonasByCategory filters personas by category (case-insensitive).
func filterPersonasByCategory(personas []PersonaJSON, category string) []PersonaJSON {
	category = strings.ToLower(category)
	var filtered []PersonaJSON
	for _, p := range personas {
		if strings.ToLower(p.Category) == category {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// filterPersonasBySearch filters personas by name or focus (case-insensitive).
func filterPersonasBySearch(personas []PersonaJSON, search string) []PersonaJSON {
	search = strings.ToLower(search)
	var filtered []PersonaJSON
	for _, p := range personas {
		if strings.Contains(strings.ToLower(p.Name), search) ||
			strings.Contains(strings.ToLower(p.Focus), search) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

var personasInstallCmd = &cobra.Command{
	Use:   "install <url>",
	Short: "Install personas from a git repository",
	Long: `Clones a git repository containing custom personas.

The repository will be cloned to your local config directory and
its personas will be available in council setup and sync.

Examples:
  council personas install https://github.com/dhh/my-council.git
  council personas install git@github.com:dhh/my-council.git`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := args[0]

		fmt.Printf("Installing from %s...\n\n", url)

		name, err := install.Install(url)
		if err != nil {
			return err
		}

		// List experts in installed repo
		experts, _ := install.ListInstalledExperts()
		var repoExperts []*expert.Expert
		for _, e := range experts {
			if e.Source == "installed:"+name {
				repoExperts = append(repoExperts, e)
			}
		}

		fmt.Println()
		fmt.Printf("Installed to: %s\n\n", name)

		if len(repoExperts) > 0 {
			fmt.Println("Experts added:")
			for _, e := range repoExperts {
				fmt.Printf("  - %s - %s\n", e.ID, e.Focus)
			}
			fmt.Println()
		}

		fmt.Println("Personas available via:")
		fmt.Println("  council personas --json")

		return nil
	},
}

var personasInstalledCmd = &cobra.Command{
	Use:   "installed",
	Short: "List installed persona repositories",
	Long:  `Shows all persona repositories installed from remote sources.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		installed, err := install.ListInstalled()
		if err != nil {
			return err
		}

		if len(installed) == 0 {
			fmt.Println("No repositories installed.")
			fmt.Println()
			fmt.Println("Install one with:")
			fmt.Println("  council personas install <url>")
			return nil
		}

		// Get experts for each installed repo
		experts, _ := install.ListInstalledExperts()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "REPOSITORY\tEXPERTS")

		for _, name := range installed {
			count := 0
			for _, e := range experts {
				if e.Source == "installed:"+name {
					count++
				}
			}
			_, _ = fmt.Fprintf(w, "%s\t%d\n", name, count)
		}

		_ = w.Flush()
		return nil
	},
}

var personasUpdateCmd = &cobra.Command{
	Use:   "update [name]",
	Short: "Update installed persona repositories",
	Long: `Pulls the latest changes from installed repositories.

If no name is specified, updates all installed repositories.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			// Update specific repo
			name := args[0]
			fmt.Printf("Updating %s...\n", name)

			if err := install.Update(name); err != nil {
				return err
			}

			fmt.Println("Done")
			return nil
		}

		// Update all
		fmt.Println("Updating all installed repositories...")
		fmt.Println()

		updated, err := install.UpdateAll()
		if err != nil {
			return err
		}

		if len(updated) == 0 {
			fmt.Println("No repositories to update.")
			return nil
		}

		for _, name := range updated {
			fmt.Printf("  Updated %s\n", name)
		}

		fmt.Println()
		fmt.Println("Done")
		return nil
	},
}

var personasUninstallCmd = &cobra.Command{
	Use:   "uninstall <name>",
	Short: "Uninstall a persona repository",
	Long:  `Removes an installed persona repository.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if !Confirm(fmt.Sprintf("Uninstall '%s'?", name)) {
			fmt.Println("Cancelled.")
			return nil
		}

		if err := install.Uninstall(name); err != nil {
			return err
		}

		fmt.Printf("Uninstalled %s\n", name)
		return nil
	},
}

// LookupPersona finds a curated persona by name or ID (case-insensitive).
// Returns nil if not found.
func LookupPersona(nameOrID string) *expert.Expert {
	return expert.LookupPersona(expert.SuggestionBank(loadSuggestionBank()), nameOrID)
}

// SuggestSimilar finds the closest persona match using edit distance.
// Returns nil if no close match (distance > 3), if exact match exists,
// or if the input is too short to match reliably.
// The second return value is the edit distance of the match.
func SuggestSimilar(input string) (*expert.Expert, int) {
	return expert.SuggestSimilar(expert.SuggestionBank(loadSuggestionBank()), input)
}
