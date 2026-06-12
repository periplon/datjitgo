package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/periplon/datjitgo/mcp"
)

// cmdMCP wires the `datjit mcp` subcommand, which runs a Model Context Protocol
// server over stdio so AI coding agents can drive the parse→validate→generate
// pipeline. It speaks newline-delimited JSON-RPC 2.0 on stdin/stdout and exits
// 0 when stdin reaches EOF. There are no flags in v1.
func cmdMCP() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run an MCP server over stdio (generate/validate/inspect/sample)",
		Long: `Starts a Model Context Protocol server speaking newline-delimited
JSON-RPC 2.0 on stdin/stdout. It exposes four tools — generate, validate,
inspect, and sample — backed by the datjit pipeline. Generation is offline and
seeded (deterministic). The server exits 0 on stdin EOF.`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return mcp.Serve(cmd.Context(), os.Stdin, os.Stdout, mcp.Options{Version: version})
		},
	}
}
