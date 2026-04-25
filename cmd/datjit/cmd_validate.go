package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jmcarbo/datjitgo"
)

// cmdValidate runs Parse → Validate and prints "OK" or a diagnostic.
func cmdValidate() *cobra.Command {
	c := &cobra.Command{
		Use:          "validate <schema.yaml>",
		Short:        "Parse and statically check a DDL schema",
		SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usageErrorf("validate requires exactly one schema path (got %d)", len(args))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := datjit.NewDefault()
			doc, err := parseSchemaFile(svc, args[0])
			if err != nil {
				return err
			}
			if err := svc.Validate(doc); err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "OK")
			return err
		},
	}
	return c
}
