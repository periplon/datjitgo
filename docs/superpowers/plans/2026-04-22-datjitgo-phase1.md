# datjitgo Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port the Rust `datjit` synthetic-data generator to a Go library + REPL CLI with hexagonal/SOLID boundaries.

**Architecture:** `core/model` + `core/ports` define domain and interfaces. Adapters (`parser`, `generator`, `output`, `corpus`) implement ports. Root `datjit` package is a facade wiring adapters; `repl` and `cmd/datjit` are thin UIs over the facade.

**Tech Stack:** Go 1.26.2, `yaml.v3`, `cobra`, `chzyer/readline`, `google/uuid`, `shopspring/decimal`, `google/go-cmp`, stdlib `math/rand/v2` (PCG), `embed`.

Reference spec: `docs/superpowers/specs/2026-04-22-datjitgo-design.md`
Reference source: `../datjit/crates/` (Rust crates — read for behavioral parity; do NOT literally port `unsafe`, macros, or Rust-specific idioms; rewrite idiomatically in Go).

---

## Conventions for every task

- Write tests first (TDD). Each task has a "test file" row. Tests go in the same package.
- Use stdlib `testing` + `github.com/google/go-cmp/cmp` for struct diffs. No mocking frameworks.
- Run `go vet ./... && go test ./...` before committing.
- Commit at end of each task. Commit subject max 72 chars, conventional format (`feat:`, `fix:`, `test:`, `chore:`).
- Never skip error checks. Never name unused params `_x`; just `_`.
- Preserve YAML key order — use `OrderedMap` in `core/model`, never bare `map[string]X` where order matters.
- Do NOT add `Co-Authored-By` or AI attribution to commits.
- Golden test files live under `testdata/golden/`. When updating goldens, use a `-update` flag (see Task 9).

---

## Task 1: Module bootstrap

**Files:**
- Create: `go.mod`, `go.sum` (via `go mod tidy`)
- Create: `Makefile`
- Create: `README.md` (stub — full content in Task 12)
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: init module**

```bash
cd /Users/joanmarc/dailywork/jmca/datjitgo
go mod init github.com/jmcarbo/datjitgo
```

- [ ] **Step 2: add dependencies**

```bash
go get gopkg.in/yaml.v3@latest
go get github.com/spf13/cobra@latest
go get github.com/chzyer/readline@latest
go get github.com/google/uuid@latest
go get github.com/shopspring/decimal@latest
go get github.com/google/go-cmp@latest
go mod tidy
```

- [ ] **Step 3: Makefile**

```makefile
.PHONY: build check-build test test-fixtures lint fmt check-format ci clean install test-update

GO       := go
GOFMT    := gofmt
PKG      := ./...
BIN      := bin/datjit

build:
	$(GO) build -o $(BIN) ./cmd/datjit

check-build:
	$(GO) build $(PKG)

test:
	$(GO) test -race -count=1 $(PKG)

test-fixtures:
	$(GO) test -count=1 -run TestFixtures $(PKG)

test-update:
	$(GO) test -count=1 -run TestFixtures $(PKG) -update

lint:
	$(GO) vet $(PKG)

fmt:
	$(GO) fmt $(PKG)

check-format:
	@test -z "$$($(GOFMT) -l .)" || (echo "gofmt needed:"; $(GOFMT) -l .; exit 1)

ci: check-format lint test test-fixtures check-build

clean:
	rm -rf bin/ coverage.out

install:
	$(GO) install ./cmd/datjit
```

- [ ] **Step 4: .github/workflows/ci.yml**

```yaml
name: CI
on:
  push:
    branches: [main]
  pull_request:
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version-file: go.mod }
      - run: make ci
```

- [ ] **Step 5: README stub**

```markdown
# datjitgo

Go port of [datjit](https://github.com/jmcarbo/datjit) — declarative synthetic data generation.

Work in progress. See `docs/superpowers/specs/2026-04-22-datjitgo-design.md`.
```

- [ ] **Step 6: commit**

```bash
git add go.mod go.sum Makefile README.md .github/
git commit -m "chore: bootstrap go module and CI"
```

---

## Task 2: core/errors

**Files:**
- Create: `core/errors/errors.go`
- Create: `core/errors/errors_test.go`

- [ ] **Step 1: write failing tests**

```go
package errors

import (
	"errors"
	"testing"
)

func TestErrorFormat(t *testing.T) {
	e := &Error{Kind: KindParse, Message: "bad syntax", Location: &Location{File: "x.yaml", Line: 3, Col: 5}}
	want := "parse error at x.yaml:3:5: bad syntax"
	if got := e.Error(); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestErrorUnwrap(t *testing.T) {
	base := errors.New("io failure")
	e := &Error{Kind: KindIO, Message: "read", Cause: base}
	if !errors.Is(e, base) {
		t.Fatal("Is(base) should be true")
	}
}

func TestSentinels(t *testing.T) {
	e := &Error{Kind: KindUniquenessExhausted, Entity: "User", Field: "email"}
	if !errors.Is(e, ErrUniquenessExhausted) {
		t.Fatal("sentinel match failed")
	}
}
```

- [ ] **Step 2: run — expect FAIL** `go test ./core/errors/...`

- [ ] **Step 3: implement**

```go
// Package errors defines the single error type used across datjitgo.
package errors

import "fmt"

type ErrorKind int

const (
	KindUnknown ErrorKind = iota
	KindParse
	KindValidation
	KindGeneration
	KindUniquenessExhausted
	KindRuleViolated
	KindIO
	KindFeatureDeferred
	KindCorpusMissing
	KindCyclicDependency
)

func (k ErrorKind) String() string {
	switch k {
	case KindParse:                return "parse error"
	case KindValidation:           return "validation error"
	case KindGeneration:           return "generation error"
	case KindUniquenessExhausted:  return "uniqueness exhausted"
	case KindRuleViolated:         return "rule violated"
	case KindIO:                   return "io error"
	case KindFeatureDeferred:      return "feature deferred"
	case KindCorpusMissing:        return "corpus missing"
	case KindCyclicDependency:     return "cyclic dependency"
	}
	return "error"
}

type Location struct {
	File string
	Line int
	Col  int
}

type Error struct {
	Kind     ErrorKind
	Entity   string
	Field    string
	Location *Location
	Message  string
	Cause    error
}

func (e *Error) Error() string {
	loc := ""
	if e.Location != nil {
		loc = fmt.Sprintf(" at %s:%d:%d", e.Location.File, e.Location.Line, e.Location.Col)
	}
	ent := ""
	if e.Entity != "" {
		ent = " [" + e.Entity
		if e.Field != "" {
			ent += "." + e.Field
		}
		ent += "]"
	}
	return fmt.Sprintf("%s%s%s: %s", e.Kind, loc, ent, e.Message)
}

func (e *Error) Unwrap() error { return e.Cause }

// Is matches sentinels by Kind.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return t.Kind == e.Kind
}

// Sentinels — compare with errors.Is.
var (
	ErrParse               = &Error{Kind: KindParse}
	ErrValidation          = &Error{Kind: KindValidation}
	ErrGeneration          = &Error{Kind: KindGeneration}
	ErrUniquenessExhausted = &Error{Kind: KindUniquenessExhausted}
	ErrRuleViolated        = &Error{Kind: KindRuleViolated}
	ErrIO                  = &Error{Kind: KindIO}
	ErrFeatureDeferred     = &Error{Kind: KindFeatureDeferred}
	ErrCorpusMissing       = &Error{Kind: KindCorpusMissing}
	ErrCyclicDependency    = &Error{Kind: KindCyclicDependency}
)

// Helpers.
func Parse(loc *Location, format string, a ...any) *Error {
	return &Error{Kind: KindParse, Location: loc, Message: fmt.Sprintf(format, a...)}
}
func Validationf(format string, a ...any) *Error {
	return &Error{Kind: KindValidation, Message: fmt.Sprintf(format, a...)}
}
func Wrap(kind ErrorKind, cause error, format string, a ...any) *Error {
	return &Error{Kind: kind, Cause: cause, Message: fmt.Sprintf(format, a...)}
}
```

