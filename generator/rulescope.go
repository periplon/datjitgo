package generator

import (
	"strings"

	"github.com/periplon/datjitgo/core/model"
	corerules "github.com/periplon/datjitgo/core/rules"
)

// collectPaths walks an expression AST and invokes fn for every path node's
// dotted string (e.g. "User.age", "age", "items.amount").
func collectPaths(n exprNode, fn func(string)) {
	if n.kind == exprPath {
		fn(n.str)
	}
	for i := range n.children {
		collectPaths(n.children[i], fn)
	}
}

// ruleReferences parses a rule expression and splits the paths it references
// into entity-qualified names (the first segment is a known entity) and bare
// field names (single-segment paths, or first segment of a row-relative path).
// ok is false when the expression does not parse; callers then conservatively
// scope the rule to every entity so the malformed rule still fails loudly
// during evaluation rather than being silently skipped.
func ruleReferences(expr string, entityNames map[string]struct{}) (qualified map[string]struct{}, bare []string, ok bool) {
	node, err := parseExpr(corerules.NormalizeExpr(expr))
	if err != nil {
		return nil, nil, false
	}
	qualified = map[string]struct{}{}
	seen := map[string]struct{}{}
	collectPaths(node, func(path string) {
		segs := strings.Split(path, ".")
		if len(segs) >= 2 {
			if _, ok := entityNames[segs[0]]; ok {
				qualified[segs[0]] = struct{}{}
				return
			}
		}
		f := segs[0]
		if _, ok := seen[f]; !ok {
			seen[f] = struct{}{}
			bare = append(bare, f)
		}
	})
	return qualified, bare, true
}

// allEntityNames returns the set of every entity name in the document.
func allEntityNames(doc *model.Document) map[string]struct{} {
	names := map[string]struct{}{}
	doc.Entities.Each(func(n string, _ *model.Entity) bool {
		names[n] = struct{}{}
		return true
	})
	return names
}

// ruleTargetEntities returns the set of entity names a rule applies to.
//
//   - A rule that names entities explicitly (`Entity.field`) applies only to
//     those entities.
//   - An unqualified rule (bare field names) applies to every entity that
//     declares all the referenced fields, and never to entities lacking them.
//     This prevents a bare rule from being enforced against unrelated entities,
//     which previously produced spurious violations and @strict retry
//     exhaustion.
func ruleTargetEntities(expr string, doc *model.Document) map[string]struct{} {
	out := map[string]struct{}{}
	if doc == nil || doc.Entities == nil {
		return out
	}
	names := allEntityNames(doc)

	qualified, bare, ok := ruleReferences(expr, names)
	if !ok {
		// Malformed expression: scope to every entity so evaluation surfaces
		// the error instead of silently skipping the rule.
		return names
	}
	if len(qualified) > 0 {
		for n := range qualified {
			out[n] = struct{}{}
		}
		return out
	}
	if len(bare) == 0 {
		return out
	}
	doc.Entities.Each(func(n string, ent *model.Entity) bool {
		for _, f := range bare {
			if _, ok := ent.Fields.Get(f); !ok {
				return true // entity missing a referenced field — skip it
			}
		}
		out[n] = struct{}{}
		return true
	})
	return out
}

// computeRuleScope precomputes, for each rule in the document, the set of
// entity names it targets. The returned slice is aligned with doc.Rules by
// index. Cross-row rules carry no Expr and target nothing here.
func computeRuleScope(doc *model.Document) []map[string]struct{} {
	if doc == nil {
		return nil
	}
	scope := make([]map[string]struct{}, len(doc.Rules))
	for i := range doc.Rules {
		r := doc.Rules[i]
		if r.Kind != model.RuleKindExpr || r.Expr == "" {
			scope[i] = map[string]struct{}{}
			continue
		}
		scope[i] = ruleTargetEntities(r.Expr, doc)
	}
	return scope
}
