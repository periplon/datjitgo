package generator

import (
	"strconv"

	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/ports"
	"github.com/periplon/datjitgo/core/value"
)

// dirtyKind enumerates the corruption operators supported by @dirty.
type dirtyKind int

const (
	dirtyTypo dirtyKind = iota
	dirtyCase
	dirtyWhitespace
	dirtyNull
	dirtyFormatMix
	dirtyDuplicate
)

// dirtyKindNames maps decorator idents to corruption kinds. Unknown idents
// are silently ignored so future kinds stay forward-compatible.
var dirtyKindNames = map[string]dirtyKind{
	"typo":       dirtyTypo,
	"case":       dirtyCase,
	"whitespace": dirtyWhitespace,
	"null":       dirtyNull,
	"format_mix": dirtyFormatMix,
	"duplicate":  dirtyDuplicate,
}

// defaultDirtyRate is the per-row corruption probability used when @dirty is
// declared without an explicit rate.
const defaultDirtyRate = 0.05

// defaultDirtyKinds is the kind pool used when @dirty names no kinds (and by
// the global GenerateOptions.DirtyRate dial).
func defaultDirtyKinds() []dirtyKind {
	return []dirtyKind{dirtyTypo, dirtyCase, dirtyWhitespace}
}

// dirtyConfig is one parsed @dirty(...) decorator: a per-row corruption rate
// plus the kind pool to draw from.
type dirtyConfig struct {
	rate  float64
	kinds []dirtyKind
}

// parseDirtyConfig reads a @dirty decorator into a dirtyConfig. rate= is the
// only recognised KV argument (clamped to [0, 1]); remaining bare idents name
// corruption kinds. Missing pieces fall back to defaults per the spec.
func parseDirtyConfig(d *model.Decorator) dirtyConfig {
	cfg := dirtyConfig{rate: defaultDirtyRate}
	for _, a := range d.Args {
		switch a.Kind {
		case model.ArgKV:
			if a.Key == "rate" {
				if f, err := strconv.ParseFloat(a.Value, 64); err == nil {
					cfg.rate = f
				}
			}
		case model.ArgIdent:
			if k, ok := dirtyKindNames[a.Ident]; ok {
				cfg.kinds = append(cfg.kinds, k)
			}
		default:
			// Literals and ranges have no meaning for @dirty; ignore.
		}
	}
	cfg.rate = clampDirtyRate(cfg.rate)
	if len(cfg.kinds) == 0 {
		cfg.kinds = defaultDirtyKinds()
	}
	return cfg
}

func clampDirtyRate(r float64) float64 {
	if r < 0 {
		return 0
	}
	if r > 1 {
		return 1
	}
	return r
}

// dirtyFieldPlan is the resolved corruption config for one field: precedence
// (field decorator > entity meta > global option) and static kind filtering
// already applied.
type dirtyFieldPlan struct {
	name   string
	rate   float64
	kinds  []dirtyKind
	unique bool
}

// dirtyPlan is an entity's full dirty configuration, built once per entity
// before the post-pass row loop. A nil plan means "no dirty config anywhere"
// and must cost zero substream derivations and zero RNG draws.
type dirtyPlan struct {
	fields []dirtyFieldPlan

	// dupRate > 0 enables the entity-level duplicate kind: each row draws
	// once against dupRate and, when triggered, becomes a near-copy of the
	// previous row.
	dupRate float64
	// copyFields are the fields a duplicate copies from the previous row
	// (safety-exempt and @unique fields excluded so identity, references and
	// uniqueness survive).
	copyFields []string
	// corruptFields is the subset of copyFields statically eligible for the
	// typo/whitespace re-corruption that makes a duplicate "near".
	corruptFields []string
}