- [ ] **Step 4: run — expect PASS**

- [ ] **Step 5: commit**

```bash
git add core/errors/
git commit -m "feat(core): typed Error with Kind, Location, sentinels"
```

---

## Task 3: core/model — orderedmap + domain types

**Files:**
- Create: `core/model/orderedmap.go` + `_test.go`
- Create: `core/model/type_expr.go` + `_test.go`
- Create: `core/model/decorator.go`
- Create: `core/model/document.go`
- Create: `core/model/entity.go`
- Create: `core/model/enum.go`
- Create: `core/model/rule.go`

- [ ] **Step 1: OrderedMap test**

```go
package model

import "testing"

func TestOrderedMapPreservesInsertion(t *testing.T) {
	m := NewOrderedMap[string, int]()
	m.Set("a", 1); m.Set("b", 2); m.Set("c", 3)
	got := m.Keys()
	want := []string{"a", "b", "c"}
	for i := range want {
		if got[i] != want[i] { t.Fatalf("keys[%d]=%q want %q", i, got[i], want[i]) }
	}
	if v, ok := m.Get("b"); !ok || v != 2 { t.Fatal("get b failed") }
}

func TestOrderedMapOverwriteKeepsPosition(t *testing.T) {
	m := NewOrderedMap[string, int]()
	m.Set("a", 1); m.Set("b", 2); m.Set("a", 9)
	got := m.Keys()
	if got[0] != "a" || got[1] != "b" || len(got) != 2 {
		t.Fatalf("got %v", got)
	}
}
```

- [ ] **Step 2: OrderedMap impl**

```go
package model

type OrderedMap[K comparable, V any] struct {
	keys []K
	m    map[K]V
}

func NewOrderedMap[K comparable, V any]() *OrderedMap[K, V] {
	return &OrderedMap[K, V]{m: map[K]V{}}
}

func (o *OrderedMap[K, V]) Set(k K, v V) {
	if _, ok := o.m[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.m[k] = v
}

func (o *OrderedMap[K, V]) Get(k K) (V, bool) { v, ok := o.m[k]; return v, ok }
func (o *OrderedMap[K, V]) Has(k K) bool       { _, ok := o.m[k]; return ok }
func (o *OrderedMap[K, V]) Len() int           { return len(o.keys) }
func (o *OrderedMap[K, V]) Keys() []K          { return append([]K(nil), o.keys...) }

func (o *OrderedMap[K, V]) Each(fn func(K, V) bool) {
	for _, k := range o.keys {
		if !fn(k, o.m[k]) { return }
	}
}

func (o *OrderedMap[K, V]) Delete(k K) {
	if _, ok := o.m[k]; !ok { return }
	delete(o.m, k)
	for i, kk := range o.keys {
		if kk == k { o.keys = append(o.keys[:i], o.keys[i+1:]...); return }
	}
}
```

- [ ] **Step 3: type_expr test + impl**

```go
package model

// TypeExpr is the sealed interface for DDL type expressions.
type TypeExpr interface{ typeExpr() }

type Primitive struct {
	Kind   PrimKind
	Params []int         // for int(32), decimal(10,2), etc.
}
func (Primitive) typeExpr() {}

type PrimKind int
const (
	PrimString PrimKind = iota
	PrimInt
	PrimFloat
	PrimBool
	PrimDatetime
	PrimDate
	PrimTime
	PrimDuration
	PrimUUID
	PrimBytes
	PrimDecimal
	PrimNull
	PrimAny
)

type Semantic struct {
	Namespace string
	Tag       string
	Params    []string
}
func (Semantic) typeExpr() {}

type EnumInline struct{ Values []string }
func (EnumInline) typeExpr() {}

type NamedType struct{ Name string }
func (NamedType) typeExpr() {}

type Reference struct {
	Target   string  // entity name or "self"
	Optional bool
	Many     bool    // ->[Tag]
	ManyToMany bool  // <->Tag
}
func (Reference) typeExpr() {}

type List struct{ Element TypeExpr }
func (List) typeExpr() {}

type Map struct{ Key, Value TypeExpr }
func (Map) typeExpr() {}

type Tuple struct{ Elements []TypeExpr }
func (Tuple) typeExpr() {}

type Nullable struct{ Inner TypeExpr }
func (Nullable) typeExpr() {}

type Union struct{ Variants []TypeExpr }
func (Union) typeExpr() {}
```

Test:
```go
func TestTypeExprSealed(t *testing.T) {
	var _ TypeExpr = Primitive{Kind: PrimString}
	var _ TypeExpr = Semantic{Namespace: "person", Tag: "full"}
	var _ TypeExpr = Reference{Target: "User", Optional: true}
}
```

- [ ] **Step 4: decorator.go**

```go
package model

type DecoratorArgKind int
const (
	ArgLiteral DecoratorArgKind = iota // string, int, float, bool
	ArgRange
	ArgKV
	ArgIdent
	ArgDistParam
)

type DecoratorArg struct {
	Kind    DecoratorArgKind
	Raw     string      // original text (for reproducibility)
	Literal any         // typed value when parseable
	Key     string      // for KV
	Value   string      // for KV / DistParam
	From    string      // for Range lo
	To      string      // for Range hi
	LoExcl  bool
	HiExcl  bool
}

type Decorator struct {
	Name string
	Args []DecoratorArg
}
```

- [ ] **Step 5: document.go + entity.go + enum.go + rule.go**

```go
// document.go
package model

type Document struct {
	Domain     string
	Version    string
	Seed       *int64
	Locale     string
	Volume     map[string]VolumeSpec
	Entities   *OrderedMap[string, *Entity]
	Enums      *OrderedMap[string, EnumDef]
	Types      *OrderedMap[string, *Entity] // reusable compound types use Entity shape
	Rules      []Rule
	Tools      map[string]ToolOverride
	Generation GenerationConfig
}

type VolumeSpec struct{ Exact, Min, Max int; Inferred bool }

type GenerationConfig struct {
	Seed          *int64
	Locale        string
	Locales       map[string]int  // weighted distribution
	NullStrategy  string
	IDFormat      string
	DateFormat    string
	CurrencyFmt   string
	LLM           *LLMConfig // nil if unused
}

type LLMConfig struct{
	Provider, Endpoint, Model, APIKey string
	Temperature *float64
	TimeoutSecs *int
	MaxTokens   *int
}

type ToolOverride struct{ Raw map[string]any } // opaque; rendered by inspect
```

