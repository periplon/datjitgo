package generator

import (
	"math"
	"testing"

	"github.com/periplon/datjitgo/core/model"
)

func TestDistNormalMeanAndStddev(t *testing.T) {
	spec := distSpec{Kind: distNormal, Mu: 100, Sigma: 15}
	rng := NewRand(42)
	const n = 10_000
	var sum float64
	xs := make([]float64, n)
	for i := 0; i < n; i++ {
		v := sampleFloat(rng, spec, 0, 0, false)
		xs[i] = v
		sum += v
	}
	mean := sum / n
	if math.Abs(mean-100) > 2.5 {
		t.Fatalf("mean %.2f too far from 100", mean)
	}
	var sq float64
	for _, x := range xs {
		sq += (x - mean) * (x - mean)
	}
	sd := math.Sqrt(sq / (n - 1))
	if math.Abs(sd-15) > 2.5 {
		t.Fatalf("stddev %.2f too far from 15", sd)
	}
}

func TestDistWeightedEnumIndex(t *testing.T) {
	rng := NewRand(1)
	weights := []float64{70, 25, 5}
	const n = 10_000
	counts := make([]int, len(weights))
	for i := 0; i < n; i++ {
		counts[sampleEnumIndex(rng, weights)]++
	}
	total := 0.0
	for _, w := range weights {
		total += w
	}
	for i, w := range weights {
		want := w / total
		got := float64(counts[i]) / n
		if math.Abs(got-want) > 0.05 {
			t.Fatalf("enum[%d] got %.3f want %.3f", i, got, want)
		}
	}
}

func TestDistUniformInRange(t *testing.T) {
	spec := distSpec{Kind: distUniform}
	rng := NewRand(3)
	for i := 0; i < 500; i++ {
		v := sampleFloat(rng, spec, 10, 20, true)
		if v < 10 || v > 20 {
			t.Fatalf("uniform escaped range: %v", v)
		}
	}
}

func TestDistLogNormalRangeFit(t *testing.T) {
	// Default mu/sigma with a range should fit the bulk of samples inside it.
	spec := distSpec{Kind: distLogNormal, Mu: 0, Sigma: 1}
	rng := NewRand(5)
	lo, hi := 1e4, 5e7
	inside := 0
	for i := 0; i < 500; i++ {
		v := sampleFloat(rng, spec, lo, hi, true)
		if v >= lo && v <= hi {
			inside++
		}
	}
	if inside < 490 {
		t.Fatalf("lognormal range fit ineffective: %d/500 inside", inside)
	}
}

func TestDistParseDecorator(t *testing.T) {
	// @dist(normal, mu=50, sigma=15)
	d := &model.Decorator{
		Name: "dist",
		Args: []model.DecoratorArg{
			{Kind: model.ArgIdent, Ident: "normal"},
			{Kind: model.ArgKV, Key: "mu", Value: "50"},
			{Kind: model.ArgKV, Key: "sigma", Value: "15"},
		},
	}
	spec := parseDistDecorator(d)
	if spec.Kind != distNormal || spec.Mu != 50 || spec.Sigma != 15 {
		t.Fatalf("wrong spec: %+v", spec)
	}

	// @dist(70, 25, 5)
	d2 := &model.Decorator{
		Name: "dist",
		Args: []model.DecoratorArg{
			{Kind: model.ArgLiteral, Literal: int64(70)},
			{Kind: model.ArgLiteral, Literal: int64(25)},
			{Kind: model.ArgLiteral, Literal: int64(5)},
		},
	}
	spec2 := parseDistDecorator(d2)
	if spec2.Kind != distCategorical || len(spec2.Probs) != 3 {
		t.Fatalf("wrong categorical: %+v", spec2)
	}
}

func TestDistBimodalHasTwoPeaks(t *testing.T) {
	spec := distSpec{Kind: distBimodal, PeakA: 20, PeakB: 80}
	rng := NewRand(9)
	nearA, nearB := 0, 0
	for i := 0; i < 10_000; i++ {
		v := sampleFloat(rng, spec, 0, 0, false)
		if math.Abs(v-20) < 15 {
			nearA++
		}
		if math.Abs(v-80) < 15 {
			nearB++
		}
	}
	if nearA < 2000 || nearB < 2000 {
		t.Fatalf("bimodal not bimodal enough: nearA=%d nearB=%d", nearA, nearB)
	}
}
