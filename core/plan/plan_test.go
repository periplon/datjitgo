package plan

import (
	stderrors "errors"
	"testing"

	"github.com/periplon/datjitgo/core/errors"
	"github.com/periplon/datjitgo/core/model"
)

func mkDoc(entities ...*model.Entity) *model.Document {
	d := model.NewDocument()
	for _, e := range entities {
		d.Entities.Set(e.Name, e)
	}
	return d
}

func entWithRef(name, target string) *model.Entity {
	e := model.NewEntity(name)
	e.Fields.Set("ref", &model.Field{Name: "ref", Type: model.Reference{Target: target}})
	return e
}

func TestEntitiesLinearChain(t *testing.T) {
	doc := mkDoc(
		entWithRef("Order", "User"),
		entWithRef("User", "Department"),
		model.NewEntity("Department"),
	)
	got, err := Entities(doc)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Department", "User", "Order"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestEntitiesCycle(t *testing.T) {
	doc := mkDoc(entWithRef("A", "B"), entWithRef("B", "A"))
	_, err := Entities(doc)
	if err == nil {
		t.Fatal("expected cycle error")
	}
	if !stderrors.Is(err, errors.ErrCyclicDependency) {
		t.Fatalf("wrong error: %v", err)
	}
}

func TestEntitiesDiamond(t *testing.T) {
	a := model.NewEntity("A")
	b := entWithRef("B", "A")
	c := entWithRef("C", "A")
	d := model.NewEntity("D")
	d.Fields.Set("b", &model.Field{Name: "b", Type: model.Reference{Target: "B"}})
	d.Fields.Set("c", &model.Field{Name: "c", Type: model.Reference{Target: "C"}})
	doc := mkDoc(d, c, b, a)

	got, err := Entities(doc)
	if err != nil {
		t.Fatal(err)
	}
	if got[0] != "A" {
		t.Fatalf("first should be A, got %v", got)
	}
	if got[len(got)-1] != "D" {
		t.Fatalf("last should be D, got %v", got)
	}
}

func TestEntitiesSelfRefIgnored(t *testing.T) {
	a := model.NewEntity("A")
	a.Fields.Set("parent", &model.Field{Name: "parent", Type: model.Reference{Target: "self", Optional: true}})
	doc := mkDoc(a)

	got, err := Entities(doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "A" {
		t.Fatalf("want [A], got %v", got)
	}
}

func TestEntitiesTiesByInsertionOrder(t *testing.T) {
	doc := mkDoc(
		model.NewEntity("Alpha"),
		model.NewEntity("Beta"),
		model.NewEntity("Gamma"),
	)

	got, err := Entities(doc)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Alpha", "Beta", "Gamma"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestEntitiesThroughCompositeReferences(t *testing.T) {
	a := model.NewEntity("A")
	b := model.NewEntity("B")
	b.Fields.Set("refs", &model.Field{Name: "refs", Type: model.Union{Variants: []model.TypeExpr{
		model.List{Element: model.Reference{Target: "A"}},
		model.Nullable{Inner: model.Tuple{Elements: []model.TypeExpr{
			model.Map{
				Key:   model.Reference{Target: "Missing"},
				Value: model.Reference{Target: "A"},
			},
		}}},
	}}})
	doc := mkDoc(b, a)

	got, err := Entities(doc)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"A", "B"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestCollectRefsIncludesNestedCompositeTargets(t *testing.T) {
	fields := model.NewOrderedMap[string, *model.Field]()
	fields.Set("complex", &model.Field{Name: "complex", Type: model.Tuple{Elements: []model.TypeExpr{
		model.Map{Key: model.Reference{Target: "Key"}, Value: model.Reference{Target: "Value"}},
		model.Nullable{Inner: model.Union{Variants: []model.TypeExpr{
			model.Reference{Target: "A"},
			model.List{Element: model.Reference{Target: "B"}},
		}}},
	}}})
	seen := map[string]struct{}{}
	CollectRefs(fields, seen)
	for _, want := range []string{"Key", "Value", "A", "B"} {
		if _, ok := seen[want]; !ok {
			t.Fatalf("missing ref %q in %v", want, seen)
		}
	}
}

func TestCyclesTwoNode(t *testing.T) {
	doc := mkDoc(entWithRef("A", "B"), entWithRef("B", "A"))
	cycles := Cycles(doc)
	if len(cycles) != 1 {
		t.Fatalf("expected one cycle, got %v", cycles)
	}
	c := cycles[0]
	if len(c) < 3 || c[0] != c[len(c)-1] {
		t.Fatalf("malformed cycle path %v", c)
	}
}

func TestCyclesSelfRefExcluded(t *testing.T) {
	a := model.NewEntity("A")
	a.Fields.Set("parent", &model.Field{Name: "parent", Type: model.Reference{Target: "self"}})
	if cycles := Cycles(mkDoc(a)); len(cycles) != 0 {
		t.Fatalf("self-reference must not be a cycle, got %v", cycles)
	}
}

func TestCyclesNoneForDAG(t *testing.T) {
	doc := mkDoc(entWithRef("Order", "User"), model.NewEntity("User"))
	if cycles := Cycles(doc); len(cycles) != 0 {
		t.Fatalf("expected no cycles, got %v", cycles)
	}
}

func TestEntitiesCycleErrorMessageHasPath(t *testing.T) {
	doc := mkDoc(entWithRef("A", "B"), entWithRef("B", "A"))
	_, err := Entities(doc)
	if err == nil {
		t.Fatal("expected cycle error")
	}
	// Exact match guards against the Kind prefix being duplicated in the
	// message (the Error formatter already prepends "cyclic dependency: ").
	if got, want := err.Error(), "cyclic dependency: A -> B -> A"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}
