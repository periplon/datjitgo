package parser

import (
	"strings"
	"testing"

	"github.com/periplon/datjitgo/core/model"
)

// TestParseDirtyFieldDecorator checks the field-level @dirty shorthand:
// rate= classifies as a KV arg and the kind names as bare idents.
func TestParseDirtyFieldDecorator(t *testing.T) {
	doc, err := New().Parse(strings.NewReader(`
domain: d
entities:
  User:
    email: email @unique @dirty(rate=0.1, typo, whitespace)
`), "test")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ent, _ := doc.Entities.Get("User")
	f, _ := ent.Fields.Get("email")
	d := model.FindDecorator(f.Decorators, "dirty")
	if d == nil {
		t.Fatalf("@dirty not parsed: %+v", f.Decorators)
	}
	if got, ok := d.ArgByKey("rate"); !ok || got != "0.1" {
		t.Fatalf("rate arg = %q, ok=%v", got, ok)
	}
	var idents []string
	for _, a := range d.Args {
		if a.Kind == model.ArgIdent {
			idents = append(idents, a.Ident)
		}
	}
	if len(idents) != 2 || idents[0] != "typo" || idents[1] != "whitespace" {
		t.Fatalf("kind idents = %v", idents)
	}
	if !model.HasDecorator(f.Decorators, "unique") {
		t.Fatal("@unique lost next to @dirty")
	}
}

// TestParseDirtyEntityMeta checks @dirty rides the existing _meta decorator
// mechanism (the same one @llm uses), including the bare `null` kind ident.
func TestParseDirtyEntityMeta(t *testing.T) {
	doc, err := New().Parse(strings.NewReader(`
domain: d
entities:
  User:
    _meta: "@dirty(rate=0.02, typo, case, null)"
    id: uuid @primary
    name: person.full
`), "test")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ent, _ := doc.Entities.Get("User")
	d := model.FindDecorator(ent.Meta, "dirty")
	if d == nil {
		t.Fatalf("entity meta @dirty not parsed: %+v", ent.Meta)
	}
	if got, ok := d.ArgByKey("rate"); !ok || got != "0.02" {
		t.Fatalf("rate arg = %q, ok=%v", got, ok)
	}
	var idents []string
	for _, a := range d.Args {
		if a.Kind == model.ArgIdent {
			idents = append(idents, a.Ident)
		}
	}
	want := []string{"typo", "case", "null"}
	if len(idents) != len(want) {
		t.Fatalf("kind idents = %v, want %v", idents, want)
	}
	for i := range want {
		if idents[i] != want[i] {
			t.Fatalf("kind idents = %v, want %v", idents, want)
		}
	}
	// _meta must not leak into the field list.
	if ent.Fields.Has("_meta") {
		t.Fatal("_meta leaked into fields")
	}
}
