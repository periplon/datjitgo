# Unqualified rules checked against every entity → spurious violations

Status: done
Branch: `fix/rule-entity-scoping`
Date: 2026-06-02

## Problem (rationalization)

`ruleTargetsEntity` (`generator/engine.go`) decides whether a cross-entity rule
applies to the entity currently being generated:

```go
func ruleTargetsEntity(expr, entity string) bool {
	if strings.Contains(expr, entity+".") { return true }
	if !strings.Contains(expr, ".")       { return true }  // bare field → EVERY entity
	return strings.HasPrefix(expr, entity+" ")
}
```

A rule with a **bare** (unqualified) field — e.g. `age >= 18 @strict` — contains
no `.`, so the second branch returns `true` for **every** entity. The rule is
then enforced against entities that do not even have an `age` field.

For such an entity, `resolvePath("age")` returns `Null` (confirmed in
`generator/expr.go`), so `null >= 18` is falsey → the row is treated as a rule
violation. For `@strict` rules `enforceRowRules` retries the row up to 10 times
and then fails generation with `KindRuleViolated` — **retry exhaustion** on an
entity the rule was never meant to constrain.

(`@warn` dataset rules use a separate inline filter that requires an `Entity.`
prefix, so bare `@warn` rules are silently never checked — the opposite
mis-scoping. This fix routes both paths through one scoping function.)

## Fix

Scope rules by **field membership** instead of textual `.` presence:

- A rule that names entities explicitly (`Entity.field`) applies only to those
  named entities (unchanged behaviour).
- An unqualified rule (bare field names) applies to an entity **iff that entity
  declares every referenced field** — never to entities lacking them.

Implementation (`generator/rulescope.go`):
- `collectPaths` walks the parsed expression AST collecting path nodes.
- `ruleReferences(expr, entityNames)` → (entity-qualified set, bare field list).
- `ruleTargetEntities(expr, doc)` → set of entity names the rule targets.
- Precompute per-rule target sets once in `Generate` (`generationState.ruleScope`,
  aligned with `doc.Rules` by index); `enforceRowRules` / `enforceDatasetRules`
  consult the set instead of `ruleTargetsEntity`.

## Definition of Done

- [x] Root cause documented (this file).
- [x] Bare-field rules apply only to entities that declare the referenced fields.
- [x] Entity-qualified rules unchanged (named entities only).
- [x] Bare `@warn` rules now evaluated on matching entities (consistency) — both
      enforcement paths route through `computeRuleScope`.
- [x] Regression tests: bug fix (`TestBareRuleScopedToEntitiesWithField`),
      enforcement preserved (`TestBareRuleStillEnforcedOnMatchingEntity`), and
      scoping resolver unit test (`TestRuleTargetEntitiesScoping`). Negative
      control: old blanket matching → bare rule fails on the entity lacking the
      field.
- [x] Determinism preserved — scoping only filters which rules run per entity;
      RNG sequence unchanged. Race tests + goldens green.
- [x] All existing golden fixtures unchanged (every fixture rule is qualified).
- [x] `make ci` green (gofmt, lint, race tests, fixtures, build).
- [x] Self-review + independent review pass — see below.

## Ledger

| # | Step | Result |
|---|------|--------|
| 1 | Confirmed claim 1: bare rule → `ruleTargetsEntity` true for all; missing field → null → falsey → retry exhaustion | confirmed |
| 2 | Created worktree `../datjitgo-rule-scope`, branch `fix/rule-entity-scoping` from main | done |
| 3 | Wrote rationalization + DOD + ledger | done |
| 4 | Added `generator/rulescope.go` (collectPaths, ruleReferences, ruleTargetEntities, computeRuleScope) | done |
| 5 | `generationState.ruleScope`; replaced `ruleTargetsEntity` + inline @warn filter with scope lookups; removed dead func | done |
| 6 | Added 3 regression tests | pass |
| 7 | Negative control: old blanket matching → bare rule fails on Team (no score) | confirmed guard |
| 8 | `make ci` green; goldens unchanged | green |
| 9 | Independent reviewer pass | done |
| 10 | Hardened parse-error path: malformed expr → scope to all entities (fail loud, not silent skip); added unit case | done |

## Review notes

Independent reviewer (separate lane) raised:
- **Parse-error silent skip** — addressed: `ruleReferences` now returns `ok`;
  a malformed expression scopes the rule to every entity so it fails loudly
  during evaluation (matching pre-fix behaviour). `Validate` also parses every
  rule expr up front, so this only matters when `Engine.Generate` is called
  without validation.
- **Determinism with bare `@probability` rules** — qualified rules scope
  identically to before, so their `rng.Float()` consumption is unchanged
  (goldens confirm). Only newly-fixed bare rules differ, and those had no prior
  stable contract. No regression for existing or qualified schemas.
- `collectPaths` traverses all node kinds via `children` (binary/unary/in/func);
  `exprIn` list elements are children and are walked. Confirmed.
