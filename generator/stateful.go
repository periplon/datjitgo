package generator

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/periplon/datjitgo/core/errors"
	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/ports"
	"github.com/periplon/datjitgo/core/value"
)

// Stateful sequence decorators (@series, @walk, @chain) let an entity's rows
// form an ordered sequence: monotonic timestamps, cumulative numeric walks
// and Markov state progressions. Stateful fields are skipped during per-row
// generation (null placeholder, like @derived) and filled by a per-entity
// post-pass that runs after the row loop and before dataset-rule checks.
//
// Determinism rules (see docs/superpowers/specs/2026-06-12-time-series-design.md):
//
//   - entities without stateful fields derive no substreams and make no
//     draws, so schemas that don't use the feature are byte-identical;
//   - draw counts depend only on (row index, static config) — never on
//     runtime state. Absorbing @chain states still consume their one uniform
//     draw per row; @series with jitter=0 makes no draw for any row; @walk
//     row 0 is exactly start with no draw.
//
// Interactions: row-level @strict rules and @derived/@compute expressions
// evaluate before the stateful pass and therefore see null for stateful
// fields (v1 limitation); @warn dataset rules run after and see final
// values. When combined with other decorators the stateful decorator wins:
// @range is honoured as min/max clamping for @walk and ignored for
// @series/@chain; @unique, @null_rate and @dist are ignored.

// seriesConfig is the parsed form of @series(start=..., interval=..., jitter=...).
type seriesConfig struct {
	Start    time.Time
	Interval time.Duration
	Jitter   time.Duration
}

// walkConfig is the parsed form of @walk(start=..., drift=..., volatility=..., min=..., max=...).
type walkConfig struct {
	Start      float64
	Drift      float64
	Volatility float64
	Min        *float64
	Max        *float64
}

// chainEdge is one outgoing transition of a @chain state, with its
// probability normalized so each state's outgoing probabilities sum to 1.
type chainEdge struct {
	To   string
	Prob float64
}

// chainConfig is the parsed form of @chain("from>to:prob, ...", start=...).
type chainConfig struct {
	// Start is the explicit start state; empty means "first declared enum
	// variant" (resolved at generation/validation time).
	Start string
	// Transitions maps each source state to its normalized outgoing edges.
	// States absent from the map are absorbing.
	Transitions map[string][]chainEdge
	// States lists every state mentioned in the table (and start, when set)
	// in first-mention order, for validation against the enum's variants.
	States []string
}

// statefulDecorators are the decorator names handled by the stateful pass.
var statefulDecorators = []string{"series", "walk", "chain"}

// isStateful reports whether f carries a stateful sequence decorator.
func isStateful(f *model.Field) bool {
	for _, name := range statefulDecorators {
		if model.HasDecorator(f.Decorators, name) {
			return true
		}
	}
	return false
}

// parseSeriesConfig parses a @series decorator into a typed config.
// start (RFC3339 or YYYY-MM-DD) and interval (Go duration or Nd for days)
// are required; jitter defaults to 0 and must not be negative.
func parseSeriesConfig(d *model.Decorator) (seriesConfig, error) {
	var cfg seriesConfig
	raw, ok := d.ArgByKey("start")
	if !ok {
		return cfg, fmt.Errorf("@series: missing required start=")
	}
	start, err := parseStatefulTime(raw)
	if err != nil {
		return cfg, fmt.Errorf("@series: %w", err)
	}
	cfg.Start = start

	raw, ok = d.ArgByKey("interval")
	if !ok {
		return cfg, fmt.Errorf("@series: missing required interval=")
	}
	cfg.Interval, err = parseStatefulDuration(raw)
	if err != nil {
		return cfg, fmt.Errorf("@series: interval: %w", err)
	}

	if raw, ok = d.ArgByKey("jitter"); ok {
		cfg.Jitter, err = parseStatefulDuration(raw)
		if err != nil {
			return cfg, fmt.Errorf("@series: jitter: %w", err)
		}
		if cfg.Jitter < 0 {
			return cfg, fmt.Errorf("@series: jitter must not be negative: %s", raw)
		}
	}
	return cfg, nil
}

