package generator

import (
	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/ports"
	"github.com/periplon/datjitgo/core/value"
)

// generateReference resolves a reference field against already-generated rows
// in state. Phase-1 semantics:
//   - single belongs-to (`-> Target`) picks a primary-key value uniformly.
//   - optional ref (`?`) or explicit `@null_rate(p)` may produce null.
//   - self-reference picks from the host entity's prior rows (or null).
//   - has-many (`->[Target]`) and many-to-many (`<->Target`) produce a list
//     of picks; cardinality comes from `@count` (fallback 0..4).
func (e *Engine) generateReference(entity *model.Entity, f *model.Field, t model.Reference, st *generationState, rng ports.Randomizer) value.Value {
	// @null_rate is handled in generateField already; optional refs also
	// have a small nullish chance.
	if t.Optional && rng.Float() < 0.15 {
		return value.Null()
	}

	target := t.Target
	if target == "self" {
		target = entity.Name
	}
	rows := st.generated[target]
	pkField := st.pk[target]

	if t.Many || t.ManyToMany {
		lo, hi := countRange(f.Decorators, 0, 4)
		if hi > len(rows) {
			hi = len(rows)
		}
		if lo > hi {
			lo = hi
		}
		n := lo
		if hi-lo > 0 {
			n += int(rng.IntN(int64(hi - lo + 1)))
		}
		if n == 0 {
			return value.List(nil)
		}
		// Pick without replacement using a Fisher-Yates prefix.
		idxs := make([]int, len(rows))
		for i := range idxs {
			idxs[i] = i
		}
		rng.Shuffle(len(idxs), func(i, j int) { idxs[i], idxs[j] = idxs[j], idxs[i] })
		picks := make([]value.Value, 0, n)
		for i := 0; i < n; i++ {
			row := rows[idxs[i]]
			picks = append(picks, referenceValue(row, pkField))
		}
		return value.List(picks)
	}

	// Self-ref with an optional marker prefers null when no rows exist.
	if len(rows) == 0 {
		if t.Optional {
			return value.Null()
		}
		return value.Null()
	}
	idx := int(rng.IntN(int64(len(rows))))
	return referenceValue(rows[idx], pkField)
}

// firstField returns the first field in an ordered object (assumed primary key).
// Falls back to Null if the row has no fields.
func firstField(row *value.Object) value.Value {
	keys := row.Keys()
	if len(keys) == 0 {
		return value.Null()
	}
	v, _ := row.Get(keys[0])
	return v
}
