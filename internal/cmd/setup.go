package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(setupRedirectCmd, startRedirectCmd)
}

// setupRedirectCmd and startRedirectCmd point users of removed setup
// commands to 'council init'.
var setupRedirectCmd = &cobra.Command{
	Use:    "setup",
	Short:  "Removed: use 'council init'",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("'council setup' has been removed\n\nUse 'council init' to set up a council")
	},
}

var startRedirectCmd = &cobra.Command{
	Use:    "start",
	Short:  "Removed: use 'council init'",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("'council start' has been removed: Council no longer picks people for you\n\nUse 'council init', then add members with 'council add'")
	},
}