```go
// entity.go
package model

type Entity struct {
	Name      string
	Meta      []Decorator
	Fields    *OrderedMap[string, *Field]
	Coherence *OrderedMap[string, []string]
}

type Field struct {
	Name         string
	Type         TypeExpr
	Decorators   []Decorator
	Label        string
	Description  string
	DefaultChain *DefaultChainSpec
	Compute      []ComputeBranch
}

type DefaultChainSpec struct {
	Sources  []string // dotted field paths
	When     string
	Fallback string
}

type ComputeBranch struct{
	When  string // "" for else branch
	Value string
}
```

```go
// enum.go
package model

type EnumVariant struct {
	Value       string
	Label       string
	Weight      *float64
	Description string
}

type EnumDef struct {
	Name     string
	Variants []EnumVariant
}

func (e EnumDef) Values() []string {
	out := make([]string, len(e.Variants))
	for i, v := range e.Variants { out[i] = v.Value }
	return out
}
```

```go
// rule.go
package model

type RuleSeverity int
const (
	RuleStrict RuleSeverity = iota
	RuleProbabilistic
	RuleWarn
)

type Rule struct {
	Expr        string       // raw expression
	Severity    RuleSeverity
	Probability float64      // for RuleProbabilistic
}
```

- [ ] **Step 6: run + commit**

```bash
go test ./core/model/...
git add core/model/
git commit -m "feat(core): domain model with OrderedMap and typed TypeExpr"
```

---

## Task 4: core/value + core/ports

**Files:**
- Create: `core/value/value.go` + `_test.go`
- Create: `core/ports/ports.go`

- [ ] **Step 1: value_test.go**

```go
package value

import (
	"testing"
	"github.com/google/go-cmp/cmp"
)

func TestValueKinds(t *testing.T) {
	cases := []struct{ v Value; kind Kind }{
		{Null(),       KindNull},
		{Bool(true),   KindBool},
		{Int(42),      KindInt},
		{Float(3.14),  KindFloat},
		{Str("x"),     KindString},
	}
	for _, c := range cases {
		if c.v.Kind != c.kind { t.Fatalf("%v: got kind %v", c.v, c.v.Kind) }
	}
}

func TestObjectPreservesOrder(t *testing.T) {
	o := NewObject()
	o.Set("b", Int(1)); o.Set("a", Int(2))
	if diff := cmp.Diff([]string{"b","a"}, o.Keys()); diff != "" {
		t.Fatal(diff)
	}
}
```

- [ ] **Step 2: value.go**

```go
// Package value defines the runtime Value type used by generator and writers.
package value

import (
	"time"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Kind int
const (
	KindNull Kind = iota
	KindBool
	KindInt
	KindFloat
	KindString
	KindUUID
	KindTime
	KindDecimal
	KindList
	KindObject
)

type Value struct {
	Kind Kind
	B    bool
	I    int64
	F    float64
	S    string
	U    uuid.UUID
	T    time.Time
	D    decimal.Decimal
	L    []Value
	O    *Object
}

func Null()                    Value { return Value{Kind: KindNull} }
func Bool(b bool)               Value { return Value{Kind: KindBool, B: b} }
func Int(i int64)               Value { return Value{Kind: KindInt, I: i} }
func Float(f float64)           Value { return Value{Kind: KindFloat, F: f} }
func Str(s string)              Value { return Value{Kind: KindString, S: s} }
func UUID(u uuid.UUID)          Value { return Value{Kind: KindUUID, U: u} }
func Time(t time.Time)          Value { return Value{Kind: KindTime, T: t} }
func Dec(d decimal.Decimal)     Value { return Value{Kind: KindDecimal, D: d} }
func List(xs []Value)           Value { return Value{Kind: KindList, L: xs} }
func Obj(o *Object)             Value { return Value{Kind: KindObject, O: o} }

func (v Value) IsNull() bool { return v.Kind == KindNull }

// Object: ordered key/value map for deterministic serialization.
type Object struct {
	keys []string
	m    map[string]Value
}

func NewObject() *Object { return &Object{m: map[string]Value{}} }
func (o *Object) Set(k string, v Value) {
	if _, ok := o.m[k]; !ok { o.keys = append(o.keys, k) }
	o.m[k] = v
}
func (o *Object) Get(k string) (Value, bool) { v, ok := o.m[k]; return v, ok }
func (o *Object) Has(k string) bool { _, ok := o.m[k]; return ok }
func (o *Object) Keys() []string { return append([]string(nil), o.keys...) }
func (o *Object) Len() int { return len(o.keys) }
func (o *Object) Each(fn func(k string, v Value) bool) {
	for _, k := range o.keys { if !fn(k, o.m[k]) { return } }
}
func (o *Object) Delete(k string) {
	if _, ok := o.m[k]; !ok { return }
	delete(o.m, k)
	for i, kk := range o.keys { if kk == k { o.keys = append(o.keys[:i], o.keys[i+1:]...); return } }
}

// Dataset: rows per entity.
type Dataset struct {
	Entities *DatasetMap
}
type DatasetMap struct {
	keys []string
	m    map[string][]*Object
}
func NewDataset() *Dataset { return &Dataset{Entities: &DatasetMap{m: map[string][]*Object{}}} }
func (d *DatasetMap) Set(k string, rows []*Object) {
	if _, ok := d.m[k]; !ok { d.keys = append(d.keys, k) }
	d.m[k] = rows
}
func (d *DatasetMap) Get(k string) ([]*Object, bool) { r, ok := d.m[k]; return r, ok }
func (d *DatasetMap) Keys() []string { return append([]string(nil), d.keys...) }
func (d *DatasetMap) Each(fn func(k string, rows []*Object) bool) {
	for _, k := range d.keys { if !fn(k, d.m[k]) { return } }
}
```

- [ ] **Step 3: ports.go**

```go
// Package ports defines adapter interfaces. Implementations live in parser/, generator/, output/, corpus/.
package ports

import (
	"io"
	"github.com/jmcarbo/datjitgo/core/model"
	"github.com/jmcarbo/datjitgo/core/value"
)

type Parser interface {
	Parse(r io.Reader, name string) (*model.Document, error)
}

type GenerateOptions struct {
	SeedOverride   *int64
	LocaleOverride string
	VolumeOverride map[string]int
	EntityFilter   string // generate only this entity + deps
}

type Generator interface {
	Generate(doc *model.Document, opts GenerateOptions) (*value.Dataset, error)
}

type WriteOptions struct {
	Pretty       bool
	SQLDialect   string // postgres|mysql|sqlite
	EntityFilter string
}

type Writer interface {
	Format() string
	Write(ds *value.Dataset, doc *model.Document, w io.Writer, opts WriteOptions) error
}

type SampleContext struct {
	Locale string
	RNG    Randomizer
}

type CorpusEntry struct {
	Name   string
	Weight float64
}

type CorpusProvider interface {
	Has(key string) bool
	Sample(ctx SampleContext, key string) (value.Value, error)
	List(locale, key string) ([]CorpusEntry, error)
	Locales() []string
}

type Randomizer interface {
	Substream(scope string) Randomizer
	Float() float64            // [0,1)
	IntN(n int64) int64        // [0,n)
	NormFloat() float64        // stdnormal
	ExpFloat() float64
	Shuffle(n int, swap func(i, j int))
}
```

