# datjitgo — Go port of datjit (Phase 1)

Status: design approved, autonomous build authorized
Date: 2026-04-22
Module path: `github.com/jmcarbo/datjitgo`

## 1. Goal

Port the Rust `datjit` synthetic-data generator to Go as:
1. A reusable library other Go projects can embed (hexagonal + SOLID).
2. A CLI binary with a REPL for interactive use plus one-shot subcommands matching the Rust CLI (`generate`, `validate`, `inspect`, `corpus`, `repl`).

Phase 1 scope: core + parser + generator + output + embedded corpus + REPL/CLI. Out of scope (deferred to phases 2–3): live corpus downloaders, LLM-backed generation (`@llm`, `@llm_values`). Parser still tolerates LLM decorators but generator emits `ErrFeatureDeferred` when a schema actually uses them, so fixtures parse cleanly.

## 2. Architecture (hexagonal)

```
             ┌────────────────────────┐
   cmd/      │   cmd/datjit (CLI)     │
   main      │   Cobra + REPL driver  │
             └──────────┬─────────────┘
                        │ uses facade
             ┌──────────▼─────────────┐
   app       │   datjit.Service       │   (facade: wires adapters, exposes Generate/Validate/Inspect)
             └──┬──────┬──────┬───────┘
                │      │      │
    ports ─────►│ Parser│ Generator │ Writer │ CorpusProvider  (interfaces in core/ports)
                │      │      │
             ┌──▼───┐ ┌▼────────┐ ┌▼──────┐ ┌───────────┐
  adapters   │parser│ │generator│ │output │ │  corpus   │
             └──────┘ └─────────┘ └───────┘ └───────────┘
                        ▲
                        │ reads
                    core/model (Document, Entity, Field, TypeExpr, Decorator, Value, Rule)
                    core/errors
```

Dependency rule: adapters import `core`; `core` imports nothing internal. `datjit.Service` (facade at module root) imports adapters and wires them via the port interfaces — consumers can also build their own `Service` with custom adapters (e.g. a different `CorpusProvider`).

## 3. Package layout

```
datjitgo/
├── go.mod                              module github.com/jmcarbo/datjitgo
├── datjit.go                           Service facade + NewDefault()
├── options.go                          Option funcs (WithParser, WithCorpus, WithSeed, …)
├── core/
│   ├── model/                          Document, Entity, Field, TypeExpr, Decorator, EnumDef, Rule, VolumeSpec
│   ├── value/                          Value (tagged union), helpers
│   ├── ports/                          Parser, Generator, Writer, CorpusProvider, Randomizer
│   └── errors.go                       Err* sentinels + typed Error struct
├── parser/                             adapter: YAML → Document
│   ├── yaml.go                         top-level doc parser
│   ├── types.go                        recursive-descent type expression parser
│   └── decorators.go                   decorator tokenizer + parser
├── generator/                          adapter: Document → map[Entity][]Row
│   ├── engine.go                       orchestrator
│   ├── plan.go                         topological sort (Kahn)
│   ├── field.go                        single-field dispatch
│   ├── primitive.go                    primitive generators
│   ├── semantic.go                     semantic type dispatch (consults CorpusProvider)
│   ├── pattern.go                      `@pattern` template expansion
│   ├── distribution.go                 uniform/normal/lognormal/exp/geom/zipf/bimodal/weighted
│   ├── coherence.go                    coherence groups + @from
│   ├── derived.go                      expression AST + evaluator for @derived/@compute/@default_chain
│   ├── constraint.go                   uniqueness retry loop, range checks, rules
│   └── rng.go                          deterministic Rand wrapper (seed → per-entity/per-field substreams)
├── output/                             adapter: rows → bytes/stream
│   ├── json.go
│   ├── csv.go
│   ├── sql.go
│   ├── yaml.go
│   └── ndjson.go
├── corpus/                             adapter: embedded + optional on-disk overlay
│   ├── provider.go                     CorpusProvider impl
│   ├── embedded.go                     `//go:embed data/*.json`
│   └── data/                           embedded JSON corpora (names, emails, cities, …)
├── repl/                               REPL subsystem (importable too)
│   ├── repl.go                         session loop
│   ├── commands.go                     command registry
│   └── completer.go                    tab completion
├── cmd/datjit/                         CLI main (Cobra)
│   └── main.go
├── testdata/
│   ├── fixtures/                       YAML fixtures mirrored from Rust
│   └── golden/                         expected outputs for golden tests
└── docs/
    └── superpowers/specs/              this document + future specs
