package generator

import (
	"github.com/jmcarbo/datjitgo/core/model"
	"github.com/jmcarbo/datjitgo/core/value"
)

// applyDerived evaluates every `@derived(expr)` decorator on the entity and
// stores the resulting value in the row.
func (e *Engine) applyDerived(entity *model.Entity, row *value.Object, st *generationState) error {
	env := evalEnv{row: row, data: st.generated}
	var firstErr error
	entity.Fields.Each(func(name string, f *model.Field) bool {
		d := model.FindDecorator(f.Decorators, "derived")
		if d == nil || len(d.Args) == 0 {
			return true
		}
		src := decoratorLiteralString(d.Args[0])
		node, err := parseExpr(src)
		if err != nil {
			firstErr = err
			return false
		}
		v, err := evalExpr(node, env)
		if err != nil {
			firstErr = err
			return false
		}
		row.Set(name, v)
		return true
	})
	return firstErr
}

// applyDefaultChain runs through each field's default_chain (if set), taking
// the first non-null source. `When` gates the whole chain; `Fallback` is used
// when every source resolves to null.
func (e *Engine) applyDefaultChain(entity *model.Entity, row *value.Object, st *generationState) error {
	env := evalEnv{row: row, data: st.generated}
	var firstErr error
	entity.Fields.Each(func(name string, f *model.Field) bool {
		if f.DefaultChain == nil {
			return true
		}
		spec := f.DefaultChain
		if spec.When != "" {
			node, err := parseExpr(spec.When)
			if err != nil {
				firstErr = err
				return false
			}
			cond, err := evalExpr(node, env)
			if err != nil {
				firstErr = err
				return false
			}
			if !truthy(cond) {
				return true
			}
		}
		picked := value.Null()
		for _, src := range spec.Sources {
			v := resolvePath(src, env)
			if v.Kind != value.KindNull {
				picked = v
				break
			}
		}
		if picked.Kind == value.KindNull && spec.Fallback != "" {
			node, err := parseExpr(spec.Fallback)
			if err != nil {
				firstErr = err
				return false
			}
			v, err := evalExpr(node, env)
			if err != nil {
				firstErr = err
				return false
			}
			picked = v
		}
		row.Set(name, picked)
		return true
	})
	return firstErr
}

// applyCompute evaluates compute branches — first matching `When` wins,
// otherwise the branch without a `When` (else) applies.
func (e *Engine) applyCompute(entity *model.Entity, row *value.Object, st *generationState) error {
	env := evalEnv{row: row, data: st.generated}
	var firstErr error
	entity.Fields.Each(func(name string, f *model.Field) bool {
		if len(f.Compute) == 0 {
			return true
		}
		for _, b := range f.Compute {
			if b.When != "" {
				node, err := parseExpr(b.When)
				if err != nil {
					firstErr = err
					return false
				}
				cond, err := evalExpr(node, env)
				if err != nil {
					firstErr = err
					return false
				}
				if !truthy(cond) {
					continue
				}
			}
			node, err := parseExpr(b.Value)
			if err != nil {
				firstErr = err
				return false
			}
			v, err := evalExpr(node, env)
			if err != nil {
				firstErr = err
				return false
			}
			row.Set(name, v)
			return true
		}
		return true
	})
	return firstErr
}
