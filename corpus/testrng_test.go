package corpus

import (
	"hash/fnv"
	"math"
	"math/rand/v2"

	"github.com/jmcarbo/datjitgo/core/ports"
)

// testRNG is a deterministic ports.Randomizer backed by math/rand/v2.PCG,
// used only in package tests. It is intentionally minimal — the real
// generator will own the production RNG (Task 7). This implementation is
// declared in a _test.go file so it never leaks into production builds.
type testRNG struct {
	seed uint64
	r    *rand.Rand
}

func newTestRNG(seed uint64) *testRNG {
	pcg := rand.NewPCG(seed, seed^0x9E3779B97F4A7C15)
	return &testRNG{seed: seed, r: rand.New(pcg)}
}

func (t *testRNG) Substream(scope string) ports.Randomizer {
	h := fnv.New64a()
	_, _ = h.Write([]byte(scope))
	return newTestRNG(t.seed ^ h.Sum64())
}

func (t *testRNG) Float() float64     { return t.r.Float64() }
func (t *testRNG) IntN(n int64) int64 { return t.r.Int64N(n) }
func (t *testRNG) NormFloat() float64 { return t.r.NormFloat64() }

// ExpFloat returns a standard exponential (rate 1) sample.
func (t *testRNG) ExpFloat() float64 {
	// math/rand/v2 exposes ExpFloat64 on *Rand.
	u := t.r.Float64()
	if u == 0 {
		u = math.SmallestNonzeroFloat64
	}
	return -math.Log(u)
}

func (t *testRNG) Shuffle(n int, swap func(i, j int)) {
	t.r.Shuffle(n, swap)
}

// Compile-time check.
var _ ports.Randomizer = (*testRNG)(nil)