```

### 3.1 Package boundaries (what does each do, what does each depend on)

| Package | Purpose | Deps |
|---|---|---|
| `core/model` | plain structs, no behavior beyond accessors | none |
| `core/value` | `Value` tagged-union + JSON marshal | `core/model` |
| `core/ports` | interfaces | `core/model`, `core/value` |
| `core/errors` | `Error` struct + `Err*` sentinels | stdlib |
| `parser` | YAML → `Document`; pure, no I/O of its own beyond accepting `io.Reader` | `core/*`, `yaml.v3` |
| `generator` | `Document` → entity rows; depends on `CorpusProvider` injected at construction | `core/*` |
| `output` | rows → bytes via `io.Writer` | `core/*`, stdlib |
| `corpus` | built-in semantic-data provider | `core/ports`, `embed` |
| `repl` | interactive session over a `Service` | root facade |
| `cmd/datjit` | CLI entrypoint | Cobra + root facade |

### 3.2 Facade API (root `datjit` package)

```go
package datjit

type Service struct { /* unexported adapters */ }

func NewDefault() *Service               // wires default adapters
func New(opts ...Option) (*Service, error)

// Options (in options.go) wire alternative implementations:
func WithParser(p ports.Parser) Option
func WithGenerator(g ports.Generator) Option
func WithWriter(format string, w ports.Writer) Option
func WithCorpus(c ports.CorpusProvider) Option
func WithSeed(seed int64) Option
func WithLocale(loc string) Option
func WithVolume(vols map[string]int) Option

// Primary operations
func (s *Service) Parse(r io.Reader) (*model.Document, error)
func (s *Service) Validate(doc *model.Document) error
func (s *Service) Inspect(doc *model.Document) (*model.Inspection, error)
func (s *Service) Generate(doc *model.Document) (*model.Dataset, error)
func (s *Service) Write(ds *model.Dataset, format string, w io.Writer) error

// Convenience
func (s *Service) GenerateFile(path, format string, w io.Writer) error
```

Library consumers interact only with `datjit.Service` (plus `core/model`, `core/ports`, `core/errors` for types). Everything else is implementation detail.

## 4. Core domain model

Ported literally from Rust. Key types in `core/model`:

```go
type Document struct {
    Domain     string
    Version    string
    Seed       *int64
    Locale     string
    Volume     map[string]VolumeSpec
    Entities   *OrderedMap[string, Entity]     // insertion-ordered
    Enums      map[string]EnumDef
    Types      map[string]Entity               // reusable compound types
    Rules      []Rule
    Tools      map[string]ToolOverride
    Generation GenerationConfig                // seed/locale/locales/llm
}

type Entity struct {
    Name     string
    Meta     []Decorator
    Fields   *OrderedMap[string, Field]
    Coherence map[string][]string              // group name → field names
}

type Field struct {
    Name        string
    Type        TypeExpr
    Decorators  []Decorator
    Label       string
    Description string
    DefaultChain *DefaultChainSpec
    Compute      []ComputeBranch
}

type TypeExpr interface{ isTypeExpr() }        // sealed via unexported method
// implementations: Primitive, Semantic, EnumType (inline), NamedType,
// Reference, Compound (List/Map/Tuple/Nullable/Union)

