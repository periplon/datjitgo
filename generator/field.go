package generator

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jmcarbo/datjitgo/core/errors"
	"github.com/jmcarbo/datjitgo/core/model"
	"github.com/jmcarbo/datjitgo/core/ports"
	"github.com/jmcarbo/datjitgo/core/value"
)

// generateRow drives one row's worth of field generation through the full
// pipeline: coherence groups → per-field generation (with uniqueness retry)
// → @derived → @default_chain → @compute → @timestamps → @internal strip.
//
// It is split out from Engine.Generate so unit tests can exercise a single
// row without spinning up the whole entity loop.
func (e *Engine) generateRow(entity *model.Entity, st *generationState, rowRNG ports.Randomizer) (*value.Object, error) {
	row := value.NewObject()

	// 1. Coherence groups — pick an anchor per group and derive members.
	coherenceKeys, err := e.applyCoherence(entity, row, rowRNG)
	if err != nil {
		return nil, err
	}

	// 2. Field-by-field generation for everything not already set by
	// coherence and not marked derived/compute/default_chain.
	entity.Fields.Each(func(fname string, f *model.Field) bool {
		if _, ok := coherenceKeys[fname]; ok {
			// Coherence already produced a value; no further generation needed.
			return true
		}
		if isDerived(f) || isCompute(f) || isDefaultChain(f) {
			row.Set(fname, value.Null()) // placeholder until phase 3.
			return true
		}

		val, gerr := e.generateField(entity, f, row, st, rowRNG)
		if gerr != nil {
			err = gerr
			return false
		}

		// @unique retry loop (skipped if value is already null — null rows
		// don't collide with other nulls by design).
		if hasDecorator(f.Decorators, "unique") && !val.IsNull() {
			uk := st.uniqueKey(entity.Name, fname)
			attempts := 0
			for {
				key := valueKey(val)
				if _, hit := uk[key]; !hit {
					uk[key] = struct{}{}
					break
				}
				attempts++
				if attempts > 100 {
					err = &errors.Error{
						Kind:    errors.KindUniquenessExhausted,
						Entity:  entity.Name,
						Field:   fname,
						Message: fmt.Sprintf("uniqueness exhausted after %d attempts", attempts),
					}
					return false
				}
				v2, gerr := e.generateField(entity, f, row, st, rowRNG)
				if gerr != nil {
					err = gerr
					return false
				}
				val = v2
			}
		}

		row.Set(fname, val)
		return true
	})
	if err != nil {
		return nil, err
	}

	// 3. @from sibling derivations for fields outside coherence groups.
	if err := e.applyFromDerivations(entity, row, rowRNG, coherenceKeys); err != nil {
		return nil, err
	}

	// 4. Derived / default_chain / compute evaluated against the row.
	if err := e.applyDerived(entity, row, st); err != nil {
		return nil, err
	}
	if err := e.applyDefaultChain(entity, row, st); err != nil {
		return nil, err
	}
	if err := e.applyCompute(entity, row, st); err != nil {
		return nil, err
	}

	// 5. @timestamps entity decorator.
	if model.HasDecorator(entity.Meta, "timestamps") {
		if !row.Has("created_at") {
			row.Set("created_at", value.Time(now()))
		}
		if !row.Has("updated_at") {
			row.Set("updated_at", value.Time(now()))
		}
	}

	// 6. Strip @internal fields before returning.
	entity.Fields.Each(func(fname string, f *model.Field) bool {
		if model.HasDecorator(f.Decorators, "internal") {
			row.Delete(fname)
		}
		return true
	})

	return row, nil
}

