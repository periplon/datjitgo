package value

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
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
		{UUID(uuid.Nil), KindUUID},
		{Time(time.Unix(0, 0)), KindTime},
		{Dec(decimal.NewFromInt(12)), KindDecimal},
		{List([]Value{Int(1)}), KindList},
		{Obj(NewObject()), KindObject},
	}
	for _, c := range cases {
		if c.v.Kind != c.kind {
			t.Fatalf("%v: got kind %v want %v", c.v, c.v.Kind, c.kind)
		}
	}
	if !Null().IsNull() || Int(0).IsNull() {
		t.Fatal("IsNull mismatch")
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
	if !o.Has("a") || o.Len() != 2 {
		t.Fatalf("object presence/len failed")
	}
	visited := 0
	o.Each(func(_ string, _ Value) bool {
		visited++
		return false
	})
	if visited != 1 {
		t.Fatalf("Each early stop visited %d", visited)
	}
	o.Delete("a")
	o.Delete("missing")
	if o.Has("a") || o.Len() != 1 {
		t.Fatalf("delete failed: keys=%v", o.Keys())
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
	if !ds.Entities.Has("User") || ds.Entities.Len() != 2 {
		t.Fatalf("dataset presence/len failed")
	}
	visited := 0
	ds.Entities.Each(func(_ string, _ []*Object) bool {
		visited++
		return false
	})
	if visited != 1 {
		t.Fatalf("Each early stop visited %d", visited)
	}
}
