package datjit_test

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"strings"
	"testing"

	"github.com/jmcarbo/datjitgo"
	"github.com/jmcarbo/datjitgo/core/errors"
	"github.com/jmcarbo/datjitgo/core/model"
)

// miniSchema is the smallest-possible YAML schema that exercises the full
// parse → generate → write pipeline. Kept inline so tests don't depend on
// fixture files.
const miniSchema = `domain: test_service
version: 0.1.0
seed: 42

volume:
  User: 3

entities:
  User:
    id: uuid @primary
    name: person.full
    age: int @range(18..65)
`

// miniSchemaWithRef has a second entity referencing User so plans and
// validation have something non-trivial to look at.
const miniSchemaWithRef = `domain: test_service_refs
version: 0.1.0
seed: 7

volume:
  User: 2
  Order: 3

entities:
  User:
    id: uuid @primary
    name: person.full

  Order:
    id: uuid @primary
    customer: ->User
    amount: float @range(1..100)
`

func TestServiceEndToEnd(t *testing.T) {
	svc := datjit.NewDefault()
	doc, err := svc.Parse(strings.NewReader(miniSchema), "mini.yaml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Domain != "test_service" {
		t.Fatalf("wrong domain: %q", doc.Domain)
	}

	ds, err := svc.Generate(doc)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	rows, ok := ds.Entities.Get("User")
	if !ok || len(rows) != 3 {
		t.Fatalf("expected 3 User rows, got %d (ok=%v)", len(rows), ok)
	}

	var buf bytes.Buffer
	if err := svc.Write(ds, doc, "json", &buf, datjit.WriteOpts{Pretty: true}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("\"User\"")) {
		t.Fatalf("JSON output missing User key: %s", buf.String())
	}
}

func TestServiceNilGuards(t *testing.T) {
	svc := datjit.NewDefault()
	if _, err := svc.Generate(nil); !stderrors.Is(err, errors.ErrValidation) {
		t.Fatalf("Generate(nil) should return ErrValidation, got %v", err)
	}
	if _, err := svc.Inspect(nil); !stderrors.Is(err, errors.ErrValidation) {
		t.Fatalf("Inspect(nil) should return ErrValidation, got %v", err)
	}
	if err := svc.Validate(nil); !stderrors.Is(err, errors.ErrValidation) {
		t.Fatalf("Validate(nil) should return ErrValidation, got %v", err)
	}
	if _, err := svc.Parse(nil, ""); !stderrors.Is(err, errors.ErrValidation) {
		t.Fatalf("Parse(nil,*) should return ErrValidation, got %v", err)
	}
}

func TestServiceValidateCycle(t *testing.T) {
	svc := datjit.NewDefault()
	doc := model.NewDocument()
	a := model.NewEntity("A")
	a.Fields.Set("b", &model.Field{Name: "b", Type: model.Reference{Target: "B"}})
	b := model.NewEntity("B")
	b.Fields.Set("a", &model.Field{Name: "a", Type: model.Reference{Target: "A"}})
	doc.Entities.Set("A", a)
	doc.Entities.Set("B", b)
	err := svc.Validate(doc)
	if err == nil {
		t.Fatal("expected cycle error")
	}
	if !stderrors.Is(err, errors.ErrCyclicDependency) {
		t.Fatalf("want ErrCyclicDependency, got %v", err)
	}
}

