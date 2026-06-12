package generator

import (
	"fmt"
	"log"

	"github.com/periplon/datjitgo/core/errors"
	"github.com/periplon/datjitgo/core/model"
	coreplan "github.com/periplon/datjitgo/core/plan"
	"github.com/periplon/datjitgo/core/ports"
	"github.com/periplon/datjitgo/core/value"
)

// Engine is the default ports.Generator implementation. Construct with New()
// passing the corpus provider the engine should consult for semantic types.
type Engine struct {
	corpus ports.CorpusProvider
	llm    ports.LLMProvider

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

// WithLLMProvider installs an optional live LLM backend. Nil keeps the
// deterministic offline stub behavior.
func (e *Engine) WithLLMProvider(p ports.LLMProvider) *Engine {
	e.llm = p
	return e
}

var _ ports.Generator = (*Engine)(nil)

// Generate implements ports.Generator: it walks the entity dependency graph
// in topological order and, for each entity, produces the configured number
// of rows according to the pipeline described in the design spec.
func (e *Engine) Generate(doc *model.Document, opts ports.GenerateOptions) (*value.Dataset, error) {
	seed := resolveSeed(doc, opts)
	locale := resolveLocale(doc, opts)
	e.locale = locale

	order, err := coreplan.Entities(doc)
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
	typeByName := map[string]*model.Entity{}
	doc.Types.Each(func(name string, ent *model.Entity) bool {
		typeByName[name] = ent
		return true
	})

	root := NewRand(seed)
	rowState := &generationState{
		doc:       doc,
		llmConfig: doc.Generation.LLM,
		enumDefs:  enumByName,
		typeDefs:  typeByName,
		rng:       root,
		seqs:      newSeqCounters(),
		unique:    map[string]map[string]struct{}{},
		generated: map[string][]*value.Object{},
		pk:        primaryKeyMap(doc),
		ruleScope: computeRuleScope(doc),
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
			var row *value.Object
			for attempt := 0; attempt <= 10; attempt++ {
				uniqueBefore := cloneUnique(rowState.unique)
				attemptRNG := rowRNG
				if attempt > 0 {
					attemptRNG = rowRNG.Substream(fmt.Sprintf("attempt:%d", attempt))
				}
				row, err = e.generateRow(entity, rowState, attemptRNG)
				if err != nil {
					return nil, err
				}
				if ruleErr := e.enforceRowRules(doc, name, row, rowState, rowRNG); ruleErr != nil {
					rowState.unique = uniqueBefore
					if attempt == 10 {
						return nil, ruleErr
					}
					continue
				}
				break
			}
			rows = append(rows, row)
			rowState.generated[name] = append(rowState.generated[name], row)
		}

		// Stateful sequence post-pass: fill @series/@walk/@chain fields in
		// row-index order from per-field substreams. Runs before dataset
		// rules so @warn checks see final values. No-op (zero substream
		// derivations, zero draws) for entities without stateful fields.
		if err := e.applyStatefulSequences(entity, rows, entSub, rowState); err != nil {
			return nil, err
		}

		// Entity-level rule validation post-pass: emit warnings or enforce
		// strict rules across the produced set.
		e.enforceDatasetRules(doc, name, rows, rowState.pk, rowState.ruleScope)

		// Dirty-data post-pass (@dirty / GenerateOptions.DirtyRate). Runs
		// after rule enforcement by design: dirty data may violate rules.
		// Entities without any dirty config cost zero RNG draws here.
		e.applyDirty(entity, rows, entSub, opts, rowState)

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
	llmConfig *model.LLMConfig
	enumDefs  map[string]model.EnumDef
	typeDefs  map[string]*model.Entity
	rng       ports.Randomizer
	entityRNG ports.Randomizer
	seqs      *seqCounters

	// Unique values per entity+field, for @unique retry logic.
	unique map[string]map[string]struct{}

	// Already-generated rows keyed by entity name; used for reference
	// resolution, cross-row rule evaluation and expression lookups.
	generated map[string][]*value.Object

	// Primary-key field name per entity (only entities that declare @primary).
	// Drives FK resolution so reference identity does not depend on field
	// insertion order. Absent entries fall back to positional first-field.
	pk map[string]string

	// Per-rule target entity sets, aligned with doc.Rules by index. A rule is
	// enforced against an entity only if the entity is in its set, so bare
	// (unqualified) rules never apply to entities lacking the referenced
	// fields.
	ruleScope []map[string]struct{}
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

func cloneUnique(in map[string]map[string]struct{}) map[string]map[string]struct{} {
	out := make(map[string]map[string]struct{}, len(in))
	for k, values := range in {
		cp := make(map[string]struct{}, len(values))
		for v := range values {
			cp[v] = struct{}{}
		}
		out[k] = cp
	}
	return out
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
func (e *Engine) enforceDatasetRules(doc *model.Document, entity string, rows []*value.Object, pk map[string]string, scope []map[string]struct{}) {
	for i := range doc.Rules {
		r := doc.Rules[i]
		if r.Severity != model.RuleWarn {
			continue
		}
		// Only check rules scoped to this entity (named explicitly, or bare
		// rules whose fields this entity declares).
		if _, ok := scope[i][entity]; !ok {
			continue
		}
		for _, row := range rows {
			v, err := evalRule(r.Expr, entity, row, nil, pk)
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

func (e *Engine) enforceRowRules(doc *model.Document, entity string, row *value.Object, st *generationState, rng ports.Randomizer) error {
	for i := range doc.Rules {
		r := doc.Rules[i]
		if r.Kind != model.RuleKindExpr || r.Severity == model.RuleWarn {
			continue
		}
		if _, ok := st.ruleScope[i][entity]; !ok {
			continue
		}
		if r.Severity == model.RuleProbabilistic {
			p := r.Probability
			if p <= 0 {
				continue
			}
			if p > 1 {
				p = 1
			}
			if rng.Float() >= p {
				continue
			}
		}
		v, err := evalRule(r.Expr, entity, row, st.generated, st.pk)
		if err != nil {
			return &errors.Error{Kind: errors.KindRuleViolated, Entity: entity, Message: fmt.Sprintf("rule %q: %v", r.Expr, err), Cause: err}
		}
		if !truthy(v) {
			msg := r.ErrorMessage
			if msg == "" {
				msg = fmt.Sprintf("rule %q violated", r.Expr)
			}
			return &errors.Error{Kind: errors.KindRuleViolated, Entity: entity, Message: msg}
		}
	}
	return nil
}
