package model

import "testing"

func TestDecoratorHelpers(t *testing.T) {
	decs := []Decorator{
		{Name: "primary"},
		{Name: "dist", Args: []DecoratorArg{
			{Kind: ArgIdent, Ident: "normal"},
			{Kind: ArgKV, Key: "mu", Value: "10"},
		}},
	}
	if !HasDecorator(decs, "primary") {
		t.Fatal("expected primary decorator")
	}
	if HasDecorator(decs, "missing") {
		t.Fatal("missing decorator should be false")
	}
	d := FindDecorator(decs, "dist")
	if d == nil {
		t.Fatal("expected dist decorator")
	}
	if got, ok := d.ArgByKey("mu"); !ok || got != "10" {
		t.Fatalf("mu arg = %q, %v", got, ok)
	}
	if _, ok := d.ArgByKey("sigma"); ok {
		t.Fatal("unexpected sigma arg")
	}
	if FindDecorator(decs, "missing") != nil {
		t.Fatal("missing decorator should return nil")
	}
}

func TestEnumHelpers(t *testing.T) {
	weight := 2.5
	def := EnumDef{Variants: []EnumVariant{
		{Value: "new"},
		{Value: "done", Weight: &weight},
	}}
	values := def.Values()
	if len(values) != 2 || values[0] != "new" || values[1] != "done" {
		t.Fatalf("values: %v", values)
	}
	weights := def.WeightsOrNil()
	if len(weights) != 2 || weights[0] != 1 || weights[1] != weight {
		t.Fatalf("weights: %v", weights)
	}
	if got := (EnumDef{Variants: []EnumVariant{{Value: "x"}}}).WeightsOrNil(); got != nil {
		t.Fatalf("unweighted enum should return nil, got %v", got)
	}
}

func TestDocumentAndEntityConstructors(t *testing.T) {
	doc := NewDocument()
	if doc.Volume == nil || doc.Entities == nil || doc.Enums == nil || doc.Types == nil || doc.Tools == nil {
		t.Fatalf("document maps not initialised: %+v", doc)
	}
	ent := NewEntity("User")
	if ent.Name != "User" || ent.Fields == nil || ent.Coherence == nil {
		t.Fatalf("entity not initialised: %+v", ent)
	}
	if !(VolumeSpec{Min: 1, Max: 2}).IsRange() {
		t.Fatal("range volume not detected")
	}
	if (VolumeSpec{Exact: 2}).IsRange() {
		t.Fatal("exact volume should not be range")
	}
}
