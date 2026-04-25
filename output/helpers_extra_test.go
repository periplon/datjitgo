package output

import (
	"strings"
	"testing"

	"github.com/jmcarbo/datjitgo/core/model"
	"github.com/jmcarbo/datjitgo/core/value"
)

func TestSQLTypeHelpersCoverDialects(t *testing.T) {
	cases := []model.TypeExpr{
		model.Nullable{Inner: model.Primitive{Kind: model.PrimInt}},
		model.Primitive{Kind: model.PrimFloat},
		model.Primitive{Kind: model.PrimBool},
		model.Primitive{Kind: model.PrimDatetime},
		model.Primitive{Kind: model.PrimDate},
		model.Primitive{Kind: model.PrimTime},
		model.Primitive{Kind: model.PrimDuration},
		model.Primitive{Kind: model.PrimUUID},
		model.Primitive{Kind: model.PrimBytes},
		model.Primitive{Kind: model.PrimDecimal, Params: []int{10, 4}},
		model.Primitive{Kind: model.PrimAny},
		model.Semantic{Namespace: "uuid"},
		model.EnumInline{Values: []string{"a"}},
		model.Reference{Target: "User"},
		model.List{Element: model.Primitive{Kind: model.PrimString}},
		model.Union{Variants: []model.TypeExpr{model.Primitive{Kind: model.PrimString}}},
	}
	for _, dialect := range []string{"postgres", "mysql", "sqlite"} {
		for _, typ := range cases {
			if got := sqlTypeFor(typ, dialect); got == "" {
				t.Fatalf("%s %#v returned empty type", dialect, typ)
			}
		}
	}
}

func TestScalarAndQuotingHelpers(t *testing.T) {
	for _, s := range []string{"", "true", " no ", "-item", "a:b", "line\nbreak"} {
		if !yamlNeedsQuoting(s) {
			t.Fatalf("%q should need YAML quoting", s)
		}
	}
	if yamlNeedsQuoting("plain") {
		t.Fatal("plain should not need quoting")
	}
	if got := hexEncode([]byte{0x0f, 0xa0}); got != "0fa0" {
		t.Fatalf("hex=%q", got)
	}
	obj := value.NewObject()
	obj.Set("x", value.Int(1))
	scalar, err := renderValueScalar(value.Obj(obj))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(scalar, "x") {
		t.Fatalf("object scalar=%q", scalar)
	}
}