// generateField produces a single value for f. It applies @null_rate, then
// dispatches by type, then layers shaping decorators (range, pattern, len,
// multiple_of, values). @unique is handled upstream by the row pipeline.
func (e *Engine) generateField(entity *model.Entity, f *model.Field, row *value.Object, st *generationState, rng ports.Randomizer) (value.Value, error) {
	// 1. @null_rate.
	if d := model.FindDecorator(f.Decorators, "null_rate"); d != nil {
		rate := firstFloatArg(d.Args, 0)
		if rng.Float() < rate {
			return value.Null(), nil
		}
	}

	// 2. @pattern(...) short-circuits: the template yields the value.
	if d := model.FindDecorator(f.Decorators, "pattern"); d != nil {
		if len(d.Args) > 0 {
			tmpl := decoratorLiteralString(d.Args[0])
			key := entity.Name + "." + f.Name
			return value.Str(expandPattern(tmpl, rng, st.seqs, key)), nil
		}
	}

	// 3a. @llm(...) — use a live provider when configured; otherwise keep
	// deterministic corpus-backed stub content.
	if d := model.FindDecorator(f.Decorators, "llm"); d != nil {
		text, err := e.generateLLMValue(*d, st, rng)
		if err != nil {
			return value.Null(), err
		}
		return value.Str(text), nil
	}
	if d := findLLM(entity.Meta); d != nil && shouldInheritEntityLLM(f) {
		text, err := e.generateLLMValue(*d, st, rng)
		if err != nil {
			return value.Null(), err
		}
		return value.Str(text), nil
	}

	// 3b. @llm_values(N, "prompt") — materialise N candidate strings, then
	// sample uniformly. Keeps behaviour aligned with @values so @null_rate
	// / @unique / @dist compose naturally.
	if d := model.FindDecorator(f.Decorators, "llm_values"); d != nil {
		pool, err := e.generateLLMValues(*d, st, rng)
		if err != nil {
			return value.Null(), err
		}
		if len(pool) > 0 {
			idx := int(rng.IntN(int64(len(pool))))
			return value.Str(pool[idx]), nil
		}
	}

	// 3. @values(...) — pick from the literal list.
	if d := model.FindDecorator(f.Decorators, "values"); d != nil && len(d.Args) > 0 {
		idx := int(rng.IntN(int64(len(d.Args))))
		return literalAsValue(d.Args[idx]), nil
	}

	// 4. Base generation by TypeExpr.
	val, err := e.generateByType(entity, f, f.Type, row, st, rng)
	if err != nil {
		return value.Null(), err
	}

	// 5. Post-generation shaping.
	val = applyRange(val, f.Decorators)
	val = applyMultipleOf(val, f.Decorators)
	val = applyLen(val, f.Decorators, rng)
	return val, nil
}

// generateByType is the TypeExpr dispatch. Composite types recurse.
func (e *Engine) generateByType(entity *model.Entity, f *model.Field, t model.TypeExpr, row *value.Object, st *generationState, rng ports.Randomizer) (value.Value, error) {
	switch tt := t.(type) {
	case model.Primitive:
		return e.generatePrimitiveField(f, tt, rng), nil
	case model.Semantic:
		return e.generateSemantic(tt, rng)
	case model.EnumInline:
		return e.generateEnum(f, tt.Values, rng), nil
	case model.NamedType:
		if def, ok := st.enumDefs[tt.Name]; ok {
			weights := def.WeightsOrNil()
			if weights == nil {
				return e.generateEnum(f, def.Values(), rng), nil
			}
			idx := sampleEnumIndex(rng, weights)
			return value.Str(def.Variants[idx].Value), nil
		}
		if def, ok := st.typeDefs[tt.Name]; ok {
			return e.generateNamedType(def, row, st, rng)
		}
		// Unknown named types should have been rejected by validation. Keep
		// the old placeholder fallback for direct generator use.
		return value.Str(tt.Name), nil
	case model.Reference:
		return e.generateReference(entity, f, tt, st, rng), nil
	case model.List:
		return e.generateList(entity, f, tt, row, st, rng)
	case model.Map:
		return e.generateMap(entity, f, tt, row, st, rng)
	case model.Tuple:
		out := make([]value.Value, 0, len(tt.Elements))
		for _, el := range tt.Elements {
			v, err := e.generateByType(entity, f, el, row, st, rng)
			if err != nil {
				return value.Null(), err
			}
			out = append(out, v)
		}
		return value.List(out), nil
	case model.Nullable:
		if rng.Float() < 0.2 {
			return value.Null(), nil
		}
		return e.generateByType(entity, f, tt.Inner, row, st, rng)
	case model.Union:
		if len(tt.Variants) == 0 {
			return value.Null(), nil
		}
		idx := int(rng.IntN(int64(len(tt.Variants))))
		return e.generateByType(entity, f, tt.Variants[idx], row, st, rng)
	}
	return value.Null(), &errors.Error{Kind: errors.KindGeneration, Message: fmt.Sprintf("unsupported type %T", t)}
}

