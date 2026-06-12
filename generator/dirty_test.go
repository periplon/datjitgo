package generator

import (
	"strings"
	"testing"
	"time"

	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/ports"
	"github.com/periplon/datjitgo/core/value"
	"github.com/periplon/datjitgo/parser"
)

// dirtyDecorator parses "@dirty(...)" source into a model.Decorator using the
// real parser so tests exercise the same argument classification as schemas.
func dirtyDecorator(t *testing.T, src string) *model.Decorator {
	t.Helper()
	doc, err := parser.New().Parse(strings.NewReader(
		"domain: d\nentities:\n  E:\n    f: string "+src+"\n"), "test")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ent, _ := doc.Entities.Get("E")
	f, _ := ent.Fields.Get("f")
	d := model.FindDecorator(f.Decorators, "dirty")
	if d == nil {
		t.Fatalf("no @dirty parsed from %q", src)
	}
	return d
}

func TestParseDirtyConfigDefaults(t *testing.T) {
	cfg := parseDirtyConfig(dirtyDecorator(t, "@dirty"))
	if cfg.rate != defaultDirtyRate {
		t.Fatalf("rate = %v, want %v", cfg.rate, defaultDirtyRate)
	}
	want := defaultDirtyKinds()
	if len(cfg.kinds) != len(want) {
		t.Fatalf("kinds = %v, want %v", cfg.kinds, want)
	}
	for i := range want {
		if cfg.kinds[i] != want[i] {
			t.Fatalf("kinds = %v, want %v", cfg.kinds, want)
		}
	}
}

func TestParseDirtyConfigExplicit(t *testing.T) {
	cfg := parseDirtyConfig(dirtyDecorator(t, "@dirty(rate=0.25, typo, null, format_mix)"))
	if cfg.rate != 0.25 {
		t.Fatalf("rate = %v, want 0.25", cfg.rate)
	}
	want := []dirtyKind{dirtyTypo, dirtyNull, dirtyFormatMix}
	if len(cfg.kinds) != len(want) {
		t.Fatalf("kinds = %v, want %v", cfg.kinds, want)
	}
	for i := range want {
		if cfg.kinds[i] != want[i] {
			t.Fatalf("kinds = %v, want %v", cfg.kinds, want)
		}
	}
}

func TestParseDirtyConfigClampsRate(t *testing.T) {
	if got := parseDirtyConfig(dirtyDecorator(t, "@dirty(rate=7)")).rate; got != 1 {
		t.Fatalf("rate = %v, want clamp to 1", got)
	}
	if got := parseDirtyConfig(dirtyDecorator(t, "@dirty(rate=-1)")).rate; got != 0 {
		t.Fatalf("rate = %v, want clamp to 0", got)
	}
}

func TestBuildDirtyPlanNilWhenUnconfigured(t *testing.T) {
	ent := model.NewEntity("User")
	ent.Fields.Set("name", &model.Field{Name: "name", Type: model.Primitive{Kind: model.PrimString}})
	if plan := buildDirtyPlan(ent, 0); plan != nil {
		t.Fatalf("expected nil plan, got %+v", plan)
	}
}

func TestBuildDirtyPlanPrecedenceAndExemptions(t *testing.T) {
	doc, err := parser.New().Parse(strings.NewReader(`
domain: d
entities:
  Dep:
    id: uuid @primary
  User:
    _meta: "@dirty(rate=0.2, typo, null, duplicate)"
    id: uuid @primary
    name: person.full
    email: email @unique
    bio: string @dirty(rate=0.9, whitespace)
    dep: ->Dep
    secret: string @internal
`), "test")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ent, _ := doc.Entities.Get("User")
	plan := buildDirtyPlan(ent, 0.5)
	if plan == nil {
		t.Fatal("expected a plan")
	}
	byName := map[string]dirtyFieldPlan{}
	for _, fp := range plan.fields {
		byName[fp.name] = fp
	}
	// Field-level decorator wins over entity meta.
	if fp, ok := byName["bio"]; !ok || fp.rate != 0.9 || len(fp.kinds) != 1 || fp.kinds[0] != dirtyWhitespace {
		t.Fatalf("bio plan = %+v", byName["bio"])
	}
	// Entity meta applies to eligible fields (rate 0.2, not the 0.5 global).
	if fp, ok := byName["name"]; !ok || fp.rate != 0.2 {
		t.Fatalf("name plan = %+v", byName["name"])
	}
	// @unique under entity config: null filtered from the pool.
	fp, ok := byName["email"]
	if !ok || !fp.unique {
		t.Fatalf("email plan = %+v", fp)
	}
	for _, k := range fp.kinds {
		if k == dirtyNull {
			t.Fatalf("email pool retains null: %v", fp.kinds)
		}
	}
	// Exemptions: primary, references, internal fields stay out.
	for _, name := range []string{"id", "dep", "secret"} {
		if _, ok := byName[name]; ok {
			t.Fatalf("exempt field %q is in the plan", name)
		}
	}
	// duplicate configured at entity level; copy fields exclude exempt+unique.
	if plan.dupRate != 0.2 {
		t.Fatalf("dupRate = %v, want 0.2", plan.dupRate)
	}
	for _, name := range plan.copyFields {
		if name == "id" || name == "dep" || name == "email" || name == "secret" {
			t.Fatalf("copyFields contains %q: %v", name, plan.copyFields)
		}
	}
}