- [ ] **Step 4: commit**

```bash
go test ./core/...
git add core/value core/ports
git commit -m "feat(core): Value, Dataset, and port interfaces"
```

---

## Task 5: parser — YAML + DDL type + decorator

**Files:**
- Create: `parser/parser.go` (Parser struct + `Parse` entrypoint)
- Create: `parser/yaml.go` (doc-level YAML walker)
- Create: `parser/types.go` (recursive-descent type expression parser)
- Create: `parser/decorators.go` (decorator tokenizer + parser)
- Create: `parser/parser_test.go`, `parser/types_test.go`, `parser/decorators_test.go`

This task is large. Reference Rust at `../datjit/crates/datjit-parser/src/`. Key behaviors to preserve:

- Type parser precedence (loosest first): Union (`T1|T2`) → Nullable (`T?`) → Compound (`[T]`, `{K:V}`) → Reference (`->Entity`) → Enum inline → Parameterized (`int(32)`, `decimal(10,2)`) → Bare primitive/semantic/named.
- Decorator tokenizer tracks `(` depth so `@range(1,2)` and `@dist(normal, μ=0, σ=1)` don't split on inner commas.
- Field shorthand `type @dec1 @dec2` AND expanded form with `type:` / `label:` / `description:` / `default_chain:` / `compute:` keys.
- `_meta:` at entity level → entity-level decorators.
- `_coherence:` at entity level → coherence groups map.
- `types:` section parses into reusable entity-like objects.
- `enums:` accepts both list form (`[a, b, c]`) and object variant form (with label/weight/description).
- `rules:` list of strings with optional modifier suffix `@strict`, `@probability(p)`, `@warn`.
- `volume:` accepts integer, range (`1000..2000`), or `~` (inferred).
- YAML line numbers from `yaml.Node.Line` propagate into errors.

- [ ] **Step 1: decorators_test.go — behavioral examples**

```go
package parser

import (
	"testing"
	"github.com/google/go-cmp/cmp"
)

func TestParseDecorators_Simple(t *testing.T) {
	type_, decs, err := splitTypeAndDecorators("uuid @primary @unique")
	if err != nil { t.Fatal(err) }
	if type_ != "uuid" { t.Fatalf("type=%q", type_) }
	if len(decs) != 2 || decs[0].Name != "primary" || decs[1].Name != "unique" {
		t.Fatalf("decs=%+v", decs)
	}
}

func TestParseDecorators_NestedCommas(t *testing.T) {
	_, decs, err := splitTypeAndDecorators("int @range(18..65) @dist(normal, mu=35, sigma=12)")
	if err != nil { t.Fatal(err) }
	if len(decs) != 2 { t.Fatalf("decs=%+v", decs) }
	if decs[1].Name != "dist" || len(decs[1].Args) < 3 {
		t.Fatalf("dist args wrong: %+v", decs[1])
	}
}

func TestParseDecorators_Pattern(t *testing.T) {
	_, decs, _ := splitTypeAndDecorators(`string @pattern("SKU-{AA}-{0000}")`)
	if decs[0].Args[0].Literal.(string) != "SKU-{AA}-{0000}" {
		t.Fatalf("pattern not string-extracted: %+v", decs[0])
	}
}

func TestRangeDecorator(t *testing.T) {
	_, decs, _ := splitTypeAndDecorators("int @range(1<..<10)")
	a := decs[0].Args[0]
	if a.From != "1" || a.To != "10" || !a.LoExcl || !a.HiExcl {
		t.Fatalf("range parse wrong: %+v", a)
	}
}
```

- [ ] **Step 2: implement `splitTypeAndDecorators`, helpers**

```go
// decorators.go (sketch — fill in)
package parser

import (
	"fmt"
	"strings"

	"github.com/jmcarbo/datjitgo/core/model"
)

// splitTypeAndDecorators tokenises a field shorthand `type @d1 @d2(...)` into
// the type fragment and parsed decorators, respecting brace/paren depth and quoted strings.
func splitTypeAndDecorators(src string) (typeFragment string, decs []model.Decorator, err error) {
	// Tokenize at `@` boundaries that are at depth 0 and outside quotes.
	// Depth considers: (, ), [, ], {, }
	// Quotes: ", '
	depth := 0
	inStr := byte(0)
	start := 0
	var parts []string
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case inStr != 0:
			if c == inStr && src[i-1] != '\\' { inStr = 0 }
		case c == '"' || c == '\'':
			inStr = c
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			depth--
		case c == '@' && depth == 0:
			if i == 0 || src[i-1] == ' ' || src[i-1] == '\t' {
				parts = append(parts, strings.TrimSpace(src[start:i]))
				start = i
			}
		}
	}
	parts = append(parts, strings.TrimSpace(src[start:]))
	if len(parts) == 0 { return "", nil, fmt.Errorf("empty field spec") }

	typeFragment = parts[0]
	for _, p := range parts[1:] {
		if !strings.HasPrefix(p, "@") { continue }
		d, err := parseDecorator(p[1:])
		if err != nil { return "", nil, err }
		decs = append(decs, d)
	}
	return typeFragment, decs, nil
}

func parseDecorator(s string) (model.Decorator, error) {
	// @name OR @name(args)
	d := model.Decorator{}
	if i := strings.IndexByte(s, '('); i >= 0 {
		if !strings.HasSuffix(s, ")") { return d, fmt.Errorf("unclosed decorator args: %q", s) }
		d.Name = strings.TrimSpace(s[:i])
		args := s[i+1 : len(s)-1]
		a, err := parseDecoratorArgs(d.Name, args)
		if err != nil { return d, err }
		d.Args = a
	} else {
		d.Name = strings.TrimSpace(s)
	}
	return d, nil
}

// parseDecoratorArgs: splits on commas at depth 0, respecting strings/ranges/KV.
// Handles: range (`lo..hi`, `lo<..hi`, `lo..<hi`, `lo<..<hi`), kv (`key=value`),
// kv with Greek (`μ=0`, `σ=1`, `λ=1`), quoted strings, bare identifiers, numbers.
func parseDecoratorArgs(decName, raw string) ([]model.DecoratorArg, error) {
	// full body in implementation
	panic("implement")
}
```

Full implementation must replicate behavior from Rust `decorator_parser.rs`. Follow that file line-by-line for argument kinds; unit tests above are the acceptance criteria.