// generatePrimitiveField layers @dist on top of the default primitive
// generator for numeric kinds.
func (e *Engine) generatePrimitiveField(f *model.Field, p model.Primitive, rng ports.Randomizer) value.Value {
	if d := model.FindDecorator(f.Decorators, "dist"); d != nil {
		spec := parseDistDecorator(d)
		lo, hi, have := extractRange(f.Decorators)
		switch p.Kind {
		case model.PrimInt:
			sample := sampleFloat(rng, spec, lo, hi, have)
			return value.Int(int64(sample + 0.5))
		case model.PrimFloat:
			sample := sampleFloat(rng, spec, lo, hi, have)
			return value.Float(roundTo(sample, 2))
		case model.PrimDecimal:
			sample := sampleFloat(rng, spec, lo, hi, have)
			return value.Float(roundTo(sample, 2))
		}
	}
	return generatePrimitive(p, rng)
}

// generateEnum picks a value, optionally biased by a @dist decorator.
func (e *Engine) generateEnum(f *model.Field, values []string, rng ports.Randomizer) value.Value {
	if len(values) == 0 {
		return value.Null()
	}
	if d := model.FindDecorator(f.Decorators, "dist"); d != nil {
		spec := parseDistDecorator(d)
		if spec.Kind == distCategorical && len(spec.Probs) == len(values) {
			idx := sampleEnumIndex(rng, spec.Probs)
			return value.Str(values[idx])
		}
	}
	idx := int(rng.IntN(int64(len(values))))
	return value.Str(values[idx])
}

// generateList handles `[T]` fields honouring @count.
func (e *Engine) generateList(entity *model.Entity, f *model.Field, t model.List, row *value.Object, st *generationState, rng ports.Randomizer) (value.Value, error) {
	lo, hi := countRange(f.Decorators, 0, 4)
	span := hi - lo
	n := lo
	if span > 0 {
		n += int(rng.IntN(int64(span + 1)))
	}
	out := make([]value.Value, 0, n)
	for i := 0; i < n; i++ {
		v, err := e.generateByType(entity, f, t.Element, row, st, rng)
		if err != nil {
			return value.Null(), err
		}
		out = append(out, v)
	}
	return value.List(out), nil
}

func (e *Engine) generateMap(entity *model.Entity, f *model.Field, t model.Map, row *value.Object, st *generationState, rng ports.Randomizer) (value.Value, error) {
	n := 1 + int(rng.IntN(4))
	obj := value.NewObject()
	for i := 0; i < n; i++ {
		kv, err := e.generateByType(entity, f, t.Key, row, st, rng)
		if err != nil {
			return value.Null(), err
		}
		vv, err := e.generateByType(entity, f, t.Value, row, st, rng)
		if err != nil {
			return value.Null(), err
		}
		key := valueDisplay(kv)
		obj.Set(key, vv)
	}
	return value.Obj(obj), nil
}

func (e *Engine) generateNamedType(def *model.Entity, row *value.Object, st *generationState, rng ports.Randomizer) (value.Value, error) {
	obj := value.NewObject()
	var firstErr error
	def.Fields.Each(func(fname string, f *model.Field) bool {
		val, err := e.generateField(def, f, row, st, rng.Substream("type:"+def.Name+"."+fname))
		if err != nil {
			firstErr = err
			return false
		}
		obj.Set(fname, val)
		return true
	})
	if firstErr != nil {
		return value.Null(), firstErr
	}
	return value.Obj(obj), nil
}

// applyRange clamps numeric values to the @range declared on the field.
func applyRange(val value.Value, decs []model.Decorator) value.Value {
	d := model.FindDecorator(decs, "range")
	if d == nil || len(d.Args) == 0 {
		return val
	}
	a := d.Args[0]
	if a.Kind != model.ArgRange {
		return val
	}
	lo, errLo := strconv.ParseFloat(a.From, 64)
	hi, errHi := strconv.ParseFloat(a.To, 64)
	if errLo != nil || errHi != nil {
		if val.Kind == value.KindTime {
			return applyTimeRange(val, a)
		}
		return val
	}
	// Exclusive bounds → shrink by ε.
	span := hi - lo
	eps := 1e-10
	if span > 0 {
		eps = span * 1e-9
	}
	if a.LoExcl {
		lo += eps
	}
	if a.HiExcl {
		hi -= eps
	}
	switch val.Kind {
	case value.KindInt:
		iLo := int64(lo)
		iHi := int64(hi)
		if val.I < iLo {
			val.I = iLo
		}
		if val.I > iHi {
			val.I = iHi
		}
	case value.KindFloat:
		if val.F < lo {
			val.F = lo
		}
		if val.F > hi {
			val.F = hi
		}
	case value.KindTime:
		return applyTimeRange(val, a)
	}
	return val
}