type Decorator struct {
    Name string
    Args []DecoratorArg                        // literal | range | kv | dist-spec
}
```

`OrderedMap` is a tiny generic wrapper (`map[K]V` + ordered key slice) to preserve YAML insertion order — the Rust code uses `IndexMap` for the same reason; deterministic output depends on it.

`value.Value` mirrors `serde_json::Value` + `Decimal`/`Time` variants:

```go
type Value struct{ /* kind + payload */ }
func Null() Value
func Bool(b bool) Value
func Int(i int64) Value
func Float(f float64) Value
func Str(s string) Value
func UUID(u uuid.UUID) Value
func Time(t time.Time) Value
func Dec(d decimal.Decimal) Value
func List(xs []Value) Value
func Object(m *OrderedMap[string, Value]) Value
```

## 5. Port interfaces (core/ports)

```go
type Parser interface {
    Parse(r io.Reader) (*model.Document, error)
}

type Generator interface {
    Generate(doc *model.Document, opts GenerateOptions) (*model.Dataset, error)
}

type Writer interface {
    Format() string                             // e.g. "json", "csv"
    Write(ds *model.Dataset, w io.Writer, opts WriteOptions) error
}

type CorpusProvider interface {
    Sample(ctx SampleContext, key string) (value.Value, error)  // weighted random
    List(key string) ([]CorpusEntry, error)
    Has(key string) bool
    Locales() []string
}

