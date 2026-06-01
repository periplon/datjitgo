package model

import "testing"

func TestTypeExprSealed(_ *testing.T) {
	var _ TypeExpr = Primitive{Kind: PrimString}
	var _ TypeExpr = Semantic{Namespace: "person", Tag: "full"}
	var _ TypeExpr = EnumInline{Values: []string{"a", "b"}}
	var _ TypeExpr = NamedType{Name: "Address"}
	var _ TypeExpr = Reference{Target: "User", Optional: true}
	var _ TypeExpr = List{Element: Primitive{Kind: PrimInt}}
	var _ TypeExpr = Map{Key: Primitive{Kind: PrimString}, Value: Primitive{Kind: PrimInt}}
	var _ TypeExpr = Tuple{Elements: []TypeExpr{Primitive{Kind: PrimInt}}}
	var _ TypeExpr = Nullable{Inner: Primitive{Kind: PrimInt}}
	var _ TypeExpr = Union{Variants: []TypeExpr{Primitive{Kind: PrimString}, Primitive{Kind: PrimInt}}}
}

func TestTypeExprMarkerMethods(_ *testing.T) {
	Primitive{Kind: PrimString}.typeExpr()
	Semantic{Namespace: "person", Tag: "full"}.typeExpr()
	EnumInline{Values: []string{"a", "b"}}.typeExpr()
	NamedType{Name: "Address"}.typeExpr()
	Reference{Target: "User", Optional: true}.typeExpr()
	List{Element: Primitive{Kind: PrimInt}}.typeExpr()
	Map{Key: Primitive{Kind: PrimString}, Value: Primitive{Kind: PrimInt}}.typeExpr()
	Tuple{Elements: []TypeExpr{Primitive{Kind: PrimInt}}}.typeExpr()
	Nullable{Inner: Primitive{Kind: PrimInt}}.typeExpr()
	Union{Variants: []TypeExpr{Primitive{Kind: PrimString}, Primitive{Kind: PrimInt}}}.typeExpr()
}

func TestPrimKindString(t *testing.T) {
	cases := []struct {
		k    PrimKind
		want string
	}{
		{PrimString, "string"},
		{PrimInt, "int"},
		{PrimFloat, "float"},
		{PrimBool, "bool"},
		{PrimDatetime, "datetime"},
		{PrimDate, "date"},
		{PrimTime, "time"},
		{PrimDuration, "duration"},
		{PrimUUID, "uuid"},
		{PrimBytes, "bytes"},
		{PrimDecimal, "decimal"},
		{PrimNull, "null"},
		{PrimAny, "any"},
		{PrimKind(99), "?"},
	}
	for _, c := range cases {
		if got := c.k.String(); got != c.want {
			t.Fatalf("%d: got %q want %q", c.k, got, c.want)
		}
	}
}
