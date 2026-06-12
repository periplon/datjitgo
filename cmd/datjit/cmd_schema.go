package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	datjit "github.com/periplon/datjitgo"
	"github.com/periplon/datjitgo/core/model"
)

// cmdSchema wires the `datjit schema` command group: export, diff, and deps.
// All three are read-only introspection over a parsed document — no generation.
func cmdSchema() *cobra.Command {
	c := &cobra.Command{
		Use:          "schema",
		Short:        "Schema introspection: export, diff, dependency graph",
		SilenceUsage: true,
	}
	c.AddCommand(cmdSchemaExport(), cmdSchemaDiff(), cmdSchemaDeps())
	return c
}

// cmdSchemaExport emits a SchemaSummary signature for a schema, defaulting to
// pretty JSON. The output is deterministic so it can be committed as a CI
// drift fixture.
func cmdSchemaExport() *cobra.Command {
	var (
		output string
		format string
	)
	c := &cobra.Command{
		Use:          "export <schema.yaml>",
		Short:        "Export a machine-readable schema signature",
		SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usageErrorf("schema export requires exactly one schema path (got %d)", len(args))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "json" && format != "yaml" {
				return usageErrorf("unknown --format %q (want json|yaml)", format)
			}
			svc := datjit.NewDefault()
			doc, err := parseSchemaFile(svc, args[0])
			if err != nil {
				return err
			}
			sum := svc.SchemaSummary(doc)

			data, err := encodeSummary(sum, format)
			if err != nil {
				return err
			}
			w, closer, err := openOutput(output)
			if err != nil {
				return err
			}
			defer func() { _ = closer() }()
			_, err = w.Write(data)
			return err
		},
	}
	c.Flags().StringVarP(&output, "output", "o", "", "output path (default: stdout)")
	c.Flags().StringVar(&format, "format", "json", "output format (json|yaml)")
	return c
}

// cmdSchemaDiff compares two schemas (or previously exported JSON summaries)
// and reports breaking/compatible changes. With --strict it exits 1 when any
// breaking change is present.
func cmdSchemaDiff() *cobra.Command {
	var (
		format string
		strict bool
	)
	c := &cobra.Command{
		Use:          "diff <old> <new>",
		Short:        "Diff two schemas or exported summaries",
		SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 2 {
				return usageErrorf("schema diff requires exactly two paths (got %d)", len(args))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "text" && format != "json" {
				return usageErrorf("unknown --format %q (want text|json)", format)
			}
			svc := datjit.NewDefault()
			oldSum, err := loadSummary(svc, args[0])
			if err != nil {
				return err
			}
			newSum, err := loadSummary(svc, args[1])
			if err != nil {
				return err
			}
			diff := datjit.DiffSchemaSummaries(oldSum, newSum)

			if err := printDiff(cmd.OutOrStdout(), diff, format); err != nil {
				return err
			}
			if strict && diff.Breaking() {
				return fmt.Errorf("breaking changes detected")
			}
			return nil
		},
	}
	c.Flags().StringVar(&format, "format", "text", "output format (text|json)")
	c.Flags().BoolVar(&strict, "strict", false, "exit 1 when breaking changes are present")
	return c
}

// cmdSchemaDeps prints the entity dependency graph, as text or Graphviz dot.
func cmdSchemaDeps() *cobra.Command {
	var format string
	c := &cobra.Command{
		Use:          "deps <schema.yaml>",
		Short:        "Print the entity dependency graph",
		SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usageErrorf("schema deps requires exactly one schema path (got %d)", len(args))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "text" && format != "dot" {
				return usageErrorf("unknown --format %q (want text|dot)", format)
			}
			svc := datjit.NewDefault()
			doc, err := parseSchemaFile(svc, args[0])
			if err != nil {
				return err
			}
			g := svc.DependencyGraph(doc)
			return printDeps(cmd.OutOrStdout(), g, format)
		},
	}
	c.Flags().StringVar(&format, "format", "text", "output format (text|dot)")
	return c
}

// encodeSummary serializes a SchemaSummary as pretty JSON or YAML. JSON keys
// use lower_snake (set by the struct tags) and a trailing newline is added.
func encodeSummary(sum *model.SchemaSummary, format string) ([]byte, error) {
	switch format {
	case "yaml":
		return marshalSummaryYAML(sum)
	default:
		return marshalJSONNoEscape(sum)
	}
}

// marshalSummaryYAML serializes a SchemaSummary as YAML while preserving the
// JSON (lower_snake) key names: it round-trips through JSON into a generic
// structure so the YAML keys match the exported JSON exactly.
func marshalSummaryYAML(sum *model.SchemaSummary) ([]byte, error) {
	j, err := json.Marshal(sum)
	if err != nil {
		return nil, err
	}
	var generic any
	if err := json.Unmarshal(j, &generic); err != nil {
		return nil, err
	}
	return yaml.Marshal(generic)
}

