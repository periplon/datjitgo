package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jmcarbo/datjitgo"
	"github.com/jmcarbo/datjitgo/core/model"
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
	fmt.Fprintf(w, "domain: %s (v%s)\n", insp.Domain, insp.Version)
	fmt.Fprintf(w, "entities (%d):\n", insp.EntityCount)

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
		fmt.Fprintf(w, "  %-*s  fields=%d  deps=%s volume=%s\n",
			nameWidth, e.Name, e.FieldCount, deps, vol)
	}

	if len(insp.Enums) > 0 {
		parts := make([]string, 0, len(insp.Enums))
		for _, e := range insp.Enums {
			parts = append(parts, fmt.Sprintf("%s(%d)", e.Name, len(e.Variants)))
		}
		fmt.Fprintf(w, "enums (%d): %s\n", len(insp.Enums), strings.Join(parts, " "))
	}

	if len(insp.Rules) > 0 {
		fmt.Fprintf(w, "rules (%d):\n", len(insp.Rules))
		for _, r := range insp.Rules {
			fmt.Fprintf(w, "  - %s %s\n", r.Expr, severityTag(r.Severity))
		}
	}

	if inferTools {
		fmt.Fprintln(w, "tools:")
		doc.Entities.Each(func(name string, ent *model.Entity) bool {
			fmt.Fprintf(w, "  %s: %s\n", name, strings.Join(inferToolSurface(ent), ", "))
			return true
		})
	}
	return nil
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
	}
	return "@strict"
}

// inferToolSurface produces the phase-1 auto-inferred tool list for an
// entity based on its meta decorators:
//   - @readonly  → list, get
//   - @immutable → list, get, create
//   - default    → list, get, create, update, delete
func inferToolSurface(ent *model.Entity) []string {
	if model.HasDecorator(ent.Meta, "readonly") {
		return []string{"list", "get"}
	}
	if model.HasDecorator(ent.Meta, "immutable") {
		return []string{"list", "get", "create"}
	}
	return []string{"list", "get", "create", "update", "delete"}
}