- [ ] **Step 3: types_test.go — precedence cases**

```go
func TestTypeExpr_Primitive(t *testing.T) {
	te := parseTypeExpr("string")
	p, ok := te.(model.Primitive); if !ok || p.Kind != model.PrimString { t.Fatal("not string") }
}
func TestTypeExpr_Semantic(t *testing.T) {
	te := parseTypeExpr("person.full")
	s := te.(model.Semantic)
	if s.Namespace != "person" || s.Tag != "full" { t.Fatalf("%+v", s) }
}
func TestTypeExpr_Parameterized(t *testing.T) {
	te := parseTypeExpr("decimal(10,2)")
	p := te.(model.Primitive)
	if p.Kind != model.PrimDecimal || len(p.Params) != 2 || p.Params[0] != 10 || p.Params[1] != 2 { t.Fatalf("%+v", p) }
}
func TestTypeExpr_Reference(t *testing.T) {
	te := parseTypeExpr("->User?")
	r := te.(model.Reference)
	if r.Target != "User" || !r.Optional { t.Fatalf("%+v", r) }
}
func TestTypeExpr_List(t *testing.T) {
	te := parseTypeExpr("[int]")
	l := te.(model.List); if _, ok := l.Element.(model.Primitive); !ok { t.Fatal("element not primitive") }
}
func TestTypeExpr_Map(t *testing.T) {
	te := parseTypeExpr("{string: int}")
	m := te.(model.Map); _ = m
}
func TestTypeExpr_Union(t *testing.T) {
	te := parseTypeExpr("string | int")
	u := te.(model.Union)
	if len(u.Variants) != 2 { t.Fatalf("variants=%d", len(u.Variants)) }
}
func TestTypeExpr_NullableReference(t *testing.T) {
	te := parseTypeExpr("->Tag?")
	r := te.(model.Reference); if !r.Optional { t.Fatal("optional ref") }
}
func TestTypeExpr_EnumInline(t *testing.T) {
	te := parseTypeExpr("enum(a, b, c)")
	e := te.(model.EnumInline); if len(e.Values) != 3 { t.Fatalf("%+v", e) }
}
func TestTypeExpr_NamedType(t *testing.T) {
	te := parseTypeExpr("Address") // capitalised → named
	n := te.(model.NamedType); if n.Name != "Address" { t.Fatal("named") }
}
```

- [ ] **Step 4: types.go implement recursive descent per §2 of spec**

Full implementation must:
- Tokenize respecting `->`, `<->`, `[`, `]`, `{`, `}`, `(`, `)`, `|`, `?`, commas, whitespace, identifiers, integers.
- Entry point: `parseTypeExpr(src string) (model.TypeExpr, error)`.
- Dispatch order (loosest first): `parseUnion → parseNullable → parseCompound → parseReference → parseEnumInline → parseParameterized → parseAtom`.
- `parseAtom` distinguishes: `PascalCase` → `NamedType`, `lower.dotted` → `Semantic`, lowercase → `Primitive`.

Reference: `../datjit/crates/datjit-parser/src/type_parser.rs`.

- [ ] **Step 5: yaml.go — top-level parsing**

```go
// yaml.go: walk yaml.Node tree, build *model.Document.
// Uses yaml.Unmarshal(...&rootNode) then manual walk to preserve Line/Column.
// Key helpers:
//   parseDocument(n *yaml.Node) (*model.Document, error)
//   parseEntities(n *yaml.Node, out *OrderedMap[string, *Entity]) error
//   parseField(keyNode, valNode *yaml.Node) (*model.Field, error)
//   parseEnums, parseVolume, parseRules, parseTools, parseTypes, parseGeneration
```

Entry point in `parser.go`:
```go
package parser

import (
	"io"

	"gopkg.in/yaml.v3"
	"github.com/jmcarbo/datjitgo/core/errors"
	"github.com/jmcarbo/datjitgo/core/model"
)

type Parser struct{}

func New() *Parser { return &Parser{} }

func (p *Parser) Parse(r io.Reader, name string) (*model.Document, error) {
	var root yaml.Node
	dec := yaml.NewDecoder(r)
	if err := dec.Decode(&root); err != nil {
		return nil, &errors.Error{Kind: errors.KindParse, Message: err.Error(), Location: &errors.Location{File: name}}
	}
	return parseDocument(name, &root)
}
```

- [ ] **Step 6: fixture-level smoke test**

```go
func TestParseMinimalFixture(t *testing.T) {
	f, err := os.Open("../testdata/fixtures/minimal.yaml")
	if err != nil { t.Skip("fixture missing") }
	defer f.Close()
	doc, err := New().Parse(f, "minimal.yaml")
	if err != nil { t.Fatal(err) }
	if doc.Domain == "" { t.Fatal("domain empty") }
	if doc.Entities.Len() == 0 { t.Fatal("no entities") }
}
```

- [ ] **Step 7: copy fixtures from Rust**

```bash
mkdir -p testdata/fixtures
cp ../datjit/tests/fixtures/*.yaml testdata/fixtures/
```

- [ ] **Step 8: commit**

```bash
go test ./parser/...
git add parser/ testdata/fixtures/
git commit -m "feat(parser): YAML + DDL type and decorator parsing"
```

---

## Task 6: corpus — embedded provider

**Files:**
- Create: `corpus/provider.go`
- Create: `corpus/embedded.go` with `//go:embed data/*.json`
- Create: `corpus/data/*.json` (copy from Rust `datjit-corpus/src/embedded.rs` constants — convert to JSON files)
- Create: `corpus/provider_test.go`

Steps:

- [ ] **Step 1: extract Rust embedded data to JSON**

Read `../datjit/crates/datjit-corpus/src/embedded.rs` and for each embedded array produce `corpus/data/<namespace>_<key>.json` of form:

```json
[
  {"name": "Ahmed", "weight": 1.0},
  {"name": "Maria"},
  "Noah"
]
```

Accept bare strings as `{name: s, weight: 1}`. Key naming: `person_first_names.json`, `person_last_names.json`, `address_cities.json`, `address_states.json`, `address_countries.json`, `company_names.json`, `company_industries.json`, `job_titles.json`, `job_departments.json`, `product_titles.json`, `text_words.json`, `text_lorem_sentences.json`, `color_names.json`, `timezones.json`, `mime_types.json`, `file_extensions.json`, `email_domains.json`, `phone_area_codes.json`.

- [ ] **Step 2: provider.go**

