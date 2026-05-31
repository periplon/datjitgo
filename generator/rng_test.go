package generator

import (
	"testing"

	"github.com/periplon/datjitgo/core/ports"
)

type countingRandomizer struct{ next int64 }

func (r *countingRandomizer) Float() float64     { return 0 }
func (r *countingRandomizer) IntN(n int64) int64 { r.next++; return r.next % n }
func (r *countingRandomizer) NormFloat() float64 { return 0 }
func (r *countingRandomizer) ExpFloat() float64  { return 0 }
func (r *countingRandomizer) Shuffle(int, func(i, j int)) {
}
func (r *countingRandomizer) Substream(string) ports.Randomizer { return r }

func TestRNGDeterministic(t *testing.T) {
	a := NewRand(42)
	b := NewRand(42)
	for i := 0; i < 32; i++ {
		va := a.IntN(1 << 30)
		vb := b.IntN(1 << 30)
		if va != vb {
			t.Fatalf("iter %d: %d != %d", i, va, vb)
		}
	}
}

func TestRNGDifferentSeeds(t *testing.T) {
	a := NewRand(1)
	b := NewRand(2)
	// Extremely unlikely to draw the same 32-draw sequence.
	same := true
	for i := 0; i < 32 && same; i++ {
		if a.IntN(1<<30) != b.IntN(1<<30) {
			same = false
		}
	}
	if same {
		t.Fatal("different seeds produced identical 32-draw sequence")
	}
}

func TestSubstreamDivergesByScope(t *testing.T) {
	parent := NewRand(123)
	childA := parent.Substream("a")
	childB := parent.Substream("b")
	if childA.IntN(1<<30) == childB.IntN(1<<30) {
		// One match is possible but extremely rare; try again
		if childA.IntN(1<<30) == childB.IntN(1<<30) {
			t.Fatal("scope 'a' and 'b' produced identical streams")
		}
	}
}

func TestSubstreamStableForSameScope(t *testing.T) {
	// Substream must be deterministic given parent state + scope.
	p1 := NewRand(7)
	p2 := NewRand(7)
	c1 := p1.Substream("x")
	c2 := p2.Substream("x")
	for i := 0; i < 16; i++ {
		if c1.IntN(1<<30) != c2.IntN(1<<30) {
			t.Fatalf("substream diverged at iter %d", i)
		}
	}
}

func TestSubstreamIdempotent(t *testing.T) {
	// Substream() on the same parent state with the same scope yields the
	// same stream whether called once or repeatedly without drawing from
	// the parent first.
	parent := NewRand(11)
	c1 := parent.Substream("x")
	c2 := parent.Substream("x")
	for i := 0; i < 8; i++ {
		if c1.IntN(1<<30) != c2.IntN(1<<30) {
			t.Fatalf("substream not idempotent at iter %d", i)
		}
	}
}

func TestFloatRange(t *testing.T) {
	r := NewRand(5)
	for i := 0; i < 1000; i++ {
		f := r.Float()
		if f < 0 || f >= 1 {
			t.Fatalf("Float out of range: %v", f)
		}
	}
}

func TestIntNZeroOrNegative(t *testing.T) {
	r := NewRand(3)
	if r.IntN(0) != 0 {
		t.Fatal("IntN(0) should return 0")
	}
	if r.IntN(-5) != 0 {
		t.Fatal("IntN(-5) should return 0")
	}
}

func TestUint64OfConcreteAndFallbackRandomizers(t *testing.T) {
	if got := uint64Of(NewRand(3)); got == 0 {
		t.Fatal("concrete uint64Of returned zero")
	}
	fallback := &countingRandomizer{}
	if got := uint64Of(fallback); got != (uint64(1)<<32)|uint64(2) {
		t.Fatalf("fallback uint64Of = %d", got)
	}
}
