package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jmcarbo/datjitgo"
	"github.com/jmcarbo/datjitgo/core/model"
)

// cmdGenerate wires the `datjit generate` subcommand.
func cmdGenerate() *cobra.Command {
	var (
		output     string
		format     string
		seed       int64
		seedSet    bool
		locale     string
		volume     []string
		entity     string
		sqlDialect string
		pretty     bool
		dryRun     bool
	)

	c := &cobra.Command{
		Use:          "generate <schema.yaml>",
		Short:        "Generate a dataset from a DDL schema",
		SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usageErrorf("generate requires exactly one schema path (got %d)", len(args))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Capture whether --seed was explicitly set so we distinguish
			// zero-value from "use the document's seed".
			seedSet = cmd.Flags().Changed("seed")

			volumeMap, err := parseVolumeFlags(volume)
			if err != nil {
				return &usageErr{err: err}
			}

			opts := []datjit.Option{}
			if seedSet {
				opts = append(opts, datjit.WithSeed(seed))
			}
			if locale != "" {
				opts = append(opts, datjit.WithLocale(locale))
			}
			if len(volumeMap) > 0 {
				opts = append(opts, datjit.WithVolume(volumeMap))
			}

			svc, err := datjit.New(opts...)
			if err != nil {
				return err
			}

			// Bail early on unknown format so users get a usage-class error
			// before paying the parse cost.
			if !formatSupported(svc, format) {
				return &usageErr{err: fmt.Errorf("unknown format %q (available: %s)", format, strings.Join(svc.Formats(), ", "))}
			}

			doc, err := parseSchemaFile(svc, args[0])
			if err != nil {
				return err
			}
			if err := svc.Validate(doc); err != nil {
				return err
			}

			if dryRun {
				return printGeneratePlan(cmd.OutOrStdout(), doc, volumeMap)
			}

			ds, err := svc.Generate(doc)
			if err != nil {
				return err
			}

			w, closer, err := openOutput(output)
			if err != nil {
				return err
			}
			defer func() { _ = closer() }()

			wo := datjit.WriteOpts{
				Pretty:       pretty,
				SQLDialect:   sqlDialect,
				EntityFilter: entity,
			}
			return svc.Write(ds, doc, format, w, wo)
		},
	}

	c.Flags().StringVarP(&output, "output", "o", "", "output path (default: stdout). Use '-' for stdout explicitly.")
	c.Flags().StringVarP(&format, "format", "f", "json", "output format (json|ndjson|csv|yaml|sql)")
	c.Flags().Int64Var(&seed, "seed", 0, "override document seed")
	c.Flags().StringVar(&locale, "locale", "", "override locale (BCP47, e.g. en-US)")
	c.Flags().StringSliceVar(&volume, "volume", nil, "per-entity volume override, e.g. User=100,Org=5")
	c.Flags().StringVar(&entity, "entity", "", "emit only rows for this entity")
	c.Flags().StringVar(&sqlDialect, "sql-dialect", "postgres", "SQL dialect (postgres|mysql|sqlite) — used with -f sql")
	c.Flags().BoolVar(&pretty, "pretty", false, "emit human-friendly output where the format supports it")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "parse and validate only; print the plan and exit 0")

	return c
}

// parseVolumeFlags converts the repeated --volume arguments into a single
// map. Each token must be of the form Name=Int; malformed entries return an
// error (the caller wraps it as usageErr so the process exits 2).
func parseVolumeFlags(items []string) (map[string]int, error) {
	if len(items) == 0 {
		return nil, nil
	}
	out := map[string]int{}
	for _, raw := range items {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		eq := strings.IndexByte(raw, '=')
		if eq <= 0 || eq == len(raw)-1 {
			return nil, fmt.Errorf("invalid --volume %q (expected Name=N)", raw)
		}
		name := strings.TrimSpace(raw[:eq])
		valStr := strings.TrimSpace(raw[eq+1:])
		n, err := strconv.Atoi(valStr)
		if err != nil {
			return nil, fmt.Errorf("invalid --volume %q: %w", raw, err)
		}
		if n < 0 {
			return nil, fmt.Errorf("invalid --volume %q: negative count", raw)
		}
		out[name] = n
	}
	return out, nil
}

// parseSchemaFile opens and parses path via the façade. Open errors are
// reported with the path for clarity.
func parseSchemaFile(svc *datjit.Service, path string) (*model.Document, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	return svc.Parse(f, path)
}

// openOutput returns the io.Writer for the chosen --output path plus a
// closer callback that the caller defers. An empty path or "-" means
// stdout; for a real path the file is created/truncated.
func openOutput(path string) (io.Writer, func() error, error) {
	if path == "" || path == "-" {
		return os.Stdout, func() error { return nil }, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, func() error { return nil }, fmt.Errorf("create %s: %w", path, err)
	}
	return f, f.Close, nil
}

// formatSupported checks whether the chosen format identifier is registered
// with the service. We keep this case-sensitive to match the writers map.
func formatSupported(svc *datjit.Service, f string) bool {
	for _, name := range svc.Formats() {
		if name == f {
			return true
		}
	}
	return false
}

// printGeneratePlan renders the --dry-run summary. It reports per-entity
// volume (after override) so users can sanity-check large runs before
// paying the generation cost.
func printGeneratePlan(w io.Writer, doc *model.Document, override map[string]int) error {
	type row struct {
		name  string
		count int
	}
	entities := make([]row, 0, doc.Entities.Len())
	totalEntities := 0
	totalRows := 0
	doc.Entities.Each(func(name string, _ *model.Entity) bool {
		totalEntities++
		count := plannedVolume(name, doc, override)
		totalRows += count
		entities = append(entities, row{name: name, count: count})
		return true
	})
	// Sort by entity name for stable output irrespective of declaration
	// order — only used for the itemised totals below.
	sorted := make([]row, len(entities))
	copy(sorted, entities)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].name < sorted[j].name })

	parts := make([]string, 0, len(sorted))
	for _, r := range sorted {
		parts = append(parts, fmt.Sprintf("%s=%d", r.name, r.count))
	}
	fmt.Fprintf(w, "plan: %d entities, totals: %s (rows=%d)\n",
		totalEntities, strings.Join(parts, ", "), totalRows)
	return nil
}

// plannedVolume reproduces the façade's volume precedence without running
// Generate: flag override > document volume > default 10 (kept in sync
// with inspect.defaultVolume).
func plannedVolume(name string, doc *model.Document, override map[string]int) int {
	if v, ok := override[name]; ok {
		return v
	}
	if v, ok := doc.Volume[name]; ok {
		if v.Exact > 0 {
			return v.Exact
		}
		if v.Max > 0 {
			// Use the upper bound as the displayed count; it's the
			// worst-case row budget callers usually care about.
			return v.Max
		}
	}
	return 10
}