// buildDirtyPlan resolves the entity's dirty configuration with full
// precedence: a field-level @dirty wins over the entity _meta @dirty, which
// wins over the global rate (globalRate > 0 acts like an entity-level
// @dirty(rate=globalRate) with default kinds for entities without their own
// meta config). Returns nil when nothing is configured.
func buildDirtyPlan(entity *model.Entity, globalRate float64) *dirtyPlan {
	var entityCfg *dirtyConfig
	if d := model.FindDecorator(entity.Meta, "dirty"); d != nil {
		c := parseDirtyConfig(d)
		entityCfg = &c
	} else if globalRate > 0 {
		c := dirtyConfig{rate: clampDirtyRate(globalRate), kinds: defaultDirtyKinds()}
		entityCfg = &c
	}

	plan := &dirtyPlan{}
	entity.Fields.Each(func(fname string, f *model.Field) bool {
		if hasDecorator(f.Decorators, "internal") {
			// @internal fields are stripped from rows before the post-pass;
			// @dirty on them is pointless and statically excluded.
			return true
		}
		unique := hasDecorator(f.Decorators, "unique")
		if d := model.FindDecorator(f.Decorators, "dirty"); d != nil {
			// Field-level @dirty wins, even on safety-exempt fields — the
			// user asked for it explicitly.
			cfg := parseDirtyConfig(d)
			kinds := filterDirtyKinds(cfg.kinds, f.Type, false)
			if cfg.rate > 0 && len(kinds) > 0 {
				plan.fields = append(plan.fields, dirtyFieldPlan{name: fname, rate: cfg.rate, kinds: kinds, unique: unique})
			}
			return true
		}
		if entityCfg == nil || dirtyExempt(f) {
			return true
		}
		kinds := filterDirtyKinds(entityCfg.kinds, f.Type, unique)
		if entityCfg.rate > 0 && len(kinds) > 0 {
			plan.fields = append(plan.fields, dirtyFieldPlan{name: fname, rate: entityCfg.rate, kinds: kinds, unique: unique})
		}
		return true
	})

	if entityCfg != nil && entityCfg.rate > 0 && containsDirtyKind(entityCfg.kinds, dirtyDuplicate) {
		entity.Fields.Each(func(fname string, f *model.Field) bool {
			if dirtyExempt(f) || hasDecorator(f.Decorators, "unique") {
				return true
			}
			plan.copyFields = append(plan.copyFields, fname)
			if dirtyTypeAllows(f.Type, dirtyTypo) {
				plan.corruptFields = append(plan.corruptFields, fname)
			}
			return true
		})
		if len(plan.copyFields) > 0 {
			plan.dupRate = entityCfg.rate
		}
	}

	if len(plan.fields) == 0 && plan.dupRate == 0 {
		return nil
	}
	return plan
}

// filterDirtyKinds intersects the requested kind pool with what the field's
// declared type statically supports. duplicate is always removed (it is
// entity-level only); dropUniqueUnsafe additionally removes null, used for
// entity/global config on @unique fields so corruption cannot violate the
// uniqueness the schema promised. The filter is purely static so per-field
// RNG draw counts never depend on runtime value content.
func filterDirtyKinds(kinds []dirtyKind, t model.TypeExpr, dropUniqueUnsafe bool) []dirtyKind {
	out := make([]dirtyKind, 0, len(kinds))
	for _, k := range kinds {
		if k == dirtyDuplicate {
			continue
		}
		if dropUniqueUnsafe && k == dirtyNull {
			continue
		}
		if !dirtyTypeAllows(t, k) {
			continue
		}
		out = append(out, k)
	}
	return out
}

// dirtyTypeAllows reports whether kind k can apply to a value of declared
// type t. Known scalar types are filtered exactly; composite/named types are
// unknown statically, so string-class kinds and format_mix stay in the pool
// and the operator no-ops at runtime (still consuming its draws).
func dirtyTypeAllows(t model.TypeExpr, k dirtyKind) bool {
	if n, ok := t.(model.Nullable); ok {
		return dirtyTypeAllows(n.Inner, k)
	}
	if k == dirtyNull {
		return true
	}
	switch tt := t.(type) {
	case model.Primitive:
		switch tt.Kind {
		case model.PrimString:
			return k == dirtyTypo || k == dirtyCase || k == dirtyWhitespace
		case model.PrimDatetime, model.PrimDate, model.PrimTime:
			return k == dirtyFormatMix
		default:
			return false
		}
	case model.Semantic, model.EnumInline:
		return k == dirtyTypo || k == dirtyCase || k == dirtyWhitespace
	case model.Reference:
		return false
	default:
		return true
	}
}