// parseWalkConfig parses a @walk decorator into a typed config. start is
// required; drift defaults to 0, volatility to 1; min/max are optional clamps.
func parseWalkConfig(d *model.Decorator) (walkConfig, error) {
	cfg := walkConfig{Volatility: 1}
	raw, ok := d.ArgByKey("start")
	if !ok {
		return cfg, fmt.Errorf("@walk: missing required start=")
	}
	start, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return cfg, fmt.Errorf("@walk: start: not a number: %s", raw)
	}
	cfg.Start = start

	if raw, ok = d.ArgByKey("drift"); ok {
		if cfg.Drift, err = strconv.ParseFloat(raw, 64); err != nil {
			return cfg, fmt.Errorf("@walk: drift: not a number: %s", raw)
		}
	}
	if raw, ok = d.ArgByKey("volatility"); ok {
		if cfg.Volatility, err = strconv.ParseFloat(raw, 64); err != nil {
			return cfg, fmt.Errorf("@walk: volatility: not a number: %s", raw)
		}
	}
	if raw, ok = d.ArgByKey("min"); ok {
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return cfg, fmt.Errorf("@walk: min: not a number: %s", raw)
		}
		cfg.Min = &f
	}
	if raw, ok = d.ArgByKey("max"); ok {
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return cfg, fmt.Errorf("@walk: max: not a number: %s", raw)
		}
		cfg.Max = &f
	}
	if cfg.Min != nil && cfg.Max != nil && *cfg.Min > *cfg.Max {
		return cfg, fmt.Errorf("@walk: min %v exceeds max %v", *cfg.Min, *cfg.Max)
	}
	return cfg, nil
}

// parseChainConfig parses a @chain decorator: a quoted transition table
// "from>to:prob, ..." as the first argument plus an optional start= state.
// Probabilities must be > 0 and are normalized per source state.
func parseChainConfig(d *model.Decorator) (chainConfig, error) {
	cfg := chainConfig{Transitions: map[string][]chainEdge{}}
	if len(d.Args) == 0 || d.Args[0].Kind != model.ArgLiteral {
		return cfg, fmt.Errorf("@chain: first argument must be a quoted transition table")
	}
	table, ok := d.Args[0].Literal.(string)
	if !ok {
		return cfg, fmt.Errorf("@chain: first argument must be a quoted transition table")
	}

	seen := map[string]struct{}{}
	mention := func(s string) {
		if _, dup := seen[s]; !dup {
			seen[s] = struct{}{}
			cfg.States = append(cfg.States, s)
		}
	}

	for _, entry := range strings.Split(table, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		arrow := strings.Index(entry, ">")
		colon := strings.LastIndex(entry, ":")
		if arrow <= 0 || colon <= arrow+1 || colon == len(entry)-1 {
			return cfg, fmt.Errorf("@chain: malformed transition %q (want from>to:prob)", entry)
		}
		from := strings.TrimSpace(entry[:arrow])
		to := strings.TrimSpace(entry[arrow+1 : colon])
		probRaw := strings.TrimSpace(entry[colon+1:])
		if from == "" || to == "" {
			return cfg, fmt.Errorf("@chain: malformed transition %q (want from>to:prob)", entry)
		}
		prob, err := strconv.ParseFloat(probRaw, 64)
		if err != nil {
			return cfg, fmt.Errorf("@chain: transition %q: probability not a number: %s", entry, probRaw)
		}
		if prob <= 0 || math.IsInf(prob, 0) || math.IsNaN(prob) {
			return cfg, fmt.Errorf("@chain: transition %q: probability must be a finite number > 0", entry)
		}
		mention(from)
		mention(to)
		cfg.Transitions[from] = append(cfg.Transitions[from], chainEdge{To: to, Prob: prob})
	}
	if len(cfg.Transitions) == 0 {
		return cfg, fmt.Errorf("@chain: transition table is empty")
	}

	// Normalize each state's outgoing probabilities to sum to 1.
	for from, edges := range cfg.Transitions {
		var sum float64
		for _, e := range edges {
			sum += e.Prob
		}
		for i := range edges {
			edges[i].Prob /= sum
		}
		cfg.Transitions[from] = edges
	}

	if raw, ok := d.ArgByKey("start"); ok {
		cfg.Start = stripArgQuotes(raw)
		mention(cfg.Start)
	}
	return cfg, nil
}

