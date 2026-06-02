package model

import "testing"

func TestOrderedMapInsertAfter(t *testing.T) {
	eq := func(t *testing.T, got, want []string) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("keys=%v want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("keys=%v want %v", got, want)
			}
		}
	}

	t.Run("middle", func(t *testing.T) {
		m := NewOrderedMap[string, int]()
		m.Set("a", 1)
		m.Set("b", 2)
		m.Set("c", 3)
		m.InsertAfter("a", "x", 9)
		eq(t, m.Keys(), []string{"a", "x", "b", "c"})
		if v, _ := m.Get("x"); v != 9 {
			t.Fatalf("x=%d want 9", v)
		}
	})

	t.Run("last", func(t *testing.T) {
		m := NewOrderedMap[string, int]()
		m.Set("a", 1)
		m.Set("b", 2)
		m.InsertAfter("b", "x", 9)
		eq(t, m.Keys(), []string{"a", "b", "x"})
	})

	t.Run("missing anchor appends", func(t *testing.T) {
		m := NewOrderedMap[string, int]()
		m.Set("a", 1)
		m.InsertAfter("zzz", "x", 9)
		eq(t, m.Keys(), []string{"a", "x"})
	})

	t.Run("existing key updates in place", func(t *testing.T) {
		m := NewOrderedMap[string, int]()
		m.Set("a", 1)
		m.Set("b", 2)
		m.Set("c", 3)
		m.InsertAfter("a", "c", 99) // c already exists; position unchanged
		eq(t, m.Keys(), []string{"a", "b", "c"})
		if v, _ := m.Get("c"); v != 99 {
			t.Fatalf("c=%d want 99", v)
		}
	})
}

func TestOrderedMapPreservesInsertion(t *testing.T) {
	m := NewOrderedMap[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("c", 3)
	got := m.Keys()
	want := []string{"a", "b", "c"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("keys[%d]=%q want %q", i, got[i], want[i])
		}
	}
	if v, ok := m.Get("b"); !ok || v != 2 {
		t.Fatal("get b failed")
	}
}

func TestOrderedMapOverwriteKeepsPosition(t *testing.T) {
	m := NewOrderedMap[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("a", 9)
	got := m.Keys()
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("got %v", got)
	}
	if v, _ := m.Get("a"); v != 9 {
		t.Fatal("overwrite failed")
	}
}

func TestOrderedMapDelete(t *testing.T) {
	m := NewOrderedMap[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("c", 3)
	m.Delete("b")
	if m.Len() != 2 || m.Has("b") {
		t.Fatalf("delete failed: %v", m.Keys())
	}
	if got := m.Keys(); got[0] != "a" || got[1] != "c" {
		t.Fatalf("order broken: %v", got)
	}
}

func TestOrderedMapEachEarlyExit(t *testing.T) {
	m := NewOrderedMap[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("c", 3)
	visited := 0
	m.Each(func(_ string, v int) bool {
		visited++
		return v < 2
	})
	if visited != 2 {
		t.Fatalf("expected early stop at 2 visits, got %d", visited)
	}
}
