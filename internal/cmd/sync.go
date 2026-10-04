package cmd

import (
	"fmt"

	"github.com/luuuc/council/internal/config"
	"github.com/luuuc/council/internal/sync"
	"github.com/spf13/cobra"
)

var (
	syncDryRun bool
	syncForce  bool
	syncClean  bool
)

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "Show what would be done without making changes")
	syncCmd.Flags().BoolVar(&syncForce, "force", false, "Overwrite existing files without prompting")
	syncCmd.Flags().BoolVar(&syncClean, "clean", false, "Remove stale command and agent files")
}

var syncCmd = &cobra.Command{
	Use:   "sync [target]",
	Short: "Sync council to AI tool configs",
	Long: `Syncs your .council/ experts to AI tool-specific locations.

Without arguments, syncs to all configured targets.
With a target name, syncs only to that target.

Supported targets:
  claude     .claude/agents/ and .claude/skills/
  opencode   .opencode/agents/ and .opencode/commands/
  generic    AGENTS.md and .agents/skills/ (Codex and other tools)`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !config.Exists() {
			return fmt.Errorf("council not initialized: run 'council init' first")
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		opts := sync.Options{
			DryRun: syncDryRun,
			Clean:  syncClean,
		}

		if len(args) == 1 {
			// Sync specific target
			return sync.SyncTarget(args[0], cfg, opts)
		}

		// Sync all configured targets
		return sync.SyncAll(cfg, opts)
	},
}
