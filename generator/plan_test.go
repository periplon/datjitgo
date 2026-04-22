package generator

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

func TestPlanLinearChain(t *testing.T) {
	// Order -> User -> Department (insertion order forward; dependencies backward)
	doc := mkDoc(
		entWithRef("Order", "User"),
		entWithRef("User", "Department"),
		model.NewEntity("Department"),
	)
	got, err := plan(doc)
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

func TestPlanDiamond(t *testing.T) {
	// D -> B, C; B -> A; C -> A
	A := model.NewEntity("A")
	B := entWithRef("B", "A")
	C := entWithRef("C", "A")
	D := model.NewEntity("D")
	D.Fields.Set("b", &model.Field{Name: "b", Type: model.Reference{Target: "B"}})
	D.Fields.Set("c", &model.Field{Name: "c", Type: model.Reference{Target: "C"}})
	doc := mkDoc(D, C, B, A)
	got, err := plan(doc)
	if err != nil {
		t.Fatal(err)
	}
	// Must see A first, then B and C (insertion-order C before B since we
	// added C before B), then D.
	if got[0] != "A" {
		t.Fatalf("first should be A, got %v", got)
	}
	if got[len(got)-1] != "D" {
		t.Fatalf("last should be D, got %v", got)
	}
}

func TestPlanCycle(t *testing.T) {
	// A -> B -> A
	doc := mkDoc(entWithRef("A", "B"), entWithRef("B", "A"))
	_, err := plan(doc)
	if err == nil {
		t.Fatal("expected cycle error")
	}
	if !stderrors.Is(err, errors.ErrCyclicDependency) {
		t.Fatalf("wrong error: %v", err)
	}
}

func TestPlanSelfRefIgnored(t *testing.T) {
	A := model.NewEntity("A")
	A.Fields.Set("parent", &model.Field{Name: "parent", Type: model.Reference{Target: "self", Optional: true}})
	doc := mkDoc(A)
	got, err := plan(doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "A" {
		t.Fatalf("want [A], got %v", got)
	}
}

func TestPlanTiesByInsertionOrder(t *testing.T) {
	// All independent, declared in order Alpha, Beta, Gamma.
	doc := mkDoc(
		model.NewEntity("Alpha"),
		model.NewEntity("Beta"),
		model.NewEntity("Gamma"),
	)
	got, err := plan(doc)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Alpha", "Beta", "Gamma"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tie-break failed: got %v want %v", got, want)
		}
	}
}

func TestPlanThroughList(t *testing.T) {
	A := model.NewEntity("A")
	B := model.NewEntity("B")
	B.Fields.Set("as", &model.Field{Name: "as", Type: model.List{Element: model.Reference{Target: "A"}}})
	doc := mkDoc(B, A)
	got, err := plan(doc)
	if err != nil {
		t.Fatal(err)
	}
	if got[0] != "A" {
		t.Fatalf("want A first (through List[ref]), got %v", got)
	}
}
