// Command datjit is the CLI entry point for the datjitgo library. It exposes
// the façade subcommands (generate, validate, inspect, corpus, version, and
// the REPL stub) via cobra. See the package-level docs of
// github.com/periplon/datjitgo for the library-level entry points.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is the semver string printed by the `version` subcommand. The
// release workflow stamps the real tag in at link time via
// `-ldflags "-X main.version=<semver>"`; plain `go build`/`go install` leaves
// the "dev" default.
var version = "dev"

// usageErr wraps an error so main can exit with code 2 for flag/argument
// problems vs. code 1 for runtime failures.
type usageErr struct{ err error }

func (u *usageErr) Error() string { return u.err.Error() }
func (u *usageErr) Unwrap() error { return u.err }

// usageErrorf constructs a usageErr from a printf-style format string.
func usageErrorf(format string, a ...any) error {
	return &usageErr{err: fmt.Errorf(format, a...)}
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(exitCodeFor(err))
	}
}

// newRootCmd builds the top-level cobra command. It is a separate function
// so tests and alternative entry points can reuse the wiring.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "datjit",
		Short:         "Synthetic data generation from declarative schemas",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		cmdGenerate(),
		cmdValidate(),
		cmdInspect(),
		cmdSchema(),
		cmdCorpus(),
		cmdRepl(),
		cmdVersion(),
	)
	return root
}

// exitCodeFor maps an error to a shell exit code. Usage errors (bad flags,
// wrong arg counts) → 2, everything else → 1.
func exitCodeFor(err error) int {
	var ue *usageErr
	if errors.As(err, &ue) {
		return 2
	}
	return 1
}
