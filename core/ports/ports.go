// Package ports declares the adapter interfaces used by datjitgo.
// Concrete implementations live in the parser/, generator/, output/ and
// corpus/ packages and are wired into a datjit.Service via options.
package ports

import (
	"context"
	"io"

	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/value"
)

// Parser turns YAML bytes into a parsed Document. Name is used for error
// location (file path in messages); pass "" if unknown.
type Parser interface {
	Parse(r io.Reader, name string) (*model.Document, error)
}

// GenerateOptions lets callers override per-run knobs without mutating the
// Document in place.
type GenerateOptions struct {
	SeedOverride   *int64
	LocaleOverride string
	VolumeOverride map[string]int
	EntityFilter   string // generate only this entity + its dependencies
}

// Generator produces a Dataset from a Document.
type Generator interface {
	Generate(doc *model.Document, opts GenerateOptions) (*value.Dataset, error)
}

// LLMRequest is a single text-generation request produced by @llm or
// @llm_values. Config fields are resolved from generation.llm and any
// decorator-level overrides before the request reaches a provider.
type LLMRequest struct {
	Provider    string
	Endpoint    string
	Model       string
	APIKey      string
	Prompt      string
	Temperature *float64
	MaxTokens   *int
	TimeoutSecs *int
}

// LLMProvider completes prompts through a live or test-double backend.
// The default generator leaves this nil so generation remains deterministic
// and offline unless callers explicitly opt into a provider.
type LLMProvider interface {
	Complete(ctx context.Context, req LLMRequest) (string, error)
}

// WriteOptions are format-agnostic output knobs.
type WriteOptions struct {
	Pretty       bool
	SQLDialect   string // postgres|mysql|sqlite
	EntityFilter string
	// SQLIndexes selects which index definitions the SQL writer emits:
	// "" or "manual" → declared indexes only; "auto" → declared plus inferred;
	// "none" → suppress all. Ignored by non-SQL writers.
	SQLIndexes string
}

// Writer serialises a Dataset to an io.Writer in a specific format.
type Writer interface {
	Format() string
	Write(ds *value.Dataset, doc *model.Document, w io.Writer, opts WriteOptions) error
}

// CorpusEntry is one sampleable corpus item.
type CorpusEntry struct {
	Name   string  `json:"name"`
	Weight float64 `json:"weight,omitempty"`
}

// SampleContext is the per-call state passed to CorpusProvider.Sample.
type SampleContext struct {
	Locale string
	RNG    Randomizer
}

// CorpusProvider supplies semantic type sample pools (names, cities, etc.).
type CorpusProvider interface {
	Has(key string) bool
	Sample(ctx SampleContext, key string) (value.Value, error)
	List(locale, key string) ([]CorpusEntry, error)
	Locales() []string
}

// Randomizer isolates random number generation so tests and alternative
// determinism schemes can swap it out.
type Randomizer interface {
	// Substream returns a derived RNG seeded deterministically from this
	// RNG's state and the given scope string. Used to give each entity,
	// field, and row a stable substream.
	Substream(scope string) Randomizer
	Float() float64     // [0, 1)
	IntN(n int64) int64 // [0, n)
	NormFloat() float64 // standard normal, mean 0 variance 1
	ExpFloat() float64  // standard exponential, rate 1
	Shuffle(n int, swap func(i, j int))
}
