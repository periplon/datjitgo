package datjit

import (
	"testing"

	"github.com/periplon/datjitgo/core/model"
)

// polyEntity builds an entity with a single field of the given type, plus any
// extra pre-existing fields (by name) inserted before it.
func polyEntity(t *testing.T, fieldName string, typ model.TypeExpr, pre ...string) *model.Entity {
	t.Helper()
	e := model.NewEntity("Comment")
	for _, p := range pre {
		e.Fields.Set(p, &model.Field{Name: p, Type: model.Primitive{Kind: model.PrimString}})
	}
	e.Fields.Set(fieldName, &model.Field{Name: fieldName, Type: typ})
	return e
}

func refUnion(targets ...string) model.Union {
	u := model.Union{}
	for _, tg := range targets {
		u.Variants = append(u.Variants, model.Reference{Target: tg})
	}
	return u
}

func docWith(e *model.Entity) *model.Document {
	d := model.NewDocument()
	d.Entities.Set(e.Name, e)
	return d
}

func TestNormalize_InsertsDiscriminatorAfterSource(t *testing.T) {
	e := polyEntity(t, "author", refUnion("User", "Org"), "id", "body")
	normalizePolymorphicReferences(docWith(e))

	keys := e.Fields.Keys()
	want := []string{"id", "body", "author", "author_type"}
	if len(keys) != len(want) {
		t.Fatalf("field order = %v, want %v", keys, want)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("field order = %v, want %v", keys, want)
		}
	}

	src, _ := e.Fields.Get("author")
	if src.Discriminator != "author_type" {
		t.Fatalf("source Discriminator = %q, want author_type", src.Discriminator)
	}
	disc, ok := e.Fields.Get("author_type")
	if !ok {
		t.Fatal("author_type field missing")
	}
	if disc.DiscriminatorFor != "author" {
		t.Fatalf("DiscriminatorFor = %q, want author", disc.DiscriminatorFor)
	}
	if p, ok := disc.Type.(model.Primitive); !ok || p.Kind != model.PrimString {
		t.Fatalf("discriminator type = %T, want string Primitive", disc.Type)
	}
}

func TestNormalize_NullableWrappedUnion(t *testing.T) {
	e := polyEntity(t, "author", model.Nullable{Inner: refUnion("User", "Org")}, "id")
	normalizePolymorphicReferences(docWith(e))
	src, _ := e.Fields.Get("author")
	if src.Discriminator != "author_type" {
		t.Fatalf("nullable-wrapped polymorphic union not detected: Discriminator=%q", src.Discriminator)
	}
	if !e.Fields.Has("author_type") {
		t.Fatal("author_type missing for nullable-wrapped union")
	}
}

func TestNormalize_NonPolymorphicUntouched(t *testing.T) {
	cases := map[string]model.TypeExpr{
		"scalar union":     model.Union{Variants: []model.TypeExpr{model.Primitive{Kind: model.PrimString}, model.Primitive{Kind: model.PrimInt}}},
		"single reference": model.Reference{Target: "User"},
		"ref or scalar":    model.Union{Variants: []model.TypeExpr{model.Reference{Target: "User"}, model.Primitive{Kind: model.PrimString}}},
	}
	for name, typ := range cases {
		t.Run(name, func(t *testing.T) {
			e := polyEntity(t, "f", typ, "id")
			normalizePolymorphicReferences(docWith(e))
			if e.Fields.Len() != 2 {
				t.Fatalf("expected no discriminator added; fields=%v", e.Fields.Keys())
			}
			src, _ := e.Fields.Get("f")
			if src.Discriminator != "" {
				t.Fatalf("Discriminator unexpectedly set to %q", src.Discriminator)
			}
		})
	}
}

func TestNormalize_Idempotent(t *testing.T) {
	e := polyEntity(t, "author", refUnion("User", "Org"), "id")
	d := docWith(e)
	normalizePolymorphicReferences(d)
	normalizePolymorphicReferences(d)
	n := 0
	e.Fields.Each(func(_ string, f *model.Field) bool {
		if f.DiscriminatorFor != "" {
			n++
		}
		return true
	})
	if n != 1 {
		t.Fatalf("idempotency broken: %d discriminator fields", n)
	}
}

func TestNormalize_NameCollisionAvoided(t *testing.T) {
	e := model.NewEntity("Comment")
	e.Fields.Set("id", &model.Field{Name: "id", Type: model.Primitive{Kind: model.PrimString}})
	e.Fields.Set("author", &model.Field{Name: "author", Type: refUnion("User", "Org")})
	// Pre-existing field already named "author_type".
	e.Fields.Set("author_type", &model.Field{Name: "author_type", Type: model.Primitive{Kind: model.PrimString}})
	normalizePolymorphicReferences(docWith(e))

	src, _ := e.Fields.Get("author")
	if src.Discriminator != "author_type_2" {
		t.Fatalf("collision name = %q, want author_type_2", src.Discriminator)
	}
	if !e.Fields.Has("author_type_2") {
		t.Fatal("author_type_2 not inserted")
	}
}
