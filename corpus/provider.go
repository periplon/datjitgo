// Package corpus provides a CorpusProvider backed by JSON data files that are
// embedded in the binary with //go:embed. It supports weighted sampling via a
// ports.Randomizer and an optional on-disk overlay (phase 2).
package corpus

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/jmcarbo/datjitgo/core/errors"
	"github.com/jmcarbo/datjitgo/core/ports"
	"github.com/jmcarbo/datjitgo/core/value"
)

// Provider implements ports.CorpusProvider using JSON files compiled in via
// go:embed. Loaded corpora are cached after first access. An optional overlay
// directory may supply additional or replacement JSON files at runtime (the
// phase-1 implementation records the directory but does not yet consult disk).
type Provider struct {
	overlayDir string

	mu    sync.RWMutex
	cache map[string][]ports.CorpusEntry
}

// NewEmbedded returns a Provider that reads only from the embedded JSON files.
func NewEmbedded() *Provider {
	return &Provider{cache: map[string][]ports.CorpusEntry{}}
}

// NewWithOverlay returns a Provider that reads from the embedded JSON files
// and, optionally, an on-disk overlay directory. The overlay is not yet
// consulted in phase 1 but is wired for forward-compatibility with the live
// corpus updater planned for phase 2.
func NewWithOverlay(baseDir string) *Provider {
	return &Provider{
		overlayDir: baseDir,
		cache:      map[string][]ports.CorpusEntry{},
	}
}

// Has reports whether the provider can resolve the given key.
func (p *Provider) Has(key string) bool {
	_, err := p.load(key)
	return err == nil
}

// List returns every entry for key in deterministic (alphabetical) order.
// locale is currently ignored; only "en-US" is supported.
func (p *Provider) List(_ string, key string) ([]ports.CorpusEntry, error) {
	entries, err := p.load(key)
	if err != nil {
		return nil, err
	}
	out := make([]ports.CorpusEntry, len(entries))
	copy(out, entries)
	return out, nil
}

// Sample picks a weighted entry from key using ctx.RNG.Float().
func (p *Provider) Sample(ctx ports.SampleContext, key string) (value.Value, error) {
	entries, err := p.load(key)
	if err != nil {
		return value.Null(), err
	}
	if len(entries) == 0 {
		return value.Null(), &errors.Error{
			Kind:    errors.KindCorpusMissing,
			Message: fmt.Sprintf("empty corpus: %q", key),
		}
	}
	if ctx.RNG == nil {
		return value.Null(), &errors.Error{
			Kind:    errors.KindGeneration,
			Message: "corpus sample: nil RNG",
		}
	}
	var total float64
	for _, e := range entries {
		total += effectiveWeight(e.Weight)
	}
	if total <= 0 {
		// Shouldn't happen (effectiveWeight enforces >= 1 for every entry),
		// but guard against pathological input.
		return value.Str(entries[0].Name), nil
	}
	pick := ctx.RNG.Float() * total
	for _, e := range entries {
		w := effectiveWeight(e.Weight)
		if pick < w {
			return value.Str(e.Name), nil
		}
		pick -= w
	}
	return value.Str(entries[len(entries)-1].Name), nil
}

// Locales returns the supported locales. Phase 1 ships only en-US.
func (p *Provider) Locales() []string {
	return []string{"en-US"}
}

// load fetches entries for key, using the cache when possible. Entries are
// returned in deterministic alphabetical order (stable for diffs and tests).
func (p *Provider) load(key string) ([]ports.CorpusEntry, error) {
	p.mu.RLock()
	if v, ok := p.cache[key]; ok {
		p.mu.RUnlock()
		return v, nil
	}
	p.mu.RUnlock()

	name := "data/" + strings.ReplaceAll(key, ".", "_") + ".json"
	raw, err := embedded.ReadFile(name)
	if err != nil {
		return nil, &errors.Error{
			Kind:    errors.KindCorpusMissing,
			Message: fmt.Sprintf("corpus key %q", key),
			Cause:   err,
		}
	}
	entries, err := parseEntries(raw)
	if err != nil {
		return nil, &errors.Error{
			Kind:    errors.KindCorpusMissing,
			Message: fmt.Sprintf("corpus %q: %v", key, err),
			Cause:   err,
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	p.mu.Lock()
	p.cache[key] = entries
	p.mu.Unlock()

	return entries, nil
}

// parseEntries accepts the JSON array format described in the spec:
//
//	[
//	  {"name": "Maria", "weight": 2.0},
//	  {"name": "Noah"},
//	  "Sofia"
//	]
//
// A bare string becomes {Name: s, Weight: 1}. A weight of zero or below is
// treated as 1 at sample time (see effectiveWeight).
func parseEntries(data []byte) ([]ports.CorpusEntry, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	out := make([]ports.CorpusEntry, 0, len(raw))
	for _, rm := range raw {
		trimmed := bytesTrimSpace(rm)
		if len(trimmed) == 0 {
			continue
		}
		switch trimmed[0] {
		case '{':
			var entry ports.CorpusEntry
			if err := json.Unmarshal(rm, &entry); err != nil {
				return nil, err
			}
			if entry.Name == "" {
				continue
			}
			out = append(out, entry)
		case '"':
			var s string
			if err := json.Unmarshal(rm, &s); err != nil {
				return nil, err
			}
			if s == "" {
				continue
			}
			out = append(out, ports.CorpusEntry{Name: s, Weight: 1})
		default:
			return nil, fmt.Errorf("unexpected JSON token: %s", string(rm))
		}
	}
	return out, nil
}

// effectiveWeight treats non-positive weights as 1, matching the spec.
func effectiveWeight(w float64) float64 {
	if w <= 0 {
		return 1
	}
	return w
}

// bytesTrimSpace trims ASCII whitespace without allocating. Avoids importing
// bytes just for this helper.
func bytesTrimSpace(b []byte) []byte {
	i := 0
	for i < len(b) && isSpace(b[i]) {
		i++
	}
	j := len(b)
	for j > i && isSpace(b[j-1]) {
		j--
	}
	return b[i:j]
}

func isSpace(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\r':
		return true
	}
	return false
}

// Compile-time check that Provider satisfies ports.CorpusProvider.
var _ ports.CorpusProvider = (*Provider)(nil)
