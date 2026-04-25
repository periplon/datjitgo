package datjit

import "github.com/jmcarbo/datjitgo/core/ports"

type corpusKeyLister interface {
	Keys() []string
}

// Corpus exposes the underlying corpus provider so introspection tools
// (CLI, REPL, tests) can enumerate available keys without poking at
// internal fields. It is strictly additive and never mutates state.
func (s *Service) Corpus() ports.CorpusProvider {
	if s == nil {
		return nil
	}
	return s.corpus
}

// CorpusKeys returns the sorted corpus keys the configured provider can
// resolve. Providers without a listing API return nil.
func (s *Service) CorpusKeys() []string {
	if s == nil || s.corpus == nil {
		return nil
	}
	lister, ok := s.corpus.(corpusKeyLister)
	if !ok {
		return nil
	}
	return lister.Keys()
}
