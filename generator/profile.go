package generator

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/periplon/datjitgo/core/errors"
	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/ports"
	"github.com/periplon/datjitgo/core/value"
)

// Generation profiles bias eligible field values toward boundary and
// adversarial cases for negative testing. The default (realistic) profile is
// exactly today's behavior with zero additional RNG draws; edge and hostile
// are their own deterministic outputs (same schema + seed + profile →
// identical bytes). See docs/superpowers/specs/2026-06-12-generation-profiles-design.md.
const (
	// ProfileRealistic is the default profile: no boundary substitution.
	ProfileRealistic = "realistic"
	// ProfileEdge substitutes curated boundary values (empty/oversized
	// strings, numeric extremes, epoch dates, …) into eligible fields.
	ProfileEdge = "edge"
	// ProfileHostile extends the edge tables with adversarial payloads aimed
	// at downstream CSV/SQL/spreadsheet consumers.
	ProfileHostile = "hostile"
)

// resolveProfile canonicalises the GenerateOptions profile. The empty string
// means realistic (the pre-profile behavior). Unknown values are rejected
// with a validation error rather than silently ignored.
func resolveProfile(opts ports.GenerateOptions) (string, error) {
	switch opts.Profile {
	case "", ProfileRealistic:
		return ProfileRealistic, nil
	case ProfileEdge, ProfileHostile:
		return opts.Profile, nil
	}
	return "", &errors.Error{
		Kind:    errors.KindValidation,
		Message: fmt.Sprintf("unknown generation profile %q (valid: realistic|edge|hostile)", opts.Profile),
	}
}

// edgeStrings is the curated edge-profile boundary table for string-like
// values (string primitives and semantic types). Entries are versioned
// constants: changing them changes profile goldens.
var edgeStrings = []string{
	"",                       // empty
	"x",                      // single character
	strings.Repeat("a", 255), // common column-width boundary
	"héllo wörld",            // multi-byte UTF-8
	"مرحبا",                  // RTL script
	"🎉🚀",                     // emoji (astral plane)
	"e\u0301",                // combining character: e + COMBINING ACUTE ACCENT
	" leading space",
	"trailing space ",
}

// hostileStrings is edgeStrings plus adversarial payloads for CSV/SQL/
// spreadsheet consumers. NUL bytes are deliberately excluded — they break too
// many writers and stores to make a useful regression signal (see spec §3).
var hostileStrings = append(append([]string{}, edgeStrings...),
	"comma,separated,value",
	"embedded \"double\" quotes",
	"it's got 'single' quotes",
	"semi;colons; everywhere",
	"line\nbreak",
	"tab\tseparated",
	"=cmd()",                    // spreadsheet formula injection shape
	"Robert'); DROP TABLE x;--", // SQL injection shape
	strings.Repeat("A", 4096),   // 4 KiB oversized value
	"\u0440\u0430ypal",          // mixed-script homoglyph: Cyrillic r/a lookalikes + Latin "ypal"
)

// applyProfileSubstitution is the generation-profile hook applied to every
// value produced by generateField. When a non-realistic profile is active and
// the field is statically eligible (see profileEligible), it consumes exactly
// two draws from the field's generation substream — one uniform float and one
// table index — and substitutes the table entry when the float falls below
// 0.5. Both draws happen unconditionally for every eligible field so the
// stream position never depends on the generated value; with the realistic
// profile (or "") the function returns immediately with zero draws.
func applyProfileSubstitution(entity *model.Entity, f *model.Field, val value.Value, profile string, rng ports.Randomizer) value.Value {
	if profile == "" || profile == ProfileRealistic {
		return val
	}
	if !profileEligible(entity, f) {
		return val
	}
	table := profileTable(profile, f.Type, f.Decorators)
	if len(table) == 0 {
		return val
	}
	u := rng.Float()
	idx := rng.IntN(int64(len(table)))
	if u < 0.5 {
		return table[idx]
	}
	return val
}

