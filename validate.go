package datjit

import (
	"github.com/jmcarbo/datjitgo/core/errors"
	"github.com/jmcarbo/datjitgo/core/model"
	coreplan "github.com/jmcarbo/datjitgo/core/plan"
	"github.com/jmcarbo/datjitgo/core/ports"
	corerules "github.com/jmcarbo/datjitgo/core/rules"
	"github.com/jmcarbo/datjitgo/generator"
)

// Validate performs a cheap static analysis of the parsed document, catching
// the most common authoring mistakes before a (potentially expensive)
// Generate run. It enforces:
//
//   - every Reference targets an existing entity (or "self");
//   - every NamedType resolves to a types: entry or an enum;
//   - every enum referenced by a NamedType declares at least one variant;
//   - every rule expression parses cleanly;
//   - the entity dependency graph has no cycles.
//
// Violations are returned as the first encountered *errors.Error with
// Kind == KindValidation (or KindCyclicDependency for the topological
// check). This ordering mirrors the Rust implementation so behaviour stays
// consistent across ports.
func (s *Service) Validate(doc *model.Document) error {
	if s == nil {
		return nilServiceErr("Validate")
	}
	if doc == nil {
		return &errors.Error{Kind: errors.KindValidation, Message: "nil document"}
	}
	return validateDoc(doc, s.corpus)
}

// validateDoc is the implementation behind Service.Validate. It is a free
// function rather than a method to keep the validation logic unit-testable
// without instantiating a full Service.
func validateDoc(doc *model.Document, corpus ports.CorpusProvider) error {
	// Precompute lookup sets so the per-field checks stay O(1).
	entitySet := make(map[string]struct{}, doc.Entities.Len())
	doc.Entities.Each(func(name string, _ *model.Entity) bool {
		entitySet[name] = struct{}{}
		return true
	})
	typeSet := make(map[string]struct{}, doc.Types.Len())
	doc.Types.Each(func(name string, _ *model.Entity) bool {
		typeSet[name] = struct{}{}
		return true
	})
	enumSet := make(map[string]model.EnumDef, doc.Enums.Len())
	doc.Enums.Each(func(name string, def model.EnumDef) bool {
		enumSet[name] = def
		return true
	})

	// Entity-scoped checks: references + named-type resolution.
	var firstErr error
	doc.Entities.Each(func(ename string, ent *model.Entity) bool {
		ent.Fields.Each(func(fname string, f *model.Field) bool {
			if err := checkTypeExpr(f.Type, ename, fname, entitySet, typeSet, enumSet, corpus); err != nil {
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
			if err := checkTypeExpr(f.Type, tname, fname, entitySet, typeSet, enumSet, corpus); err != nil {
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

	// Rule expressions — cheap syntactic check via the generator's parser.
	// Cross-row rules carry a YAML body instead of an expression; skip them.
	for i, r := range doc.Rules {
		if r.Kind == model.RuleKindCrossRow {
			continue
		}
		expr := corerules.NormalizeExpr(r.Expr)
		if err := generator.ParseExpr(expr); err != nil {
			return &errors.Error{
				Kind:    errors.KindValidation,
				Message: "rule " + ruleIndex(i) + ": " + err.Error(),
				Cause:   err,
			}
		}
	}

	// Topological check — reuses the generator's Kahn pass.
	if _, err := coreplan.Entities(doc); err != nil {
		return err
	}
	return nil
}

// checkTypeExpr walks a TypeExpr and verifies that every Reference target
// and NamedType name can be resolved against the document-level lookup
// sets. Composite types are descended recursively.
func checkTypeExpr(t model.TypeExpr, entity, field string, entities, types map[string]struct{}, enums map[string]model.EnumDef, corpus ports.CorpusProvider) error {
	switch v := t.(type) {
	case model.Reference:
		if v.Target == "self" || v.Target == entity {
			return nil
		}
		if _, ok := entities[v.Target]; !ok {
			return &errors.Error{
				Kind:    errors.KindValidation,
				Entity:  entity,
				Field:   field,
				Message: "reference target not found: " + v.Target,
			}
		}
	case model.NamedType:
		if _, ok := types[v.Name]; ok {
			return nil
		}
		if def, ok := enums[v.Name]; ok {
			if len(def.Variants) == 0 {
				return &errors.Error{
					Kind:    errors.KindValidation,
					Entity:  entity,
					Field:   field,
					Message: "enum " + v.Name + " has no variants",
				}
			}
			return nil
		}
		return &errors.Error{
			Kind:    errors.KindValidation,
			Entity:  entity,
			Field:   field,
			Message: "named type not found: " + v.Name,
		}
	case model.Semantic:
		if !knownSemantic(v, corpus) {
			return &errors.Error{
				Kind:    errors.KindValidation,
				Entity:  entity,
				Field:   field,
				Message: "unknown semantic type: " + semanticName(v),
			}
		}
	case model.List:
		return checkTypeExpr(v.Element, entity, field, entities, types, enums, corpus)
	case model.Map:
		if err := checkTypeExpr(v.Key, entity, field, entities, types, enums, corpus); err != nil {
			return err
		}
		return checkTypeExpr(v.Value, entity, field, entities, types, enums, corpus)
	case model.Tuple:
		for _, e := range v.Elements {
			if err := checkTypeExpr(e, entity, field, entities, types, enums, corpus); err != nil {
				return err
			}
		}
	case model.Nullable:
		return checkTypeExpr(v.Inner, entity, field, entities, types, enums, corpus)
	case model.Union:
		for _, e := range v.Variants {
			if err := checkTypeExpr(e, entity, field, entities, types, enums, corpus); err != nil {
				return err
			}
		}
	}
	return nil
}

// ruleIndex formats i as a 1-based decimal without pulling in fmt.
func ruleIndex(i int) string {
	n := i + 1
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return "#" + string(buf[pos:])
}

func knownSemantic(v model.Semantic, corpus ports.CorpusProvider) bool {
	name := semanticName(v)
	if _, ok := knownSemanticNames[name]; ok {
		return true
	}
	return corpus != nil && corpus.Has(name)
}

func semanticName(v model.Semantic) string {
	if v.Tag == "" {
		return v.Namespace
	}
	return v.Namespace + "." + v.Tag
}

var knownSemanticNames = map[string]struct{}{
	"person.full": {}, "person.first": {}, "person.last": {}, "person.username": {}, "person.prefix": {}, "person.suffix": {}, "person.bio": {}, "person.avatar": {}, "person.gender": {}, "person.dob": {}, "person.age": {},
	"email": {}, "phone": {}, "phone.mobile": {}, "phone.landline": {},
	"url": {}, "url.image": {}, "url.avatar": {},
	"ipv4": {}, "ipv6": {}, "mac": {},
	"address.full": {}, "address.street": {}, "address.city": {}, "address.state": {}, "address.zip": {}, "address.country": {},
	"geo.lat": {}, "geo.lng": {}, "timezone": {},
	"currency.usd": {}, "currency.eur": {},
	"credit_card": {}, "credit_card.type": {}, "iban": {}, "swift": {},
	"text.word": {}, "text.sentence": {}, "text.paragraph": {}, "text.slug": {},
	"product.title": {}, "product.description": {}, "product.sku": {}, "sku": {},
	"company.name": {}, "company.industry": {}, "company.catch_phrase": {},
	"job.title": {}, "job.department": {},
	"color.hex": {}, "color.rgb": {}, "color.name": {},
	"file.name": {}, "file.extension": {}, "file.mime": {},
	"hash.md5": {}, "hash.sha256": {},
	"slug": {}, "code": {},
}