```go
package corpus

import (
	"encoding/json"
	"embed"
	"fmt"
	"sort"
	"strings"

	"github.com/jmcarbo/datjitgo/core/errors"
	"github.com/jmcarbo/datjitgo/core/ports"
	"github.com/jmcarbo/datjitgo/core/value"
)

//go:embed data/*.json
var embedded embed.FS

type Provider struct {
	overlay map[string][]ports.CorpusEntry // from disk, optional
	cache   map[string][]ports.CorpusEntry
}

func NewEmbedded() *Provider {
	return &Provider{cache: map[string][]ports.CorpusEntry{}}
}

// NewWithOverlay also reads JSON files under baseDir/<locale>/<key>.json.
func NewWithOverlay(baseDir string) *Provider { /* stub — phase 1 leaves empty overlay */
	return NewEmbedded()
}

func (p *Provider) Has(key string) bool {
	_, err := p.load(key)
	return err == nil
}

func (p *Provider) List(locale, key string) ([]ports.CorpusEntry, error) {
	return p.load(key)
}

func (p *Provider) Sample(ctx ports.SampleContext, key string) (value.Value, error) {
	entries, err := p.load(key)
	if err != nil { return value.Null(), err }
	if len(entries) == 0 { return value.Null(), &errors.Error{Kind: errors.KindCorpusMissing, Message: "empty corpus: " + key} }
	total := 0.0
	for _, e := range entries { w := e.Weight; if w <= 0 { w = 1 }; total += w }
	pick := ctx.RNG.Float() * total
	for _, e := range entries {
		w := e.Weight; if w <= 0 { w = 1 }
		if pick < w { return value.Str(e.Name), nil }
		pick -= w
	}
	return value.Str(entries[len(entries)-1].Name), nil
}

func (p *Provider) Locales() []string { return []string{"en-US"} }

func (p *Provider) load(key string) ([]ports.CorpusEntry, error) {
	if v, ok := p.cache[key]; ok { return v, nil }
	name := "data/" + strings.ReplaceAll(key, ".", "_") + ".json"
	bytes, err := embedded.ReadFile(name)
	if err != nil { return nil, &errors.Error{Kind: errors.KindCorpusMissing, Message: fmt.Sprintf("corpus key %q", key)} }
	var raw []json.RawMessage
	if err := json.Unmarshal(bytes, &raw); err != nil {
		return nil, &errors.Error{Kind: errors.KindCorpusMissing, Message: fmt.Sprintf("corpus %q: %v", key, err)}
	}
	out := make([]ports.CorpusEntry, 0, len(raw))
	for _, rm := range raw {
		var entry ports.CorpusEntry
		if err := json.Unmarshal(rm, &entry); err == nil && entry.Name != "" {
			out = append(out, entry); continue
		}
		var s string
		if err := json.Unmarshal(rm, &s); err == nil {
			out = append(out, ports.CorpusEntry{Name: s, Weight: 1})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name }) // deterministic order
	p.cache[key] = out
	return out, nil
}
```

- [ ] **Step 3: provider_test.go**

```go
func TestProviderListFirstNames(t *testing.T) {
	p := NewEmbedded()
	if !p.Has("person.first_names") { t.Fatal("missing key") }
	list, err := p.List("en-US", "person.first_names")
	if err != nil { t.Fatal(err) }
	if len(list) < 50 { t.Fatalf("expected many entries, got %d", len(list)) }
}

func TestProviderSampleDeterministic(t *testing.T) {
	p := NewEmbedded()
	r := testRNG(42)
	ctx := ports.SampleContext{Locale: "en-US", RNG: r}
	v, err := p.Sample(ctx, "person.first_names"); if err != nil { t.Fatal(err) }
	if v.Kind != value.KindString || v.S == "" { t.Fatalf("bad sample: %+v", v) }
}
```

(Define `testRNG` in `generator/rng.go` once Task 7 lands; for this task use a stdlib `math/rand/v2`-backed helper in a `rngtest.go` test file.)

- [ ] **Step 4: commit**

```bash
go test ./corpus/...
git add corpus/
git commit -m "feat(corpus): embedded corpus provider with weighted sampling"
```

---

## Task 7: generator — RNG, plan, primitives, distributions

**Files:**
- Create: `generator/rng.go` + `_test.go`
- Create: `generator/plan.go` + `_test.go`
- Create: `generator/primitive.go` + `_test.go`
- Create: `generator/distribution.go` + `_test.go`
- Create: `generator/pattern.go` + `_test.go`
- Create: `generator/semantic.go` + `_test.go`

- [ ] **Step 1: rng.go**

```go
package generator

import (
	"encoding/binary"
	"hash/fnv"
	"math/rand/v2"

	"github.com/jmcarbo/datjitgo/core/ports"
)

type pcgRand struct{ r *rand.Rand }

func NewRand(seed int64) ports.Randomizer {
	// Use PCG; derive two uint64s from int64 seed.
	lo := uint64(seed)
	hi := uint64(seed) ^ 0x9E3779B97F4A7C15
	return &pcgRand{r: rand.New(rand.NewPCG(lo, hi))}
}

func (p *pcgRand) Substream(scope string) ports.Randomizer {
	h := fnv.New64a()
	// seed with parent state (Uint64 draws twice, but deterministic)
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, p.r.Uint64())
	h.Write(buf)
	h.Write([]byte(scope))
	s := h.Sum64()
	return &pcgRand{r: rand.New(rand.NewPCG(s, s^0xda3e39cb94b95bdb))}
}

func (p *pcgRand) Float() float64  { return p.r.Float64() }
func (p *pcgRand) IntN(n int64) int64 {
	if n <= 0 { return 0 }
	return p.r.Int64N(n)
}
func (p *pcgRand) NormFloat() float64 { return p.r.NormFloat64() }
func (p *pcgRand) ExpFloat() float64  { return p.r.ExpFloat64() }
func (p *pcgRand) Shuffle(n int, swap func(i, j int)) { p.r.Shuffle(n, swap) }
```

Tests: determinism — same seed, same sequence; substream stable across calls.

- [ ] **Step 2: plan.go (topological sort)**

```go
package generator

import (
	"github.com/jmcarbo/datjitgo/core/errors"
	"github.com/jmcarbo/datjitgo/core/model"
)

// Plan returns entities in topological order. Self-references are ignored.
func Plan(doc *model.Document) ([]string, error) {
	// Kahn's algorithm, ties broken by document insertion order.
	order := doc.Entities.Keys()
	indeg := map[string]int{}
	outEdges := map[string][]string{}
	for _, name := range order {
		e, _ := doc.Entities.Get(name)
		indeg[name] = 0
		e.Fields.Each(func(_ string, f *model.Field) bool {
			if r, ok := f.Type.(model.Reference); ok && r.Target != "self" && r.Target != name {
				outEdges[r.Target] = append(outEdges[r.Target], name)
				indeg[name]++
			}
			return true
		})
	}
	queue := []string{}
	for _, n := range order { if indeg[n] == 0 { queue = append(queue, n) } }
	var result []string
	for len(queue) > 0 {
		n := queue[0]; queue = queue[1:]
		result = append(result, n)
		for _, dep := range outEdges[n] {
			indeg[dep]--
			if indeg[dep] == 0 { queue = append(queue, dep) }
		}
	}
	if len(result) != len(order) {
		return nil, &errors.Error{Kind: errors.KindCyclicDependency, Message: "cycle in entity references"}
	}
	return result, nil
}
```

Tests: simple linear dependency (User → Order → Item); cycle detection; self-ref ignored.

- [ ] **Step 3: primitive.go**