// profileEligible reports whether f may receive boundary substitution under a
// non-realistic profile. Eligibility is static per field — it depends only on
// the declaration (type, decorators, coherence membership), never on
// generated values. Always-realistic fields per the spec: @primary, @auto and
// @unique fields; references and synthetic discriminators; coherence-group
// members; @derived/@compute/@default_chain fields; @pattern fields; and
// fields pinned with @profile(realistic).
func profileEligible(entity *model.Entity, f *model.Field) bool {
	if f == nil {
		return false
	}
	if f.Discriminator != "" || f.DiscriminatorFor != "" {
		return false
	}
	if isDerived(f) || isCompute(f) || isDefaultChain(f) {
		return false
	}
	for _, name := range []string{"primary", "auto", "unique", "pattern"} {
		if model.HasDecorator(f.Decorators, name) {
			return false
		}
	}
	if profilePinnedRealistic(f.Decorators) {
		return false
	}
	if typeHasReference(f.Type) {
		return false
	}
	if entity != nil && entity.Coherence != nil {
		member := false
		entity.Coherence.Each(func(_ string, fields []string) bool {
			for _, fn := range fields {
				if fn == f.Name {
					member = true
					return false
				}
			}
			return true
		})
		if member {
			return false
		}
	}
	return true
}

// profilePinnedRealistic reports whether the field carries the
// @profile(realistic) opt-out decorator. A bare @profile (no argument) also
// pins realistic; arguments other than "realistic" are ignored in v1.
func profilePinnedRealistic(decs []model.Decorator) bool {
	d := model.FindDecorator(decs, "profile")
	if d == nil {
		return false
	}
	if len(d.Args) == 0 {
		return true
	}
	a := d.Args[0]
	switch a.Kind {
	case model.ArgIdent:
		return a.Ident == ProfileRealistic
	case model.ArgLiteral:
		s, ok := a.Literal.(string)
		return ok && s == ProfileRealistic
	default:
		return false
	}
}

// typeHasReference reports whether t contains an entity reference anywhere in
// its structure. Reference-bearing fields are never substituted: their values
// must stay valid foreign keys.
func typeHasReference(t model.TypeExpr) bool {
	switch tt := t.(type) {
	case model.Reference:
		return true
	case model.List:
		return typeHasReference(tt.Element)
	case model.Map:
		return typeHasReference(tt.Key) || typeHasReference(tt.Value)
	case model.Tuple:
		for _, el := range tt.Elements {
			if typeHasReference(el) {
				return true
			}
		}
	case model.Nullable:
		return typeHasReference(tt.Inner)
	case model.Union:
		for _, v := range tt.Variants {
			if typeHasReference(v) {
				return true
			}
		}
	}
	return false
}

// profileTable returns the boundary table for a value of type t under the
// given profile. The table depends only on the static declaration (type plus
// decorators — @range bounds replace type extremes), never on generated
// values, so the per-field draw count stays stable for a given schema and
// profile. A nil/empty table means the value class has no boundary entries
// (bool, enums, maps, …) and the field is left untouched with zero draws.
func profileTable(profile string, t model.TypeExpr, decs []model.Decorator) []value.Value {
	switch tt := t.(type) {
	case model.Primitive:
		return primitiveProfileTable(profile, tt, decs)
	case model.Semantic:
		return stringProfileTable(profile)
	case model.List:
		elem := profileTable(profile, tt.Element, nil)
		out := make([]value.Value, 0, len(elem)+1)
		out = append(out, value.List([]value.Value{}))
		for _, v := range elem {
			out = append(out, value.List([]value.Value{v}))
		}
		return out
	case model.Nullable:
		inner := profileTable(profile, tt.Inner, decs)
		return append([]value.Value{value.Null()}, inner...)
	}
	return nil
}

// primitiveProfileTable dispatches per primitive kind. Kinds without an entry
// in the spec's boundary tables (bool, bytes, time-of-day, duration, …)
// return nil and are generated realistically.
func primitiveProfileTable(profile string, p model.Primitive, decs []model.Decorator) []value.Value {
	switch p.Kind {
	case model.PrimString:
		return stringProfileTable(profile)
	case model.PrimInt:
		return intProfileTable(decs)
	case model.PrimFloat:
		return floatProfileTable(decs, value.Float)
	case model.PrimDecimal:
		return floatProfileTable(decs, func(f float64) value.Value {
			return value.Dec(decimal.NewFromFloat(f))
		})
	case model.PrimDate, model.PrimDatetime:
		return timeProfileTable(decs)
	case model.PrimUUID:
		return []value.Value{value.UUID(uuid.Nil)} // all-zeros UUID
	default:
		return nil
	}
}

