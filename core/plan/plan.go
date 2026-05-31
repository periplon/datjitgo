// Package plan contains pure domain planning helpers.
package plan

import (
	"cmp"
	"slices"

	"github.com/periplon/datjitgo/core/errors"
	"github.com/periplon/datjitgo/core/model"
)

// Entities returns the entity names of doc in topological order. Reference
// fields whose target is "self" or equal to the hosting entity are ignored.
// Ties are broken by document insertion order so output is deterministic.
func Entities(doc *model.Document) ([]string, error) {
	order := doc.Entities.Keys()
	rank := make(map[string]int, len(order))
	for i, n := range order {
		rank[n] = i
	}
	indeg := make(map[string]int, len(order))
	outEdges := make(map[string][]string, len(order))

	for _, name := range order {
		indeg[name] = 0
	}

	for _, name := range order {
		e, _ := doc.Entities.Get(name)
		seen := make(map[string]struct{})
		CollectRefs(e.Fields, seen)
		for target := range seen {
			if target == "self" || target == name {
				continue
			}
			if _, ok := doc.Entities.Get(target); !ok {
				continue
			}
			outEdges[target] = append(outEdges[target], name)
			indeg[name]++
		}
	}

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

		ready := make([]string, 0)
		for _, dep := range outEdges[n] {
			indeg[dep]--
			if indeg[dep] == 0 {
				ready = append(ready, dep)
			}
		}
		slices.SortFunc(ready, func(a, b string) int { return cmp.Compare(rank[a], rank[b]) })
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

// CollectRefs walks a field OrderedMap and records every referenced entity
// target into seen. List/Map/Tuple/Nullable/Union composites are descended.
func CollectRefs(fields *model.OrderedMap[string, *model.Field], seen map[string]struct{}) {
	fields.Each(func(_ string, f *model.Field) bool {
		WalkTypeRefs(f.Type, seen)
		return true
	})
}

// WalkTypeRefs records every reference target reachable from t into seen.
func WalkTypeRefs(t model.TypeExpr, seen map[string]struct{}) {
	switch v := t.(type) {
	case model.Reference:
		seen[v.Target] = struct{}{}
	case model.List:
		WalkTypeRefs(v.Element, seen)
	case model.Map:
		WalkTypeRefs(v.Key, seen)
		WalkTypeRefs(v.Value, seen)
	case model.Tuple:
		for _, e := range v.Elements {
			WalkTypeRefs(e, seen)
		}
	case model.Nullable:
		WalkTypeRefs(v.Inner, seen)
	case model.Union:
		for _, e := range v.Variants {
			WalkTypeRefs(e, seen)
		}
	}
}
