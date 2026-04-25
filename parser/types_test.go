package parser

import (
	"testing"

	"github.com/jmcarbo/datjitgo/core/model"
)

func mustParseType(t *testing.T, src string) model.TypeExpr {
	t.Helper()
	te, err := parseTypeExpr(src)
	if err != nil {
		t.Fatalf("parseTypeExpr(%q) error: %v", src, err)
	}
	return te
}

func TestParseTypeExpr_Primitives(t *testing.T) {
	cases := map[string]model.PrimKind{
		"string":   model.PrimString,
		"int":      model.PrimInt,
		"float":    model.PrimFloat,
		"bool":     model.PrimBool,
		"datetime": model.PrimDatetime,
		"date":     model.PrimDate,
		"time":     model.PrimTime,
		"duration": model.PrimDuration,
		"uuid":     model.PrimUUID,
		"bytes":    model.PrimBytes,
	}
	for src, want := range cases {
		te := mustParseType(t, src)
		p, ok := te.(model.Primitive)
		if !ok {
			t.Fatalf("%s: not a primitive: %T", src, te)
		}
		if p.Kind != want {
			t.Fatalf("%s: kind=%v want %v", src, p.Kind, want)
		}
		if len(p.Params) != 0 {
			t.Fatalf("%s: unexpected params %+v", src, p.Params)
		}
	}
}

func TestParseTypeExpr_ParameterizedInt(t *testing.T) {
	te := mustParseType(t, "int(32)")
	p := te.(model.Primitive)
	if p.Kind != model.PrimInt || len(p.Params) != 1 || p.Params[0] != 32 {
		t.Fatalf("%+v", p)
	}
}

func TestParseTypeExpr_ParameterizedString(t *testing.T) {
	te := mustParseType(t, "string(80)")
	p := te.(model.Primitive)
	if p.Kind != model.PrimString || len(p.Params) != 1 || p.Params[0] != 80 {
		t.Fatalf("%+v", p)
	}
}

func TestParseTypeExpr_ParameterizedDecimal(t *testing.T) {
	te := mustParseType(t, "decimal(10, 2)")
	p := te.(model.Primitive)
	if p.Kind != model.PrimDecimal || len(p.Params) != 2 || p.Params[0] != 10 || p.Params[1] != 2 {
		t.Fatalf("%+v", p)
	}
}

func TestParseTypeExpr_ParameterizedBytes(t *testing.T) {
	te := mustParseType(t, "bytes(64)")
	p := te.(model.Primitive)
	if p.Kind != model.PrimBytes || len(p.Params) != 1 || p.Params[0] != 64 {
		t.Fatalf("%+v", p)
	}
}

func TestParseTypeExpr_SemanticDotted(t *testing.T) {
	te := mustParseType(t, "person.full")
	s, ok := te.(model.Semantic)
	if !ok {
		t.Fatalf("not semantic: %T", te)
	}
	if s.Namespace != "person" || s.Tag != "full" {
		t.Fatalf("%+v", s)
	}
}

func TestParseTypeExpr_SemanticTopLevel(t *testing.T) {
	te := mustParseType(t, "email")
	s, ok := te.(model.Semantic)
	if !ok {
		t.Fatalf("not semantic: %T", te)
	}
	if s.Namespace != "email" || s.Tag != "" {
		t.Fatalf("%+v", s)
	}
}

func TestParseTypeExpr_SemanticWithParams(t *testing.T) {
	te := mustParseType(t, "currency(USD)")
	s, ok := te.(model.Semantic)
	if !ok {
		t.Fatalf("not semantic: %T", te)
	}
	if s.Namespace != "currency" || len(s.Params) != 1 || s.Params[0] != "USD" {
		t.Fatalf("%+v", s)
	}
}

func TestParseTypeExpr_SemanticDottedWithParams(t *testing.T) {
	te := mustParseType(t, `accounting.group("ES")`)
	s, ok := te.(model.Semantic)
	if !ok {
		t.Fatalf("not semantic: %T", te)
	}
	if s.Namespace != "accounting" || s.Tag != "group" {
		t.Fatalf("%+v", s)
	}
	if len(s.Params) != 1 || s.Params[0] != `"ES"` {
		t.Fatalf("params: %+v", s.Params)
	}
}

func TestParseTypeExpr_EnumInline(t *testing.T) {
	te := mustParseType(t, "enum(a, b, c)")
	e, ok := te.(model.EnumInline)
	if !ok {
		t.Fatalf("not enum: %T", te)
	}
	if len(e.Values) != 3 || e.Values[0] != "a" || e.Values[2] != "c" {
		t.Fatalf("%+v", e)
	}
}

func TestParseTypeExpr_ReferenceRequired(t *testing.T) {
	te := mustParseType(t, "->User")
	r, ok := te.(model.Reference)
	if !ok {
		t.Fatalf("not reference: %T", te)
	}
	if r.Target != "User" || r.Optional || r.Many || r.ManyToMany {
		t.Fatalf("%+v", r)
	}
}

