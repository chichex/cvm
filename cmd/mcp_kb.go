// Spec: S-014
package cmd

import (
	"github.com/chichex/cvm/internal/mcpkb"
	"github.com/spf13/cobra"
)

var mcpKbCmd = &cobra.Command{
	Use:   "mcp-kb",
	Short: "MCP server for KB search and get tools",
	Long:  "Starts an MCP (Model Context Protocol) server exposing kb_search and kb_get tools via stdio JSON-RPC 2.0.",
	Run: func(cmd *cobra.Command, args []string) {
		mcpkb.Serve()
	},
	// Hide from main help — this is used by Claude Code, not humans
	Hidden: true,
}
