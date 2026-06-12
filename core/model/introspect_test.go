package model

import "testing"

func TestRenderType(t *testing.T) {
	cases := []struct {
		name string
		in   TypeExpr
		want string
	}{
		{"primitive", Primitive{Kind: PrimInt}, "int"},
		{"primitive-params", Primitive{Kind: PrimDecimal, Params: []int{10, 2}}, "decimal(10, 2)"},
		{"string-maxlen", Primitive{Kind: PrimString, Params: []int{64}}, "string(64)"},
		{"semantic", Semantic{Namespace: "person", Tag: "full"}, "person.full"},
		{"semantic-bare", Semantic{Namespace: "email"}, "email"},
		{"semantic-params", Semantic{Namespace: "currency", Tag: "usd", Params: []string{"min", "max"}}, "currency.usd(min, max)"},
		{"enum-inline", EnumInline{Values: []string{"a", "b", "c"}}, "enum(a, b, c)"},
		{"named", NamedType{Name: "Address"}, "Address"},
		{"reference", Reference{Target: "User"}, "->User"},
		{"reference-optional", Reference{Target: "User", Optional: true}, "->User?"},
		{"reference-many", Reference{Target: "Tag", Many: true}, "->[Tag]"},
		{"many-to-many", Reference{Target: "Tag", ManyToMany: true}, "<->Tag"},
		{"self", Reference{Target: "self"}, "->self"},
		{"list", List{Element: Primitive{Kind: PrimInt}}, "[int]"},
		{"map", Map{Key: Primitive{Kind: PrimString}, Value: Primitive{Kind: PrimInt}}, "{string: int}"},
		{"tuple", Tuple{Elements: []TypeExpr{Primitive{Kind: PrimInt}, Primitive{Kind: PrimString}}}, "(int, string)"},
		{"nullable", Nullable{Inner: Primitive{Kind: PrimInt}}, "int?"},
		{"union", Union{Variants: []TypeExpr{Primitive{Kind: PrimString}, Primitive{Kind: PrimInt}}}, "string | int"},
		{
			"polymorphic",
			Union{Variants: []TypeExpr{Reference{Target: "User"}, Reference{Target: "Org"}}},
			"->User | ->Org",
		},
		{"nil", nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := RenderType(c.in); got != c.want {
				t.Fatalf("RenderType(%s) = %q, want %q", c.name, got, c.want)
			}
		})
	}
}

func TestRenderDecorator(t *testing.T) {
	cases := []struct {
		name string
		in   Decorator
		want string
	}{
		{"bare", Decorator{Name: "unique"}, "@unique"},
		{
			"raw-args",
			Decorator{Name: "range", Args: []DecoratorArg{{Kind: ArgRange, Raw: "18..65"}}},
			"@range(18..65)",
		},
		{
			"kv-no-raw",
			Decorator{Name: "dist", Args: []DecoratorArg{
				{Kind: ArgIdent, Ident: "normal"},
				{Kind: ArgKV, Key: "mu", Value: "35"},
			}},
			"@dist(normal, mu=35)",
		},
		{
			"range-no-raw",
			Decorator{Name: "range", Args: []DecoratorArg{{Kind: ArgRange, From: "1", To: "10", HiExcl: true}}},
			"@range(1..<10)",
		},
		{
			"literal-no-raw",
			Decorator{Name: "default", Args: []DecoratorArg{{Kind: ArgLiteral, Literal: int64(7)}}},
			"@default(7)",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := RenderDecorator(c.in); got != c.want {
				t.Fatalf("RenderDecorator(%s) = %q, want %q", c.name, got, c.want)
			}
		})
	}
}

func TestRenderDecoratorsEmpty(t *testing.T) {
	if got := RenderDecorators(nil); got != nil {
		t.Fatalf("RenderDecorators(nil) = %v, want nil", got)
	}
}

func TestSchemaDiffHelpers(t *testing.T) {
	var nilDiff *SchemaDiff
	if nilDiff.Breaking() {
		t.Fatal("nil diff must not be breaking")
	}
	if !nilDiff.Empty() {
		t.Fatal("nil diff must be empty")
	}

	d := &SchemaDiff{Changes: []SchemaChange{{Kind: "field-added"}}}
	if d.Breaking() {
		t.Fatal("non-breaking change must not report Breaking")
	}
	if d.Empty() {
		t.Fatal("diff with a change must not be empty")
	}

	d.Changes = append(d.Changes, SchemaChange{Kind: "entity-removed", Breaking: true})
	if !d.Breaking() {
		t.Fatal("diff with a breaking change must report Breaking")
	}
}
