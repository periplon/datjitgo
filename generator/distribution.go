package generator

import (
	"math"
	"strconv"

	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/ports"
)

// distKind enumerates the distributions the generator knows how to sample.
type distKind int

const (
	distUniform distKind = iota
	distNormal
	distLogNormal
	distExponential
	distGeometric
	distZipf
	distBimodal
	distCategorical // positional probabilities matching an enum's variants
	distWeighted    // name -> weight pairs (used by enums with @dist({a=1}))
)

// distSpec is the normalised form of a @dist decorator extracted from a
// parsed model.Decorator. Only fields relevant to Kind are populated.
type distSpec struct {
	Kind distKind

	// Continuous parameters
	Mu, Sigma, Lambda, P, S float64
	PeakA, PeakB            float64
	ZipfN                   uint64

	// Categorical/weighted
	Probs   []float64
	Weights []weighted
}

type weighted struct {
	Name   string
	Weight float64
}

// parseDistDecorator turns a parsed @dist(...) decorator into a distSpec. If
// the decorator is missing or malformed the returned spec falls back to
// uniform.
//
// Argument shapes supported (in priority order):
//
//   - `@dist(normal, mu=X, sigma=Y)` — ident + KV params
//   - `@dist(lognormal)` / `@dist(exponential, lambda=N)` / `@dist(geometric, p=N)` etc.
//   - `@dist(zipf, s=N)` — N defaults to 1000
//   - `@dist(bimodal, peaks=X,Y)` — peaks may arrive as KV then a positional literal
//   - `@dist(70, 25, 5)` — positional probabilities (Categorical, enum order)
//   - `@dist({free=70, pro=25, enterprise=5})` — weighted names (not used in fixtures yet)
func parseDistDecorator(d *model.Decorator) distSpec {
	if d == nil {
		return distSpec{Kind: distUniform}
	}
	// Try to locate an identifier first arg identifying the family.
	family := ""
	positional := make([]float64, 0, len(d.Args))
	kv := map[string]string{}
	for _, a := range d.Args {
		switch a.Kind {
		case model.ArgIdent:
			if family == "" {
				family = a.Ident
			}
		case model.ArgKV:
			kv[a.Key] = a.Value
		case model.ArgLiteral:
			if f, ok := argAsFloat(a.Literal); ok {
				positional = append(positional, f)
			}
		default:
			// other arg kinds are ignored
		}
	}

	switch family {
	case "normal":
		return distSpec{
			Kind:  distNormal,
			Mu:    parseKVFloat(kv, "mu", 0),
			Sigma: parseKVFloat(kv, "sigma", 1),
		}
	case "lognormal":
		return distSpec{
			Kind:  distLogNormal,
			Mu:    parseKVFloat(kv, "mu", 0),
			Sigma: parseKVFloat(kv, "sigma", 1),
		}
	case "exponential":
		return distSpec{
			Kind:   distExponential,
			Lambda: parseKVFloat(kv, "lambda", 1),
		}
	case "geometric":
		return distSpec{
			Kind: distGeometric,
			P:    parseKVFloat(kv, "p", 0.5),
		}
	case "zipf":
		n := uint64(parseKVFloat(kv, "N", 1000))
		if n < 2 {
			n = 1000
		}
		return distSpec{
			Kind:  distZipf,
			S:     parseKVFloat(kv, "s", 1.5),
			ZipfN: n,
		}
	case "bimodal":
		peaks := []float64{}
		if raw, ok := kv["peaks"]; ok {
			if f, err := strconv.ParseFloat(raw, 64); err == nil {
				peaks = append(peaks, f)
			}
		}
		peaks = append(peaks, positional...)
		if len(peaks) < 2 {
			// Fallback to sensible defaults if parsing went sideways.
			return distSpec{Kind: distNormal, Mu: 0, Sigma: 1}
		}
		return distSpec{
			Kind:  distBimodal,
			PeakA: peaks[0],
			PeakB: peaks[1],
		}
	case "weighted":
		// @dist(weighted, foo=1, bar=2). Rarely used; wire anyway.
		out := make([]weighted, 0, len(kv))
		for k, v := range kv {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				out = append(out, weighted{Name: k, Weight: f})
			}
		}
		return distSpec{Kind: distWeighted, Weights: out}
	}

	// No ident recognised — treat positional literals as categorical weights.
	if len(positional) > 0 {
		return distSpec{Kind: distCategorical, Probs: positional}
	}
	return distSpec{Kind: distUniform}
}