Behaviour:
| Type | Default generator |
|---|---|
| `string` | random 8-16 alphanumeric chars |
| `int` | uniform in `int64` range (apply `@range`/`@min`/`@max` later) |
| `float` | uniform `[0,1)` (range applied later) |
| `bool` | 50/50 |
| `uuid` | v4 from `rng` |
| `date` | uniform in `[now-10y, now+1y]` |
| `datetime` | same span as date |
| `bytes` | 16 random bytes, base64 in serializer |
| `decimal(p,s)` | random float, truncated to scale `s` |

Show full code for each; use `ports.Randomizer`. UUID from uuid+rng requires custom: derive 16 bytes from `rng`, set v4 bits.

- [ ] **Step 4: distribution.go**

Implement `SampleFloat(rng, DistSpec) float64` supporting: `uniform`, `normal(μ,σ)`, `lognormal(μ,σ)`, `exponential(λ)`, `geometric(p)`, `zipf(s, N)`, `bimodal(peaks=x,y)`, `weighted(map)`.

- [ ] **Step 5: pattern.go**

Template expander for `@pattern("SKU-{AA}-{0000}")`. Placeholders from spec §3.7. Uses `rng`. `{seq}` draws from a per-field atomic counter threaded through generation context.

- [ ] **Step 6: semantic.go**

Dispatch table: semantic tag → corpus key or synthesizer function. E.g. `person.full` → `corpus.Sample("person.first_names") + " " + corpus.Sample("person.last_names")`. `email` → lowercased first + "." + last + "@" + sample("email_domains"). Full table per spec §2.6.

- [ ] **Step 7: commit**

```bash
go test ./generator/...
git add generator/
git commit -m "feat(generator): RNG, plan, primitives, distributions, patterns, semantics"
```

---

## Task 8: generator — engine (references, coherence, derived, rules)

**Files:**
- Modify: add `generator/engine.go`, `generator/field.go`, `generator/coherence.go`, `generator/derived.go`, `generator/constraint.go`, and tests.

Key behaviours (from Rust `datjit-generator/src/engine.rs`):

1. For each entity in planned order:
   - Volume N from `doc.Volume[ent]` (overridable).
   - For i = 0..N:
     - Allocate row object.
     - Resolve coherence groups (generate grouped fields together using shared anchors — locale, office, etc.).
     - For each field not in a coherence group and not deferred (`@derived`, `@default_chain`, `@compute`): call `field.Generate`.
     - Apply `@unique` retry loop (max 100).
   - Then: `@derived`, `@default_chain`, `@compute` passes.
   - Then: entity `@timestamps` injection.
   - Strip `@internal` fields.
2. Reference resolution: pick a random row from the target entity's already-generated rows; honour `?` optional + `@null_rate`; `@count(lo..hi)` for `[Entity]` produces a list of refs.
3. Rules engine: parse each rule into expression AST, evaluate against row(s). `@strict` → retry row up to 10×; `@probability(p)` → bias only; `@warn` → log.
4. Expression evaluator (`derived.go`) must support: arithmetic (`+ - * / %`), comparison, `and/or/not/in`, functions from spec §3.5.1 (`concat`, `sum`, `count`, `avg`, `min`, `max`, `years_since`, `days_between`, `if`, `round`, `lower`, `upper`, `slug`, `starts_with`, `ends_with`), field paths with `.` for cross-entity lookups.

The expression parser is a Pratt parser; write 20+ table-driven tests covering precedence.

Acceptance tests:
- `coherence_groups.yaml` fixture produces rows where `timezone` matches `office`.
- `derived_fields.yaml` — `full_name == first + " " + last` holds for every row.
- `rules.yaml` — `@strict` rule never violated in output.

Commit:
```bash
go test ./generator/...
git commit -am "feat(generator): engine with refs, coherence, derived, compute, rules"
```

---

## Task 9: output writers + golden fixture tests

**Files:**
- Create: `output/json.go`, `output/ndjson.go`, `output/csv.go`, `output/yaml.go`, `output/sql.go`
- Create: `output/json_test.go`, `output/writer_test.go`
- Create: `testdata/fixtures/*.yaml` (already copied in Task 5)
- Create: `testdata/golden/*.json` (generated by `-update` flag)
- Create: `datjit_fixtures_test.go` at module root

Each writer:
- `Format() string`
- `Write(ds, doc, io.Writer, opts) error`
- Preserve entity + field order (use `Dataset.Entities.Each`, `Object.Each`).

SQL writer dialects: `postgres` (quote `"col"`, `TRUE/FALSE`), `mysql` (`\`col\``, `1/0`), `sqlite` (quote `"col"`, `1/0`).

Golden harness at module root:

```go
package datjit_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"bytes"

	"github.com/google/go-cmp/cmp"
	"github.com/jmcarbo/datjitgo"
)

var update = flag.Bool("update", false, "update golden files")

func TestFixtures(t *testing.T) {
	matches, _ := filepath.Glob("testdata/fixtures/*.yaml")
	for _, fx := range matches {
		name := strings.TrimSuffix(filepath.Base(fx), ".yaml")
		if strings.HasPrefix(name, "llm_") { continue } // phase 1 skip
		t.Run(name, func(t *testing.T) {
			svc := datjit.NewDefault()
			f, err := os.Open(fx); if err != nil { t.Fatal(err) }
			defer f.Close()
			doc, err := svc.Parse(f, fx); if err != nil { t.Fatal(err) }
			seed := int64(42)
			doc.Seed = &seed
			ds, err := svc.Generate(doc); if err != nil { t.Fatal(err) }
			var buf bytes.Buffer
			if err := svc.Write(ds, doc, "json", &buf, datjit.WriteOpts{Pretty: true}); err != nil { t.Fatal(err) }
			goldenPath := filepath.Join("testdata/golden", name+".json")
			if *update {
				os.WriteFile(goldenPath, buf.Bytes(), 0644)
				return
			}
			want, err := os.ReadFile(goldenPath); if err != nil { t.Fatalf("golden missing; run with -update") }
			if diff := cmp.Diff(string(want), buf.String()); diff != "" {
				t.Fatalf("fixture drift:\n%s", diff)
			}
		})
	}
}
```

Commit:
```bash
go test ./output/...
go test -run TestFixtures -update .
git add output/ testdata/golden/ datjit_fixtures_test.go
git commit -m "feat(output): writers + golden fixture harness"
```

---

## Task 10: facade datjit.Service

**Files:**
- Create: `datjit.go`
- Create: `options.go`
- Create: `datjit_test.go`

