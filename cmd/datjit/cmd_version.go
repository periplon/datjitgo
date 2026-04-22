package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// cmdVersion prints the CLI semver on a single line.
func cmdVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print datjit version and exit",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "datjit v%s\n", version)
			return nil
		},
	}
}
