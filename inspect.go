package datjit

import (
	"github.com/jmcarbo/datjitgo/core/errors"
	"github.com/jmcarbo/datjitgo/core/model"
)

// defaultVolume is used when neither the document nor options specify a
// per-entity volume. Kept in sync with generator.resolveVolume.
const defaultVolume = 10

// Inspect returns a structured summary of the document suitable for CLI
// `inspect` output or programmatic introspection. It does not generate any
// data.
//
// The returned Inspection is deterministic: entities are listed in
// declaration order and each EntitySummary's Dependencies are sorted.
func (s *Service) Inspect(doc *model.Document) (*model.Inspection, error) {
	if s == nil {
		return nil, nilServiceErr("Inspect")
	}
	if doc == nil {
		return nil, &errors.Error{Kind: errors.KindValidation, Message: "nil document"}
	}

	insp := &model.Inspection{
		Domain:      doc.Domain,
		Version:     doc.Version,
		EntityCount: doc.Entities.Len(),
	}

	// Build entity summaries in declaration order.
	doc.Entities.Each(func(name string, ent *model.Entity) bool {
		deps := entityDependencies(ent, name)
		vol := volumePlan(name, doc, s.volumes)
		insp.Entities = append(insp.Entities, model.EntitySummary{
			Name:         name,
			FieldCount:   ent.Fields.Len(),
			Dependencies: deps,
			VolumePlan:   vol,
		})
		return true
	})

	// Enums — preserve declaration order.
	doc.Enums.Each(func(_ string, def model.EnumDef) bool {
		insp.Enums = append(insp.Enums, def)
		return true
	})

	// Rules pass through verbatim.
	if len(doc.Rules) > 0 {
		insp.Rules = append([]model.Rule(nil), doc.Rules...)
	}

	return insp, nil
}

// entityDependencies returns the set of other entities referenced from any
// field in ent, excluding self-references. The slice is deterministically
// ordered (declaration-agnostic: insertion order of discovery).
func entityDependencies(ent *model.Entity, self string) []string {
	seen := map[string]struct{}{}
	var ordered []string
	var walk func(t model.TypeExpr)
	walk = func(t model.TypeExpr) {
		switch v := t.(type) {
		case model.Reference:
			if v.Target == "self" || v.Target == self {
				return
			}
			if _, ok := seen[v.Target]; !ok {
				seen[v.Target] = struct{}{}
				ordered = append(ordered, v.Target)
			}
		case model.List:
			walk(v.Element)
		case model.Map:
			walk(v.Key)
			walk(v.Value)
		case model.Tuple:
			for _, e := range v.Elements {
				walk(e)
			}
		case model.Nullable:
			walk(v.Inner)
		case model.Union:
			for _, e := range v.Variants {
				walk(e)
			}
		}
	}
	ent.Fields.Each(func(_ string, f *model.Field) bool {
		walk(f.Type)
		return true
	})
	return ordered
}

// volumePlan returns the effective volume spec that Generate would use for
// the named entity, honouring (in order): service-level overrides, the
// document's volume map, else a VolumeSpec with Exact=defaultVolume so
// downstream consumers have a non-zero number to render.
func volumePlan(name string, doc *model.Document, override map[string]int) model.VolumeSpec {
	if v, ok := override[name]; ok {
		return model.VolumeSpec{Exact: v}
	}
	if v, ok := doc.Volume[name]; ok {
		return v
	}
	return model.VolumeSpec{Exact: defaultVolume}
}
