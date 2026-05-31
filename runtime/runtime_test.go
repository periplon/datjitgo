package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"

	coreerrors "github.com/periplon/datjitgo/core/errors"
	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/value"
)

func TestDefaultGenerateDocumentUsesRunOptions(t *testing.T) {
	ctx := context.Background()
	doc := testDocument()
	rt := NewDefault()

	ds, err := rt.GenerateDocument(ctx, doc, WithSeed(7), WithVolume("users", 2), WithLocale("en-US"))
	if err != nil {
		t.Fatalf("GenerateDocument returned error: %v", err)
	}

	rows, ok := ds.Entities.Get("users")
	if !ok {
		t.Fatal("generated dataset missing users entity")
	}
	if got := len(rows); got != 2 {
		t.Fatalf("generated rows = %d, want 2", got)
	}
	if doc.Generation.Seed != nil || doc.Generation.Locale != "" {
		t.Fatalf("GenerateDocument mutated source generation config: %#v", doc.Generation)
	}
	if got := doc.Volume["users"].Exact; got != 0 {
		t.Fatalf("GenerateDocument mutated source volume = %d, want 0", got)
	}
}

func TestNewConstructsRuntimeWithOptions(t *testing.T) {
	rt, err := New()
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if rt == nil || rt.service == nil {
		t.Fatalf("New runtime = %#v, want service-backed runtime", rt)
	}
}

func TestDefaultGenerateEntityReturnsOnlyRequestedEntity(t *testing.T) {
	ctx := context.Background()
	doc := testDocument()
	rt := NewDefault()

	rows, err := rt.GenerateEntity(ctx, doc, "users", WithSeed(11), WithVolume("users", 3))
	if err != nil {
		t.Fatalf("GenerateEntity returned error: %v", err)
	}
	if got := len(rows); got != 3 {
		t.Fatalf("generated rows = %d, want 3", got)
	}
	for _, row := range rows {
		if _, ok := row.Get("email"); !ok {
			t.Fatalf("row missing generated email: %#v", row)
		}
	}
}

func TestDefaultGenerateRowsUsesRowsRequest(t *testing.T) {
	ctx := context.Background()
	doc := testDocument()
	rt := NewDefault()

	rows, err := rt.GenerateRows(ctx, RowsRequest{
		Document: doc,
		Entity:   "users",
		Seed:     int64Ptr(13),
		Locale:   "en-US",
		Volumes:  map[string]int{"users": 4},
	})
	if err != nil {
		t.Fatalf("GenerateRows returned error: %v", err)
	}
	if got := len(rows); got != 4 {
		t.Fatalf("generated rows = %d, want 4", got)
	}
}

func TestDefaultGenerateDocumentCanFilterEntity(t *testing.T) {
	ctx := context.Background()
	doc := testDocument()
	doc.Entities.Set("accounts", entityWithField("accounts", "name", model.Primitive{Kind: model.PrimString}))

	ds, err := NewDefault().GenerateDocument(ctx, doc, WithEntity("users"))
	if err != nil {
		t.Fatalf("GenerateDocument returned error: %v", err)
	}
	if ds.Entities.Len() != 1 {
		t.Fatalf("filtered dataset entity count = %d, want 1", ds.Entities.Len())
	}
	if !ds.Entities.Has("users") {
		t.Fatal("filtered dataset missing users entity")
	}
}

func TestDefaultGenerateValueSupportsSemanticAndSeed(t *testing.T) {
	ctx := context.Background()
	rt := NewDefault()
	req := ValueRequest{Semantic: "email", Seed: int64Ptr(42)}

	first, err := rt.GenerateValue(ctx, req)
	if err != nil {
		t.Fatalf("GenerateValue returned error: %v", err)
	}
	second, err := rt.GenerateValue(ctx, req)
	if err != nil {
		t.Fatalf("GenerateValue returned error on repeat: %v", err)
	}
	if first.Kind != value.KindString || !strings.Contains(first.S, "@") {
		t.Fatalf("generated semantic value = %#v, want email string", first)
	}
	if first.Kind != second.Kind || first.S != second.S {
		t.Fatalf("GenerateValue with same seed was not deterministic: %#v != %#v", first, second)
	}

	tagged, err := rt.GenerateValue(ctx, ValueRequest{Semantic: "person.first", UniqueKey: "first_name", Seed: int64Ptr(42)})
	if err != nil {
		t.Fatalf("GenerateValue with tagged semantic returned error: %v", err)
	}
	if tagged.Kind != value.KindString || tagged.S == "" {
		t.Fatalf("tagged semantic value = %#v, want string", tagged)
	}
}

