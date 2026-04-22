// Package generator implements the ports.Generator adapter: it turns a parsed
// *model.Document into a *value.Dataset by running a deterministic pipeline
// over each entity in topological order.
//
// The generator is pure Go with stdlib dependencies only (plus google/uuid
// and shopspring/decimal which are transitively used by core/value). All
// randomness flows through a ports.Randomizer backed by math/rand/v2's PCG
// generator, derived sub-streams ensure identical input produces identical
// output.
package generator

import (
	"encoding/binary"
	"hash/fnv"
	"math/rand/v2"

	"github.com/jmcarbo/datjitgo/core/ports"
)

// pcgRand adapts math/rand/v2's PCG-backed *rand.Rand to the ports.Randomizer
// interface. Two fields keep the original seed state so Substream() can fold
// parent state into the derivation hash.
type pcgRand struct {
	r  *rand.Rand
	hi uint64
	lo uint64
}

// NewRand returns a deterministic Randomizer seeded from the given int64.
// Callers typically obtain the seed from the Document (`doc.Seed` or
// `doc.Generation.Seed`) or an explicit override.
func NewRand(seed int64) ports.Randomizer {
	lo := uint64(seed)
	hi := uint64(seed) ^ 0x9E3779B97F4A7C15
	return &pcgRand{
		r:  rand.New(rand.NewPCG(lo, hi)),
		lo: lo,
		hi: hi,
	}
}

// Substream returns a child RNG derived deterministically from the parent's
// seed state and the given scope string via FNV-64a. Calling Substream(scope)
// repeatedly on the same parent state yields identical child streams —
// callers who need divergent streams must first draw from the parent.
func (p *pcgRand) Substream(scope string) ports.Randomizer {
	h := fnv.New64a()
	var buf [16]byte
	binary.LittleEndian.PutUint64(buf[0:8], p.lo)
	binary.LittleEndian.PutUint64(buf[8:16], p.hi)
	_, _ = h.Write(buf[:])
	_, _ = h.Write([]byte(scope))
	s := h.Sum64()
	childLo := s
	childHi := s ^ 0xDA3E39CB94B95BDB
	return &pcgRand{
		r:  rand.New(rand.NewPCG(childLo, childHi)),
		lo: childLo,
		hi: childHi,
	}
}

// Float returns a uniform float in [0, 1).
func (p *pcgRand) Float() float64 { return p.r.Float64() }

// IntN returns a uniform int in [0, n). Returns 0 for non-positive n.
func (p *pcgRand) IntN(n int64) int64 {
	if n <= 0 {
		return 0
	}
	return p.r.Int64N(n)
}

// NormFloat returns a standard normal sample (mean 0, variance 1).
func (p *pcgRand) NormFloat() float64 { return p.r.NormFloat64() }

// ExpFloat returns a standard exponential sample (rate 1).
func (p *pcgRand) ExpFloat() float64 { return p.r.ExpFloat64() }

// Shuffle permutes n elements via the provided swap function.
func (p *pcgRand) Shuffle(n int, swap func(i, j int)) { p.r.Shuffle(n, swap) }

// uint64Of drains one raw 64-bit word from the underlying generator. Used by
// helpers such as UUID byte generation where callers need raw bits rather
// than a float or bounded int.
func uint64Of(r ports.Randomizer) uint64 {
	// Preferred fast path: unwrap to the concrete *pcgRand.
	if pr, ok := r.(*pcgRand); ok {
		return pr.r.Uint64()
	}
	// Generic fallback: compose two IntN draws.
	hi := uint64(r.IntN(1 << 32))
	lo := uint64(r.IntN(1 << 32))
	return (hi << 32) | lo
}
