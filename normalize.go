package datjit

import (
	"fmt"

	"github.com/periplon/datjitgo/core/model"
)

// normalizePolymorphicReferences augments doc in place so that every
// polymorphic-reference field — a field whose type is a union of two or more
// entity references (e.g. `owner: ->User | ->Org`) — gains a synthetic
// companion "discriminator" field recording which target entity each generated
// row's reference points to. Without it a polymorphic reference emits a bare
// primary key with no indication of which entity it belongs to.
//
// It is invoked from Service.Parse so Validate, Generate, Write and Inspect all
// observe a consistent document. It is idempotent: a field whose Discriminator
// is already set is skipped, so re-running is a no-op.
func normalizePolymorphicReferences(doc *model.Document) {
	if doc == nil {
		return
	}
	doc.Entities.Each(func(_ string, e *model.Entity) bool {
		normalizeEntityPolymorphic(e)
		return true
	})
	doc.Types.Each(func(_ string, e *model.Entity) bool {
		normalizeEntityPolymorphic(e)
		return true
	})
}

// normalizeEntityPolymorphic inserts a discriminator field after each
// polymorphic-reference field in e.
func normalizeEntityPolymorphic(e *model.Entity) {
	if e == nil || e.Fields == nil {
		return
	}
	// Snapshot the source field names first: InsertAfter mutates the map while
	// we iterate, and we only want to inspect the originally declared fields.
	for _, name := range e.Fields.Keys() {
		f, ok := e.Fields.Get(name)
		if !ok || f == nil {
			continue
		}
		if f.Discriminator != "" || f.DiscriminatorFor != "" {
			continue // already normalized, or itself a discriminator
		}
		if _, isPoly := polymorphicUnion(f.Type); !isPoly {
			continue
		}
		companion := freeDiscriminatorName(e, f.Name)
		f.Discriminator = companion
		e.Fields.InsertAfter(f.Name, companion, &model.Field{
			Name:             companion,
			Type:             model.Primitive{Kind: model.PrimString},
			Description:      fmt.Sprintf("Target entity of the polymorphic reference %q.", f.Name),
			DiscriminatorFor: f.Name,
		})
	}
}

// freeDiscriminatorName returns a field name for source's discriminator that
// does not collide with an existing field: "<source>_type", then
// "<source>_type_2", "<source>_type_3", ...
func freeDiscriminatorName(e *model.Entity, source string) string {
	base := source + "_type"
	if !e.Fields.Has(base) {
		return base
	}
	for i := 2; ; i++ {
		cand := fmt.Sprintf("%s_%d", base, i)
		if !e.Fields.Has(cand) {
			return cand
		}
	}
}

// polymorphicUnion reports whether t is a polymorphic reference: a union (or a
// nullable wrapping one) whose variants include two or more entity references.
// A union with fewer than two reference variants (e.g. `->A | string` or
// `string | int`) is not polymorphic and yields no discriminator.
func polymorphicUnion(t model.TypeExpr) (model.Union, bool) {
	switch v := t.(type) {
	case model.Union:
		if countReferenceVariants(v) >= 2 {
			return v, true
		}
	case model.Nullable:
		return polymorphicUnion(v.Inner)
	}
	return model.Union{}, false
}

// countReferenceVariants counts the direct Reference variants of a union.
func countReferenceVariants(u model.Union) int {
	n := 0
	for _, variant := range u.Variants {
		if _, ok := variant.(model.Reference); ok {
			n++
		}
	}
	return n
}
