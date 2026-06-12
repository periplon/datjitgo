package datjit

import (
	"sort"

	"github.com/periplon/datjitgo/core/model"
	coreplan "github.com/periplon/datjitgo/core/plan"
)

// SchemaSummary returns a stable, ordered, machine-readable signature of doc
// suitable for committing as a CI drift fixture or feeding DiffSchemaSummaries.
// It performs no generation and uses no randomness. Entities and rules follow
// document order; enums and volumes are sorted by name so the summary is
// deterministic regardless of map iteration order.
//
// Synthetic polymorphic-reference discriminator fields injected by Parse are
// included as ordinary fields. Index metadata (_indexes / inferred indexes) is
// intentionally not summarized — it is output-only and does not affect the
// consumer-visible shape.
func (s *Service) SchemaSummary(doc *model.Document) *model.SchemaSummary {
	if s == nil || doc == nil {
		return nil
	}

	sum := &model.SchemaSummary{
		Domain:  doc.Domain,
		Version: doc.Version,
		Locale:  s.summaryLocale(doc),
	}

	doc.Entities.Each(func(name string, ent *model.Entity) bool {
		es := model.SchemaEntitySummary{Name: name}
		ent.Fields.Each(func(fname string, f *model.Field) bool {
			es.Fields = append(es.Fields, model.FieldSummary{
				Name:       fname,
				Type:       model.RenderType(f.Type),
				Decorators: model.RenderDecorators(f.Decorators),
			})
			return true
		})
		sum.Entities = append(sum.Entities, es)
		return true
	})

	doc.Enums.Each(func(name string, def model.EnumDef) bool {
		sum.Enums = append(sum.Enums, model.EnumSummary{
			Name:     name,
			Variants: def.Values(),
		})
		return true
	})
	sort.Slice(sum.Enums, func(i, j int) bool { return sum.Enums[i].Name < sum.Enums[j].Name })

	for _, r := range doc.Rules {
		sum.Rules = append(sum.Rules, ruleString(r))
	}

	for _, name := range doc.Entities.Keys() {
		sum.Volumes = append(sum.Volumes, model.VolumeSummary{
			Entity: name,
			Spec:   renderVolumeSpec(volumePlan(name, doc, s.volumes)),
		})
	}
	sort.Slice(sum.Volumes, func(i, j int) bool { return sum.Volumes[i].Entity < sum.Volumes[j].Entity })

	return sum
}

// summaryLocale resolves the locale recorded in the summary: a service-level
// override wins over the document's locale.
func (s *Service) summaryLocale(doc *model.Document) string {
	if s.locale != "" {
		return s.locale
	}
	return doc.Locale
}

// ruleString renders a Rule as a canonical, copy-paste-friendly string with a
// trailing severity tag matching the inspect output convention.
func ruleString(r model.Rule) string {
	body := r.Expr
	if r.Kind == model.RuleKindCrossRow {
		body = "cross_row"
	}
	switch r.Severity {
	case model.RuleProbabilistic:
		return body + " @prob"
	case model.RuleWarn:
		return body + " @warn"
	default:
		return body + " @strict"
	}
}

// renderVolumeSpec formats a VolumeSpec the same way the inspect CLI does:
// "min..max" for ranges, the exact count otherwise.
func renderVolumeSpec(v model.VolumeSpec) string {
	if v.IsRange() {
		return itoa(v.Min) + ".." + itoa(v.Max)
	}
	return itoa(v.Exact)
}

// itoa formats an int as decimal without fmt, mirroring the model helper.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

// DependencyGraph returns the entity-reference structure of doc: nodes in
// document order, one edge per referencing field (polymorphic unions expanded
// to one edge per target), and exemplar cycle paths. Self-references are
// excluded from edges' cycle participation but reported with Kind "self".
func (s *Service) DependencyGraph(doc *model.Document) *model.DependencyGraph {
	if s == nil || doc == nil {
		return nil
	}

	g := &model.DependencyGraph{
		Nodes:  doc.Entities.Keys(),
		Cycles: coreplan.Cycles(doc),
	}

	known := make(map[string]struct{}, doc.Entities.Len())
	for _, n := range g.Nodes {
		known[n] = struct{}{}
	}

	doc.Entities.Each(func(from string, ent *model.Entity) bool {
		ent.Fields.Each(func(fname string, f *model.Field) bool {
			for _, e := range fieldEdges(from, fname, f.Type) {
				if _, ok := known[e.To]; !ok && e.Kind != "self" {
					continue
				}
				g.Edges = append(g.Edges, e)
			}
			return true
		})
		return true
	})

	return g
}

