package generator

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmcarbo/datjitgo/core/errors"
	"github.com/jmcarbo/datjitgo/core/model"
	"github.com/jmcarbo/datjitgo/core/ports"
	"github.com/jmcarbo/datjitgo/core/value"
	"github.com/jmcarbo/datjitgo/corpus"
	"github.com/jmcarbo/datjitgo/parser"
)

func loadFixture(t *testing.T, path string) *model.Document {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	doc, err := parser.New().Parse(f, path)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return doc
}

func newEngine() *Engine { return New(corpus.NewEmbedded()) }

func TestEngineMinimalFixture(t *testing.T) {
	doc := loadFixture(t, "../testdata/fixtures/minimal.yaml")
	ds, err := newEngine().Generate(doc, ports.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if ds.Entities.Len() != 1 {
		t.Fatalf("expected 1 entity, got %d", ds.Entities.Len())
	}
	rows, _ := ds.Entities.Get("User")
	if len(rows) != 10 {
		t.Fatalf("expected 10 User rows, got %d", len(rows))
	}
	for _, r := range rows {
		if !r.Has("id") || !r.Has("name") || !r.Has("email") {
			t.Fatalf("missing columns: %v", r.Keys())
		}
	}
}

func TestEngineDerivedFullName(t *testing.T) {
	// Synthetic schema where full_name must equal first + " " + last.
	doc := model.NewDocument()
	seed := int64(11)
	doc.Seed = &seed
	u := model.NewEntity("User")
	u.Fields.Set("first", &model.Field{Name: "first", Type: model.Semantic{Namespace: "person", Tag: "first"}})
	u.Fields.Set("last", &model.Field{Name: "last", Type: model.Semantic{Namespace: "person", Tag: "last"}})
	u.Fields.Set("full_name", &model.Field{
		Name: "full_name",
		Type: model.Primitive{Kind: model.PrimString},
		Decorators: []model.Decorator{{
			Name: "derived",
			Args: []model.DecoratorArg{{Kind: model.ArgLiteral, Literal: `concat(first, " ", last)`, Raw: `concat(first, " ", last)`}},
		}},
	})
	doc.Entities.Set("User", u)
	doc.Volume["User"] = model.VolumeSpec{Exact: 8}

	ds, err := newEngine().Generate(doc, ports.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := ds.Entities.Get("User")
	for _, r := range rows {
		first, _ := r.Get("first")
		last, _ := r.Get("last")
		full, _ := r.Get("full_name")
		if full.S != first.S+" "+last.S {
			t.Fatalf("full_name mismatch: %q vs %q %q", full.S, first.S, last.S)
		}
	}
}

func TestEngineRulesFixtureStrictHolds(t *testing.T) {
	doc := loadFixture(t, "../testdata/fixtures/rules.yaml")
	ds, err := newEngine().Generate(doc, ports.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// Every User.age >= 18 (strict), every Order.amount > 0 (strict).
	users, _ := ds.Entities.Get("User")
	for _, r := range users {
		age, _ := r.Get("age")
		if age.I < 18 {
			t.Fatalf("age < 18: %d", age.I)
		}
	}
	orders, _ := ds.Entities.Get("Order")
	for _, r := range orders {
		amt, _ := r.Get("amount")
		if amt.F <= 0 {
			t.Fatalf("amount <= 0: %v", amt.F)
		}
	}
}

// TestEngineLLMStubFills confirms that @llm-decorated fields produce
// non-empty stub content in phase 1 (the deferred-generation path was
// retired when the deterministic stub backend landed).
func TestEngineLLMStubFills(t *testing.T) {
	doc := loadFixture(t, "../testdata/fixtures/llm_field_level.yaml")
	ds, err := newEngine().Generate(doc, ports.GenerateOptions{})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	rows, ok := ds.Entities.Get("Product")
	if !ok || len(rows) == 0 {
		t.Fatal("no Product rows")
	}
	for i, r := range rows {
		desc, _ := r.Get("description")
		tagline, _ := r.Get("tagline")
		if desc.S == "" {
			t.Fatalf("row %d: empty description", i)
		}
		if tagline.S == "" {
			t.Fatalf("row %d: empty tagline", i)
		}
	}
}

func TestEngineDeterministicOutput(t *testing.T) {
	doc := loadFixture(t, "../testdata/fixtures/minimal.yaml")
	eng := newEngine()
	ds1, err := eng.Generate(doc, ports.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ds2, err := eng.Generate(doc, ports.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	j1 := datasetToJSON(t, ds1)
	j2 := datasetToJSON(t, ds2)
	if !bytes.Equal(j1, j2) {
		t.Fatalf("non-deterministic output:\n%s\nvs\n%s", j1, j2)
	}
}

func TestEngineUniquenessExhaustion(t *testing.T) {
	doc := model.NewDocument()
	seed := int64(1)
	doc.Seed = &seed
	u := model.NewEntity("User")
	u.Fields.Set("name", &model.Field{
		Name:       "name",
		Type:       model.EnumInline{Values: []string{"a", "b"}},
		Decorators: []model.Decorator{{Name: "unique"}},
	})
	doc.Entities.Set("User", u)
	doc.Volume["User"] = model.VolumeSpec{Exact: 5}

	_, err := newEngine().Generate(doc, ports.GenerateOptions{})
	if err == nil {
		t.Fatal("expected ErrUniquenessExhausted")
	}
	if !stderrors.Is(err, errors.ErrUniquenessExhausted) {
		t.Fatalf("wrong err: %v", err)
	}
}

func TestEngineEntityFilter(t *testing.T) {
	doc := loadFixture(t, "../testdata/fixtures/derived_fields.yaml")
	ds, err := newEngine().Generate(doc, ports.GenerateOptions{EntityFilter: "Product"})
	if err != nil {
		t.Fatal(err)
	}
	if ds.Entities.Len() != 1 {
		t.Fatalf("expected only Product, got %v", ds.Entities.Keys())
	}
}

func TestEngineCoherenceGroupsPopulated(t *testing.T) {
	doc := loadFixture(t, "../testdata/fixtures/coherence_groups.yaml")
	ds, err := newEngine().Generate(doc, ports.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := ds.Entities.Get("Employee")
	if len(rows) == 0 {
		t.Fatal("no employees generated")
	}
	for _, r := range rows {
		first, _ := r.Get("first_name")
		email, _ := r.Get("email")
		if first.S == "" || email.S == "" {
			t.Fatalf("coherence group empty: %+v", r.Keys())
		}
		if !strings.Contains(strings.ToLower(email.S), strings.ToLower(first.S)) {
			t.Fatalf("email %q missing first_name %q", email.S, first.S)
		}
	}
}

func TestEngineGeneratesEveryFixtureAtLowVolume(t *testing.T) {
	matches, err := filepath.Glob("../testdata/fixtures/*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("no fixtures found")
	}
	for _, path := range matches {
		t.Run(strings.TrimSuffix(filepath.Base(path), ".yaml"), func(t *testing.T) {
			doc := loadFixture(t, path)
			overrides := make(map[string]int, doc.Entities.Len())
			doc.Entities.Each(func(name string, _ *model.Entity) bool {
				overrides[name] = 2
				return true
			})
			ds, err := newEngine().Generate(doc, ports.GenerateOptions{VolumeOverride: overrides})
			if err != nil {
				t.Fatalf("generate %s: %v", path, err)
			}
			doc.Entities.Each(func(name string, _ *model.Entity) bool {
				rows, ok := ds.Entities.Get(name)
				if !ok {
					t.Fatalf("%s missing from dataset", name)
				}
				if len(rows) != 2 {
					t.Fatalf("%s rows=%d want 2", name, len(rows))
				}
				return true
			})
		})
	}
}

// datasetToJSON converts a Dataset to deterministic JSON bytes. We use
// Object.Each so insertion order is preserved across maps.
func datasetToJSON(t *testing.T, ds *value.Dataset) []byte {
	t.Helper()
	out := map[string][]map[string]any{}
	ds.Entities.Each(func(name string, rows []*value.Object) bool {
		list := make([]map[string]any, 0, len(rows))
		for _, r := range rows {
			m := map[string]any{}
			r.Each(func(k string, v value.Value) bool {
				m[k] = valueToJSON(v)
				return true
			})
			list = append(list, m)
		}
		out[name] = list
		return true
	})
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func valueToJSON(v value.Value) any {
	switch v.Kind {
	case value.KindNull:
		return nil
	case value.KindBool:
		return v.B
	case value.KindInt:
		return v.I
	case value.KindFloat:
		return v.F
	case value.KindString:
		return v.S
	case value.KindUUID:
		return v.U.String()
	case value.KindTime:
		return v.T.Format("2006-01-02T15:04:05")
	case value.KindDecimal:
		return v.D.String()
	case value.KindList:
		out := make([]any, 0, len(v.L))
		for _, item := range v.L {
			out = append(out, valueToJSON(item))
		}
		return out
	case value.KindObject:
		m := map[string]any{}
		v.O.Each(func(k string, vv value.Value) bool {
			m[k] = valueToJSON(vv)
			return true
		})
		return m
	}
	return nil
}
