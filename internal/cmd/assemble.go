package cmd

import (
	"fmt"

	"github.com/luuuc/council/internal/brief"
	"github.com/luuuc/council/internal/config"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(assembleCmd)
}

var assembleCmd = &cobra.Command{
	Use:   "assemble",
	Short: "Print the brief your AI follows to assemble a council",
	Long: `Prints instructions for your AI: read the project, propose members
(people, roles, customers) with reasons, let you choose, build each persona,
and save it with 'council add'. Council itself names no one.

Run it from your AI tool, or use /council-assemble.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !config.Exists() {
			return fmt.Errorf("council not initialized: run 'council init' first")
		}
		text, err := brief.Assemble()
		if err != nil {
			return err
		}
		fmt.Print(text)
		return nil
	},
}
