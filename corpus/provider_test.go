package corpus

import (
	stderrors "errors"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/jmcarbo/datjitgo/core/errors"
	"github.com/jmcarbo/datjitgo/core/ports"
	"github.com/jmcarbo/datjitgo/core/value"
)

// requiredKeys is the set of corpus keys the embedded provider MUST expose
// with at least minEntries entries each. It mirrors the contract stated in
// the Task 6 brief and the phase-1 design document.
var requiredKeys = []string{
	"person.first_names",
	"person.last_names",
	"person.prefixes",
	"person.suffixes",
	"person.usernames",
	"person.bios",
	"email_domains",
	"phone_area_codes",
	"address.cities",
	"address.states",
	"address.streets",
	"address.countries",
	"address.zip_prefixes",
	"timezones",
	"company.names",
	"company.industries",
	"company.catch_phrases",
	"job.titles",
	"job.departments",
	"product.titles",
	"product.descriptions",
	"text.words",
	"text.sentences",
	"text.paragraphs",
	"color.names",
	"file.extensions",
	"mime.types",
}

const minEntries = 20

func TestHasRequiredKeys(t *testing.T) {
	p := NewEmbedded()
	for _, k := range requiredKeys {
		k := k
		t.Run(k, func(t *testing.T) {
			if !p.Has(k) {
				t.Fatalf("Has(%q) = false, want true", k)
			}
		})
	}
}

func TestHasUnknownKeyIsFalse(t *testing.T) {
	p := NewEmbedded()
	if p.Has("this.key.does.not.exist") {
		t.Fatal("Has(unknown) = true, want false")
	}
}

func TestListReturnsMinimumEntries(t *testing.T) {
	p := NewEmbedded()
	for _, k := range requiredKeys {
		k := k
		t.Run(k, func(t *testing.T) {
			entries, err := p.List("en-US", k)
			if err != nil {
				t.Fatalf("List(%q): %v", k, err)
			}
			if len(entries) < minEntries {
				t.Fatalf("List(%q) returned %d entries, want >= %d",
					k, len(entries), minEntries)
			}
			for i, e := range entries {
				if e.Name == "" {
					t.Fatalf("List(%q)[%d] has empty Name", k, i)
				}
			}
		})
	}
}

func TestListIsSortedAndStable(t *testing.T) {
	p := NewEmbedded()
	entries, err := p.List("en-US", "person.first_names")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// Verify sorted ascending by Name.
	if !sort.SliceIsSorted(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	}) {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name
		}
		t.Fatalf("List not sorted: %v", names)
	}
	// Repeated calls should produce identical results.
	again, err := p.List("en-US", "person.first_names")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(again) != len(entries) {
		t.Fatalf("len mismatch: %d vs %d", len(again), len(entries))
	}
	for i := range entries {
		if again[i] != entries[i] {
			t.Fatalf("entry %d differs: %+v vs %+v", i, again[i], entries[i])
		}
	}
}

func TestListReturnsCopy(t *testing.T) {
	p := NewEmbedded()
	a, err := p.List("en-US", "color.names")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(a) == 0 {
		t.Fatal("expected entries")
	}
	original := a[0].Name
	a[0].Name = "MUTATED"
	b, err := p.List("en-US", "color.names")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if b[0].Name != original {
		t.Fatalf("List returned internal slice: mutation leaked (got %q, want %q)",
			b[0].Name, original)
	}
}

func TestListMissingKeyErrorsCorpusMissing(t *testing.T) {
	p := NewEmbedded()
	_, err := p.List("en-US", "totally.bogus.key")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
	if !stderrors.Is(err, errors.ErrCorpusMissing) {
		t.Fatalf("wrong error kind: %v", err)
	}
}

func TestOverlayReplacesEmbeddedCorpusKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "person_first_names.json")
	if err := os.WriteFile(path, []byte(`["OverlayOnly"]`), 0o644); err != nil {
		t.Fatal(err)
	}
	p := NewWithOverlay(dir)
	entries, err := p.List("en-US", "person.first_names")
	if err != nil {
		t.Fatalf("List overlay key: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "OverlayOnly" {
		t.Fatalf("overlay entries = %+v, want only OverlayOnly", entries)
	}
}

func TestOverlayAddsKeys(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "custom_animals.json"), []byte(`["otter"]`), 0o644); err != nil {
		t.Fatal(err)
	}
	p := NewWithOverlay(dir)
	if !p.Has("custom.animals") {
		t.Fatal("overlay-only key was not resolvable")
	}
	keys := p.Keys()
	i := sort.SearchStrings(keys, "custom.animals")
	if i >= len(keys) || keys[i] != "custom.animals" {
		t.Fatalf("Keys() = %v, want custom.animals", keys)
	}
}

func TestSampleReturnsStringValue(t *testing.T) {
	p := NewEmbedded()
	ctx := ports.SampleContext{Locale: "en-US", RNG: newTestRNG(42)}
	v, err := p.Sample(ctx, "person.first_names")
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}
	if v.Kind != value.KindString {
		t.Fatalf("Sample returned kind %v, want %v", v.Kind, value.KindString)
	}
	if v.S == "" {
		t.Fatal("Sample returned empty string")
	}
}

