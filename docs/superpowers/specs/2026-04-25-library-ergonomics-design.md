# datjitgo Library Ergonomics Design

Status: design approved through brainstorming
Date: 2026-04-25

## Goal

Make datjitgo easy to embed in other Go projects without weakening the existing
hexagonal core. The library should serve four audiences:

1. App developers who want `schema -> data` with minimal ceremony.
2. Test-suite authors who want deterministic fixtures and golden snapshots.
3. Extension authors who need custom corpus, LLM, parser, generator, or writer
   adapters.
4. DSL and rule-engine authors who want datjitgo generation as an embeddable
   runtime.

The current `datjit.Service` remains the stable core. New APIs should be thin
layers over the existing `Parse -> Validate -> Generate -> Write` pipeline.

## Public API Layers

### Root Package: `datjit`

The root package keeps `Service`, `NewDefault`, `New`, and functional options.
Add convenience helpers for common app flows:

```go
func GenerateFile(path string, opts ...Option) (*value.Dataset, *model.Document, error)
func GenerateString(schema string, opts ...Option) (*value.Dataset, *model.Document, error)
func GenerateMapFile(path string, opts ...Option) (map[string][]map[string]any, error)
func GenerateRowsFile(path, entity string, opts ...Option) ([]map[string]any, error)
func GenerateJSONFile(path string, opts ...Option) ([]byte, error)
```

These helpers construct a default service, apply options, parse, validate,
generate, and return the requested shape. They should not bypass validation.

Add writer-oriented helpers only if they stay simple:

```go
func WriteFile(path, schemaPath, format string, opts ...Option) error
func WriteJSONFile(outputPath, schemaPath string, opts ...Option) error
```

### Testing Package: `datjittest`

Testing APIs should live outside the production root package and accept
`testing.TB`:

```go
func MustGenerate(t testing.TB, schema string, opts ...datjit.Option) *value.Dataset
func MustRows(t testing.TB, schema string, entity string, opts ...datjit.Option) []map[string]any
func AssertGoldenJSON(t testing.TB, goldenPath string, schema string, opts ...datjit.Option)
func UpdateGoldenJSON(t testing.TB, goldenPath string, schema string, opts ...datjit.Option)
```

`Must*` helpers fail fast with useful diagnostics. Golden helpers use stable,
pretty JSON and deterministic seeds.

### Runtime Package: `runtime`

The runtime package supports DSL and rule-engine embedding. It exposes
generation as callable operations rather than forcing external DSLs to shell
out to the CLI.

```go
type Runtime interface {
    GenerateDocument(ctx context.Context, doc *model.Document, opts ...RunOption) (*value.Dataset, error)
    GenerateEntity(ctx context.Context, doc *model.Document, entity string, opts ...RunOption) ([]*value.Object, error)
    GenerateValue(ctx context.Context, req ValueRequest) (value.Value, error)
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

`GenerateValue` is the main integration point for rule engines. It should allow
host DSLs to call datjitgo for values such as `person.full`, `email`, bounded
integers, patterned strings, or LLM-backed text. Internally, it can compile the
request into a small temporary document and reuse the existing generator.

### Compiler Boundary

datjitgo should expose compiler contracts for other DSLs without owning every
integration:

```go
type DocumentCompiler interface {
    Compile(ctx context.Context, src any) (*model.Document, error)
}

type CompileFunc func(ctx context.Context, src any) (*model.Document, error)
```

Examples can show compilers for rule-engine facts, JSON Schema, OpenAPI, or SQL
schemas. First-class packages for those DSLs should wait until there is a real
integration to support.

## Data Shapes

Support both rich internal types and plain Go values.

Rich path:

- `*model.Document`
- `*value.Dataset`
- `*value.Object`
- `value.Value`

Plain path:

- `map[string][]map[string]any`
- `[]map[string]any`
- `[]byte`
- `io.Writer`

Plain map conversion must preserve generated entity and field order where
possible. It should use existing `value.Value` conversion rules rather than
duplicating output writer logic.

## Error Handling

Keep `core/errors.Error` as the rich error type. Add root-level predicates for
common app code:

```go
func IsParseError(err error) bool
func IsValidationError(err error) bool
func IsGenerationError(err error) bool
func IsCorpusError(err error) bool
```

These helpers wrap `errors.Is` against the existing sentinels. They make the
simple APIs usable without requiring callers to import `core/errors`.

## DSL And Rule Engine Usage

A host rule engine should be able to use datjitgo in three ways:

1. Generate complete fact datasets from compiled datjit documents.
2. Generate rows for one entity as scenario setup.
3. Generate individual values from rules such as `fake("email")` or
   `fake("int", min=18, max=65)`.

The integration boundary is:

- Host DSL parses its own syntax.
- Host DSL compiles relevant data-generation requests into datjit model or
  runtime requests.
- datjitgo executes generation and returns rich or plain values.

datjitgo should not import rule-engine-specific packages in the root module.

## Examples

Add examples before adding many packages:

```text
examples/library/basic
examples/library/plain-maps
examples/library/custom-corpus
examples/library/live-llm
examples/testing/golden
examples/integrations/ruleengine
examples/integrations/jsonschema
```

Examples should compile under CI once added.

## Non-Goals

- Do not replace `Service`; layer over it.
- Do not move CLI or REPL behavior into the library runtime.
- Do not add a dependency on a specific rule engine.
- Do not make live network behavior default for tests or simple helpers.
- Do not expose unstable generator internals as public API just for convenience.

## Implementation Order

1. Add plain conversion helpers for `value.Dataset` and `value.Value`.
2. Add root package convenience helpers and tests.
3. Add error predicate helpers.
4. Add `datjittest` with deterministic fixture and golden helpers.
5. Add runtime `GenerateDocument`, `GenerateEntity`, and `GenerateValue`.
6. Add examples for app, test, extension, and rule-engine usage.

Each step should be small and covered by tests.
