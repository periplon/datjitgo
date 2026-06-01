package generator

import (
	"testing"

	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/value"
)

func TestDistributionHelperBranches(t *testing.T) {
	rng := NewRand(77)
	specs := []distSpec{
		{Kind: distExponential, Lambda: -1},
		{Kind: distGeometric, P: 2},
		{Kind: distZipf, S: 0.5, ZipfN: 1},
		{Kind: distCategorical},
		{Kind: distWeighted},
	}
	for _, spec := range specs {
		if got := sampleFloat(rng, spec, 1, 5, true); got < 0 {
			t.Fatalf("%+v produced negative %v", spec, got)
		}
	}
	d := &model.Decorator{Name: "dist", Args: []model.DecoratorArg{
		{Kind: model.ArgIdent, Ident: "weighted"},
		{Kind: model.ArgKV, Key: "a", Value: "2"},
		{Kind: model.ArgKV, Key: "b", Value: "bad"},
	}}
	if got := parseDistDecorator(d); got.Kind != distWeighted || len(got.Weights) != 1 {
		t.Fatalf("weighted spec: %+v", got)
	}
	d = &model.Decorator{Name: "dist", Args: []model.DecoratorArg{
		{Kind: model.ArgIdent, Ident: "bimodal"},
		{Kind: model.ArgKV, Key: "peaks", Value: "20"},
		{Kind: model.ArgLiteral, Literal: float64(80)},
	}}
	if got := parseDistDecorator(d); got.Kind != distBimodal || got.PeakA != 20 || got.PeakB != 80 {
		t.Fatalf("bimodal spec: %+v", got)
	}
	if idx := sampleEnumIndex(rng, nil); idx != 0 {
		t.Fatalf("empty enum index=%d", idx)
	}
	if idx := sampleEnumIndex(rng, []float64{0, 0, 0}); idx < 0 || idx > 2 {
		t.Fatalf("zero weights index=%d", idx)
	}
}

func TestDerivedDefaultComputeAndReferences(t *testing.T) {
	eng := newEngine()
	entity := model.NewEntity("Order")
	entity.Fields.Set("subtotal", &model.Field{Name: "subtotal", Type: model.Primitive{Kind: model.PrimInt}})
	entity.Fields.Set("tax", &model.Field{Name: "tax", Type: model.Primitive{Kind: model.PrimInt}})
	entity.Fields.Set("total", &model.Field{Name: "total", Type: model.Primitive{Kind: model.PrimInt}, Decorators: []model.Decorator{{Name: "derived", Args: []model.DecoratorArg{{Kind: model.ArgLiteral, Literal: "subtotal + tax", Raw: "subtotal + tax"}}}}})
	entity.Fields.Set("chosen", &model.Field{Name: "chosen", Type: model.Primitive{Kind: model.PrimInt}, DefaultChain: &model.DefaultChainSpec{Sources: []string{"missing", "subtotal"}, When: "tax > 0", Fallback: "99"}})
	entity.Fields.Set("bucket", &model.Field{Name: "bucket", Type: model.Primitive{Kind: model.PrimString}, Compute: []model.ComputeBranch{{When: "total > 20", Value: `"big"`}, {Value: `"small"`}}})
	row := mkRow(t, "subtotal", value.Int(20), "tax", value.Int(3))
	st := &generationState{generated: map[string][]*value.Object{}}
	if err := eng.applyDerived(entity, row, st); err != nil {
		t.Fatal(err)
	}
	if err := eng.applyDefaultChain(entity, row, st); err != nil {
		t.Fatal(err)
	}
	if err := eng.applyCompute(entity, row, st); err != nil {
		t.Fatal(err)
	}
	total, _ := row.Get("total")
	chosen, _ := row.Get("chosen")
	bucket, _ := row.Get("bucket")
	if total.I != 23 || chosen.I != 20 || bucket.S != "big" {
		t.Fatalf("derived/default/compute: total=%+v chosen=%+v bucket=%+v", total, chosen, bucket)
	}

	userRow := mkRow(t, "id", value.Int(1))
	userRow2 := mkRow(t, "id", value.Int(2))
	refState := &generationState{generated: map[string][]*value.Object{"User": {userRow, userRow2}, "Order": {row}}}
	refField := &model.Field{Name: "users", Decorators: []model.Decorator{{Name: "count", Args: []model.DecoratorArg{{Kind: model.ArgLiteral, Literal: int64(2)}}}}}
	many := eng.generateReference(entity, refField, model.Reference{Target: "User", Many: true}, refState, NewRand(2))
	if many.Kind != value.KindList || len(many.L) != 2 {
		t.Fatalf("many ref: %+v", many)
	}
	empty := eng.generateReference(entity, &model.Field{Name: "self"}, model.Reference{Target: "self", Optional: true}, &generationState{generated: map[string][]*value.Object{}}, NewRand(3))
	if !empty.IsNull() {
		t.Fatalf("empty optional ref: %+v", empty)
	}
}
