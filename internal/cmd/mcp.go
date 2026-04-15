package cmd

import (
	"context"
	"os"
	"os/signal"

	"github.com/luuuc/council-cli/internal/mcp"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(mcpCmd)
}

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server (stdin/stdout)",
	Long: `Start a Model Context Protocol server over stdin/stdout.

This command is designed to be spawned by MCP-capable AI tools
(Claude Code, Cursor, Claude Desktop) as a subprocess. It speaks
JSON-RPC 2.0 over stdin/stdout and exposes council tools:

  council_review   Submit code for blind council review
  council_list     List experts in a pack
  council_explain  Expand on a review note

Configuration:
  Add to .mcp.json in your project:
  {
    "mcpServers": {
      "council": {
        "command": "council",
        "args": ["mcp"]
      }
    }
  }`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()

		srv := mcp.NewServer(os.Stdin, os.Stdout, version)
		return srv.Run(ctx)
	},
}
