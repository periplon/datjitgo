package generator

import (
	"fmt"
	"strings"

	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/ports"
	"github.com/periplon/datjitgo/core/value"
)

// applyCoherence generates coherence group values for entity and writes them
// directly into row. Returns the set of field names it populated so the main
// field loop can skip them.
//
// Implementation mirrors the Rust heuristics: known group names (location,
// identity) get bespoke anchors; unknown groups fall back to a best-effort
// generate-in-order pass.
func (e *Engine) applyCoherence(entity *model.Entity, row *value.Object, rng ports.Randomizer) (map[string]struct{}, error) {
	populated := map[string]struct{}{}

	if entity.Coherence == nil {
		return populated, nil
	}
	entity.Coherence.Each(func(group string, fields []string) bool {
		if isLocationGroup(group, fields) {
			e.generateLocationGroup(fields, row, rng)
		} else if isIdentityGroup(group, fields) {
			e.generateIdentityGroup(fields, row, rng)
		} else {
			// Default: generate each as a regular field without visibility
			// into the row pipeline — phase-1 limitation.
			for _, fn := range fields {
				f, ok := entity.Fields.Get(fn)
				if !ok {
					continue
				}
				// Use default per-type generation without recursion.
				v, err := e.generateByType(entity, f, f.Type, row, &generationState{enumDefs: map[string]model.EnumDef{}, generated: map[string][]*value.Object{}, seqs: newSeqCounters()}, rng)
				if err != nil {
					continue
				}
				row.Set(fn, v)
			}
		}
		for _, fn := range fields {
			populated[fn] = struct{}{}
		}
		return true
	})

	return populated, nil
}

// applyFromDerivations handles @from decorators on fields that are not part
// of an explicit coherence group: the source sibling is generated on-demand
// if missing, and the target is derived from it using the same semantic
// heuristics the identity group uses.
func (e *Engine) applyFromDerivations(entity *model.Entity, row *value.Object, rng ports.Randomizer, alreadyCoherent map[string]struct{}) error {
	var firstErr error
	entity.Fields.Each(func(fname string, f *model.Field) bool {
		if _, ok := alreadyCoherent[fname]; ok {
			return true
		}
		d := model.FindDecorator(f.Decorators, "from")
		if d == nil {
			return true
		}
		sources := decoratorIdentSources(d.Args)
		if len(sources) == 0 {
			return true
		}
		// Gather source values (generating siblings on demand is not needed
		// here because siblings are already produced earlier in the field
		// loop — @from only matters for fields declared after their source).
		var parts []string
		for _, src := range sources {
			v, ok := row.Get(src)
			if !ok || v.IsNull() {
				continue
			}
			parts = append(parts, valueDisplay(v))
		}
		if len(parts) == 0 {
			return true
		}
		combined := strings.Join(parts, " ")
		derived, err := e.deriveFromSource(fname, combined, rng)
		if err != nil {
			firstErr = err
			return false
		}
		row.Set(fname, derived)
		return true
	})
	return firstErr
}