// stringProfileTable returns the string-like boundary table for profile.
func stringProfileTable(profile string) []value.Value {
	src := edgeStrings
	if profile == ProfileHostile {
		src = hostileStrings
	}
	out := make([]value.Value, len(src))
	for i, s := range src {
		out[i] = value.Str(s)
	}
	return out
}

// intProfileTable builds the int boundary table: 0, 1, -1 and the type
// extremes — or, when the field declares @range, the declared bounds exactly
// (so the applyRange clamp is a no-op) with the small constants kept only
// when they fall inside the range.
func intProfileTable(decs []model.Decorator) []value.Value {
	lo, hi, have := rangeBoundsFloat(decs)
	var candidates []int64
	if have {
		iLo, iHi := int64(lo), int64(hi)
		for _, c := range []int64{0, 1, -1} {
			if c >= iLo && c <= iHi {
				candidates = append(candidates, c)
			}
		}
		candidates = append(candidates, iLo, iHi)
	} else {
		candidates = []int64{0, 1, -1, math.MaxInt64, math.MinInt64}
	}
	seen := map[int64]struct{}{}
	out := make([]value.Value, 0, len(candidates))
	for _, c := range candidates {
		if _, dup := seen[c]; dup {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, value.Int(c))
	}
	return out
}

// floatProfileTable builds the float/decimal boundary table: 0, -0.0 and the
// type extremes (math.MaxFloat64 and the smallest positive subnormal) — or
// the declared @range bounds exactly when present, keeping 0/-0.0 only when
// in range. mk converts each float into the field's value kind so the same
// table serves float and decimal fields.
func floatProfileTable(decs []model.Decorator, mk func(float64) value.Value) []value.Value {
	lo, hi, have := rangeBoundsFloat(decs)
	var candidates []float64
	if have {
		negZero := math.Copysign(0, -1)
		for _, c := range []float64{0, negZero} {
			if c >= lo && c <= hi {
				candidates = append(candidates, c)
			}
		}
		candidates = append(candidates, lo, hi)
	} else {
		candidates = []float64{0, math.Copysign(0, -1), math.MaxFloat64, math.SmallestNonzeroFloat64}
	}
	seen := map[string]struct{}{}
	out := make([]value.Value, 0, len(candidates))
	for _, c := range candidates {
		// Format-based key keeps -0.0 distinct from 0 ("−0" vs "0").
		key := strconv.FormatFloat(c, 'g', -1, 64)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, mk(c))
	}
	return out
}

// timeProfileTable builds the date/datetime boundary table: Unix epoch,
// 1900-01-01 and 2100-12-31 — or, when the field declares @range, the
// declared bounds exactly (mirroring applyTimeRange's date-only and exclusive
// adjustments so the clamp is a no-op) with the fixed constants kept only
// when they fall inside the range.
func timeProfileTable(decs []model.Decorator) []value.Value {
	epoch := time.Unix(0, 0).UTC()
	early := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	late := time.Date(2100, 12, 31, 0, 0, 0, 0, time.UTC)

	if d := model.FindDecorator(decs, "range"); d != nil && len(d.Args) > 0 && d.Args[0].Kind == model.ArgRange {
		a := d.Args[0]
		lo, hi, ok := parseTimeRange(a.From, a.To, now())
		if ok {
			if a.LoExcl {
				lo = lo.Add(time.Nanosecond)
			}
			if a.HiExcl {
				hi = hi.Add(-time.Nanosecond)
			}
			out := []value.Value{value.Time(lo), value.Time(hi)}
			for _, c := range []time.Time{epoch, early, late} {
				if !c.Before(lo) && !c.After(hi) {
					out = append(out, value.Time(c))
				}
			}
			return out
		}
	}
	return []value.Value{value.Time(epoch), value.Time(early), value.Time(late)}
}

// rangeBoundsFloat extracts the field's numeric @range bounds, mirroring
// applyRange's epsilon handling for exclusive endpoints so table entries land
// exactly where the clamp would leave them (making the clamp a no-op).
func rangeBoundsFloat(decs []model.Decorator) (float64, float64, bool) {
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
	return lo, hi, true
}
