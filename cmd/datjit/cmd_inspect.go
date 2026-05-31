package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/periplon/datjitgo"
	"github.com/periplon/datjitgo/core/model"
)

// cmdInspect wires the `datjit inspect` subcommand, which prints a
// tree-like summary of the schema without generating any data.
func cmdInspect() *cobra.Command {
	var inferTools bool
	c := &cobra.Command{
		Use:          "inspect <schema.yaml>",
		Short:        "Summarise a DDL schema (entities, enums, rules, volumes)",
		SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usageErrorf("inspect requires exactly one schema path (got %d)", len(args))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := datjit.NewDefault()
			doc, err := parseSchemaFile(svc, args[0])
			if err != nil {
				return err
			}
			insp, err := svc.Inspect(doc)
			if err != nil {
				return err
			}
			return printInspection(cmd.OutOrStdout(), doc, insp, inferTools)
		},
	}
	c.Flags().BoolVar(&inferTools, "infer-tools", false, "also print the inferred tool surface per entity")
	return c
}

// printInspection renders the tree-like output defined in the spec. The
// rendering is intentionally stable so the CLI integration test can match
// substrings without reacting to layout churn.
func printInspection(w io.Writer, doc *model.Document, insp *model.Inspection, inferTools bool) error {
	writeErr := error(nil)
	writef := func(format string, args ...any) bool {
		if writeErr != nil {
			return false
		}
		_, writeErr = fmt.Fprintf(w, format, args...)
		return writeErr == nil
	}
	writeln := func(args ...any) bool {
		if writeErr != nil {
			return false
		}
		_, writeErr = fmt.Fprintln(w, args...)
		return writeErr == nil
	}

	writef("domain: %s (v%s)\n", insp.Domain, insp.Version)
	writef("entities (%d):\n", insp.EntityCount)

	// Compute a single column width for name alignment; keeps the output
	// readable for long and short schemas alike.
	nameWidth := 0
	for _, e := range insp.Entities {
		if len(e.Name) > nameWidth {
			nameWidth = len(e.Name)
		}
	}
	for _, e := range insp.Entities {
		deps := "[]"
		if len(e.Dependencies) > 0 {
			deps = "[" + strings.Join(e.Dependencies, ", ") + "]"
		}
		vol := renderVolume(e.VolumePlan)
		writef("  %-*s  fields=%d  deps=%s volume=%s\n", nameWidth, e.Name, e.FieldCount, deps, vol)
	}

	if len(insp.Enums) > 0 {
		parts := make([]string, 0, len(insp.Enums))
		for _, e := range insp.Enums {
			parts = append(parts, fmt.Sprintf("%s(%d)", e.Name, len(e.Variants)))
		}
		writef("enums (%d): %s\n", len(insp.Enums), strings.Join(parts, " "))
	}

	if len(insp.Rules) > 0 {
		writef("rules (%d):\n", len(insp.Rules))
		for _, r := range insp.Rules {
			writef("  - %s %s\n", r.Expr, severityTag(r.Severity))
		}
	}

	if inferTools {
		writeln("tools:")
		doc.Entities.Each(func(name string, ent *model.Entity) bool {
			return writef("  %s: %s\n", name, strings.Join(datjit.InferToolSurface(ent), ", "))
		})
	}
	return writeErr
}

// renderVolume formats a VolumeSpec for the inspect output. Range volumes
// are rendered as "min..max"; a zero spec falls back to the conventional
// default marker.
func renderVolume(v model.VolumeSpec) string {
	if v.IsRange() {
		return fmt.Sprintf("%d..%d", v.Min, v.Max)
	}
	return fmt.Sprintf("%d", v.Exact)
}

// severityTag maps RuleSeverity to the decorator form used in source
// schemas so the inspect output is copy-paste friendly.
func severityTag(s model.RuleSeverity) string {
	switch s {
	case model.RuleProbabilistic:
		return "@prob"
	case model.RuleWarn:
		return "@warn"
	default:
		// other severities fall through to the trailing return
	}
	return "@strict"
}

func inferToolSurface(ent *model.Entity) []string {
	return datjit.InferToolSurface(ent)
}