// fieldEdges enumerates the dependency edges contributed by a single field's
// type. A bare reference yields one edge; a union of references (polymorphic)
// yields one edge per target with Kind "polymorphic"; many-to-many references
// yield Kind "many-to-many"; self-references yield Kind "self". Composite types
// (list, map, tuple, nullable) are descended.
func fieldEdges(from, field string, t model.TypeExpr) []model.DepEdge {
	refs := collectFieldRefs(t)
	polymorphic := countDistinctTargets(refs) >= 2
	edges := make([]model.DepEdge, 0, len(refs))
	for _, r := range refs {
		kind := "reference"
		switch {
		case r.Target == "self" || r.Target == from:
			kind = "self"
		case r.ManyToMany:
			kind = "many-to-many"
		case polymorphic:
			kind = "polymorphic"
		}
		to := r.Target
		if r.Target == "self" {
			to = from
		}
		edges = append(edges, model.DepEdge{From: from, To: to, Field: field, Kind: kind})
	}
	return edges
}

// collectFieldRefs returns the References reachable from t in order. Unlike a
// set-based walk it preserves duplicates and order so polymorphic unions expand
// predictably.
func collectFieldRefs(t model.TypeExpr) []model.Reference {
	var refs []model.Reference
	var walk func(model.TypeExpr)
	walk = func(t model.TypeExpr) {
		switch v := t.(type) {
		case model.Reference:
			refs = append(refs, v)
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
	walk(t)
	return refs
}

// countDistinctTargets returns the number of distinct reference targets in refs.
func countDistinctTargets(refs []model.Reference) int {
	seen := make(map[string]struct{}, len(refs))
	for _, r := range refs {
		seen[r.Target] = struct{}{}
	}
	return len(seen)
}

// DiffSchemaSummaries compares an old and new SchemaSummary and reports every
// structural change, classifying each as breaking or compatible.
//
// Breaking changes alter the consumer-visible shape: removing an entity, field,
// or enum; removing an enum variant; changing a field's type; or changing the
// domain. Compatible changes preserve the shape: additions, volume changes,
// decorator changes (which can alter generated values but not the visible
// shape), and enum-variant additions. Either argument may be nil (treated as an
// empty summary). Output ordering is deterministic.
func DiffSchemaSummaries(old, new *model.SchemaSummary) *model.SchemaDiff {
	if old == nil {
		old = &model.SchemaSummary{}
	}
	if new == nil {
		new = &model.SchemaSummary{}
	}
	diff := &model.SchemaDiff{}

	if old.Domain != new.Domain {
		diff.Changes = append(diff.Changes, model.SchemaChange{
			Kind: "domain-changed", Old: old.Domain, New: new.Domain, Breaking: true,
		})
	}

	diffEntities(old, new, diff)
	diffEnums(old, new, diff)
	diffVolumes(old, new, diff)
	diffRules(old, new, diff)

	return diff
}

// diffEntities compares the entity (and nested field) sets of two summaries.
func diffEntities(old, new *model.SchemaSummary, diff *model.SchemaDiff) {
	oldEnts := indexEntities(old)
	newEnts := indexEntities(new)

	for _, e := range old.Entities {
		if _, ok := newEnts[e.Name]; !ok {
			diff.Changes = append(diff.Changes, model.SchemaChange{
				Kind: "entity-removed", Entity: e.Name, Breaking: true,
			})
		}
	}
	for _, e := range new.Entities {
		oldEnt, ok := oldEnts[e.Name]
		if !ok {
			diff.Changes = append(diff.Changes, model.SchemaChange{
				Kind: "entity-added", Entity: e.Name,
			})
			continue
		}
		diffFields(e.Name, oldEnt, e, diff)
	}
}

// diffFields compares the fields of one entity that exists in both summaries.
func diffFields(entity string, oldEnt, newEnt model.SchemaEntitySummary, diff *model.SchemaDiff) {
	oldFields := indexFields(oldEnt)
	newFields := indexFields(newEnt)

	for _, f := range oldEnt.Fields {
		if _, ok := newFields[f.Name]; !ok {
			diff.Changes = append(diff.Changes, model.SchemaChange{
				Kind: "field-removed", Entity: entity, Field: f.Name, Old: f.Type, Breaking: true,
			})
		}
	}
	for _, f := range newEnt.Fields {
		oldF, ok := oldFields[f.Name]
		if !ok {
			diff.Changes = append(diff.Changes, model.SchemaChange{
				Kind: "field-added", Entity: entity, Field: f.Name, New: f.Type,
			})
			continue
		}
		if oldF.Type != f.Type {
			diff.Changes = append(diff.Changes, model.SchemaChange{
				Kind: "field-type-changed", Entity: entity, Field: f.Name,
				Old: oldF.Type, New: f.Type, Breaking: true,
			})
		}
		if oldD, newD := joinDecorators(oldF.Decorators), joinDecorators(f.Decorators); oldD != newD {
			diff.Changes = append(diff.Changes, model.SchemaChange{
				Kind: "field-decorators-changed", Entity: entity, Field: f.Name,
				Old: oldD, New: newD,
			})
		}
	}
}

// diffEnums compares the enum sets of two summaries.
func diffEnums(old, new *model.SchemaSummary, diff *model.SchemaDiff) {
	oldEnums := indexEnums(old)
	newEnums := indexEnums(new)

	for _, e := range old.Enums {
		if _, ok := newEnums[e.Name]; !ok {
			diff.Changes = append(diff.Changes, model.SchemaChange{
				Kind: "enum-removed", Entity: e.Name, Breaking: true,
			})
		}
	}
	for _, e := range new.Enums {
		oldE, ok := oldEnums[e.Name]
		if !ok {
			diff.Changes = append(diff.Changes, model.SchemaChange{
				Kind: "enum-added", Entity: e.Name,
			})
			continue
		}
		if !equalStrings(oldE.Variants, e.Variants) {
			diff.Changes = append(diff.Changes, model.SchemaChange{
				Kind: "enum-variants-changed", Entity: e.Name,
				Old: joinStrings(oldE.Variants), New: joinStrings(e.Variants),
				Breaking: removesVariant(oldE.Variants, e.Variants),
			})
		}
	}
}

// diffVolumes compares per-entity volume specs.
func diffVolumes(old, new *model.SchemaSummary, diff *model.SchemaDiff) {
	oldVols := indexVolumes(old)
	for _, v := range new.Volumes {
		if oldSpec, ok := oldVols[v.Entity]; ok && oldSpec != v.Spec {
			diff.Changes = append(diff.Changes, model.SchemaChange{
				Kind: "volume-changed", Entity: v.Entity, Old: oldSpec, New: v.Spec,
			})
		}
	}
}

// diffRules compares the rule sets of two summaries as ordered multisets,
// reporting net removals and additions.
func diffRules(old, new *model.SchemaSummary, diff *model.SchemaDiff) {
	oldCount := countStrings(old.Rules)
	newCount := countStrings(new.Rules)

	for _, r := range old.Rules {
		if oldCount[r] > newCount[r] {
			oldCount[r]--
			diff.Changes = append(diff.Changes, model.SchemaChange{
				Kind: "rule-removed", Old: r, Breaking: true,
			})
		}
	}
	for _, r := range new.Rules {
		if newCount[r] > oldCount[r] {
			newCount[r]--
			diff.Changes = append(diff.Changes, model.SchemaChange{
				Kind: "rule-added", New: r,
			})
		}
	}
}

// indexEntities maps entity name to its summary.
func indexEntities(s *model.SchemaSummary) map[string]model.SchemaEntitySummary {
	m := make(map[string]model.SchemaEntitySummary, len(s.Entities))
	for _, e := range s.Entities {
		m[e.Name] = e
	}
	return m
}

// indexFields maps field name to its summary within an entity.
func indexFields(e model.SchemaEntitySummary) map[string]model.FieldSummary {
	m := make(map[string]model.FieldSummary, len(e.Fields))
	for _, f := range e.Fields {
		m[f.Name] = f
	}
	return m
}

// indexEnums maps enum name to its summary.
func indexEnums(s *model.SchemaSummary) map[string]model.EnumSummary {
	m := make(map[string]model.EnumSummary, len(s.Enums))
	for _, e := range s.Enums {
		m[e.Name] = e
	}
	return m
}

// indexVolumes maps entity name to its volume spec string.
func indexVolumes(s *model.SchemaSummary) map[string]string {
	m := make(map[string]string, len(s.Volumes))
	for _, v := range s.Volumes {
		m[v.Entity] = v.Spec
	}
	return m
}

// countStrings returns a multiset count of the values in xs.
func countStrings(xs []string) map[string]int {
	m := make(map[string]int, len(xs))
	for _, x := range xs {
		m[x]++
	}
	return m
}

// joinDecorators joins decorator strings with a single space for comparison
// and reporting.
func joinDecorators(d []string) string { return joinSep(d, " ") }

// joinStrings joins values with ", " for human-readable change reporting.
func joinStrings(d []string) string { return joinSep(d, ", ") }

// joinSep joins xs with sep without importing strings into hot paths.
func joinSep(xs []string, sep string) string {
	out := ""
	for i, x := range xs {
		if i > 0 {
			out += sep
		}
		out += x
	}
	return out
}

// equalStrings reports whether two ordered string slices are element-wise equal.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// removesVariant reports whether any value present in old is absent from new
// (i.e. the change removes at least one enum variant, which is breaking).
func removesVariant(old, new []string) bool {
	present := make(map[string]struct{}, len(new))
	for _, v := range new {
		present[v] = struct{}{}
	}
	for _, v := range old {
		if _, ok := present[v]; !ok {
			return true
		}
	}
	return false
}
