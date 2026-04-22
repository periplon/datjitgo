package generator

import (
	"fmt"
	"log"
	"strings"

	"github.com/jmcarbo/datjitgo/core/model"
	"github.com/jmcarbo/datjitgo/core/ports"
	"github.com/jmcarbo/datjitgo/core/value"
)

// Engine is the default ports.Generator implementation. Construct with New()
// passing the corpus provider the engine should consult for semantic types.
type Engine struct {
	corpus ports.CorpusProvider

	// locale is resolved at Generate()-time and stashed here so helpers can
	// read it without threading an extra parameter through every call.
	locale string
}

// New returns an Engine bound to the given CorpusProvider. The provider may
// be nil — in which case every semantic type falls back to the embedded
// synthesisers — but real use cases will always pass a valid corpus.
func New(c ports.CorpusProvider) *Engine {
	return &Engine{corpus: c, locale: "en-US"}
}

var _ ports.Generator = (*Engine)(nil)

// Generate implements ports.Generator: it walks the entity dependency graph
// in topological order and, for each entity, produces the configured number
// of rows according to the pipeline described in the design spec.
func (e *Engine) Generate(doc *model.Document, opts ports.GenerateOptions) (*value.Dataset, error) {
	seed := resolveSeed(doc, opts)
	locale := resolveLocale(doc, opts)
	e.locale = locale

	// LLM preprocessing: expand @llm_values into @values using corpus-backed
	// stub content, and push an entity-level _meta @llm down onto every
	// string-shaped field that has no generator of its own. Phase 1 ships
	// with a deterministic stub backend; live providers are phase 2.
	preprocessLLM(doc)

	order, err := plan(doc)
	if err != nil {
		return nil, err
	}

	// Resolve volumes with full override precedence.
	volumes := make(map[string]int, len(order))
	for _, name := range order {
		volumes[name] = resolveVolume(name, doc, opts)
	}

	// Build a lookup of named-enum definitions so we can resolve NamedType
	// references that point at enums.
	enumByName := map[string]model.EnumDef{}
	doc.Enums.Each(func(name string, d model.EnumDef) bool {
		enumByName[name] = d
		return true
	})

	root := NewRand(seed)
	rowState := &generationState{
		doc:       doc,
		enumDefs:  enumByName,
		rng:       root,
		seqs:      newSeqCounters(),
		unique:    map[string]map[string]struct{}{},
		generated: map[string][]*value.Object{},
	}

	ds := value.NewDataset()
	for _, name := range order {
		entity, _ := doc.Entities.Get(name)
		vol := volumes[name]
		if vol < 0 {
			vol = 0
		}

		rows := make([]*value.Object, 0, vol)
		entSub := root.Substream("entity:" + name)
		rowState.entityRNG = entSub

		for i := 0; i < vol; i++ {
			rowRNG := entSub.Substream(fmt.Sprintf("row:%d", i))
			row, err := e.generateRow(entity, rowState, rowRNG)
			if err != nil {
				return nil, err
			}
			rows = append(rows, row)
			rowState.generated[name] = append(rowState.generated[name], row)
		}

		// Entity-level rule validation post-pass: emit warnings or enforce
		// strict rules across the produced set.
		e.enforceDatasetRules(doc, name, rows)

		ds.Entities.Set(name, rows)
	}

	if opts.EntityFilter != "" {
		filtered := value.NewDataset()
		rows, ok := ds.Entities.Get(opts.EntityFilter)
		if ok {
			filtered.Entities.Set(opts.EntityFilter, rows)
		}
		return filtered, nil
	}

	return ds, nil
}

// generationState groups the mutable state that travels with each row.
// Keeping it in one struct makes the many helpers easier to read than
// pushing a dozen args down the call chain.
type generationState struct {
	doc       *model.Document
	enumDefs  map[string]model.EnumDef
	rng       ports.Randomizer
	entityRNG ports.Randomizer
	seqs      *seqCounters

	// Unique values per entity+field, for @unique retry logic.
	unique map[string]map[string]struct{}

	// Already-generated rows keyed by entity name; used for reference
	// resolution, cross-row rule evaluation and expression lookups.
	generated map[string][]*value.Object
}

func (s *generationState) uniqueKey(entity, field string) map[string]struct{} {
	k := entity + "." + field
	m, ok := s.unique[k]
	if !ok {
		m = map[string]struct{}{}
		s.unique[k] = m
	}
	return m
}

// resolveSeed honours the GenerateOptions override, then document.Generation,
// then document.Seed, then 0.
func resolveSeed(doc *model.Document, opts ports.GenerateOptions) int64 {
	if opts.SeedOverride != nil {
		return *opts.SeedOverride
	}
	if doc.Generation.Seed != nil {
		return *doc.Generation.Seed
	}
	if doc.Seed != nil {
		return *doc.Seed
	}
	return 0
}

func resolveLocale(doc *model.Document, opts ports.GenerateOptions) string {
	if opts.LocaleOverride != "" {
		return opts.LocaleOverride
	}
	if doc.Generation.Locale != "" {
		return doc.Generation.Locale
	}
	if doc.Locale != "" {
		return doc.Locale
	}
	return "en-US"
}

func resolveVolume(name string, doc *model.Document, opts ports.GenerateOptions) int {
	if v, ok := opts.VolumeOverride[name]; ok {
		return v
	}
	if v, ok := doc.Volume[name]; ok {
		if v.Exact > 0 {
			return v.Exact
		}
		if v.Min != 0 || v.Max != 0 {
			// Deterministic midpoint keeps output stable.
			return (v.Min + v.Max) / 2
		}
	}
	return 10
}

// enforceDatasetRules is a cheap post-pass that logs @warn rule violations
// for the named entity. @strict rules are handled in-line during row
// generation; phase-1 coarsely logs mismatches here when no retry could
// recover.
func (e *Engine) enforceDatasetRules(doc *model.Document, entity string, rows []*value.Object) {
	for _, r := range doc.Rules {
		if r.Severity != model.RuleWarn {
			continue
		}
		// Only check rules that reference this entity — cheap fallback.
		if !strings.Contains(r.Expr, entity+".") && !strings.HasPrefix(r.Expr, entity+" ") {
			continue
		}
		for _, row := range rows {
			v, err := evalRule(r.Expr, entity, row, nil)
			if err != nil {
				continue
			}
			if !truthy(v) {
				log.Printf("warn: rule %q violated", r.Expr)
				break
			}
		}
	}
}