func applyTimeRange(val value.Value, a model.DecoratorArg) value.Value {
	lo, hi, ok := parseTimeRange(a.From, a.To, val.T)
	if !ok {
		return val
	}
	if a.LoExcl {
		lo = lo.Add(time.Nanosecond)
	}
	if a.HiExcl {
		hi = hi.Add(-time.Nanosecond)
	}
	if val.T.Before(lo) {
		val.T = lo
	}
	if val.T.After(hi) {
		val.T = hi
	}
	return val
}

func parseTimeRange(from, to string, sample time.Time) (time.Time, time.Time, bool) {
	lo, _, okLo := parseRangeTimeBound(from, sample)
	hi, hiDateOnly, okHi := parseRangeTimeBound(to, sample)
	if !okLo || !okHi {
		return time.Time{}, time.Time{}, false
	}
	if hiDateOnly {
		hi = hi.Add(24*time.Hour - time.Nanosecond)
	}
	if hi.Before(lo) {
		return time.Time{}, time.Time{}, false
	}
	return lo, hi, true
}

func parseRangeTimeBound(raw string, sample time.Time) (time.Time, bool, bool) {
	raw = strings.TrimSpace(raw)
	layouts := []struct {
		layout   string
		dateOnly bool
	}{
		{"2006-01-02", true},
		{time.RFC3339Nano, false},
		{"2006-01-02T15:04:05", false},
		{"15:04:05", false},
	}
	for _, item := range layouts {
		t, err := time.Parse(item.layout, raw)
		if err != nil {
			continue
		}
		if item.layout == "15:04:05" {
			t = time.Date(sample.Year(), sample.Month(), sample.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
		}
		return t, item.dateOnly, true
	}
	return time.Time{}, false, false
}

// extractRange returns the field's @range as floats with a presence flag.
func extractRange(decs []model.Decorator) (float64, float64, bool) {
	d := model.FindDecorator(decs, "range")
	if d == nil || len(d.Args) == 0 || d.Args[0].Kind != model.ArgRange {
		return 0, 0, false
	}
	a := d.Args[0]
	lo, errLo := strconv.ParseFloat(a.From, 64)
	hi, errHi := strconv.ParseFloat(a.To, 64)
	if errLo != nil || errHi != nil {
		return 0, 0, false
	}
	return lo, hi, true
}

func applyMultipleOf(val value.Value, decs []model.Decorator) value.Value {
	d := model.FindDecorator(decs, "multiple_of")
	if d == nil || len(d.Args) == 0 {
		return val
	}
	f, ok := argAsFloat(d.Args[0].Literal)
	if !ok || f == 0 {
		return val
	}
	switch val.Kind {
	case value.KindInt:
		val.I = int64(f) * (val.I / int64(f))
	case value.KindFloat:
		val.F = f * float64(int64(val.F/f))
	}
	return val
}

// applyLen reshapes string/list values to satisfy a @len decorator. For
// strings it pads/truncates alphanumerically; for lists it trims.
func applyLen(val value.Value, decs []model.Decorator, rng ports.Randomizer) value.Value {
	d := model.FindDecorator(decs, "len")
	if d == nil || len(d.Args) == 0 {
		return val
	}
	lo, hi := lenBounds(d.Args[0])
	if hi < lo {
		hi = lo
	}
	target := lo + int(rng.IntN(int64(hi-lo+1)))
	switch val.Kind {
	case value.KindString:
		if len(val.S) == target {
			return val
		}
		if len(val.S) > target {
			val.S = val.S[:target]
			return val
		}
		// pad
		pad := make([]byte, target-len(val.S))
		for i := range pad {
			pad[i] = alphanum[rng.IntN(int64(len(alphanum)))]
		}
		val.S = val.S + string(pad)
	case value.KindList:
		if len(val.L) > target {
			val.L = val.L[:target]
		}
	}
	return val
}

// countRange returns the lo, hi values from a @count(...) decorator, with
// defaults applied when the decorator is absent.
func countRange(decs []model.Decorator, defLo, defHi int) (int, int) {
	d := model.FindDecorator(decs, "count")
	if d == nil || len(d.Args) == 0 {
		return defLo, defHi
	}
	a := d.Args[0]
	if a.Kind == model.ArgRange {
		lo, errLo := strconv.Atoi(a.From)
		hi, errHi := strconv.Atoi(a.To)
		if errLo == nil && errHi == nil {
			return lo, hi
		}
	}
	if a.Kind == model.ArgLiteral {
		if n, ok := argAsFloat(a.Literal); ok {
			return int(n), int(n)
		}
	}
	return defLo, defHi
}

func lenBounds(a model.DecoratorArg) (int, int) {
	if a.Kind == model.ArgRange {
		lo, errLo := strconv.Atoi(a.From)
		hi, errHi := strconv.Atoi(a.To)
		if errLo == nil && errHi == nil {
			return lo, hi
		}
	}
	if a.Kind == model.ArgLiteral {
		if n, ok := argAsFloat(a.Literal); ok {
			return int(n), int(n)
		}
	}
	return 0, 0
}

func firstFloatArg(args []model.DecoratorArg, fallback float64) float64 {
	for _, a := range args {
		if a.Kind == model.ArgLiteral {
			if f, ok := argAsFloat(a.Literal); ok {
				return f
			}
		}
	}
	return fallback
}

func decoratorLiteralString(a model.DecoratorArg) string {
	if a.Kind == model.ArgLiteral {
		switch v := a.Literal.(type) {
		case string:
			return v
		case int64:
			return strconv.FormatInt(v, 10)
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64)
		case bool:
			return strconv.FormatBool(v)
		}
	}
	return a.Raw
}

