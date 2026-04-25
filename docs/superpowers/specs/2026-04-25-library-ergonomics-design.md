# datjitgo Library Ergonomics Developer Spec

Status: approved design, ready for implementation planning
Date: 2026-04-25

## Purpose

Make `github.com/jmcarbo/datjitgo` easy to embed from Go applications, tests,
custom extensions, and host DSLs without weakening the existing hexagonal
architecture.

The design keeps `datjit.Service` as the stable facade over the existing
`Parse -> Validate -> Generate -> Write` pipeline. New convenience APIs must
layer over that facade. They must not fork parsing, validation, generation,
writer, corpus, or LLM behavior.

Success means a caller can choose the smallest surface that fits their use case:

1. App code can call one package-level helper and receive data.
2. Tests can generate deterministic fixtures and assert golden JSON.
3. Extension authors can continue wiring custom adapters through `Service`
   options.
4. Rule engines and other DSLs can call datjitgo as a runtime instead of
   shelling out to the CLI.

## Current Architecture

The current repo is intentionally hexagonal:

- `core/model`, `core/value`, `core/errors`, and `core/ports` define the stable
  domain, value, error, and port contracts.
- `parser`, `generator`, `output`, `corpus`, and `llm` are adapters.
- The root `datjit` package owns `Service`, functional options, and default
  adapter wiring.
- `cmd/datjit` and `repl` are user interfaces over the service facade.

Dependency direction remains unchanged:

```text
cmd/datjit, repl
        |
        v
datjit Service + helpers
        |
        v
core ports and domain contracts
        ^
        |
parser, generator, output, corpus, llm adapters
```

No new public package may import `cmd/datjit` or `repl`. No `core/*` package may
import adapters. Adapter behavior should be reused through `datjit.Service` or
existing `ports.*` contracts.

## API Layers

### Layer 1: Root `datjit` Helpers

The root package remains the entry point for application developers. It should
keep `Service`, `NewDefault`, `New`, `WriteOpts`, and functional options. Add
package-level helpers for common flows.

```go
func GenerateString(schema string, opts ...Option) (*value.Dataset, *model.Document, error)
func GenerateFile(path string, opts ...Option) (*value.Dataset, *model.Document, error)

func GenerateMapString(schema string, opts ...Option) (map[string][]map[string]any, error)
func GenerateMapFile(path string, opts ...Option) (map[string][]map[string]any, error)

func GenerateRowsString(schema, entity string, opts ...Option) ([]map[string]any, error)
func GenerateRowsFile(path, entity string, opts ...Option) ([]map[string]any, error)

func GenerateJSONString(schema string, opts ...Option) ([]byte, error)
func GenerateJSONFile(path string, opts ...Option) ([]byte, error)

func WriteFile(outputPath, schemaPath, format string, opts ...Option) error
func WriteJSONFile(outputPath, schemaPath string, opts ...Option) error
```

Implementation contract:

- Construct the service with `New(opts...)`.
- Parse input with the service parser.
- Always call `Validate` before `Generate`.
- Return the parsed `*model.Document` with rich dataset helpers so callers can
  reuse it for writing or inspection.
- Use existing writers for JSON and file output. Do not duplicate writer
  formatting rules.
- Return `core/errors.Error` values, or wrapped causes, consistently with the
  current facade.

These helpers are convenience, not a replacement for `Service`. Advanced callers
should still use `Service` directly when they need custom parse/generate/write
control.

### Layer 2: Plain Data Conversion

Many application and DSL hosts cannot reasonably expose `core/value` types in
their public surface. Add plain conversion helpers to the root package.

```go
func ValueAny(v value.Value) any
func ObjectMap(o *value.Object) map[string]any
func RowsMap(rows []*value.Object) []map[string]any
func DatasetMap(ds *value.Dataset) map[string][]map[string]any
```

Conversion rules:

- `null` -> `nil`
- `bool`, `int`, `float`, and `string` -> matching Go primitives
- `uuid` -> canonical string
- `time` -> UTC RFC3339Nano string
- `decimal` -> decimal string using the existing value representation
- lists -> `[]any`
- objects -> `map[string]any`

