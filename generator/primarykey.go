package generator

import (
	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/value"
)

// primaryKeyField returns the name of the entity's primary-key field — the
// first field carrying the @primary decorator. It returns "" when no field is
// marked @primary, signalling callers to fall back to positional resolution.
func primaryKeyField(ent *model.Entity) string {
	if ent == nil || ent.Fields == nil {
		return ""
	}
	pk := ""
	ent.Fields.Each(func(name string, f *model.Field) bool {
		if model.HasDecorator(f.Decorators, "primary") {
			pk = name
			return false
		}
		return true
	})
	return pk
}

// primaryKeyMap precomputes entity name → primary-key field name for every
// entity in the document. Entities without an explicit @primary are absent
// from the map, so callers fall back to positional first-field resolution.
func primaryKeyMap(doc *model.Document) map[string]string {
	out := map[string]string{}
	if doc == nil || doc.Entities == nil {
		return out
	}
	doc.Entities.Each(func(name string, ent *model.Entity) bool {
		if pk := primaryKeyField(ent); pk != "" {
			out[name] = pk
		}
		return true
	})
	return out
}

// referenceValue resolves the value a foreign key should carry for a target
// row: the explicit @primary field when the target declares one, otherwise the
// first field (legacy positional behaviour). Decoupling FK identity from
// insertion order keeps resolution correct even when coherence groups populate
// other fields before the primary key.
func referenceValue(row *value.Object, pkField string) value.Value {
	if pkField != "" {
		if v, ok := row.Get(pkField); ok {
			return v
		}
	}
	return firstField(row)
}
