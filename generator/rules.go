package generator

import (
	"strings"

	corerules "github.com/periplon/datjitgo/core/rules"
	"github.com/periplon/datjitgo/core/value"
)

// evalRule evaluates a single rule expression against the given row. The
// parser used here is forgiving of the DSL shorthand `if X then Y` (used in
// rules.yaml) — it rewrites such expressions into regular boolean form
// before handing them to the general expression evaluator.
func evalRule(expr string, entity string, row *value.Object, data map[string][]*value.Object, pk map[string]string) (value.Value, error) {
	expr = corerules.NormalizeExpr(expr)
	// Rewrite fully-qualified field paths like "User.age" → "age" when the
	// prefix matches the current entity.
	expr = strings.ReplaceAll(expr, entity+".", "")

	node, err := parseExpr(expr)
	if err != nil {
		return value.Null(), err
	}
	return evalExpr(node, evalEnv{row: row, data: data, pk: pk})
}