The returned map type cannot promise Go map iteration order. Callers that need
schema/entity/field order should keep the rich `value.Dataset`, `value.Object`,
or writer output path. The conversion helpers should still iterate source values
in model order so any downstream ordered encoder can preserve it.

### Layer 3: `datjittest`

Add a small `datjittest` package for test authors. It should wrap the root
helpers instead of reimplementing the pipeline.

```go
func MustGenerate(t testing.TB, schema string, opts ...datjit.Option) *value.Dataset
func MustRows(t testing.TB, schema string, entity string, opts ...datjit.Option) []map[string]any
func AssertGoldenJSON(t testing.TB, goldenPath string, schema string, opts ...datjit.Option)
func UpdateGoldenJSON(t testing.TB, goldenPath string, schema string, opts ...datjit.Option)
```

Behavior contract:

- Every helper calls `t.Helper()`.
- `Must*` helpers fail fast with enough context to identify the schema/entity
  path that failed.
- Golden helpers use stable pretty JSON generated through the production JSON
  writer.
- `UpdateGoldenJSON` creates parent directories when needed.
- No live network behavior is enabled unless the caller passes an explicit LLM
  provider through `datjit.Option`.

This package should stay intentionally small. It is for deterministic fixture
workflows, not a second assertion framework.

### Layer 4: `runtime`

Add `runtime` for DSLs, rule engines, and host runtimes that need callable
generation operations.

```go
type Runtime interface {
    GenerateDocument(ctx context.Context, doc *model.Document, opts ...RunOption) (*value.Dataset, error)
    GenerateEntity(ctx context.Context, doc *model.Document, entity string, opts ...RunOption) ([]*value.Object, error)
    GenerateRows(ctx context.Context, req RowsRequest) ([]*value.Object, error)
    GenerateValue(ctx context.Context, req ValueRequest) (value.Value, error)
}
```

Request types:

```go
type RowsRequest struct {
    Document *model.Document
    Entity   string
    Seed     *int64
    Locale   string
    Volumes  map[string]int
}

type ValueRequest struct {
    Type       model.TypeExpr
    Semantic   string
    Decorators []model.Decorator
    Seed       *int64
    Locale     string
    UniqueKey  string
}
```

Construction:

```go
func NewDefault() *Default
func New(opts ...datjit.Option) (*Default, error)
```

Per-call options:

```go
func WithSeed(seed int64) RunOption
func WithLocale(locale string) RunOption
func WithVolume(entity string, volume int) RunOption
func WithVolumes(volumes map[string]int) RunOption
func WithEntity(entity string) RunOption
```

Runtime behavior contract:

- Honor `context.Context` before generation starts and at safe checkpoints.
- Validate documents before generation.
- Clone input documents before applying per-call seed, locale, volume, or entity
  overrides.
- Return typed validation errors for nil documents, unknown entities, and nil
  runtime receivers.
- `GenerateEntity` is a filtered document generation path, not a separate
  generator.
- `GenerateRows` is the request-oriented equivalent of `GenerateEntity`.
- `GenerateValue` compiles a temporary one-entity, one-field document and reuses
  the normal generator. It must not invent a parallel single-value engine.

`GenerateValue` is the key rule-engine primitive. It should support host calls
such as `fake("email")`, `fake("person.full")`, `fake("int", min=18, max=65)`,
or an LLM-backed field by translating the host request into a `ValueRequest`.

## Compiler Boundary For Host DSLs

datjitgo should expose a compiler seam, but not take dependencies on every host
language.

```go
type DocumentCompiler interface {
    Compile(ctx context.Context, src any) (*model.Document, error)
}

type CompileFunc func(ctx context.Context, src any) (*model.Document, error)
```

Host integrations own their syntax and compilation:

1. The host DSL parses its own rules, schemas, or facts.
2. It compiles generation needs into `*model.Document`, `RowsRequest`, or
   `ValueRequest`.
3. datjitgo validates and generates through `runtime`.
4. The host converts rich values to its native representation, using
   `datjit.ValueAny` or a custom adapter.

