package generator

import (
	"strings"
	"testing"
)

func TestPatternUpperLetters(t *testing.T) {
	rng := NewRand(1)
	ctrs := newSeqCounters()
	got := expandPattern("{A}{AAA}", rng, ctrs, "k")
	if len(got) != 4 {
		t.Fatalf("len=%d (%q)", len(got), got)
	}
	for _, r := range got {
		if r < 'A' || r > 'Z' {
			t.Fatalf("non-upper: %q", got)
		}
	}
}

func TestPatternLowerLetters(t *testing.T) {
	rng := NewRand(1)
	ctrs := newSeqCounters()
	got := expandPattern("{aa}", rng, ctrs, "k")
	if len(got) != 2 || strings.ToLower(got) != got {
		t.Fatalf("lower fail: %q", got)
	}
}

func TestPatternDigits(t *testing.T) {
	rng := NewRand(1)
	ctrs := newSeqCounters()
	got := expandPattern("{0000}", rng, ctrs, "k")
	if len(got) != 4 {
		t.Fatalf("len=%d", len(got))
	}
	for _, r := range got {
		if r < '0' || r > '9' {
			t.Fatalf("not digit: %q", got)
		}
	}
}

func TestPatternHex(t *testing.T) {
	rng := NewRand(1)
	ctrs := newSeqCounters()
	got := expandPattern("{####}", rng, ctrs, "k")
	if len(got) != 4 {
		t.Fatalf("len=%d", len(got))
	}
	for _, r := range got {
		if (r < '0' || r > '9') && (r < 'A' || r > 'F') {
			t.Fatalf("not hex: %q", got)
		}
	}
}

func TestPatternWordAndUUID(t *testing.T) {
	rng := NewRand(1)
	ctrs := newSeqCounters()
	w := expandPattern("{word}", rng, ctrs, "k")
	found := false
	for _, pw := range patternWords {
		if pw == w {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("not in word list: %q", w)
	}
	u := expandPattern("{uuid}", rng, ctrs, "k")
	if len(u) != 36 || strings.Count(u, "-") != 4 {
		t.Fatalf("bad uuid shape: %q", u)
	}
}

func TestPatternSeqIncrements(t *testing.T) {
	rng := NewRand(1)
	ctrs := newSeqCounters()
	a := expandPattern("{seq}", rng, ctrs, "k")
	b := expandPattern("{seq}", rng, ctrs, "k")
	c := expandPattern("{seq}", rng, ctrs, "k")
	if a != "1" || b != "2" || c != "3" {
		t.Fatalf("seq out: %q %q %q", a, b, c)
	}
	// Different key is independent.
	d := expandPattern("{seq}", rng, ctrs, "other")
	if d != "1" {
		t.Fatalf("seq per-key failed: %q", d)
	}
}

func TestPatternComposite(t *testing.T) {
	rng := NewRand(5)
	ctrs := newSeqCounters()
	got := expandPattern("SKU-{AA}-{0000}", rng, ctrs, "k")
	if !strings.HasPrefix(got, "SKU-") {
		t.Fatalf("prefix missing: %q", got)
	}
	parts := strings.Split(got, "-")
	if len(parts) != 3 || len(parts[1]) != 2 || len(parts[2]) != 4 {
		t.Fatalf("shape wrong: %q", got)
	}
}

func TestPatternUnknownPassthrough(t *testing.T) {
	rng := NewRand(1)
	ctrs := newSeqCounters()
	got := expandPattern("{what}", rng, ctrs, "k")
	if got != "{what}" {
		t.Fatalf("unknown should passthrough: %q", got)
	}
}
