// Package main — cmdRepl stub.
//
// This file is intentionally minimal. The real REPL subcommand is
// implemented by the repl package; this stub exists so the CLI still
// compiles and exposes a discoverable `repl` subcommand name while the
// REPL worktree is under development. The REPL agent will replace this
// file wholesale with the production wiring.
package main

import (
	"errors"

	"github.com/spf13/cobra"
)

// cmdRepl returns the placeholder REPL subcommand. It fails loudly so a
// user who invokes it on a pre-release build does not silently get a no-op.
func cmdRepl() *cobra.Command {
	return &cobra.Command{
		Use:          "repl [schema.yaml]",
		Short:        "Interactive REPL (wired by the repl package)",
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			// implemented by repl package
			return errors.New("repl subcommand is wired by the repl package; build with the full release tag")
		},
	}
}