func TestSampleDeterministicForSeed(t *testing.T) {
	p := NewEmbedded()
	// Two providers, same seed: same sequence.
	runA := make([]string, 10)
	runB := make([]string, 10)
	rngA := newTestRNG(12345)
	rngB := newTestRNG(12345)
	ctxA := ports.SampleContext{Locale: "en-US", RNG: rngA}
	ctxB := ports.SampleContext{Locale: "en-US", RNG: rngB}
	for i := 0; i < 10; i++ {
		va, err := p.Sample(ctxA, "address.cities")
		if err != nil {
			t.Fatalf("A: %v", err)
		}
		vb, err := p.Sample(ctxB, "address.cities")
		if err != nil {
			t.Fatalf("B: %v", err)
		}
		runA[i] = va.S
		runB[i] = vb.S
	}
	for i := range runA {
		if runA[i] != runB[i] {
			t.Fatalf("nondeterministic at %d: %q vs %q", i, runA[i], runB[i])
		}
	}
}

func TestSampleDifferentSeedsDiffer(t *testing.T) {
	p := NewEmbedded()
	ctx1 := ports.SampleContext{Locale: "en-US", RNG: newTestRNG(1)}
	ctx2 := ports.SampleContext{Locale: "en-US", RNG: newTestRNG(2)}
	match := 0
	for i := 0; i < 20; i++ {
		a, err := p.Sample(ctx1, "person.first_names")
		if err != nil {
			t.Fatal(err)
		}
		b, err := p.Sample(ctx2, "person.first_names")
		if err != nil {
			t.Fatal(err)
		}
		if a.S == b.S {
			match++
		}
	}
	// With a 100-entry pool, 20 shared draws should almost never fully match.
	if match >= 20 {
		t.Fatalf("streams with different seeds produced identical runs")
	}
}

func TestSampleRespectsWeight(t *testing.T) {
	// email_domains has gmail.com at weight 30 and example.com at weight 20.
	// Over many draws, gmail.com should clearly dominate most low-weight items.
	p := NewEmbedded()
	ctx := ports.SampleContext{Locale: "en-US", RNG: newTestRNG(7)}
	counts := map[string]int{}
	const n = 5000
	for i := 0; i < n; i++ {
		v, err := p.Sample(ctx, "email_domains")
		if err != nil {
			t.Fatal(err)
		}
		counts[v.S]++
	}
	gmail := counts["gmail.com"]
	aol := counts["aol.com"] // weight 2
	if gmail == 0 {
		t.Fatal("gmail.com never sampled — weighted sampling broken")
	}
	if gmail <= aol {
		t.Fatalf("expected gmail.com (weight 30) to outdraw aol.com (weight 2); got %d vs %d",
			gmail, aol)
	}
}

func TestSampleEmptyRNGReturnsGenerationError(t *testing.T) {
	p := NewEmbedded()
	_, err := p.Sample(ports.SampleContext{Locale: "en-US"}, "person.first_names")
	if err == nil {
		t.Fatal("expected error with nil RNG")
	}
}

func TestSampleMissingKey(t *testing.T) {
	p := NewEmbedded()
	ctx := ports.SampleContext{Locale: "en-US", RNG: newTestRNG(1)}
	_, err := p.Sample(ctx, "nope.nope")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
	if !stderrors.Is(err, errors.ErrCorpusMissing) {
		t.Fatalf("wrong error kind: %v", err)
	}
}

func TestLocalesIncludesEnUS(t *testing.T) {
	p := NewEmbedded()
	found := false
	for _, l := range p.Locales() {
		if l == "en-US" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Locales() = %v, want en-US", p.Locales())
	}
}

func TestParseEntriesMixedFormats(t *testing.T) {
	data := []byte(`[
		{"name": "Alpha", "weight": 2.5},
		{"name": "Beta"},
		"Gamma",
		{"name": "Delta", "weight": 0},
		{"name": "Epsilon", "weight": -4}
	]`)
	got, err := parseEntries(data)
	if err != nil {
		t.Fatalf("parseEntries: %v", err)
	}
	want := []ports.CorpusEntry{
		{Name: "Alpha", Weight: 2.5},
		{Name: "Beta", Weight: 0},
		{Name: "Gamma", Weight: 1},
		{Name: "Delta", Weight: 0},
		{Name: "Epsilon", Weight: -4},
	}
	if len(got) != len(want) {
		t.Fatalf("len mismatch: got %d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("entry %d: got %+v want %+v", i, got[i], want[i])
		}
	}
}

func TestEffectiveWeight(t *testing.T) {
	cases := []struct {
		in, want float64
	}{
		{0, 1},
		{-1, 1},
		{0.5, 0.5},
		{3, 3},
	}
	for _, c := range cases {
		if got := effectiveWeight(c.in); got != c.want {
			t.Fatalf("effectiveWeight(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestNewWithOverlayReturnsProvider(t *testing.T) {
	p := NewWithOverlay("/tmp/nonexistent-overlay")
	if p == nil {
		t.Fatal("nil provider")
	}
	// Still backed by embedded data.
	if !p.Has("person.first_names") {
		t.Fatal("overlay provider lost embedded data")
	}
}