func TestBuildDirtyPlanGlobalRateFillsEntities(t *testing.T) {
	ent := model.NewEntity("User")
	ent.Fields.Set("id", &model.Field{Name: "id", Type: model.Primitive{Kind: model.PrimUUID},
		Decorators: []model.Decorator{{Name: "primary"}}})
	ent.Fields.Set("name", &model.Field{Name: "name", Type: model.Semantic{Namespace: "person", Tag: "full"}})
	plan := buildDirtyPlan(ent, 0.1)
	if plan == nil || len(plan.fields) != 1 || plan.fields[0].name != "name" || plan.fields[0].rate != 0.1 {
		t.Fatalf("plan = %+v", plan)
	}
	if plan.dupRate != 0 {
		t.Fatalf("global rate must not enable duplicate, dupRate = %v", plan.dupRate)
	}
}

func TestDirtyOpsTable(t *testing.T) {
	base := "hello world"
	tests := []struct {
		name  string
		kind  dirtyKind
		in    value.Value
		check func(t *testing.T, out value.Value)
	}{
		{
			name: "typo changes string",
			kind: dirtyTypo,
			in:   value.Str(base),
			check: func(t *testing.T, out value.Value) {
				t.Helper()
				if out.Kind != value.KindString {
					t.Fatalf("kind = %v", out.Kind)
				}
				if d := len(out.S) - len(base); d < -1 || d > 1 {
					t.Fatalf("typo length delta %d for %q", d, out.S)
				}
			},
		},
		{
			name: "typo no-op on short string",
			kind: dirtyTypo,
			in:   value.Str("x"),
			check: func(t *testing.T, out value.Value) {
				t.Helper()
				if out.S != "x" {
					t.Fatalf("short string mutated: %q", out.S)
				}
			},
		},
		{
			name: "case mangles letters",
			kind: dirtyCase,
			in:   value.Str("Hello"),
			check: func(t *testing.T, out value.Value) {
				t.Helper()
				switch out.S {
				case "HELLO", "hello":
				default:
					t.Fatalf("unexpected case output %q", out.S)
				}
			},
		},
		{
			name: "whitespace injects a space",
			kind: dirtyWhitespace,
			in:   value.Str(base),
			check: func(t *testing.T, out value.Value) {
				t.Helper()
				ok := strings.HasPrefix(out.S, " ") || strings.HasSuffix(out.S, " ") || strings.Contains(out.S, "  ")
				if !ok || len(out.S) != len(base)+1 {
					t.Fatalf("unexpected whitespace output %q", out.S)
				}
			},
		},
		{
			name: "whitespace single word falls back to trailing",
			kind: dirtyWhitespace,
			in:   value.Str("word"),
			check: func(t *testing.T, out value.Value) {
				t.Helper()
				if out.S != " word" && out.S != "word " {
					t.Fatalf("unexpected single-word output %q", out.S)
				}
			},
		},
		{
			name: "null nulls anything",
			kind: dirtyNull,
			in:   value.Int(42),
			check: func(t *testing.T, out value.Value) {
				t.Helper()
				if !out.IsNull() {
					t.Fatalf("want null, got %+v", out)
				}
			},
		},
		{
			name: "format_mix degrades time to string",
			kind: dirtyFormatMix,
			in:   value.Time(time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC)),
			check: func(t *testing.T, out value.Value) {
				t.Helper()
				if out.Kind != value.KindString {
					t.Fatalf("kind = %v, want string", out.Kind)
				}
				switch out.S {
				case "05/06/2024", "2024-05-06 07:08:09", "May 6, 2024":
				default:
					t.Fatalf("unexpected layout %q", out.S)
				}
			},
		},
		{
			name: "format_mix no-op on non-time",
			kind: dirtyFormatMix,
			in:   value.Str("not a time"),
			check: func(t *testing.T, out value.Value) {
				t.Helper()
				if out.Kind != value.KindString || out.S != "not a time" {
					t.Fatalf("non-time mutated: %+v", out)
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Sweep several substreams so each operator branch is hit.
			for seed := int64(0); seed < 8; seed++ {
				out := applyDirtyOp(tc.kind, tc.in, NewRand(seed))
				tc.check(t, out)
				// Pure function: same value + same substream → same output.
				again := applyDirtyOp(tc.kind, tc.in, NewRand(seed))
				if valueKey(out) != valueKey(again) {
					t.Fatalf("operator not deterministic: %q vs %q", valueKey(out), valueKey(again))
				}
			}
		})
	}
}

