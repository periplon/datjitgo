package model

// TypeExpr is the sealed interface for DDL type expressions. Concrete
// implementations all live in this package.
type TypeExpr interface{ typeExpr() }

// PrimKind enumerates the datjit primitive type families.
type PrimKind int

// The Prim constants enumerate the primitive type families.
const (
	PrimString PrimKind = iota
	PrimInt
	PrimFloat
	PrimBool
	PrimDatetime
	PrimDate
	PrimTime
	PrimDuration
	PrimUUID
	PrimBytes
	PrimDecimal
	PrimNull
	PrimAny
)

func (k PrimKind) String() string {
	switch k {
	case PrimString:
		return "string"
	case PrimInt:
		return "int"
	case PrimFloat:
		return "float"
	case PrimBool:
		return "bool"
	case PrimDatetime:
		return "datetime"
	case PrimDate:
		return "date"
	case PrimTime:
		return "time"
	case PrimDuration:
		return "duration"
	case PrimUUID:
		return "uuid"
	case PrimBytes:
		return "bytes"
	case PrimDecimal:
		return "decimal"
	case PrimNull:
		return "null"
	case PrimAny:
		return "any"
	}
	return "?"
}

// Primitive is a bare primitive type. Params carries bit-width, maxlen, or
// decimal(precision, scale).
type Primitive struct {
	Kind   PrimKind
	Params []int
}

func (Primitive) typeExpr() {}

// Semantic represents dot-namespaced semantic tags like person.full or email.
// Tag may be empty for top-level semantic types like "email" — in that case
// Namespace holds the full tag and Tag is "".
type Semantic struct {
	Namespace string
	Tag       string
	Params    []string
}

func (Semantic) typeExpr() {}

// EnumInline is an inline enum(...) type.
type EnumInline struct {
	Values []string
}

func (EnumInline) typeExpr() {}

// NamedType refers to a type defined under the top-level types: section or
// an enum defined under enums:.
type NamedType struct {
	Name string
}

func (NamedType) typeExpr() {}

// Reference points to another entity. Target == "self" for self-references.
// Many is true for list references (->[Tag]); ManyToMany for <->Tag.
type Reference struct {
	Target     string
	Optional   bool
	Many       bool
	ManyToMany bool
}

func (Reference) typeExpr() {}

// List is [T].
type List struct{ Element TypeExpr }

func (List) typeExpr() {}

// Map is {K: V}.
type Map struct {
	Key   TypeExpr
	Value TypeExpr
}

func (Map) typeExpr() {}

// Tuple is (T1, T2, ...).
type Tuple struct{ Elements []TypeExpr }

func (Tuple) typeExpr() {}

// Nullable is T?.
type Nullable struct{ Inner TypeExpr }

func (Nullable) typeExpr() {}

// Union is T1 | T2 | ... .
type Union struct{ Variants []TypeExpr }

func (Union) typeExpr() {}
