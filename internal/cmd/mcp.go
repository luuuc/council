package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/luuuc/council/internal/mcp"
	"github.com/spf13/cobra"
)

var mcpDir string

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.Flags().StringVar(&mcpDir, "dir", "", "Project directory with the .council/ to use (for clients like Claude Desktop that don't start servers in a project)")
}

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server (stdin/stdout)",
	Long: `Start a Model Context Protocol server over stdin/stdout.

This command is designed to be spawned by MCP-capable AI tools
(Claude Code, Cursor, Claude Desktop) as a subprocess. It speaks
JSON-RPC 2.0 over stdin/stdout and exposes council tools:

  council_room         The room prompt: the client's model writes the whole
                       debate in one pass
  council_record       Check the debate, save it, and return the review
  council_assemble     The brief for proposing and building members
  council_add          Save a member the client built (person, role, customer)
  council_list         List experts in a pack

Council makes no model calls here: the client's own model does the work.

It also offers a "council" prompt for the client's prompt menu.

Configuration:
  Add to .mcp.json in your project:
  {
    "mcpServers": {
      "council": {
        "command": "council",
        "args": ["mcp"]
      }
    }
  }

  Claude Desktop starts servers outside any project. To use a project's
  council there, pass its directory: "args": ["mcp", "--dir", "/path/to/project"]`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if mcpDir != "" {
			if err := os.Chdir(mcpDir); err != nil {
				return fmt.Errorf("cannot use --dir %q: %w", mcpDir, err)
			}
		}

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()

		srv := mcp.NewServer(os.Stdin, os.Stdout, version)
		return srv.Run(ctx)
	},
}
