package plan

import (
	stderrors "errors"
	"testing"

	"github.com/jmcarbo/datjitgo/core/errors"
	"github.com/jmcarbo/datjitgo/core/model"
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
	b.Fields.Set("refs", &model.Field{Name: "refs", Type: model.List{Element: model.Reference{Target: "A"}}})
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