// parseStatefulTime parses a @series start value: RFC3339 or YYYY-MM-DD.
func parseStatefulTime(raw string) (time.Time, error) {
	raw = stripArgQuotes(raw)
	if t, ok := parseYMD(raw); ok {
		return t, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("not an RFC3339 or YYYY-MM-DD timestamp: %s", raw)
	}
	return t, nil
}

// parseStatefulDuration parses a Go duration ("90s", "1h30m") or an Nd
// day count ("7d" → 168h).
func parseStatefulDuration(raw string) (time.Duration, error) {
	raw = stripArgQuotes(strings.TrimSpace(raw))
	if strings.HasSuffix(raw, "d") {
		if n, err := strconv.Atoi(raw[:len(raw)-1]); err == nil {
			return time.Duration(n) * 24 * time.Hour, nil
		}
	}
	dur, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("not a duration: %s", raw)
	}
	return dur, nil
}

// stripArgQuotes removes matching surrounding quotes from a KV arg value.
func stripArgQuotes(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

// ValidateStateful checks every @series/@walk/@chain decorator in the
// document: configs must parse, @series fields must be date/datetime,
// @walk fields int/float/decimal, @chain fields enums whose variants cover
// every state mentioned (and the explicit start). Stateful decorators on
// coherence-group members, reference fields, compound types or reusable
// type fields are rejected. Violations are returned as *errors.Error with
// Kind == KindValidation and Entity/Field populated.
func ValidateStateful(doc *model.Document) error {
	enums := map[string]model.EnumDef{}
	doc.Enums.Each(func(name string, def model.EnumDef) bool {
		enums[name] = def
		return true
	})

	var firstErr error
	doc.Entities.Each(func(ename string, ent *model.Entity) bool {
		coherent := map[string]struct{}{}
		if ent.Coherence != nil {
			ent.Coherence.Each(func(_ string, members []string) bool {
				for _, m := range members {
					coherent[m] = struct{}{}
				}
				return true
			})
		}
		ent.Fields.Each(func(fname string, f *model.Field) bool {
			if err := checkStatefulField(ename, fname, f, coherent, enums); err != nil {
				firstErr = err
				return false
			}
			return true
		})
		return firstErr == nil
	})
	if firstErr != nil {
		return firstErr
	}

	doc.Types.Each(func(tname string, ent *model.Entity) bool {
		ent.Fields.Each(func(fname string, f *model.Field) bool {
			if isStateful(f) {
				firstErr = statefulErr(tname, fname, "stateful decorators are not supported on reusable type fields")
				return false
			}
			return true
		})
		return firstErr == nil
	})
	return firstErr
}

// checkStatefulField validates the stateful decorator (if any) on one field.
func checkStatefulField(ename, fname string, f *model.Field, coherent map[string]struct{}, enums map[string]model.EnumDef) error {
	var present []string
	for _, name := range statefulDecorators {
		if model.HasDecorator(f.Decorators, name) {
			present = append(present, "@"+name)
		}
	}
	if len(present) == 0 {
		return nil
	}
	if len(present) > 1 {
		return statefulErr(ename, fname, "field declares multiple stateful decorators: "+strings.Join(present, ", "))
	}
	if _, ok := coherent[fname]; ok {
		return statefulErr(ename, fname, present[0]+" is not allowed on a coherence-group member")
	}
	switch f.Type.(type) {
	case model.Reference:
		return statefulErr(ename, fname, present[0]+" is not allowed on a reference field")
	case model.List, model.Map, model.Tuple, model.Nullable, model.Union:
		return statefulErr(ename, fname, present[0]+" is not allowed on a compound type")
	}

	switch present[0] {
	case "@series":
		prim, ok := f.Type.(model.Primitive)
		if !ok || (prim.Kind != model.PrimDate && prim.Kind != model.PrimDatetime) {
			return statefulErr(ename, fname, "@series requires a date or datetime field")
		}
		if _, err := parseSeriesConfig(model.FindDecorator(f.Decorators, "series")); err != nil {
			return statefulErr(ename, fname, err.Error())
		}
	case "@walk":
		prim, ok := f.Type.(model.Primitive)
		if !ok || (prim.Kind != model.PrimInt && prim.Kind != model.PrimFloat && prim.Kind != model.PrimDecimal) {
			return statefulErr(ename, fname, "@walk requires an int, float or decimal field")
		}
		if _, err := parseWalkConfig(model.FindDecorator(f.Decorators, "walk")); err != nil {
			return statefulErr(ename, fname, err.Error())
		}
	case "@chain":
		variants, ok := chainEnumValues(f, enums)
		if !ok {
			return statefulErr(ename, fname, "@chain requires an enum field")
		}
		cfg, err := parseChainConfig(model.FindDecorator(f.Decorators, "chain"))
		if err != nil {
			return statefulErr(ename, fname, err.Error())
		}
		variantSet := make(map[string]struct{}, len(variants))
		for _, v := range variants {
			variantSet[v] = struct{}{}
		}
		for _, s := range cfg.States {
			if _, ok := variantSet[s]; !ok {
				return statefulErr(ename, fname, "@chain: state "+s+" is not a variant of the field's enum")
			}
		}
	}
	return nil
}

// chainEnumValues resolves the enum variants behind a @chain field's type:
// inline enums directly, named types via the document's enums section.
func chainEnumValues(f *model.Field, enums map[string]model.EnumDef) ([]string, bool) {
	switch t := f.Type.(type) {
	case model.EnumInline:
		return t.Values, true
	case model.NamedType:
		if def, ok := enums[t.Name]; ok {
			return def.Values(), true
		}
	}
	return nil, false
}

// statefulErr builds the canonical validation error for stateful checks.
func statefulErr(entity, field, msg string) error {
	return &errors.Error{
		Kind:    errors.KindValidation,
		Entity:  entity,
		Field:   field,
		Message: msg,
	}
}

// applyStatefulSequences is the per-entity post-pass that fills @series,
// @walk and @chain fields across rows in index order. It runs after the
// entity's row loop and before dataset-rule enforcement. Each stateful field
// draws from its own substream of the entity stream ("series:"/"walk:"/
// "chain:" + field name), so rule-retry loops and attempt substreams cannot
// disturb the sequence — and entities without stateful fields derive no
// substreams and make no draws at all.
func (e *Engine) applyStatefulSequences(entity *model.Entity, rows []*value.Object, entSub ports.Randomizer, st *generationState) error {
	var firstErr error
	entity.Fields.Each(func(fname string, f *model.Field) bool {
		if !isStateful(f) || model.HasDecorator(f.Decorators, "internal") {
			return true
		}
		var err error
		switch {
		case model.HasDecorator(f.Decorators, "series"):
			err = applySeries(entity, fname, f, rows, entSub.Substream("series:"+fname))
		case model.HasDecorator(f.Decorators, "walk"):
			err = applyWalk(entity, fname, f, rows, entSub.Substream("walk:"+fname))
		case model.HasDecorator(f.Decorators, "chain"):
			err = applyChain(entity, fname, f, rows, entSub.Substream("chain:"+fname), st)
		}
		if err != nil {
			firstErr = err
			return false
		}
		return true
	})
	return firstErr
}

// applySeries fills a @series field: row i is start + i·interval + u·jitter
// with u uniform in [-1, 1). With jitter=0 no draw is made for any row.
// Values stay monotonically non-decreasing as long as jitter < interval/2
// (documented, not enforced). Date fields are truncated to midnight UTC.
func applySeries(entity *model.Entity, fname string, f *model.Field, rows []*value.Object, sub ports.Randomizer) error {
	cfg, err := parseSeriesConfig(model.FindDecorator(f.Decorators, "series"))
	if err != nil {
		return statefulErr(entity.Name, fname, err.Error())
	}
	dateOnly := false
	if prim, ok := f.Type.(model.Primitive); ok && prim.Kind == model.PrimDate {
		dateOnly = true
	}
	for i, row := range rows {
		t := cfg.Start.Add(time.Duration(i) * cfg.Interval)
		if cfg.Jitter > 0 {
			u := sub.Float()*2 - 1
			t = t.Add(time.Duration(u * float64(cfg.Jitter)))
		}
		if dateOnly {
			t = t.Truncate(24 * time.Hour)
		}
		row.Set(fname, value.Time(t))
	}
	return nil
}

// applyWalk fills a @walk field: row 0 is exactly start (no draw); every
// subsequent row draws exactly one standard normal and steps
// xᵢ = clamp(xᵢ₋₁ + drift + volatility·n). Clamping uses the decorator's
// min/max, falling back to the field's @range bounds. Int fields round half
// away from zero; float/decimal round to 2 places (emitted values only — the
// walk state itself stays unrounded).
func applyWalk(entity *model.Entity, fname string, f *model.Field, rows []*value.Object, sub ports.Randomizer) error {
	cfg, err := parseWalkConfig(model.FindDecorator(f.Decorators, "walk"))
	if err != nil {
		return statefulErr(entity.Name, fname, err.Error())
	}
	lo, hi := cfg.Min, cfg.Max
	if lo == nil && hi == nil {
		if rlo, rhi, ok := extractRange(f.Decorators); ok {
			lo, hi = &rlo, &rhi
		}
	}
	prim, _ := f.Type.(model.Primitive)
	x := cfg.Start
	for i, row := range rows {
		if i > 0 {
			n := sub.NormFloat()
			x += cfg.Drift + cfg.Volatility*n
			if lo != nil && x < *lo {
				x = *lo
			}
			if hi != nil && x > *hi {
				x = *hi
			}
		}
		row.Set(fname, walkValue(prim.Kind, x))
	}
	return nil
}

// walkValue renders the walk state for the field's primitive kind.
func walkValue(kind model.PrimKind, x float64) value.Value {
	switch kind {
	case model.PrimInt:
		return value.Int(int64(math.Round(x))) // math.Round is half-away-from-zero
	case model.PrimDecimal:
		return value.Dec(decimal.NewFromFloat(x).Round(2))
	default:
		return value.Float(roundTo(x, 2))
	}
}

// applyChain fills a @chain field: row 0 is the start state (no draw;
// default is the enum's first declared variant); each subsequent row
// consumes exactly one uniform draw — even when the current state is
// absorbing — and transitions along the state's normalized outgoing edges.
func applyChain(entity *model.Entity, fname string, f *model.Field, rows []*value.Object, sub ports.Randomizer, st *generationState) error {
	cfg, err := parseChainConfig(model.FindDecorator(f.Decorators, "chain"))
	if err != nil {
		return statefulErr(entity.Name, fname, err.Error())
	}
	state := cfg.Start
	if state == "" {
		variants, ok := chainEnumValues(f, st.enumDefs)
		if !ok || len(variants) == 0 {
			return statefulErr(entity.Name, fname, "@chain requires an enum field")
		}
		state = variants[0]
	}
	for i, row := range rows {
		if i > 0 {
			// One uniform draw per row regardless of state, so draw counts
			// stay a function of row index alone (never of runtime state).
			u := sub.Float()
			if edges := cfg.Transitions[state]; len(edges) > 0 {
				acc := 0.0
				next := edges[len(edges)-1].To
				for _, e := range edges {
					acc += e.Prob
					if u < acc {
						next = e.To
						break
					}
				}
				state = next
			}
		}
		row.Set(fname, value.Str(state))
	}
	return nil
}
