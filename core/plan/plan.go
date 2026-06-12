// Package plan contains pure domain planning helpers.
package plan

import (
	"cmp"
	"slices"
	"strings"

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
		// The error Kind already renders a "cyclic dependency: " prefix, so the
		// message carries only the exemplar path.
		msg := "cycle detected in entity references"
		if cycles := Cycles(doc); len(cycles) > 0 {
			msg = strings.Join(cycles[0], " -> ")
		}
		return nil, &errors.Error{
			Kind:    errors.KindCyclicDependency,
			Message: msg,
		}
	}
	return result, nil
}

// Cycles returns one exemplar cycle path per strongly-connected component that
// contains at least one cycle. Each path is a list of entity names ending where
// it began, e.g. ["A","B","A"]. Reference edges whose target is "self" or the
// hosting entity are excluded, matching Entities, so valid self-references do
// not appear as cycles. Paths are deterministic: entities are visited in
// document order. Targets that are not declared entities are ignored.
func Cycles(doc *model.Document) [][]string {
	order := doc.Entities.Keys()
	rank := make(map[string]int, len(order))
	for i, n := range order {
		rank[n] = i
	}

	// Build forward adjacency (from -> sorted targets), document-ordered.
	adj := make(map[string][]string, len(order))
	for _, name := range order {
		e, _ := doc.Entities.Get(name)
		seen := make(map[string]struct{})
		CollectRefs(e.Fields, seen)
		targets := make([]string, 0, len(seen))
		for target := range seen {
			if target == "self" || target == name {
				continue
			}
			if _, ok := doc.Entities.Get(target); !ok {
				continue
			}
			targets = append(targets, target)
		}
		slices.SortFunc(targets, func(a, b string) int { return cmp.Compare(rank[a], rank[b]) })
		adj[name] = targets
	}

	// DFS for back-edges. The first back-edge encountered for an SCC yields its
	// exemplar path; once a node is fully explored it cannot start a new cycle.
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(order))
	var stack []string
	var cycles [][]string
	reported := make(map[string]struct{})

	var dfs func(n string)
	dfs = func(n string) {
		color[n] = gray
		stack = append(stack, n)
		for _, t := range adj[n] {
			switch color[t] {
			case white:
				dfs(t)
			case gray:
				// Back-edge: extract the path from t down to n, then close it.
				if _, done := reported[t]; !done {
					start := 0
					for i, s := range stack {
						if s == t {
							start = i
							break
						}
					}
					path := append([]string(nil), stack[start:]...)
					path = append(path, t)
					cycles = append(cycles, path)
					reported[t] = struct{}{}
				}
			}
		}
		stack = stack[:len(stack)-1]
		color[n] = black
	}

	for _, n := range order {
		if color[n] == white {
			dfs(n)
		}
	}
	return cycles
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