func TestDefaultGenerateValueSupportsPrimitiveDecorators(t *testing.T) {
	ctx := context.Background()
	rt := NewDefault()
	got, err := rt.GenerateValue(ctx, ValueRequest{
		Type: model.Primitive{Kind: model.PrimInt},
		Decorators: []model.Decorator{{
			Name: "range",
			Args: []model.DecoratorArg{{Kind: model.ArgRange, From: "18", To: "65"}},
		}},
		Seed: int64Ptr(99),
	})
	if err != nil {
		t.Fatalf("GenerateValue returned error: %v", err)
	}
	if got.Kind != value.KindInt || got.I < 18 || got.I > 65 {
		t.Fatalf("generated value = %#v, want int in [18,65]", got)
	}
}

func TestDefaultGenerateValueReturnsValidationError(t *testing.T) {
	ctx := context.Background()
	_, err := NewDefault().GenerateValue(ctx, ValueRequest{Type: model.NamedType{Name: "missing"}})
	if err == nil {
		t.Fatal("GenerateValue returned nil error, want validation error")
	}
	if !errors.Is(err, coreerrors.ErrValidation) {
		t.Fatalf("GenerateValue error = %v, want validation kind", err)
	}
}

func TestDefaultValidationAndContextErrors(t *testing.T) {
	ctx := context.Background()
	doc := testDocument()

	var nilRuntime *Default
	if _, err := nilRuntime.GenerateDocument(ctx, doc); !errors.Is(err, coreerrors.ErrValidation) {
		t.Fatalf("nil runtime error = %v, want validation", err)
	}
	if _, err := NewDefault().GenerateDocument(ctx, nil); !errors.Is(err, coreerrors.ErrValidation) {
		t.Fatalf("nil document error = %v, want validation", err)
	}
	if _, err := NewDefault().GenerateDocument(ctx, doc, WithEntity("missing")); !errors.Is(err, coreerrors.ErrValidation) {
		t.Fatalf("missing entity error = %v, want validation", err)
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := NewDefault().GenerateDocument(canceled, doc); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled GenerateDocument error = %v, want context.Canceled", err)
	}
	if _, err := NewDefault().GenerateValue(canceled, ValueRequest{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled GenerateValue error = %v, want context.Canceled", err)
	}
}

func TestRunOptionAndCloneHelpers(t *testing.T) {
	cfg := applyRunOptions([]RunOption{
		nil,
		WithSeed(3),
		WithLocale("ca-ES"),
		WithVolumes(nil),
		WithVolumes(map[string]int{"users": 2, "accounts": 1}),
		WithEntity("users"),
	})
	if cfg.seed == nil || *cfg.seed != 3 || cfg.locale != "ca-ES" || cfg.entity != "users" {
		t.Fatalf("config = %#v", cfg)
	}
	if cfg.volumes["users"] != 2 || cfg.volumes["accounts"] != 1 {
		t.Fatalf("volumes = %#v", cfg.volumes)
	}

	if got := semanticType("person.first"); got.Namespace != "person" || got.Tag != "first" {
		t.Fatalf("semanticType tagged = %#v", got)
	}
	if got := semanticType("email"); got.Namespace != "email" || got.Tag != "" {
		t.Fatalf("semanticType namespace = %#v", got)
	}
	if got := filterDataset(nil, "users"); got == nil || got.Entities.Len() != 0 {
		t.Fatalf("filterDataset nil = %#v", got)
	}
	if cloneEntity(nil) != nil {
		t.Fatal("cloneEntity(nil) returned non-nil")
	}
	if cloneField(nil) != nil {
		t.Fatal("cloneField(nil) returned non-nil")
	}
	if cloneDecorators(nil) != nil {
		t.Fatal("cloneDecorators(nil) returned non-nil")
	}
	if cloneInt64Ptr(nil) != nil {
		t.Fatal("cloneInt64Ptr(nil) returned non-nil")
	}
}

func TestCompileFuncImplementsDocumentCompiler(t *testing.T) {
	ctx := context.Background()
	doc := testDocument()
	var compiler DocumentCompiler = CompileFunc(func(gotCtx context.Context, src any) (*model.Document, error) {
		if gotCtx != ctx {
			t.Fatal("CompileFunc received different context")
		}
		if src != "schema" {
			t.Fatalf("CompileFunc src = %v, want schema", src)
		}
		return doc, nil
	})

	got, err := compiler.Compile(ctx, "schema")
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}
	if got != doc {
		t.Fatal("Compile did not return compiler document")
	}
}

func testDocument() *model.Document {
	doc := model.NewDocument()
	doc.Entities.Set("users", entityWithField("users", "email", model.Semantic{Namespace: "email"}))
	return doc
}

func entityWithField(entity, field string, typ model.TypeExpr) *model.Entity {
	ent := model.NewEntity(entity)
	ent.Fields.Set(field, &model.Field{Name: field, Type: typ})
	return ent
}

func int64Ptr(v int64) *int64 {
	return &v
}
