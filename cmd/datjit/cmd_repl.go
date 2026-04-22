package main

import (
	"context"
	"fmt"
	"os"

	datjit "github.com/jmcarbo/datjitgo"
	"github.com/jmcarbo/datjitgo/repl"
	"github.com/spf13/cobra"
)

// cmdRepl builds the `datjit repl` subcommand. It is exported to the main
// package (lowercase constructor, same package) so main.go can plug it into
// the root command without reaching into repl package internals.
//
// Behaviour:
//   - With no args: drop straight into an empty REPL session.
//   - With exactly one arg: treat it as a schema path, pre-load it, then
//     drop into the REPL. This mirrors `datjit repl path/to/schema.yaml`
//     from the spec §10 examples.
//
// The command always reads from os.Stdin and writes to os.Stdout/os.Stderr.
// Cancellation is wired through context.Background() for now; when the
// broader CLI gains signal handling we can plumb through cobra.Context().
func cmdRepl() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repl [schema]",
		Short: "Start an interactive datjit REPL",
		Long: `Launches the datjit interactive shell. When a schema path is
provided, it is parsed and loaded as the active document before the prompt
appears. Type 'help' for the command list, 'exit' or Ctrl-D to leave.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := datjit.NewDefault()
			sess := repl.New(svc)

			if len(args) == 1 {
				// Preload by constructing a synthetic `load` line so the
				// status output is consistent with the interactive `load`
				// command. Errors are non-fatal: the user can `load` a
				// different file from the prompt.
				preload(sess, args[0])
			}

			return sess.Run(context.Background(), os.Stdin, os.Stdout, os.Stderr)
		},
	}
	return cmd
}

// preload opens the given schema path and stores it on the session's
// document slot before the REPL loop begins. Failures are reported to
// stderr but do not abort the shell — the user may want to load something
// else interactively.
func preload(sess *repl.Session, path string) {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: preload %s: %v\n", path, err)
		return
	}
	defer f.Close()
	doc, err := sess.Service().Parse(f, path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: parse %s: %v\n", path, err)
		return
	}
	sess.State().Doc = doc
	sess.State().Path = path
	fmt.Fprintf(os.Stderr, "loaded %s (domain=%s, entities=%d)\n", path, doc.Domain, doc.Entities.Len())
}