// dirtyExempt reports whether f is safety-exempt from entity/global dirty
// config per the spec: @primary, @auto and @internal fields, reference fields
// (including unions of references) and synthetic polymorphic discriminators
// are never corrupted unless a field-level @dirty asks for it explicitly.
func dirtyExempt(f *model.Field) bool {
	if f.DiscriminatorFor != "" {
		return true
	}
	for _, name := range []string{"primary", "auto", "internal"} {
		if hasDecorator(f.Decorators, name) {
			return true
		}
	}
	return dirtyTypeHasReference(f.Type)
}

// dirtyTypeHasReference walks a TypeExpr looking for entity references so
// foreign keys (and polymorphic unions) are exempt from entity-level dirt.
func dirtyTypeHasReference(t model.TypeExpr) bool {
	switch tt := t.(type) {
	case model.Reference:
		return true
	case model.Nullable:
		return dirtyTypeHasReference(tt.Inner)
	case model.List:
		return dirtyTypeHasReference(tt.Element)
	case model.Map:
		return dirtyTypeHasReference(tt.Key) || dirtyTypeHasReference(tt.Value)
	case model.Tuple:
		for _, e := range tt.Elements {
			if dirtyTypeHasReference(e) {
				return true
			}
		}
	case model.Union:
		for _, e := range tt.Variants {
			if dirtyTypeHasReference(e) {
				return true
			}
		}
	}
	return false
}

func containsDirtyKind(kinds []dirtyKind, k dirtyKind) bool {
	for _, kk := range kinds {
		if kk == k {
			return true
		}
	}
	return false
}

// applyDirty is the per-entity dirty-data post-pass. It runs after
// enforceDatasetRules by design: dirty data may violate cross-entity rules —
// that is the point of the feature.
//
// When the entity has no dirty configuration anywhere (no field @dirty, no
// _meta @dirty, no global DirtyRate) the pass returns before deriving any
// substream or drawing from the RNG, so unconfigured schemas remain
// byte-identical to pre-@dirty output.
func (e *Engine) applyDirty(entity *model.Entity, rows []*value.Object, entSub ports.Randomizer, opts ports.GenerateOptions, st *generationState) {
	plan := buildDirtyPlan(entity, opts.DirtyRate)
	if plan == nil {
		return
	}
	sub := entSub.Substream("dirty")
	for i, row := range rows {
		if plan.dupRate > 0 {
			// One trigger draw per row — including row 0, where the
			// duplicate is a no-op — keeps the decision stream stable.
			if sub.Float() < plan.dupRate && i > 0 {
				dirtyDuplicateRow(row, rows[i-1], plan, sub)
			}
		}
		for _, fp := range plan.fields {
			if sub.Float() >= fp.rate {
				continue
			}
			kind := fp.kinds[int(sub.IntN(int64(len(fp.kinds))))]
			orig, _ := row.Get(fp.name)
			corrupted := applyDirtyOp(kind, orig, sub)
			if fp.unique && !corrupted.IsNull() {
				// Re-check the uniqueness set; on collision keep the
				// original value so @unique still holds post-corruption.
				uk := st.uniqueKey(entity.Name, fp.name)
				key := valueKey(corrupted)
				if _, hit := uk[key]; hit {
					continue
				}
				uk[key] = struct{}{}
			}
			row.Set(fp.name, corrupted)
		}
	}
}

// dirtyDuplicateRow turns row into a near-copy of prev: every copyable field
// is taken from prev, then 1–2 seeded eligible fields are re-corrupted with
// typo/whitespace so the copy is near rather than exact. All index and
// operator choices come from sub with content-independent draw counts.
func dirtyDuplicateRow(row, prev *value.Object, plan *dirtyPlan, sub ports.Randomizer) {
	for _, name := range plan.copyFields {
		if v, ok := prev.Get(name); ok {
			row.Set(name, v)
		}
	}
	n := len(plan.corruptFields)
	if n == 0 {
		return
	}
	count := 1 + scaleDirtyIndex(sub.Float(), 2)
	for j := 0; j < count; j++ {
		name := plan.corruptFields[scaleDirtyIndex(sub.Float(), n)]
		kind := dirtyTypo
		if sub.Float() >= 0.5 {
			kind = dirtyWhitespace
		}
		v, _ := row.Get(name)
		row.Set(name, applyDirtyOp(kind, v, sub))
	}
}
