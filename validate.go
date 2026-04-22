package datjit

import (
	"github.com/jmcarbo/datjitgo/core/errors"
	"github.com/jmcarbo/datjitgo/core/model"
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
	return validateDoc(doc)
}

// validateDoc is the implementation behind Service.Validate. It is a free
// function rather than a method to keep the validation logic unit-testable
// without instantiating a full Service.
func validateDoc(doc *model.Document) error {
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
			if err := checkTypeExpr(f.Type, ename, fname, entitySet, typeSet, enumSet); err != nil {
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
		expr := r.Expr
		// Rules routinely use "if COND then THEN" shorthand; the generator
		// rewrites this before eval, so we do the same before parsing so
		// validation matches runtime behaviour.
		if normalized := rewriteIfThen(expr); normalized != "" {
			expr = normalized
		}
		if err := generator.ParseExpr(expr); err != nil {
			return &errors.Error{
				Kind:    errors.KindValidation,
				Message: "rule " + ruleIndex(i) + ": " + err.Error(),
				Cause:   err,
			}
		}
	}

	// Topological check — reuses the generator's Kahn pass.
	if _, err := generator.Plan(doc); err != nil {
		return err
	}
	return nil
}

// checkTypeExpr walks a TypeExpr and verifies that every Reference target
// and NamedType name can be resolved against the document-level lookup
// sets. Composite types are descended recursively.
func checkTypeExpr(t model.TypeExpr, entity, field string, entities, types map[string]struct{}, enums map[string]model.EnumDef) error {
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
	case model.List:
		return checkTypeExpr(v.Element, entity, field, entities, types, enums)
	case model.Map:
		if err := checkTypeExpr(v.Key, entity, field, entities, types, enums); err != nil {
			return err
		}
		return checkTypeExpr(v.Value, entity, field, entities, types, enums)
	case model.Tuple:
		for _, e := range v.Elements {
			if err := checkTypeExpr(e, entity, field, entities, types, enums); err != nil {
				return err
			}
		}
	case model.Nullable:
		return checkTypeExpr(v.Inner, entity, field, entities, types, enums)
	case model.Union:
		for _, e := range v.Variants {
			if err := checkTypeExpr(e, entity, field, entities, types, enums); err != nil {
				return err
			}
		}
	}
	return nil
}

// rewriteIfThen mirrors generator.rewriteIfThen so Validate can pre-process
// rules the same way the engine will. Keeping this duplicated (5 lines)
// avoids leaking the generator's internal helper into the stable API.
func rewriteIfThen(src string) string {
	const ifPrefix = "if "
	const thenSep = " then "
	s := src
	// Trim leading whitespace by hand to avoid an extra import.
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r') {
		s = s[1:]
	}
	if len(s) < len(ifPrefix) || s[:len(ifPrefix)] != ifPrefix {
		return ""
	}
	s = s[len(ifPrefix):]
	idx := indexOf(s, thenSep)
	if idx < 0 {
		return ""
	}
	cond := trimSpace(s[:idx])
	then := trimSpace(s[idx+len(thenSep):])
	return "not (" + cond + ") or (" + then + ")"
}

func indexOf(haystack, needle string) int {
	if len(needle) == 0 {
		return 0
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
		i++
	}
	j := len(s)
	for j > i && (s[j-1] == ' ' || s[j-1] == '\t' || s[j-1] == '\n' || s[j-1] == '\r') {
		j--
	}
	return s[i:j]
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
