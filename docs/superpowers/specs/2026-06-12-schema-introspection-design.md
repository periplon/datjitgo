# Schema introspection: export, diff, dependency graph

Status: approved for autonomous implementation
Date: 2026-06-12
Implements: `docs/enhancements.md` item #2 (round-1), killer-list #8 in
`docs/enhancements-round2.md`.

## 1. Goal

Read-only introspection over a parsed `*model.Document`:

1. **Export** a machine-readable schema signature (for committing as a CI
   drift fixture).
2. **Diff** two signatures, classifying changes as breaking or compatible.
3. **Dependency graph** of entity references, including cycle reporting with
   exemplar paths.

No generation, no RNG, no output-format changes. Purely additive API.

## 2. Library surface (additive)

New types in `core/model` (no adapter imports — hexagonal-safe):

```go
// SchemaSummary is a stable, ordered, machine-readable schema signature.
type SchemaSummary struct {
    Domain   string
    Version  string
    Locale   string
    Entities []EntitySummary   // document order
    Enums    []EnumSummary     // sorted by name
    Rules    []string          // canonical rule strings, document order
    Volumes  []VolumeSummary   // sorted by entity name
}

type EntitySummary struct {
    Name   string
    Fields []FieldSummary // declaration order
}

type FieldSummary struct {
    Name       string
    Type       string   // canonical type string, e.g. "->User | ->Org", "int"
    Decorators []string // canonical decorator strings, e.g. "@unique", "@range(18..65)"
}

type EnumSummary struct{ Name string; Variants []string }
type VolumeSummary struct{ Entity string; Spec string }

// SchemaDiff is the comparison of two summaries.
type SchemaDiff struct {
    Changes []SchemaChange
}
func (d *SchemaDiff) Breaking() bool   // any breaking change
func (d *SchemaDiff) Empty() bool

type SchemaChange struct {
    Kind     string // "entity-added" | "entity-removed" | "field-added" |
                    // "field-removed" | "field-type-changed" |
                    // "field-decorators-changed" | "enum-added" | "enum-removed" |
                    // "enum-variants-changed" | "volume-changed" | "rule-added" |
                    // "rule-removed" | "domain-changed"
    Entity   string
    Field    string
    Old, New string
    Breaking bool
}

// DependencyGraph describes entity reference structure.
type DependencyGraph struct {
    Nodes  []string    // entity names, document order
    Edges  []DepEdge
    Cycles [][]string  // each cycle as an entity path, e.g. ["A","B","A"]
}

type DepEdge struct {
    From, To string
    Field    string // referencing field name
    Kind     string // "reference" | "many-to-many" | "polymorphic" | "self"
}
```

Root `datjit` facade methods (additive):

```go
func (s *Service) SchemaSummary(doc *model.Document) *model.SchemaSummary
func (s *Service) DependencyGraph(doc *model.Document) *model.DependencyGraph
func DiffSchemaSummaries(old, new *model.SchemaSummary) *model.SchemaDiff
```

Canonical strings: type and decorator rendering must be deterministic and
round-trip-stable (same input doc → same strings). Reuse/extend whatever
string rendering `inspect` already uses; if none exists, add unexported
renderers in the root package (NOT in `core/model` if they need parser
knowledge — keep `core/model` dependency-free; pure struct-to-string is fine
in model).

Breaking classification: entity-removed, field-removed, field-type-changed,
enum-removed, enum variant removed, domain-changed are **breaking**;
additions, volume changes, decorator changes, variant additions are
**compatible** (decorator changes can alter generated values but not the
consumer-visible shape; note this in godoc).

## 3. CLI surface

New `datjit schema` command group (Cobra, `cmd_schema.go`):

```
datjit schema export <schema.yaml> [-o out] [--format json|yaml]   # default json, pretty
datjit schema diff <old.yaml|old.json> <new.yaml|new.json>
                   [--format text|json] [--strict]
datjit schema deps <schema.yaml> [--format text|dot]
```

- `export` emits the SchemaSummary; JSON keys lower_snake; output ordering
  deterministic.
- `diff` accepts either a YAML schema (parsed then summarized) or a
  previously exported JSON summary (detected by extension `.json` or by
  sniffing the first non-space byte `{`). `--strict` exits 1 when
  `diff.Breaking()`. Text format: one line per change,
  `[breaking] field-removed User.email (was: string)` style. Empty diff →
  "no changes", exit 0.
- `deps` text format: `A -> B (field, kind)` lines plus a `cycles:` section;
  dot format emits valid Graphviz (`digraph schema { ... }`).

Exit codes: 0 ok, 1 breaking-with-strict or error, 2 usage.

## 4. Cycle paths

`core/plan`'s topological sort already detects cycles. Extend it (internal,
behavior-preserving for the success path) to return one exemplar cycle path
per strongly-connected component with ≥1 cycle; `DependencyGraph.Cycles`
surfaces them, and the existing cyclic-dependency validation error message
gains the path (`cyclic dependency: A -> B -> A`). Self-references stay
excluded from cycle detection (they are valid today).

## 5. Tests

- Unit: summary construction over a fixture-rich doc (types, decorators,
  enums, polymorphic refs, `_indexes` ignored or summarized — pick ignored,
  documented); diff classification table-driven; graph edges incl.
  polymorphic (one edge per union target) and `<->` (kind many-to-many).
- Determinism: export twice → identical bytes.
- CLI: command-level tests matching existing `cmd_*_test.go` patterns.
- Godoc example for `DiffSchemaSummaries`.
- Goldens: untouched (feature is read-only). `make ci` must pass without
  `make test-update`.

## 6. Non-goals

- No signature hashing/versioning, no `--watch`, no index summarization,
  no rename detection in diff (removal+addition is fine).