First-class packages for JSON Schema, OpenAPI, SQL schemas, or specific rule
engines should wait until there is a concrete integration to support. The base
module should provide examples, not hard dependencies.

## Example Workflows

### App Developer

```go
rows, err := datjit.GenerateRowsFile("schema.yaml", "User", datjit.WithSeed(42))
if err != nil {
    return err
}
for _, row := range rows {
    fmt.Println(row["email"])
}
```

### Test Author

```go
func TestUsersFixture(t *testing.T) {
    schema := `
domain: app
seed: 42
volume: {User: 2}
entities:
  User:
    id: uuid
    email: email
`
    datjittest.AssertGoldenJSON(t, "testdata/golden/users.json", schema)
}
```

### Extension Author

```go
svc, err := datjit.New(
    datjit.WithCorpus(customCorpus),
    datjit.WithLLMProvider(provider),
    datjit.WithWriter(customWriter),
)
```

Extensions continue using `Service` and `ports.*`. The ergonomic helpers do not
remove or hide adapter wiring.

### Rule Engine Host

```go
rt := runtime.NewDefault()
v, err := rt.GenerateValue(ctx, runtime.ValueRequest{
    Semantic: "email",
    Seed:     ptrInt64(42),
})
if err != nil {
    return err
}
fact["email"] = datjit.ValueAny(v)
```

## Error Handling

The existing `core/errors.Error` type and sentinels remain authoritative. Add
root-level predicates only as a convenience for application code:

```go
func IsParseError(err error) bool
func IsValidationError(err error) bool
func IsGenerationError(err error) bool
func IsCorpusError(err error) bool
```

These helpers should delegate to `errors.Is` against the existing sentinels.
They should not introduce a second error taxonomy.

## Files And Packages

Expected implementation files:

- `helpers.go`: root package generation and write helpers.
- `convert.go`: rich-to-plain conversion helpers.
- `errors.go`: root-level error predicates.
- `datjittest/helpers.go`: deterministic test and golden helpers.
- `runtime/runtime.go`: embeddable runtime, request types, compiler contracts,
  and default implementation.
- Focused tests beside each new package/file.
- README updates showing app, test, extension, and runtime usage.

Keep examples small and buildable:

- `examples/library/basic`
- `examples/library/plain-maps`
- `examples/library/custom-corpus`
- `examples/testing/golden`
- `examples/integrations/ruleengine`

Do not add large example trees until the APIs settle.

## Testing Requirements

Minimum verification for implementation:

```bash
go test ./...
go test -race ./...
go vet ./...
go test -run TestFixtures . -update
go test ./... -count=1 -coverprofile=coverage.out -covermode=atomic
```

If `make ci` covers the same gates, run it as the final proof.

Focused tests should cover:

- Root helpers validate before generating.
- String and file helpers return the same deterministic dataset for the same
  schema and seed.
- Unknown entity errors from row helpers are typed validation errors.
- Plain conversion covers all `value.Kind` cases.
- Golden helpers compare stable JSON and update parent directories.
- Runtime per-call options do not mutate the input document.
- Runtime entity filtering returns only the requested entity.
- `GenerateValue` reuses normal validation/generation behavior and respects
  seed, semantic type, decorators, and context cancellation.

## Non-Goals

- Do not replace `datjit.Service`.
- Do not move CLI or REPL behavior into library helpers.
- Do not add a dependency on any specific rule engine or DSL.
- Do not make live LLM calls default in tests or helper APIs.
- Do not expose generator internals as public API for convenience.
- Do not weaken validation to make helper calls feel simpler.

## Implementation Order

1. Add `convert.go` and conversion tests.
2. Add root generation/write helpers and tests.
3. Add root error predicates and tests.
4. Add `datjittest` and golden-helper tests.
5. Add `runtime` request types, options, default implementation, and tests.
6. Update README and add small buildable examples.
7. Run the full verification gate and refresh fixtures only through the
   supported fixture-update path.

This order keeps the app-facing surface usable early, then builds testing and
runtime ergonomics on top of the same production path.
