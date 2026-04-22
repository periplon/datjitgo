package value

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestValueKinds(t *testing.T) {
	cases := []struct {
		v    Value
		kind Kind
	}{
		{Null(), KindNull},
		{Bool(true), KindBool},
		{Int(42), KindInt},
		{Float(3.14), KindFloat},
		{Str("x"), KindString},
	}
	for _, c := range cases {
		if c.v.Kind != c.kind {
			t.Fatalf("%v: got kind %v want %v", c.v, c.v.Kind, c.kind)
		}
	}
}

func TestObjectPreservesOrder(t *testing.T) {
	o := NewObject()
	o.Set("b", Int(1))
	o.Set("a", Int(2))
	o.Set("c", Int(3))
	if diff := cmp.Diff([]string{"b", "a", "c"}, o.Keys()); diff != "" {
		t.Fatal(diff)
	}
}

func TestObjectOverwriteKeepsPosition(t *testing.T) {
	o := NewObject()
	o.Set("b", Int(1))
	o.Set("a", Int(2))
	o.Set("b", Int(99))
	if diff := cmp.Diff([]string{"b", "a"}, o.Keys()); diff != "" {
		t.Fatal(diff)
	}
	v, _ := o.Get("b")
	if v.I != 99 {
		t.Fatalf("overwrite failed: %+v", v)
	}
}

func TestDatasetMap(t *testing.T) {
	ds := NewDataset()
	ds.Entities.Set("User", []*Object{NewObject()})
	ds.Entities.Set("Org", []*Object{NewObject(), NewObject()})
	if diff := cmp.Diff([]string{"User", "Org"}, ds.Entities.Keys()); diff != "" {
		t.Fatal(diff)
	}
	rows, _ := ds.Entities.Get("Org")
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
}