func literalAsValue(a model.DecoratorArg) value.Value {
	if a.Kind == model.ArgLiteral {
		switch v := a.Literal.(type) {
		case string:
			return value.Str(v)
		case int64:
			return value.Int(v)
		case float64:
			return value.Float(v)
		case bool:
			return value.Bool(v)
		}
	}
	if a.Kind == model.ArgIdent {
		return value.Str(a.Ident)
	}
	return value.Str(a.Raw)
}

func isDerived(f *model.Field) bool { return model.HasDecorator(f.Decorators, "derived") }
func isCompute(f *model.Field) bool {
	return len(f.Compute) > 0 || model.HasDecorator(f.Decorators, "compute")
}
func isDefaultChain(f *model.Field) bool {
	return f.DefaultChain != nil || model.HasDecorator(f.Decorators, "default_chain")
}

func hasDecorator(decs []model.Decorator, name string) bool { return model.HasDecorator(decs, name) }

// valueKey returns a stable canonical string for a value, suitable for
// uniqueness set membership.
func valueKey(v value.Value) string {
	switch v.Kind {
	case value.KindString:
		return "S:" + v.S
	case value.KindInt:
		return "I:" + strconv.FormatInt(v.I, 10)
	case value.KindFloat:
		return "F:" + strconv.FormatFloat(v.F, 'g', -1, 64)
	case value.KindBool:
		return "B:" + strconv.FormatBool(v.B)
	case value.KindUUID:
		return "U:" + v.U.String()
	case value.KindTime:
		return "T:" + v.T.Format("2006-01-02T15:04:05Z07:00")
	case value.KindDecimal:
		return "D:" + v.D.String()
	case value.KindNull:
		return "N"
	case value.KindList:
		var b strings.Builder
		b.WriteByte('[')
		for i, item := range v.L {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(valueKey(item))
		}
		b.WriteByte(']')
		return b.String()
	case value.KindObject:
		var b strings.Builder
		b.WriteByte('{')
		v.O.Each(func(k string, val value.Value) bool {
			b.WriteString(k)
			b.WriteByte('=')
			b.WriteString(valueKey(val))
			b.WriteByte(',')
			return true
		})
		b.WriteByte('}')
		return b.String()
	}
	return ""
}

// valueDisplay returns a short human-readable form used by map keys.
func valueDisplay(v value.Value) string {
	switch v.Kind {
	case value.KindString:
		return v.S
	case value.KindInt:
		return strconv.FormatInt(v.I, 10)
	case value.KindFloat:
		return strconv.FormatFloat(v.F, 'f', -1, 64)
	case value.KindBool:
		return strconv.FormatBool(v.B)
	case value.KindUUID:
		return v.U.String()
	case value.KindTime:
		return v.T.Format("2006-01-02T15:04:05")
	case value.KindDecimal:
		return v.D.String()
	}
	return valueKey(v)
}