// marshalJSONNoEscape renders v as pretty JSON without HTML escaping, so
// type strings like "->User" stay readable in committed drift fixtures. A
// trailing newline is included.
func marshalJSONNoEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// loadSummary resolves path to a SchemaSummary. A path ending in .json is
// decoded as a previously exported summary; otherwise a leading '{' triggers a
// summary-decode attempt with fallback to YAML schema parsing (flow-style YAML
// schemas also start with '{').
func loadSummary(svc *datjit.Service, path string) (*model.SchemaSummary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	if strings.HasSuffix(strings.ToLower(path), ".json") {
		var sum model.SchemaSummary
		if err := json.Unmarshal(data, &sum); err != nil {
			return nil, fmt.Errorf("parse summary %s: %w", path, err)
		}
		return &sum, nil
	}
	if startsWithBrace(data) {
		// Could be an exported summary or a flow-style YAML schema; try the
		// summary first and fall through to schema parsing if it doesn't fit.
		var sum model.SchemaSummary
		if err := json.Unmarshal(data, &sum); err == nil {
			return &sum, nil
		}
	}
	doc, err := svc.Parse(strings.NewReader(string(data)), path)
	if err != nil {
		return nil, err
	}
	return svc.SchemaSummary(doc), nil
}

// startsWithBrace reports whether data's first non-space byte is '{'.
func startsWithBrace(data []byte) bool {
	for _, b := range data {
		switch b {
		case ' ', '\t', '\r', '\n':
			continue
		case '{':
			return true
		default:
			return false
		}
	}
	return false
}

// printDiff renders a SchemaDiff as text (one line per change) or JSON. An
// empty diff prints "no changes" in text mode.
func printDiff(w io.Writer, diff *model.SchemaDiff, format string) error {
	if format == "json" {
		data, err := marshalJSONNoEscape(diff)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}
	if diff.Empty() {
		_, err := fmt.Fprintln(w, "no changes")
		return err
	}
	for _, c := range diff.Changes {
		if _, err := fmt.Fprintln(w, formatChangeLine(c)); err != nil {
			return err
		}
	}
	return nil
}

// formatChangeLine renders one SchemaChange as a text line, e.g.
// "[breaking] field-removed User.email (was: string)".
func formatChangeLine(c model.SchemaChange) string {
	var b strings.Builder
	if c.Breaking {
		b.WriteString("[breaking] ")
	} else {
		b.WriteString("[ok] ")
	}
	b.WriteString(c.Kind)
	if loc := changeLocation(c); loc != "" {
		b.WriteByte(' ')
		b.WriteString(loc)
	}
	if detail := changeDetail(c); detail != "" {
		b.WriteByte(' ')
		b.WriteString(detail)
	}
	return b.String()
}

// changeLocation formats the Entity[.Field] locator for a change, if any.
func changeLocation(c model.SchemaChange) string {
	if c.Entity == "" {
		return ""
	}
	if c.Field != "" {
		return c.Entity + "." + c.Field
	}
	return c.Entity
}

// changeDetail formats the old/new value annotation for a change line.
func changeDetail(c model.SchemaChange) string {
	switch {
	case c.Old != "" && c.New != "":
		return "(" + c.Old + " -> " + c.New + ")"
	case c.New != "":
		return "(now: " + c.New + ")"
	case c.Old != "":
		return "(was: " + c.Old + ")"
	default:
		return ""
	}
}

// printDeps renders a DependencyGraph as text or Graphviz dot.
func printDeps(w io.Writer, g *model.DependencyGraph, format string) error {
	if format == "dot" {
		return printDepsDot(w, g)
	}
	return printDepsText(w, g)
}

// printDepsText renders the graph as "A -> B (field, kind)" lines followed by
// a cycles section.
func printDepsText(w io.Writer, g *model.DependencyGraph) error {
	for _, e := range g.Edges {
		if _, err := fmt.Fprintf(w, "%s -> %s (%s, %s)\n", e.From, e.To, e.Field, e.Kind); err != nil {
			return err
		}
	}
	if len(g.Cycles) == 0 {
		_, err := fmt.Fprintln(w, "cycles: none")
		return err
	}
	if _, err := fmt.Fprintln(w, "cycles:"); err != nil {
		return err
	}
	for _, cyc := range g.Cycles {
		if _, err := fmt.Fprintf(w, "  %s\n", strings.Join(cyc, " -> ")); err != nil {
			return err
		}
	}
	return nil
}

// printDepsDot renders the graph as a valid Graphviz digraph.
func printDepsDot(w io.Writer, g *model.DependencyGraph) error {
	if _, err := fmt.Fprintln(w, "digraph schema {"); err != nil {
		return err
	}
	for _, n := range g.Nodes {
		if _, err := fmt.Fprintf(w, "  %q;\n", n); err != nil {
			return err
		}
	}
	for _, e := range g.Edges {
		if _, err := fmt.Fprintf(w, "  %q -> %q [label=%q];\n", e.From, e.To, e.Field+" ("+e.Kind+")"); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w, "}")
	return err
}