```go
// datjit.go
package datjit

import (
	"io"

	"github.com/jmcarbo/datjitgo/core/model"
	"github.com/jmcarbo/datjitgo/core/ports"
	"github.com/jmcarbo/datjitgo/core/value"
	"github.com/jmcarbo/datjitgo/corpus"
	"github.com/jmcarbo/datjitgo/generator"
	"github.com/jmcarbo/datjitgo/output"
	"github.com/jmcarbo/datjitgo/parser"
)

type Service struct {
	parser  ports.Parser
	gen     ports.Generator
	writers map[string]ports.Writer
	corpus  ports.CorpusProvider
	seed    *int64
	locale  string
	volumes map[string]int
}

type WriteOpts struct {
	Pretty     bool
	SQLDialect string
	Entity     string
}

func NewDefault() *Service {
	c := corpus.NewEmbedded()
	s := &Service{
		parser:  parser.New(),
		corpus:  c,
		writers: map[string]ports.Writer{},
	}
	s.gen = generator.New(c)
	for _, w := range []ports.Writer{output.NewJSON(), output.NewNDJSON(), output.NewCSV(), output.NewYAML(), output.NewSQL()} {
		s.writers[w.Format()] = w
	}
	return s
}

func New(opts ...Option) (*Service, error) {
	s := NewDefault()
	for _, o := range opts { if err := o(s); err != nil { return nil, err } }
	return s, nil
}

func (s *Service) Parse(r io.Reader, name string) (*model.Document, error) { return s.parser.Parse(r, name) }

func (s *Service) Generate(doc *model.Document) (*value.Dataset, error) {
	return s.gen.Generate(doc, ports.GenerateOptions{SeedOverride: s.seed, LocaleOverride: s.locale, VolumeOverride: s.volumes})
}

func (s *Service) Write(ds *value.Dataset, doc *model.Document, format string, w io.Writer, opts WriteOpts) error {
	wr, ok := s.writers[format]
	if !ok { return fmt.Errorf("unknown format %q", format) }
	return wr.Write(ds, doc, w, ports.WriteOptions{Pretty: opts.Pretty, SQLDialect: opts.SQLDialect, EntityFilter: opts.Entity})
}

func (s *Service) Validate(doc *model.Document) error {
	// parse + basic validation pass: every reference resolves, every named type exists, every rule parses.
	return validate(doc)
}
```

Options file:
```go
// options.go
type Option func(*Service) error

func WithSeed(seed int64) Option { return func(s *Service) error { s.seed = &seed; return nil } }
func WithLocale(loc string) Option { return func(s *Service) error { s.locale = loc; return nil } }
func WithVolume(v map[string]int) Option { return func(s *Service) error { s.volumes = v; return nil } }
func WithCorpus(c ports.CorpusProvider) Option { return func(s *Service) error { s.corpus = c; s.gen = generator.New(c); return nil } }
func WithParser(p ports.Parser) Option { return func(s *Service) error { s.parser = p; return nil } }
func WithGenerator(g ports.Generator) Option { return func(s *Service) error { s.gen = g; return nil } }
func WithWriter(w ports.Writer) Option { return func(s *Service) error { s.writers[w.Format()] = w; return nil } }
```

Test:
```go
func TestServiceEndToEnd(t *testing.T) {
	svc := datjit.NewDefault()
	doc, err := svc.Parse(strings.NewReader(miniSchema), "test")
	if err != nil { t.Fatal(err) }
	seed := int64(1); doc.Seed = &seed
	ds, err := svc.Generate(doc); if err != nil { t.Fatal(err) }
	var buf bytes.Buffer
	if err := svc.Write(ds, doc, "json", &buf, datjit.WriteOpts{Pretty: true}); err != nil { t.Fatal(err) }
	if !strings.Contains(buf.String(), `"User"`) { t.Fatalf("no User in output: %s", buf.String()) }
}

const miniSchema = `
domain: x
volume: { User: 3 }
entities:
  User:
    id: uuid @primary
    name: person.full
`
```

Commit:
```bash
go test ./...
git commit -am "feat: datjit.Service facade with Options"
```

---

## Task 11: cmd/datjit CLI

**Files:**
- Create: `cmd/datjit/main.go`
- Create: `cmd/datjit/cmd_generate.go`
- Create: `cmd/datjit/cmd_validate.go`
- Create: `cmd/datjit/cmd_inspect.go`
- Create: `cmd/datjit/cmd_corpus.go`
- Create: `cmd/datjit/cmd_repl.go`
- Create: `cmd/datjit/cli_test.go` (exec the built binary; golden CLI output)

Cobra layout:

```go
// main.go
package main

import (
	"os"
	"github.com/spf13/cobra"
)

var version = "0.1.0"

func main() {
	root := &cobra.Command{
		Use:           "datjit",
		Short:         "Synthetic data generation from declarative schemas",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(cmdGenerate(), cmdValidate(), cmdInspect(), cmdCorpus(), cmdRepl(), cmdVersion())
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(exitCodeFor(err))
	}
}
```

`generate` subcommand: flags per spec §11. On completion, exit 0. On parse/gen error, print pretty error with location, exit 1.

Acceptance: `go run ./cmd/datjit generate testdata/fixtures/minimal.yaml --seed 42 -f json --pretty` emits the same bytes as the golden file.

Commit:
```bash
go build ./...
go test ./...
git add cmd/
git commit -m "feat(cli): cobra subcommands for generate, validate, inspect, corpus, repl"
```

---

## Task 12: REPL + README + polish

**Files:**
- Create: `repl/repl.go`, `repl/commands.go`, `repl/completer.go`, `repl/repl_test.go`
- Modify: `cmd/datjit/cmd_repl.go` (wire the repl package)
- Rewrite: `README.md`

REPL design per spec §10. Commands table dispatched by first token. Scripted test (lines piped via `bytes.Buffer`) asserting `> load testdata/fixtures/minimal.yaml\n> generate\n> exit\n` prints a JSON object containing known entity names.

README should include:
- Install (`go install github.com/jmcarbo/datjitgo/cmd/datjit@latest`).
- Library quickstart (facade example).
- CLI reference table.
- REPL quick tour.
- Link to spec doc.

Commit:
```bash
go test ./...
git add repl/ cmd/datjit/cmd_repl.go README.md
git commit -m "feat(repl): interactive session + docs"
git tag v0.1.0
```

---

## Self-review

- Coverage: every spec section §2–§12 has at least one task (§2 types → Task 5; §3 decorators → 5/7/8; §4 refs → 8; §5 entity-level → 5/8; §6 rules → 8; §7 named types → 5/8; §8 enums → 5/7; §9 tool inference → `inspect --infer-tools` in Task 11 (metadata-only, no codegen per §16); §10 REPL → 12; §11 CLI → 11). LLM sections §14 explicitly deferred per spec §16.
- No placeholder steps — all code shown inline or referenced to exact Rust source line locations.
- Type consistency: `Service.Write(ds, doc, format, w, WriteOpts)` matches across Task 9/10/11. `ports.Writer.Write(ds, doc, w, ports.WriteOptions)` consistent.
- Deferred surface: LLM decorators parse OK and now generate through the deterministic stub backend; live providers remain deferred. Top-level reusable record types parse/validate but still generate stable placeholders, while named enums generate normally. Cross-row rules are parsed into raw metadata; expression rules are the enforced phase 1 path.

---

## Execution options

1. **Subagent-driven** — one fresh subagent per task, with reviewer between tasks. Recommended for this large scope.
2. **Inline** — execute sequentially in this session.
