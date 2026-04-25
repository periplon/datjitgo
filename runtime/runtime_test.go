package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"

	coreerrors "github.com/jmcarbo/datjitgo/core/errors"
	"github.com/jmcarbo/datjitgo/core/model"
	"github.com/jmcarbo/datjitgo/core/value"
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
