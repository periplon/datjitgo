package model

import "testing"

func TestTypeExprSealed(t *testing.T) {
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

func TestPrimKindString(t *testing.T) {
	cases := []struct {
		k    PrimKind
		want string
	}{
		{PrimString, "string"},
		{PrimInt, "int"},
		{PrimBool, "bool"},
		{PrimUUID, "uuid"},
		{PrimDecimal, "decimal"},
	}
	for _, c := range cases {
		if got := c.k.String(); got != c.want {
			t.Fatalf("%d: got %q want %q", c.k, got, c.want)
		}
	}
}
