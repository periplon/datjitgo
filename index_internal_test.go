package datjit

import (
	"testing"

	"github.com/periplon/datjitgo/core/model"
)

func userWithField(fields ...string) *model.Entity {
	e := model.NewEntity("User")
	for _, f := range fields {
		e.Fields.Set(f, &model.Field{Name: f, Type: model.Primitive{Kind: model.PrimString}})
	}
	return e
}

func TestCheckIndexesDuplicateName(t *testing.T) {
	e := userWithField("email", "name")
	e.Indexes = []model.Index{
		{Name: "dup", Fields: []string{"email"}, Source: "manual"},
		{Name: "dup", Fields: []string{"name"}, Source: "manual"},
	}
	if err := checkIndexes("User", e); err == nil {
		t.Fatal("want duplicate-name error, got nil")
	}
}

func TestCheckIndexesSkipsInferred(t *testing.T) {
	e := userWithField("email")
	// An inferred index referencing a field that doesn't exist must NOT raise
	// a validation error — inferred indexes are trusted, not validated.
	e.Indexes = []model.Index{
		{Name: "idx_user_ghost", Fields: []string{"ghost"}, Source: "inferred"},
	}
	if err := checkIndexes("User", e); err != nil {
		t.Fatalf("inferred indexes should be skipped, got %v", err)
	}
}

func TestCheckIndexesManualUnknownField(t *testing.T) {
	e := userWithField("email")
	e.Indexes = []model.Index{
		{Name: "bad", Fields: []string{"missing"}, Source: "manual"},
	}
	if err := checkIndexes("User", e); err == nil {
		t.Fatal("want unknown-field error, got nil")
	}
}

func TestNormalizeIndexesOrderAndDedup(t *testing.T) {
	e := model.NewEntity("User")
	e.Fields.Set("id", &model.Field{Name: "id", Type: model.Primitive{Kind: model.PrimUUID}, Decorators: []model.Decorator{{Name: "primary"}}})
	e.Fields.Set("email", &model.Field{Name: "email", Type: model.Primitive{Kind: model.PrimString}, Decorators: []model.Decorator{{Name: "unique"}}})
	e.Fields.Set("org", &model.Field{Name: "org", Type: model.Reference{Target: "Org"}})
	// Manual index already covering [org] — inference must skip it.
	e.Indexes = []model.Index{{Name: "my_org", Fields: []string{"org"}, Source: "manual"}}

	normalizeEntityIndexes(e)

	// Expect: manual my_org kept; inferred email unique added; inferred org dropped.
	if len(e.Indexes) != 2 {
		t.Fatalf("want 2 indexes, got %d: %+v", len(e.Indexes), e.Indexes)
	}
	if e.Indexes[0].Name != "my_org" || e.Indexes[0].Source != "manual" {
		t.Fatalf("manual index should remain first: %+v", e.Indexes[0])
	}
	got := e.Indexes[1]
	if got.Name != "idx_user_email_uniq" || !got.Unique || got.Source != "inferred" {
		t.Fatalf("inferred unique email index wrong: %+v", got)
	}
	// No inferred index on [org] (deduped against manual).
	for _, idx := range e.Indexes {
		if idx.Source == "inferred" && len(idx.Fields) == 1 && idx.Fields[0] == "org" {
			t.Fatalf("inferred org index should have been deduped: %+v", idx)
		}
	}
}
