package cmd

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"text/tabwriter"

	"github.com/luuuc/council-cli/internal/expert"
	"github.com/luuuc/council-cli/internal/pack"
	"github.com/spf13/cobra"
)

var packsJSON bool

func init() {
	rootCmd.AddCommand(packsCmd)
	packsCmd.AddCommand(packsShowCmd)
	packsCmd.AddCommand(packsCreateCmd)
	packsCmd.AddCommand(packsAddCmd)
	packsCmd.AddCommand(packsRemoveCmd)

	packsCmd.Flags().BoolVar(&packsJSON, "json", false, "Output as JSON")
	packsAddCmd.Flags().BoolVar(&packsAddBlocking, "blocking", false, "Mark expert as blocking")
}

var packsAddBlocking bool

var packsCmd = &cobra.Command{
	Use:   "packs",
	Short: "List available expert packs",
	Long: `Shows all available expert packs (built-in and custom).

A pack is a reusable group of experts for focused reviews.

Examples:
  council packs                  # List all packs
  council packs --json           # JSON output
  council packs show rails       # Show pack details
  council packs create my-api    # Create a custom pack`,
	RunE: func(cmd *cobra.Command, args []string) error {
		packs, err := pack.ListAll()
		if err != nil {
			return err
		}

		if len(packs) == 0 {
			fmt.Println("No packs available.")
			return nil
		}

		// Sort by name
		slices.SortFunc(packs, func(a, b *pack.Pack) int {
			return cmp.Compare(a.Name, b.Name)
		})

		if packsJSON {
			data, err := json.MarshalIndent(packs, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		fmt.Printf("Available packs (%d):\n\n", len(packs))
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "  NAME\tMEMBERS\tSOURCE\tDESCRIPTION\n")
		for _, p := range packs {
			source := "custom"
			if p.Source == "builtin" {
				source = "builtin"
			}
			fmt.Fprintf(w, "  %s\t%d\t%s\t%s\n", p.Name, len(p.Members), source, p.Description)
		}
		w.Flush()

		return nil
	},
}

var packsShowCmd = &cobra.Command{
	Use:   "show <pack>",
	Short: "Show pack members",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		p, err := pack.Get(name)
		if err != nil {
			return err
		}

		fmt.Printf("Pack: %s\n", p.Name)
		if p.Description != "" {
			fmt.Printf("Description: %s\n", p.Description)
		}
		if p.Source == "builtin" {
			fmt.Printf("Source: builtin\n")
		}
		fmt.Println()

		if len(p.Members) == 0 {
			fmt.Println("No members.")
			return nil
		}

		fmt.Printf("Members (%d):\n", len(p.Members))
		for _, m := range p.Members {
			blocking := ""
			if m.Blocking {
				blocking = " [blocking]"
			}
			fmt.Printf("  - %s%s\n", m.ID, blocking)
		}

		// Show known tensions between pack members.
		// Load expert data once per member for name resolution and tension lookup.
		memberIDs := make(map[string]bool, len(p.Members))
		loaded := make(map[string]*expert.Expert, len(p.Members))
		for _, m := range p.Members {
			memberIDs[m.ID] = true
			e, err := expert.Load(m.ID)
			if err != nil && !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Warning: could not load expert '%s': %v\n", m.ID, err)
			}
			if e == nil {
				e = LookupPersona(m.ID)
			}
			if e != nil {
				loaded[m.ID] = e
			}
		}

		type tensionPair struct {
			fromName, toName, topic string
		}
		var tensions []tensionPair
		seen := make(map[string]bool)

		for _, m := range p.Members {
			e := loaded[m.ID]
			if e == nil {
				continue
			}
			for _, t := range e.Tensions {
				if !memberIDs[t.Expert] {
					continue
				}
				// Deduplicate symmetric tensions by normalizing the pair
				a, b := e.ID, t.Expert
				if a > b {
					a, b = b, a
				}
				key := a + ":" + b + ":" + t.Topic
				if seen[key] {
					continue
				}
				seen[key] = true

				toName := t.Expert
				if other, ok := loaded[t.Expert]; ok {
					toName = other.Name
				}
				tensions = append(tensions, tensionPair{
					fromName: e.Name,
					toName:   toName,
					topic:    t.Topic,
				})
			}
		}

		if len(tensions) > 0 {
			fmt.Printf("\nKnown tensions in this pack:\n")
			for _, t := range tensions {
				fmt.Printf("  %s vs %s — %s\n", t.fromName, t.toName, t.topic)
			}
		}

		return nil
	},
}

var packsCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a custom pack",
	Long:  `Create a custom pack in .council/packs/. Pack names must not contain spaces or slashes.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Check if custom pack already exists
		if _, err := pack.Load(name); err == nil {
			return fmt.Errorf("pack '%s' already exists", name)
		}

		p := &pack.Pack{
			Name:    name,
			Members: []pack.Member{},
		}

		if err := pack.Save(p); err != nil {
			return err
		}

		fmt.Printf("Created pack '%s' in .council/packs/%s.yaml\n", name, name)
		fmt.Printf("Add members with: council packs add %s <expert-id>\n", name)
		return nil
	},
}

var packsAddCmd = &cobra.Command{
	Use:   "add <pack> <expert-id>",
	Short: "Add an expert to a pack",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, expertID := args[0], args[1]

		p, err := pack.Get(name)
		if err != nil {
			return err
		}

		if p.Source == "builtin" {
			return fmt.Errorf("cannot modify built-in pack '%s' — create a custom override with: council packs create %s", name, name)
		}

		if err := p.AddMember(expertID, packsAddBlocking); err != nil {
			return err
		}

		if err := pack.Save(p); err != nil {
			return err
		}

		label := ""
		if packsAddBlocking {
			label = " (blocking)"
		}
		fmt.Printf("Added '%s'%s to pack '%s'\n", expertID, label, name)
		return nil
	},
}

var packsRemoveCmd = &cobra.Command{
	Use:   "remove <pack> <expert-id>",
	Short: "Remove an expert from a pack",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, expertID := args[0], args[1]

		p, err := pack.Get(name)
		if err != nil {
			return err
		}

		if p.Source == "builtin" {
			return fmt.Errorf("cannot modify built-in pack '%s' — create a custom override with: council packs create %s", name, name)
		}

		if err := p.RemoveMember(expertID); err != nil {
			return err
		}

		if err := pack.Save(p); err != nil {
			return err
		}

		fmt.Printf("Removed '%s' from pack '%s'\n", expertID, name)
		return nil
	},
}
