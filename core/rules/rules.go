// Package rules contains shared rule-expression helpers used by validators
// and generators.
package rules

import "strings"

// NormalizeExpr rewrites DSL shorthand into the boolean expression form used
// by the expression evaluator. Plain expressions are returned unchanged.
func NormalizeExpr(src string) string {
	if normalized := normalizeIfThen(src); normalized != "" {
		return normalized
	}
	return src
}

func normalizeIfThen(src string) string {
	s := strings.TrimSpace(src)
	if !strings.HasPrefix(s, "if ") {
		return ""
	}
	s = strings.TrimPrefix(s, "if ")
	idx := strings.Index(s, " then ")
	if idx < 0 {
		return ""
	}
	cond := strings.TrimSpace(s[:idx])
	then := strings.TrimSpace(s[idx+len(" then "):])
	return "not (" + cond + ") or (" + then + ")"
}
