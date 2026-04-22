package model

// RuleSeverity controls how a rule violation is handled during generation.
type RuleSeverity int

const (
	// RuleStrict means the rule must hold for every generated row. The
	// generator retries a row up to a bounded number of attempts before
	// reporting a rule violation.
	RuleStrict RuleSeverity = iota
	// RuleProbabilistic means the rule should hold with probability P
	// during generation; it is a generation bias, not a hard constraint.
	RuleProbabilistic
	// RuleWarn logs a warning on violation but does not fail the run.
	RuleWarn
)

// RuleKind distinguishes the shape of the rule body. Regular rules carry
// a boolean expression in Expr; cross-row rules carry a YAML-encoded
// mapping in CrossRow and leave Expr empty.
type RuleKind int

const (
	RuleKindExpr RuleKind = iota
	RuleKindCrossRow
)

// Rule is a cross-entity constraint. Expression rules put a boolean
// expression in Expr; cross-row rules carry a YAML body in CrossRow. Both
// forms may attach an ErrorMessage for reporting on violation.
type Rule struct {
	Kind         RuleKind
	Expr         string
	ErrorMessage string
	CrossRow     string // raw YAML for cross_row rules
	Severity     RuleSeverity
	Probability  float64
}