func parseKVFloat(kv map[string]string, key string, fallback float64) float64 {
	if raw, ok := kv[key]; ok {
		if f, err := strconv.ParseFloat(raw, 64); err == nil {
			return f
		}
	}
	return fallback
}

func argAsFloat(lit any) (float64, bool) {
	switch x := lit.(type) {
	case int64:
		return float64(x), true
	case float64:
		return x, true
	case string:
		if f, err := strconv.ParseFloat(x, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

// sampleFloat draws one continuous sample from spec. For Categorical and
// Weighted specs the caller should use sampleEnumIndex directly.
func sampleFloat(rng ports.Randomizer, spec distSpec, rangeLo, rangeHi float64, haveRange bool) float64 {
	raw := 0.0
	switch spec.Kind {
	case distUniform:
		if haveRange {
			raw = rangeLo + rng.Float()*(rangeHi-rangeLo)
		} else {
			raw = rng.Float() * 1000
		}
	case distNormal:
		raw = spec.Mu + spec.Sigma*rng.NormFloat()
	case distLogNormal:
		mu, sigma := spec.Mu, spec.Sigma
		if mu == 0 && sigma == 1 && haveRange && rangeHi > rangeLo {
			// Fit the log-normal so ±3σ spans [lo, hi] — matches Rust.
			lo := math.Max(rangeLo, 1)
			if rangeHi > lo {
				mu = (math.Log(lo) + math.Log(rangeHi)) / 2
				sigma = math.Max((math.Log(rangeHi)-math.Log(lo))/6, 1e-6)
			}
		}
		raw = math.Exp(mu + sigma*rng.NormFloat())
	case distExponential:
		lambda := spec.Lambda
		if lambda <= 0 {
			lambda = 1
		}
		raw = rng.ExpFloat() / lambda
	case distGeometric:
		p := spec.P
		if p <= 0 || p >= 1 {
			p = 0.5
		}
		// Inverse-CDF: k = floor(ln(1-u)/ln(1-p))
		u := rng.Float()
		raw = math.Floor(math.Log(1-u) / math.Log(1-p))
	case distZipf:
		// Sample Zipf(s, N) via inverse CDF approximation.
		raw = zipfInverse(rng, spec.S, spec.ZipfN)
	case distBimodal:
		spread := math.Max(math.Abs(spec.PeakB-spec.PeakA)/6, 0.1)
		var peak float64
		if rng.IntN(2) == 0 {
			peak = spec.PeakA
		} else {
			peak = spec.PeakB
		}
		raw = peak + spread*rng.NormFloat()
	case distCategorical, distWeighted:
		// Falls back to uniform; callers must dispatch to sampleEnumIndex.
		if haveRange {
			raw = rangeLo + rng.Float()*(rangeHi-rangeLo)
		} else {
			raw = rng.Float() * 1000
		}
	}
	// Clamp continuous distributions to the declared range.
	if haveRange && spec.Kind != distUniform && spec.Kind != distCategorical && spec.Kind != distWeighted {
		if raw < rangeLo {
			raw = rangeLo
		}
		if raw > rangeHi {
			raw = rangeHi
		}
	}
	return raw
}

// sampleEnumIndex picks an index from weights using the RNG. Returns 0 when
// weights is empty.
func sampleEnumIndex(rng ports.Randomizer, weights []float64) int {
	total := 0.0
	for _, w := range weights {
		if w > 0 {
			total += w
		}
	}
	if total <= 0 {
		if len(weights) == 0 {
			return 0
		}
		return int(rng.IntN(int64(len(weights))))
	}
	pick := rng.Float() * total
	for i, w := range weights {
		if w <= 0 {
			continue
		}
		if pick < w {
			return i
		}
		pick -= w
	}
	return len(weights) - 1
}

// zipfInverse uses a rejection-free approximation. For the Phase-1 use case
// (generator fixtures) a straightforward Devroye variate is sufficient; it
// stays >=1 and decays as 1/k^s.
func zipfInverse(rng ports.Randomizer, s float64, n uint64) float64 {
	if s <= 1 {
		s = 1.5
	}
	if n < 2 {
		n = 1000
	}
	// Devroye 1986, "Non-Uniform Random Variate Generation", Ch. 10.
	b := math.Pow(2, s-1)
	for {
		u := rng.Float()
		v := rng.Float()
		x := math.Floor(math.Pow(u, -1.0/(s-1)))
		if x < 1 || x > float64(n) {
			continue
		}
		t := math.Pow(1+1.0/x, s-1)
		if v*x*(t-1)/(b-1) <= t/b {
			return x
		}
	}
}
