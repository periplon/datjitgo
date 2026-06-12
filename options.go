package datjit

import (
	"github.com/periplon/datjitgo/core/errors"
	"github.com/periplon/datjitgo/core/ports"
	"github.com/periplon/datjitgo/generator"
)

// Option mutates a Service during construction. Options are applied in the
// order they are passed to New(); later options override earlier ones when
// they touch the same field.
type Option func(*Service) error

// WithParser swaps in an alternative ports.Parser implementation.
func WithParser(p ports.Parser) Option {
	return func(s *Service) error {
		if p == nil {
			return &errors.Error{Kind: errors.KindValidation, Message: "WithParser: parser is required (cannot be nil); omit this option to use the default parser"}
		}
		s.parser = p
		return nil
	}
}

// WithGenerator swaps in an alternative ports.Generator implementation.
func WithGenerator(g ports.Generator) Option {
	return func(s *Service) error {
		if g == nil {
			return &errors.Error{Kind: errors.KindValidation, Message: "WithGenerator: generator is required (cannot be nil); omit this option to use the default engine"}
		}
		s.gen = g
		if eng, ok := s.gen.(*generator.Engine); ok && s.llm != nil {
			eng.WithLLMProvider(s.llm)
		}
		return nil
	}
}

// WithWriter registers (or replaces) a writer under the key returned by its
// Format() method.
func WithWriter(w ports.Writer) Option {
	return func(s *Service) error {
		if w == nil {
			return &errors.Error{Kind: errors.KindValidation, Message: "WithWriter: writer is required (cannot be nil)"}
		}
		if s.writers == nil {
			s.writers = map[string]ports.Writer{}
		}
		s.writers[w.Format()] = w
		return nil
	}
}

// WithCorpus replaces the corpus provider and rebinds the built-in generator
// to use it. If a custom generator has been installed via WithGenerator it
// is left untouched, since the façade cannot know how to rebind an
// arbitrary generator implementation.
func WithCorpus(c ports.CorpusProvider) Option {
	return func(s *Service) error {
		if c == nil {
			return &errors.Error{Kind: errors.KindValidation, Message: "WithCorpus: corpus is required (cannot be nil); omit this option to use the embedded corpus"}
		}
		s.corpus = c
		// Only rebind when the generator is the built-in engine: other
		// implementations might hold their own corpus reference.
		if _, ok := s.gen.(*generator.Engine); ok {
			s.gen = generator.New(c).WithLLMProvider(s.llm)
		}
		return nil
	}
}

// WithLLMProvider enables live @llm and @llm_values generation. Passing nil is
// invalid; omit this option to keep deterministic offline stub behavior.
func WithLLMProvider(p ports.LLMProvider) Option {
	return func(s *Service) error {
		if p == nil {
			return &errors.Error{Kind: errors.KindValidation, Message: "WithLLMProvider: provider is required (cannot be nil); omit this option to keep the deterministic offline stub"}
		}
		s.llm = p
		if eng, ok := s.gen.(*generator.Engine); ok {
			eng.WithLLMProvider(p)
		}
		return nil
	}
}

// WithDirtyRate enables seeded dirty-data corruption for every subsequent
// Generate call. A rate in (0, 1] acts like an entity-level
// `_meta: "@dirty(rate=rate)"` with the default kinds (typo, case,
// whitespace) for every entity that has no entity-level @dirty of its own;
// field-level @dirty decorators always win. rate must be in [0, 1]; zero
// disables the global dial (schema-declared @dirty decorators still apply).
// Corruption is applied after rule enforcement, so dirty rows may violate
// cross-entity rules — that is the point of the feature.
func WithDirtyRate(rate float64) Option {
	return func(s *Service) error {
		if rate < 0 || rate > 1 {
			return &errors.Error{Kind: errors.KindValidation, Message: "WithDirtyRate: rate must be between 0 and 1"}
		}
		s.dirtyRate = rate
		return nil
	}
}

// WithSeed pins the seed used for every subsequent Generate call. Mirrors
// the ports.GenerateOptions.SeedOverride override precedence.
func WithSeed(seed int64) Option {
	return func(s *Service) error {
		v := seed
		s.seed = &v
		return nil
	}
}

// WithLocale pins the locale applied to every subsequent Generate call.
func WithLocale(loc string) Option {
	return func(s *Service) error {
		s.locale = loc
		return nil
	}
}

// WithVolume replaces the per-entity volume override map wholesale. Pass an
// empty map (or don't call this option) to fall back to the document-declared
// volumes.
func WithVolume(v map[string]int) Option {
	return func(s *Service) error {
		if v == nil {
			s.volumes = nil
			return nil
		}
		out := make(map[string]int, len(v))
		for k, val := range v {
			out[k] = val
		}
		s.volumes = out
		return nil
	}
}
