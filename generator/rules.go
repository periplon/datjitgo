package generator

import (
	"strings"

	"github.com/jmcarbo/datjitgo/core/value"
)

// evalRule evaluates a single rule expression against the given row. The
// parser used here is forgiving of the DSL shorthand `if X then Y` (used in
// rules.yaml) — it rewrites such expressions into regular boolean form
// before handing them to the general expression evaluator.
func evalRule(expr string, entity string, row *value.Object, data map[string][]*value.Object) (value.Value, error) {
	// Strip a leading "if … then …" — treat it as: not X or Y.
	if strings.HasPrefix(strings.TrimSpace(expr), "if ") {
		normalized := rewriteIfThen(expr)
		if normalized != "" {
			expr = normalized
		}
	}
	// Rewrite fully-qualified field paths like "User.age" → "age" when the
	// prefix matches the current entity.
	expr = strings.ReplaceAll(expr, entity+".", "")

	node, err := parseExpr(expr)
	if err != nil {
		return value.Null(), err
	}
	return evalExpr(node, evalEnv{row: row, data: data})
}

// rewriteIfThen transforms `if COND then THEN` into `not (COND) or (THEN)`.
// A missing `then` clause returns "".
func rewriteIfThen(src string) string {
	s := strings.TrimSpace(src)
	s = strings.TrimPrefix(s, "if ")
	idx := strings.Index(s, " then ")
	if idx < 0 {
		return ""
	}
	cond := strings.TrimSpace(s[:idx])
	then := strings.TrimSpace(s[idx+len(" then "):])
	return "not (" + cond + ") or (" + then + ")"
}
