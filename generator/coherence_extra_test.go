package generator

import (
	"strings"
	"testing"

	"github.com/jmcarbo/datjitgo/core/model"
	"github.com/jmcarbo/datjitgo/core/value"
)

func TestCoherenceGroupsAndFromDerivations(t *testing.T) {
	eng := newEngine()
	rng := NewRand(4)
	locRow := value.NewObject()
	eng.generateLocationGroup([]string{"office_city", "state", "zip", "timezone", "phone", "street_address", "country", "other"}, locRow, rng)
	for _, k := range []string{"office_city", "state", "zip", "timezone", "phone", "street_address", "country", "other"} {
		if !locRow.Has(k) {
			t.Fatalf("location group missing %s", k)
		}
	}
	idRow := value.NewObject()
	eng.generateIdentityGroup([]string{"first_name", "last_name", "full_name", "email", "username", "other"}, idRow, rng)
	for _, k := range []string{"first_name", "last_name", "full_name", "email", "username", "other"} {
		if !idRow.Has(k) {
			t.Fatalf("identity group missing %s", k)
		}
	}
	if !isLocationGroup("geo", []string{"x"}) || !isLocationGroup("misc", []string{"city", "phone"}) {
		t.Fatal("location detection failed")
	}
	if !isIdentityGroup("person", []string{"x"}) || !isIdentityGroup("misc", []string{"first_name", "email"}) {
		t.Fatal("identity detection failed")
	}

	entity := model.NewEntity("User")
	entity.Fields.Set("name", &model.Field{Name: "name", Type: model.Primitive{Kind: model.PrimString}})
	entity.Fields.Set("email", &model.Field{
		Name:       "email",
		Type:       model.Primitive{Kind: model.PrimString},
		Decorators: []model.Decorator{{Name: "from", Args: []model.DecoratorArg{{Kind: model.ArgIdent, Ident: "name"}}}},
	})
	row := value.NewObject()
	row.Set("name", value.Str("Ada Lovelace"))
	if err := eng.applyFromDerivations(entity, row, rng, nil); err != nil {
		t.Fatal(err)
	}
	email, _ := row.Get("email")
	if !strings.Contains(email.S, "ada.lovelace@") {
		t.Fatalf("derived email=%q", email.S)
	}
}

func TestApplyCoherenceDefaultGroup(t *testing.T) {
	eng := newEngine()
	entity := model.NewEntity("Thing")
	entity.Fields.Set("label", &model.Field{Name: "label", Type: model.Primitive{Kind: model.PrimString}})
	entity.Fields.Set("count", &model.Field{Name: "count", Type: model.Primitive{Kind: model.PrimInt}})
	entity.Coherence.Set("misc", []string{"label", "count", "missing"})
	row := value.NewObject()
	populated, err := eng.applyCoherence(entity, row, NewRand(9))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := populated["label"]; !ok || !row.Has("label") || !row.Has("count") {
		t.Fatalf("default coherence failed: populated=%v keys=%v", populated, row.Keys())
	}
}
