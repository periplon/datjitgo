package generator

import (
	"github.com/jmcarbo/datjitgo/core/errors"
	"github.com/jmcarbo/datjitgo/core/model"
)

// Plan returns the entity names of doc in topological order. It is the
// exported entry point for callers (e.g. the facade package's validator)
// that need to discover the dependency order or detect cycles without
// running a full Generate. See plan for implementation details.
func Plan(doc *model.Document) ([]string, error) { return plan(doc) }

// plan returns the entity names of doc in topological order using Kahn's
// algorithm. Reference fields whose Target is "self" or equal to the hosting
// entity are ignored. Ties are broken by document insertion order so output
// is deterministic.
//
// Returns *errors.Error{Kind: KindCyclicDependency} if a cycle is detected.
func plan(doc *model.Document) ([]string, error) {
	order := doc.Entities.Keys()
	rank := make(map[string]int, len(order))
	for i, n := range order {
		rank[n] = i
	}
	indeg := make(map[string]int, len(order))
	// outEdges[from] = entities that depend on from
	outEdges := make(map[string][]string, len(order))

	for _, name := range order {
		indeg[name] = 0
	}

	for _, name := range order {
		e, _ := doc.Entities.Get(name)
		// Collect unique targets to avoid counting the same reference twice.
		seen := make(map[string]struct{})
		collectRefs(e.Fields, seen)
		for target := range seen {
			if target == "self" || target == name {
				continue
			}
			// Only count targets that are known entities.
			if _, ok := doc.Entities.Get(target); !ok {
				continue
			}
			outEdges[target] = append(outEdges[target], name)
			indeg[name]++
		}
	}

	// Initial frontier = all entities with in-degree 0, in insertion order.
	queue := make([]string, 0, len(order))
	for _, n := range order {
		if indeg[n] == 0 {
			queue = append(queue, n)
		}
	}

	result := make([]string, 0, len(order))
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		result = append(result, n)

		// Collect newly-ready dependents, sort by insertion order.
		ready := make([]string, 0)
		for _, dep := range outEdges[n] {
			indeg[dep]--
			if indeg[dep] == 0 {
				ready = append(ready, dep)
			}
		}
		sortByInsertionOrder(ready, rank)
		queue = append(queue, ready...)
	}

	if len(result) != len(order) {
		return nil, &errors.Error{
			Kind:    errors.KindCyclicDependency,
			Message: "cycle detected in entity references",
		}
	}
	return result, nil
}

// collectRefs walks a field OrderedMap and records every referenced entity
// target into seen. List/Map/Nullable/Union composites are descended.
func collectRefs(fields *model.OrderedMap[string, *model.Field], seen map[string]struct{}) {
	fields.Each(func(_ string, f *model.Field) bool {
		walkTypeRefs(f.Type, seen)
		return true
	})
}

func walkTypeRefs(t model.TypeExpr, seen map[string]struct{}) {
	switch v := t.(type) {
	case model.Reference:
		seen[v.Target] = struct{}{}
	case model.List:
		walkTypeRefs(v.Element, seen)
	case model.Map:
		walkTypeRefs(v.Key, seen)
		walkTypeRefs(v.Value, seen)
	case model.Tuple:
		for _, e := range v.Elements {
			walkTypeRefs(e, seen)
		}
	case model.Nullable:
		walkTypeRefs(v.Inner, seen)
	case model.Union:
		for _, e := range v.Variants {
			walkTypeRefs(e, seen)
		}
	}
}

func sortByInsertionOrder(xs []string, rank map[string]int) {
	// Small slice — simple insertion sort keeps this zero-allocation.
	for i := 1; i < len(xs); i++ {
		for j := i; j > 0 && rank[xs[j-1]] > rank[xs[j]]; j-- {
			xs[j-1], xs[j] = xs[j], xs[j-1]
		}
	}
}