func TestServiceValidateBadRef(t *testing.T) {
	svc := datjit.NewDefault()
	doc := model.NewDocument()
	a := model.NewEntity("A")
	a.Fields.Set("missing", &model.Field{Name: "missing", Type: model.Reference{Target: "Ghost"}})
	doc.Entities.Set("A", a)
	err := svc.Validate(doc)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !stderrors.Is(err, errors.ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

func TestServiceValidateBadNamedType(t *testing.T) {
	svc := datjit.NewDefault()
	doc := model.NewDocument()
	a := model.NewEntity("A")
	a.Fields.Set("status", &model.Field{Name: "status", Type: model.NamedType{Name: "NoSuchType"}})
	doc.Entities.Set("A", a)
	if err := svc.Validate(doc); !stderrors.Is(err, errors.ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

func TestServiceValidateEmptyEnum(t *testing.T) {
	svc := datjit.NewDefault()
	doc := model.NewDocument()
	doc.Enums.Set("Status", model.EnumDef{Name: "Status"}) // no variants
	a := model.NewEntity("A")
	a.Fields.Set("status", &model.Field{Name: "status", Type: model.NamedType{Name: "Status"}})
	doc.Entities.Set("A", a)
	if err := svc.Validate(doc); !stderrors.Is(err, errors.ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

func TestServiceValidateBadRuleExpr(t *testing.T) {
	svc := datjit.NewDefault()
	doc := model.NewDocument()
	doc.Entities.Set("A", model.NewEntity("A"))
	doc.Rules = []model.Rule{{Expr: "this is not a valid expr ))"}}
	if err := svc.Validate(doc); !stderrors.Is(err, errors.ErrValidation) {
		t.Fatalf("want ErrValidation for bad rule, got %v", err)
	}
}

func TestServiceValidateGoodDoc(t *testing.T) {
	svc := datjit.NewDefault()
	doc, err := svc.Parse(strings.NewReader(miniSchemaWithRef), "refs.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Validate(doc); err != nil {
		t.Fatalf("valid doc rejected: %v", err)
	}
}

func TestServiceInspect(t *testing.T) {
	svc := datjit.NewDefault()
	doc, err := svc.Parse(strings.NewReader(miniSchemaWithRef), "refs.yaml")
	if err != nil {
		t.Fatal(err)
	}
	insp, err := svc.Inspect(doc)
	if err != nil {
		t.Fatal(err)
	}
	if insp.Domain != "test_service_refs" {
		t.Fatalf("wrong domain: %q", insp.Domain)
	}
	if insp.EntityCount != 2 {
		t.Fatalf("want 2 entities, got %d", insp.EntityCount)
	}
	if len(insp.Entities) != 2 {
		t.Fatalf("want 2 summaries, got %d", len(insp.Entities))
	}
	// User: 2 fields, no deps, volume 2.
	u := insp.Entities[0]
	if u.Name != "User" || u.FieldCount != 2 || len(u.Dependencies) != 0 {
		t.Fatalf("User summary wrong: %+v", u)
	}
	if u.VolumePlan.Exact != 2 {
		t.Fatalf("User volume should be 2, got %+v", u.VolumePlan)
	}
	// Order: 3 fields, depends on User.
	o := insp.Entities[1]
	if o.Name != "Order" || o.FieldCount != 3 {
		t.Fatalf("Order summary wrong: %+v", o)
	}
	if len(o.Dependencies) != 1 || o.Dependencies[0] != "User" {
		t.Fatalf("Order deps wrong: %v", o.Dependencies)
	}
}

func TestServiceInspectDefaultVolume(t *testing.T) {
	svc := datjit.NewDefault()
	doc := model.NewDocument()
	doc.Entities.Set("X", model.NewEntity("X"))
	insp, err := svc.Inspect(doc)
	if err != nil {
		t.Fatal(err)
	}
	if insp.Entities[0].VolumePlan.Exact != 10 {
		t.Fatalf("default volume should be 10, got %+v", insp.Entities[0].VolumePlan)
	}
}

func TestOptionWithSeed(t *testing.T) {
	// Two services with the same seed override must produce identical JSON.
	s1, err := datjit.New(datjit.WithSeed(1234))
	if err != nil {
		t.Fatal(err)
	}
	s2, err := datjit.New(datjit.WithSeed(1234))
	if err != nil {
		t.Fatal(err)
	}
	out1 := runPipeline(t, s1, miniSchema)
	out2 := runPipeline(t, s2, miniSchema)
	if !bytes.Equal(out1, out2) {
		t.Fatalf("same seed should yield identical JSON:\n%s\n\n%s", out1, out2)
	}

	// Different seeds should (almost always) differ; with 3 rows of person
	// corpus lookups the probability of collision is effectively zero.
	s3, err := datjit.New(datjit.WithSeed(9999))
	if err != nil {
		t.Fatal(err)
	}
	out3 := runPipeline(t, s3, miniSchema)
	if bytes.Equal(out1, out3) {
		t.Fatalf("different seeds produced identical JSON:\n%s", out1)
	}
}

func TestOptionWithWriter(t *testing.T) {
	// Sanity-check that the default writer set is registered.
	svc := datjit.NewDefault()
	got := svc.Formats()
	wanted := []string{"csv", "json", "ndjson", "sql", "yaml"}
	for _, w := range wanted {
		found := false
		for _, g := range got {
			if g == w {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing writer %q (have %v)", w, got)
		}
	}
}

func TestWriteUnknownFormat(t *testing.T) {
	svc := datjit.NewDefault()
	doc, err := svc.Parse(strings.NewReader(miniSchema), "mini.yaml")
	if err != nil {
		t.Fatal(err)
	}
	ds, err := svc.Generate(doc)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	err = svc.Write(ds, doc, "toml", &buf, datjit.WriteOpts{})
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
	if !stderrors.Is(err, errors.ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

// runPipeline parses schemaSrc, runs Generate, and writes pretty JSON
// without relying on any fixture.
func runPipeline(t *testing.T, svc *datjit.Service, schemaSrc string) []byte {
	t.Helper()
	doc, err := svc.Parse(strings.NewReader(schemaSrc), "schema.yaml")
	if err != nil {
		t.Fatal(err)
	}
	ds, err := svc.Generate(doc)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := svc.Write(ds, doc, "json", &buf, datjit.WriteOpts{Pretty: true}); err != nil {
		t.Fatal(err)
	}
	// Round-trip through encoding/json so any map-ordering drift shows up.
	var probe any
	if err := json.Unmarshal(buf.Bytes(), &probe); err != nil {
		t.Fatalf("pipeline output isn't valid JSON: %v\n%s", err, buf.String())
	}
	return append([]byte(nil), buf.Bytes()...)
}