// countingRand wraps a Randomizer and counts every draw, so tests can prove
// operator draw budgets are independent of runtime value content.
type countingRand struct {
	inner ports.Randomizer
	n     *int
}

func (c *countingRand) Substream(scope string) ports.Randomizer {
	return &countingRand{inner: c.inner.Substream(scope), n: c.n}
}
func (c *countingRand) Float() float64 { *c.n++; return c.inner.Float() }
func (c *countingRand) IntN(n int64) int64 {
	*c.n++
	return c.inner.IntN(n)
}
func (c *countingRand) NormFloat() float64 { *c.n++; return c.inner.NormFloat() }
func (c *countingRand) ExpFloat() float64  { *c.n++; return c.inner.ExpFloat() }
func (c *countingRand) Shuffle(n int, swap func(i, j int)) {
	*c.n++
	c.inner.Shuffle(n, swap)
}

func TestDirtyOpsDrawCountsContentIndependent(t *testing.T) {
	// Each operator must consume the same number of draws no matter what
	// value it receives — no-op corruptions still consume their draws.
	values := []value.Value{
		value.Str("hello world"),
		value.Str("x"),
		value.Str(""),
		value.Int(7),
		value.Null(),
		value.Time(time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)),
	}
	for kind, want := range map[dirtyKind]int{
		dirtyTypo:       2,
		dirtyCase:       1,
		dirtyWhitespace: 2,
		dirtyNull:       0,
		dirtyFormatMix:  1,
	} {
		for _, v := range values {
			n := 0
			applyDirtyOp(kind, v, &countingRand{inner: NewRand(1), n: &n})
			if n != want {
				t.Fatalf("kind %d on %s: %d draws, want %d", kind, valueKey(v), n, want)
			}
		}
	}
}

func TestDirtyUniquenessPreserved(t *testing.T) {
	doc, err := parser.New().Parse(strings.NewReader(`
domain: d
seed: 7
volume:
  User: 60
entities:
  User:
    id: uuid @primary
    email: email @unique @dirty(rate=1.0, typo)
`), "test")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ds, err := newEngine().Generate(doc, ports.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := ds.Entities.Get("User")
	if len(rows) != 60 {
		t.Fatalf("rows = %d", len(rows))
	}
	seen := map[string]struct{}{}
	for _, r := range rows {
		v, _ := r.Get("email")
		if v.IsNull() {
			t.Fatal("unique field corrupted to null")
		}
		k := valueKey(v)
		if _, dup := seen[k]; dup {
			t.Fatalf("duplicate value after corruption: %s", k)
		}
		seen[k] = struct{}{}
	}
}

func TestDirtyGenerateDeterministicAndDistinctFromClean(t *testing.T) {
	doc := loadFixture(t, "../testdata/fixtures/dirty_data.yaml")
	gen := func() string {
		ds, err := newEngine().Generate(doc, ports.GenerateOptions{})
		if err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		ds.Entities.Each(func(_ string, rows []*value.Object) bool {
			for _, r := range rows {
				r.Each(func(k string, v value.Value) bool {
					b.WriteString(k + "=" + valueKey(v) + ";")
					return true
				})
			}
			return true
		})
		return b.String()
	}
	first, second := gen(), gen()
	if first != second {
		t.Fatal("dirty generation is not deterministic for a fixed seed")
	}
}

func TestDirtyGlobalRateZeroMatchesBaseline(t *testing.T) {
	// DirtyRate == 0 must take the exact same code path as before @dirty
	// existed: identical output, zero extra draws.
	doc := loadFixture(t, "../testdata/fixtures/minimal.yaml")
	gen := func(rate float64) string {
		ds, err := newEngine().Generate(doc, ports.GenerateOptions{DirtyRate: rate})
		if err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		ds.Entities.Each(func(_ string, rows []*value.Object) bool {
			for _, r := range rows {
				r.Each(func(k string, v value.Value) bool {
					b.WriteString(k + "=" + valueKey(v) + ";")
					return true
				})
			}
			return true
		})
		return b.String()
	}
	clean := gen(0)
	if clean != gen(0) {
		t.Fatal("baseline not deterministic")
	}
	if dirty := gen(1); dirty == clean {
		t.Fatal("DirtyRate=1 produced output identical to the clean baseline")
	}
}