// Randomizer isolates RNG for testing and for alt-determinism strategies.
type Randomizer interface {
    Substream(scope string) Randomizer           // derive child RNG deterministically
    Float() float64
    IntN(n int64) int64
    Shuffle(n int, swap func(i, j int))
}
```

Each adapter implements exactly one port. Consumers can swap any of them via `datjit.With*` options — the library is OCP over all four axes.

## 6. Generation pipeline

Mirrors the Rust engine (datjit-generator/src/engine.rs):

1. **Plan**: topological sort of entities via Kahn's algorithm; self-refs excluded; ties broken by document order for determinism.
2. For each entity in plan order, generate N rows (`N` = `Document.Volume[entity]`, overridable):
   1. Allocate per-entity RNG substream (`rng.Substream(entityName)`).
   2. Resolve coherence groups: pick a coherence "anchor" per group (e.g. locale/region for `identity`), then generate the group together.
   3. Generate non-derived fields in declaration order:
      - `@primary`/`@auto` first.
      - Regular fields via `field.Generate`, which dispatches on `TypeExpr` and then applies decorators in a canonical order (range → pattern → null_rate → unique).
   4. Enforce `@unique` via retry loop (max 100 attempts per value; on exhaustion → `ErrUniquenessExhausted`).
   5. Evaluate `@derived` expressions (AST evaluator with the functions from spec §3.5.1).
   6. Evaluate `@default_chain`, then `@compute` (spec §3.5.2/§3.5.3).
   7. Apply `@timestamps` entity decorator.
   8. Strip `@internal` fields before storing.
3. Validate `rules[]` (§6 of spec). `@strict` must hold → retry row up to 10×; `@probability(p)` biases generation but doesn't hard-fail; `@warn` logs.

### 6.1 Determinism strategy

Rust uses `rand_chacha` with a seeded stream; Go can't reproduce its byte stream. Instead, datjitgo defines its own deterministic model:

- Top-level seed → `math/rand/v2.PCG` instance.
- `Substream(scope string)` returns a new PCG seeded from `fnv64(parent.State, scope)`. Stable across Go versions because PCG is spec-stable.
- Entity/field/row/attempt each get their own substream. Result: changing one field's decorators doesn't shift unrelated fields' output — a property the Rust code also provides in practice via per-field RNG churn.

Guarantee: given the same `(document, seed, corpus)`, output bytes are identical across runs on any Go 1.22+ platform.

## 7. Parser

Three layers:
1. `yaml.go` parses the document skeleton with `yaml.v3`'s node API (preserves key order via `Node.Content`).
2. For each field value string like `uuid @primary`, `types.go` splits type from decorators using a brace/paren-aware tokenizer, then recursive-descent parses the type per the precedence ladder in Rust's `type_parser.rs`:
   Union → Nullable → Compound → Reference → Enum → Parameterized → Primitive/Semantic/Named.
3. `decorators.go` parses each `@dec(args)` chunk — same stateful tokenizer as Rust's `decorator_parser.rs` (tracks `(` depth so commas inside args don't split decorators).

Errors carry file/line/column (from `yaml.v3` node metadata) plus the original fragment for user-facing messages.

## 8. Output writers

Each writer implements `ports.Writer.Write(dataset, io.Writer, opts)`:
- **json**: `encoding/json`; `--pretty` → `MarshalIndent`. Entity order from document.
- **ndjson**: one entity per block, one JSON object per row, `\n`-separated.
- **csv**: `encoding/csv` per entity; header row = field names in declaration order.
- **sql**: `CREATE TABLE` + batched `INSERT`s, dialect switch `postgres|mysql|sqlite` (quote style, bool literal, type map).
- **yaml**: single YAML document via `yaml.v3` encoder with custom `Node` construction to preserve key order.

Writers do not share state; all parameters passed in `WriteOptions`.

## 9. Embedded corpus

`corpus/data/*.json` holds just enough data to run every fixture:

| Namespace | Files |
|---|---|
| person | `first_names.json`, `last_names.json`, `prefixes.json`, `bios.json` |
| address | `cities.json`, `states.json`, `streets.json`, `countries.json` |
| contact | `email_domains.json`, `phone_area_codes.json` |
| company | `company_names.json`, `industries.json`, `catch_phrases.json` |
| job | `titles.json`, `departments.json` |
| product | `titles.json`, `descriptions.json` |
| text | `words.json`, `lorem.json` |
| color | `names.json` |
| misc | `mimes.json`, `file_extensions.json`, `timezones.json` |

Entries use the `{name, weight}` schema from the Rust corpus (`weight` optional, default 1). Data is copied from the Rust embedded arrays at port time so behavior matches.

`CorpusProvider` two-tier fallback mirrors Rust: try `~/.datjit/corpus/<locale>/<key>.json`, else embedded. `XDG_DATA_HOME` is honored first if set.

## 10. REPL

Library `github.com/chzyer/readline` for line editing, history, Ctrl-C handling.

Commands:

```
load <path>                   # parse schema, hold in session
reload                        # re-parse current schema
show schema|entities|enums|rules|volume
set seed <int>
set locale <bcp47>
set format json|csv|sql|yaml|ndjson
set volume <Entity>=<N> [...]
set pretty on|off
set output <path>|stdout
set sql-dialect postgres|mysql|sqlite
generate [--entity <name>]    # uses current settings
validate
inspect [--infer-tools]
corpus list|info
help [<cmd>]
history
clear
exit | quit | Ctrl-D
```

Tab completion: commands, subcommands, loaded entity names, enum names, file paths (for `load`).
History: `$XDG_STATE_HOME/datjit/history` (fallback `~/.datjit_history`).
Output goes to the REPL's configured writer (default stdout); parse/validate errors pretty-print with source location.

The REPL is a thin view over `datjit.Service` — programmatic callers can embed `repl.New(service).Run(ctx, stdin, stdout)` in their own binary.

## 11. CLI

Cobra with subcommands mirroring Rust:

```
datjit generate <schema.yaml> [-o path] [-f json|csv|sql|yaml|ndjson]
                 [--seed N] [--locale bcp47]
                 [--volume Entity=N,…] [--entity name]
                 [--sql-dialect postgres|mysql|sqlite]
                 [--pretty] [--dry-run]

datjit validate <schema.yaml>
datjit inspect  <schema.yaml> [--infer-tools]
datjit corpus   list | info | update       # phase 1: update → "deferred" error
datjit repl     [<schema.yaml>]             # start REPL, optionally preload
datjit version
```

Exit codes: 0 success, 1 validation/generation error, 2 CLI usage error.

## 12. Error handling

Single typed error in `core/errors.go`:

```go
type Error struct {
    Kind     ErrorKind              // Parse, Validation, Generation, Uniqueness, Rule, IO, FeatureDeferred
    Entity   string
    Field    string
    Location *Location              // file, line, col
    Message  string
    Cause    error                  // wrapped underlying error
}
func (e *Error) Error() string
func (e *Error) Unwrap() error
```

Sentinels: `ErrParse`, `ErrValidation`, `ErrUniquenessExhausted`, `ErrRuleViolated`, `ErrFeatureDeferred`, `ErrCorpusMissing`, `ErrCyclicDependency`. All adapter functions return `*Error`; consumers use `errors.Is` / `errors.As`.

## 13. Testing strategy

Three tiers:

1. **Unit tests** per package (`*_test.go`), targeting Rust parity where a Rust test exists. Table-driven, stdlib `testing`, `github.com/google/go-cmp/cmp` for struct diffs.
2. **Fixture round-trip tests** in `testdata/fixtures/` (copied from Rust `tests/fixtures/`). For each fixture: parse → generate with seed 42 → compare against `testdata/golden/<fixture>.json`. UUID `id` fields stripped before compare (matches Rust test pattern). Golden files regeneratable via `go test ./... -update`.
3. **REPL integration test** drives the REPL with a scripted `io.Reader` and asserts on captured output.

Target: every fixture from the Rust tree has a matching Go golden file; every public function on `datjit.Service` has at least one unit test; coverage ≥ 80% on `core`, `parser`, `generator`, `output`.

CI (Phase 1 ships with a Makefile + `.github/workflows/ci.yml`): `go vet`, `staticcheck`, `golangci-lint`, `go test -race ./...`, `go test -run TestFixtures -update=false ./...`.

## 14. Dependencies

Minimal external deps:

| Module | Purpose |
|---|---|
| `gopkg.in/yaml.v3` | YAML parse/emit with node-level line info |
| `github.com/spf13/cobra` | CLI framework |
| `github.com/chzyer/readline` | REPL line editing |
| `github.com/google/uuid` | UUID v4 |
| `github.com/shopspring/decimal` | `decimal(p,s)` type |
| `github.com/google/go-cmp` | test diffs |

Standard library covers: JSON, CSV, regex, time, embed, `math/rand/v2` (PCG).

No cgo. No build tags. Go 1.22+ (for `math/rand/v2`).

## 15. Phased rollout inside Phase 1

Implementation order (each step green before next):

1. Module bootstrap, `core/model`, `core/value`, `core/errors`, `core/ports`.
2. `parser` + fixture round-trip of `minimal.yaml` and `primitives_and_params.yaml`.
3. `corpus` with embedded data.
4. `generator` primitives + distributions, `semantic_types.yaml` fixture green.
5. References, coherence, `enums_and_distributions.yaml`, `coherence_groups.yaml`, `references.yaml`.
6. `@pattern`, `@derived`, `@compute`, `@default_chain` → `derived_fields.yaml`, `compound_types.yaml`.
7. `rules`, named types, entity meta → `rules.yaml`, `named_types.yaml`, `entity_meta.yaml`.
8. `output` writers (json/ndjson/csv/yaml/sql).
9. `datjit.Service` facade + golden tests for every non-LLM fixture.
10. CLI (`cmd/datjit`) with `generate|validate|inspect`.
11. REPL + `datjit repl`.
12. CI, README, godoc polish, version tag `v0.1.0`.

## 16. Non-goals / deferred

- `@llm`, `@llm_values`, `generation.llm` — parser accepts, generator returns `ErrFeatureDeferred`.
- Live corpus updater downloads (`datjit corpus update`).
- Tool-inference code generation (`--infer-tools` prints the inferred surface; no codegen).
- Multi-locale corpus overlays beyond `en-US` embedded defaults.

Each deferred item becomes its own design doc under `docs/superpowers/specs/` before being implemented.

## 17. Success criteria

- `go install github.com/jmcarbo/datjitgo/cmd/datjit@latest` works.
- `datjit generate tests/fixtures/project_management.yaml --seed 42` produces non-empty JSON without errors.
- `datjit repl` starts, accepts `load`/`generate`, prints output, exits cleanly.
- Every non-LLM Rust fixture has a matching Go golden test that passes with seed 42.
- Library consumer can do:
  ```go
  svc := datjit.NewDefault()
  doc, _ := svc.Parse(f)
  ds, _ := svc.Generate(doc)
  _ = svc.Write(ds, "json", os.Stdout)
  ```
  without importing any non-root package.
- `go vet`, `staticcheck`, `golangci-lint run` all clean.
- README with quickstart + API example; godoc covers every exported identifier.
