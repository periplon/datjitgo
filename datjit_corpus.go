package datjit

import "github.com/jmcarbo/datjitgo/core/ports"

// Corpus exposes the underlying corpus provider so introspection tools
// (CLI, REPL, tests) can enumerate available keys without poking at
// internal fields. It is strictly additive and never mutates state.
func (s *Service) Corpus() ports.CorpusProvider {
	if s == nil {
		return nil
	}
	return s.corpus
}