func decoratorIdentSources(args []model.DecoratorArg) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		switch a.Kind {
		case model.ArgIdent:
			out = append(out, a.Ident)
		case model.ArgLiteral:
			if s, ok := a.Literal.(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}

// deriveFromSource matches the target field name against a few well-known
// semantic shapes (email, username, timezone, phone) so `@from(name)` on a
// `email` field produces a plausible address.
func (e *Engine) deriveFromSource(target, source string, rng ports.Randomizer) (value.Value, error) {
	lower := strings.ToLower(target)
	switch {
	case strings.Contains(lower, "email"):
		parts := strings.Fields(source)
		domain, err := e.sampleCorpusString(rng, "email.domains")
		if err != nil {
			domain = "example.com"
		}
		local := strings.ToLower(strings.ReplaceAll(source, " ", "."))
		if len(parts) >= 2 {
			local = strings.ToLower(parts[0]) + "." + strings.ToLower(parts[1])
		}
		return value.Str(local + "@" + domain), nil
	case strings.Contains(lower, "username") || strings.Contains(lower, "handle"):
		parts := strings.Fields(source)
		if len(parts) == 0 {
			return value.Str(fmt.Sprintf("user%d", rng.IntN(999)+1)), nil
		}
		return value.Str(fmt.Sprintf("%s%d", strings.ToLower(parts[0]), rng.IntN(999)+1)), nil
	case strings.Contains(lower, "timezone") || lower == "tz" || lower == "time_zone":
		// Cheap fallback — specific cities aren't baked in phase 1.
		return value.Str("America/New_York"), nil
	case strings.Contains(lower, "phone"):
		mid := rng.IntN(900) + 100
		tail := rng.IntN(9000) + 1000
		return value.Str(fmt.Sprintf("+1-555-%03d-%04d", mid, tail)), nil
	default:
		return value.Str(source), nil
	}
}

// isLocationGroup matches groups whose name or fields hint at geography.
func isLocationGroup(group string, fields []string) bool {
	g := strings.ToLower(group)
	if strings.Contains(g, "location") || strings.Contains(g, "address") || strings.Contains(g, "geo") {
		return true
	}
	hits := 0
	keywords := []string{"office", "city", "state", "zip", "timezone", "phone", "address"}
	for _, f := range fields {
		lf := strings.ToLower(f)
		for _, k := range keywords {
			if strings.Contains(lf, k) {
				hits++
				break
			}
		}
	}
	return hits >= 2
}

// isIdentityGroup matches groups whose fields hint at personal identity.
func isIdentityGroup(group string, fields []string) bool {
	g := strings.ToLower(group)
	if strings.Contains(g, "identity") || strings.Contains(g, "person") || strings.Contains(g, "name") {
		return true
	}
	hits := 0
	keywords := []string{"first_name", "last_name", "email", "username", "name"}
	for _, f := range fields {
		lf := strings.ToLower(f)
		for _, k := range keywords {
			if strings.Contains(lf, k) {
				hits++
				break
			}
		}
	}
	return hits >= 2
}

// generateLocationGroup constructs a coherent location anchor (city + state +
// zip + timezone + phone) from the corpus, then projects onto the group's
// field names using simple substring matching.
func (e *Engine) generateLocationGroup(fields []string, row *value.Object, rng ports.Randomizer) {
	city, _ := e.sampleCorpusString(rng, "address.cities")
	state, _ := e.sampleCorpusString(rng, "address.states")
	zip := fmt.Sprintf("%05d", rng.IntN(90000)+10000)
	tz, _ := e.sampleCorpusString(rng, "timezones")
	if tz == "" {
		tz = "America/New_York"
	}
	area, _ := e.sampleCorpusString(rng, "phone.area_codes")
	if area == "" {
		area = "555"
	}
	for _, fn := range fields {
		lower := strings.ToLower(fn)
		switch {
		case strings.Contains(lower, "office") || strings.Contains(lower, "city") || strings.Contains(lower, "location"):
			row.Set(fn, value.Str(fmt.Sprintf("%s, %s", city, state)))
		case strings.Contains(lower, "state") || strings.Contains(lower, "region"):
			row.Set(fn, value.Str(state))
		case strings.Contains(lower, "zip") || strings.Contains(lower, "postal"):
			row.Set(fn, value.Str(zip))
		case strings.Contains(lower, "timezone") || lower == "tz":
			row.Set(fn, value.Str(tz))
		case strings.Contains(lower, "phone"):
			row.Set(fn, value.Str(fmt.Sprintf("+1-%s-%03d-%04d", area, rng.IntN(900)+100, rng.IntN(9000)+1000)))
		case strings.Contains(lower, "address") || strings.Contains(lower, "street"):
			street, _ := e.sampleCorpusString(rng, "address.streets")
			row.Set(fn, value.Str(fmt.Sprintf("%d %s, %s, %s %s", rng.IntN(9900)+100, street, city, state, zip)))
		case strings.Contains(lower, "country"):
			row.Set(fn, value.Str("US"))
		default:
			row.Set(fn, value.Str(fmt.Sprintf("%s, %s", city, state)))
		}
	}
}

// generateIdentityGroup picks a single first/last/domain triple and projects
// them onto any name/email/username fields in the group.
func (e *Engine) generateIdentityGroup(fields []string, row *value.Object, rng ports.Randomizer) {
	first, _ := e.sampleCorpusString(rng, "person.first_names")
	last, _ := e.sampleCorpusString(rng, "person.last_names")
	domain, err := e.sampleCorpusString(rng, "email.domains")
	if err != nil || domain == "" {
		domain = "example.com"
	}
	numSuffix := rng.IntN(998) + 1

	lowerFirst := strings.ToLower(first)
	lowerLast := strings.ToLower(last)

	for _, fn := range fields {
		lower := strings.ToLower(fn)
		switch {
		case strings.Contains(lower, "first"):
			row.Set(fn, value.Str(first))
		case strings.Contains(lower, "last") || strings.Contains(lower, "surname"):
			row.Set(fn, value.Str(last))
		case lower == "name" || strings.Contains(lower, "full_name") || strings.Contains(lower, "display_name"):
			row.Set(fn, value.Str(first+" "+last))
		case strings.Contains(lower, "email"):
			row.Set(fn, value.Str(fmt.Sprintf("%s.%s@%s", lowerFirst, lowerLast, domain)))
		case strings.Contains(lower, "username") || strings.Contains(lower, "handle"):
			row.Set(fn, value.Str(fmt.Sprintf("%s%d", lowerFirst, numSuffix)))
		default:
			row.Set(fn, value.Str(first+" "+last))
		}
	}
}