func TestParseTypeExpr_ReferenceOptional(t *testing.T) {
	te := mustParseType(t, "->User?")
	r := te.(model.Reference)
	if r.Target != "User" || !r.Optional {
		t.Fatalf("%+v", r)
	}
}

func TestParseTypeExpr_ReferenceHasMany(t *testing.T) {
	te := mustParseType(t, "->[Tag]")
	r := te.(model.Reference)
	if r.Target != "Tag" || !r.Many {
		t.Fatalf("%+v", r)
	}
}

func TestParseTypeExpr_ReferenceManyToMany(t *testing.T) {
	te := mustParseType(t, "<->Tag")
	r := te.(model.Reference)
	if r.Target != "Tag" || !r.ManyToMany {
		t.Fatalf("%+v", r)
	}
}

func TestParseTypeExpr_ReferenceSelf(t *testing.T) {
	te := mustParseType(t, "->self")
	r := te.(model.Reference)
	if r.Target != "self" || r.Optional {
		t.Fatalf("%+v", r)
	}
	te = mustParseType(t, "->self?")
	r = te.(model.Reference)
	if r.Target != "self" || !r.Optional {
		t.Fatalf("%+v", r)
	}
}

func TestParseTypeExpr_NullablePrimitive(t *testing.T) {
	te := mustParseType(t, "string?")
	n, ok := te.(model.Nullable)
	if !ok {
		t.Fatalf("not nullable: %T", te)
	}
	if _, ok := n.Inner.(model.Primitive); !ok {
		t.Fatalf("inner not primitive: %T", n.Inner)
	}
}

func TestParseTypeExpr_List(t *testing.T) {
	te := mustParseType(t, "[int]")
	l, ok := te.(model.List)
	if !ok {
		t.Fatalf("not list: %T", te)
	}
	if _, ok := l.Element.(model.Primitive); !ok {
		t.Fatalf("element: %T", l.Element)
	}
}

func TestParseTypeExpr_Map(t *testing.T) {
	te := mustParseType(t, "{string: int}")
	m, ok := te.(model.Map)
	if !ok {
		t.Fatalf("not map: %T", te)
	}
	if _, ok := m.Key.(model.Primitive); !ok {
		t.Fatalf("key: %T", m.Key)
	}
	if _, ok := m.Value.(model.Primitive); !ok {
		t.Fatalf("value: %T", m.Value)
	}
}

func TestParseTypeExpr_Tuple(t *testing.T) {
	te := mustParseType(t, "(float, float)")
	tup, ok := te.(model.Tuple)
	if !ok {
		t.Fatalf("not tuple: %T", te)
	}
	if len(tup.Elements) != 2 {
		t.Fatalf("tuple arity: %+v", tup)
	}
}

func TestParseTypeExpr_Union(t *testing.T) {
	te := mustParseType(t, "string | int")
	u, ok := te.(model.Union)
	if !ok {
		t.Fatalf("not union: %T", te)
	}
	if len(u.Variants) != 2 {
		t.Fatalf("variants: %+v", u)
	}
}

func TestParseTypeExpr_NamedType(t *testing.T) {
	te := mustParseType(t, "Address")
	n, ok := te.(model.NamedType)
	if !ok {
		t.Fatalf("not named: %T", te)
	}
	if n.Name != "Address" {
		t.Fatalf("%+v", n)
	}
}

func TestParseTypeExpr_NestedList(t *testing.T) {
	te := mustParseType(t, "[string]")
	l := te.(model.List)
	p := l.Element.(model.Primitive)
	if p.Kind != model.PrimString {
		t.Fatalf("%+v", p)
	}
}

func TestParseTypeExpr_InvalidInputs(t *testing.T) {
	cases := []string{
		"",
		"string | ",
		"[]",
		"{string}",
		"{: int}",
		"{string:}",
		"(int, )",
		"->",
		"string(x)",
		"int(x)",
		"float(x)",
		"decimal(x, 2)",
		"decimal(10, x)",
		"bytes(x)",
	}
	for _, src := range cases {
		if _, err := parseTypeExpr(src); err == nil {
			t.Errorf("expected parse error for %q", src)
		}
	}
}

func TestParseTypeExpr_NullableOfList(t *testing.T) {
	te := mustParseType(t, "[string]?")
	n, ok := te.(model.Nullable)
	if !ok {
		t.Fatalf("not nullable: %T", te)
	}
	if _, ok := n.Inner.(model.List); !ok {
		t.Fatalf("inner: %T", n.Inner)
	}
}

func TestParseTypeExpr_EmptyError(t *testing.T) {
	if _, err := parseTypeExpr(""); err == nil {
		t.Fatal("expected error for empty type")
	}
	if _, err := parseTypeExpr("   "); err == nil {
		t.Fatal("expected error for blank type")
	}
}
