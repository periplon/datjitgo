package datjit

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/periplon/datjitgo/core/errors"
	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/ports"
	"github.com/periplon/datjitgo/core/value"
	"github.com/periplon/datjitgo/corpus"
	"github.com/periplon/datjitgo/generator"
	"github.com/periplon/datjitgo/output"
	"github.com/periplon/datjitgo/parser"
)

// Service is the public façade tying the datjitgo adapters together. Construct
// one with NewDefault() (sensible defaults) or New(opts...) (composable
// overrides). Corpus() and CorpusKeys() expose the configured corpus provider
// for introspection.
type Service struct {
	parser  ports.Parser
	gen     ports.Generator
	writers map[string]ports.Writer
	corpus  ports.CorpusProvider
	llm     ports.LLMProvider

	seed      *int64
	locale    string
	volumes   map[string]int
	dirtyRate float64
	profile   string
}

// WriteOpts is the façade-level write configuration exposed to callers. It
// mirrors the subset of ports.WriteOptions that is stable public API.
type WriteOpts struct {
	Pretty       bool
	SQLDialect   string
	EntityFilter string
	// SQLIndexes selects which indexes the SQL writer emits: "" / "manual"
	// (declared only, the default), "auto" (declared + inferred), or "none".
	SQLIndexes string
}

// NewDefault returns a Service wired with the default adapters:
//   - parser: parser.New()
//   - corpus: corpus.NewEmbedded()
//   - generator: generator.New(corpus)
//   - writers: JSON, NDJSON, CSV, YAML, SQL (from output.New*)
//
// NewDefault never fails; any misconfiguration is impossible with defaults.
func NewDefault() *Service {
	c := corpus.NewEmbedded()
	s := &Service{
		parser:  parser.New(),
		gen:     generator.New(c),
		corpus:  c,
		writers: map[string]ports.Writer{},
	}
	s.registerDefaultWriters()
	return s
}

// New returns a Service configured with the given functional options applied
// on top of the default wiring. Errors from any option are propagated.
func New(opts ...Option) (*Service, error) {
	s := NewDefault()
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(s); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// registerDefaultWriters populates the writers map with the built-in
// formats. Called from NewDefault so New() callers start with the full set
// and may replace any of them with WithWriter.
func (s *Service) registerDefaultWriters() {
	for _, w := range []ports.Writer{
		output.NewJSON(),
		output.NewNDJSON(),
		output.NewCSV(),
		output.NewYAML(),
		output.NewSQL(),
	} {
		s.writers[w.Format()] = w
	}
}

// Formats lists the registered writer format identifiers in alphabetical
// order. Useful for CLI help text and tests.
func (s *Service) Formats() []string {
	if s == nil {
		return nil
	}
	out := make([]string, 0, len(s.writers))
	for k := range s.writers {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Parse reads r and returns a *model.Document. name is used for error
// location; pass the file path for the best diagnostics.
func (s *Service) Parse(r io.Reader, name string) (*model.Document, error) {
	if s == nil {
		return nil, nilServiceErr("Parse")
	}
	if r == nil {
		return nil, &errors.Error{Kind: errors.KindValidation, Message: "nil reader"}
	}
	doc, err := s.parser.Parse(r, name)
	if err != nil {
		return nil, err
	}
	normalizePolymorphicReferences(doc)
	normalizeIndexes(doc)
	return doc, nil
}

// Generate produces a Dataset from doc. Service-level overrides (seed,
// locale, volumes) are applied via ports.GenerateOptions so the document
// is not mutated in place.
func (s *Service) Generate(doc *model.Document) (*value.Dataset, error) {
	if s == nil {
		return nil, nilServiceErr("Generate")
	}
	if doc == nil {
		return nil, &errors.Error{Kind: errors.KindValidation, Message: "nil document"}
	}
	opts := ports.GenerateOptions{
		SeedOverride:   s.seed,
		LocaleOverride: s.locale,
		DirtyRate:      s.dirtyRate,
		Profile:        s.profile,
	}
	if len(s.volumes) > 0 {
		opts.VolumeOverride = cloneVolumes(s.volumes)
	}
	return s.gen.Generate(doc, opts)
}

// Write serialises ds in the named format to w. Unknown formats return a
// *errors.Error{Kind: KindValidation}. doc may be nil for formats that
// don't need schema metadata (JSON, NDJSON, CSV, YAML) but SQL requires it.
func (s *Service) Write(ds *value.Dataset, doc *model.Document, format string, w io.Writer, opts WriteOpts) error {
	if s == nil {
		return nilServiceErr("Write")
	}
	if w == nil {
		return &errors.Error{Kind: errors.KindValidation, Message: "nil writer"}
	}
	writer, ok := s.writers[format]
	if !ok {
		return &errors.Error{
			Kind:    errors.KindValidation,
			Message: fmt.Sprintf("unknown format %q (available: %v)", format, s.Formats()),
		}
	}
	return writer.Write(ds, doc, w, ports.WriteOptions{
		Pretty:       opts.Pretty,
		SQLDialect:   opts.SQLDialect,
		EntityFilter: opts.EntityFilter,
		SQLIndexes:   opts.SQLIndexes,
	})
}

// GenerateFile is a convenience that opens path, parses it, and generates a
// Dataset. It returns both the dataset and the parsed document so callers
// can drive Write without re-parsing.
func (s *Service) GenerateFile(path string) (*value.Dataset, *model.Document, error) {
	if s == nil {
		return nil, nil, nilServiceErr("GenerateFile")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, &errors.Error{
			Kind:    errors.KindIO,
			Message: fmt.Sprintf("open %s: %v", path, err),
			Cause:   err,
		}
	}
	defer func() { _ = f.Close() }()
	doc, err := s.Parse(f, path)
	if err != nil {
		return nil, nil, err
	}
	ds, err := s.Generate(doc)
	if err != nil {
		return nil, doc, err
	}
	return ds, doc, nil
}

// nilServiceErr is the shared typed error for calling a method on a nil
// receiver. Using a single helper keeps the messages consistent.
func nilServiceErr(op string) error {
	return &errors.Error{
		Kind:    errors.KindValidation,
		Message: fmt.Sprintf("datjit.Service.%s: nil service", op),
	}
}

// cloneVolumes returns a shallow copy of the given volume map so mutations
// on the Service state don't leak into GenerateOptions passed down the
// stack (and vice versa).
func cloneVolumes(in map[string]int) map[string]int {
	out := make(map[string]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
