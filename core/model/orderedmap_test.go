package model

import "testing"

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
