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

// Rule is a cross-entity constraint expressed as a raw expression string
// that the engine parses and evaluates.
type Rule struct {
	Expr        string
	Severity    RuleSeverity
	Probability float64
}
