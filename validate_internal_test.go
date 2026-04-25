package datjit

import (
	"errors"
	"testing"

	derrs "github.com/jmcarbo/datjitgo/core/errors"
	"github.com/jmcarbo/datjitgo/core/model"
)

func TestCheckTypeExprCompositeBranches(t *testing.T) {
	entities := map[string]struct{}{"User": {}}
	types := map[string]struct{}{"Address": {}}
	enums := map[string]model.EnumDef{"Status": {Variants: []model.EnumVariant{{Value: "active"}}}}
	cases := []model.TypeExpr{
		model.List{Element: model.Reference{Target: "User"}},
		model.Map{Key: model.Primitive{Kind: model.PrimString}, Value: model.NamedType{Name: "Address"}},
		model.Tuple{Elements: []model.TypeExpr{model.NamedType{Name: "Status"}}},
		model.Nullable{Inner: model.Reference{Target: "self"}},
		model.Union{Variants: []model.TypeExpr{model.Reference{Target: "User"}, model.NamedType{Name: "Status"}}},
	}
	for _, typ := range cases {
		if err := checkTypeExpr(typ, "User", "field", entities, types, enums); err != nil {
			t.Fatalf("%T: %v", typ, err)
		}
	}
	err := checkTypeExpr(model.Map{Key: model.Reference{Target: "Ghost"}, Value: model.Primitive{Kind: model.PrimString}}, "User", "field", entities, types, enums)
	if !errors.Is(err, derrs.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}
