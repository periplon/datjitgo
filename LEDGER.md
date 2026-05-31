# Tech-Debt Sweep — Progress Ledger

Branch: `chore/tech-debt-sweep` · Worktree: `../datjitgo-techdebt`

Autonomous multi-agent tech-debt + refactoring sweep. Driven by expert analysis,
verified adversarially, implemented in safe per-item commits, gated by `make ci`.

## Baseline (main @ ac5b85c)

- `make ci`: **green**
- Coverage: **89.9%** of statements
- golangci-lint: 2.11.4
- Go: 1.26.2

## Invariants honored

- Determinism (same schema+seed => same output)
- Public API stability (`datjit`, `core/model`, `core/value`, `core/ports`, `core/errors`, `datjittest`, `runtime`)
- Hexagonal direction (core/* imports no adapter)
- Golden fixtures no drift

## Status legend

`todo` · `in-progress` · `done` · `skipped` · `deferred`

## Plan

Source: expert analysis workflow (17 agents, 8 lenses → adversarial verify → synthesis).
30 items deduped; 13 recommended for unattended auto-impl, 17 deferred (golden-drift
refactors / behavior changes / linter fallout needing supervision).

After my plan review I **demoted TD-06** (parser error typing): inner parser errors are
already `KindParse` via `locErr` at the yaml boundary; converting would double the
`"parse error:"` prefix in user messages for marginal gain. → deferred.

**Phase A — accepted safe wins (12):**

| ID | Title | Cat | Risk | Status | Commit |
|----|-------|-----|------|--------|--------|
| TD-01 | fmt.Sprintf for IO error messages | err | low | todo | |
| TD-04 | shared generate() pipeline helper | dup | low | todo | |
| TD-10 | ValidateString/ValidateFile helpers | api | low | todo | |
| TD-02 | slices.SortFunc in core/plan | idiom | low | todo | |
| TD-03 | requireDocument helper (csv/sql) | dup | low | todo | |
| TD-05 | KindGeneration for unknown-kind errs | err | low | todo | |
| TD-09 | actionable option validator messages | err | low | todo | |
| TD-07 | godoc Rows/ValueRequest fields | docs | low | todo | |
| TD-08 | godoc WithVolume(s)/Service introspection | docs | low | todo | |
| TD-11 | table tests for output encoders | test | low | todo | |
| TD-12 | runtime cancel/nil-type tests | test | low | todo | |
| TD-13 | corpus Update MkdirAll failure test | test | low | todo | |

**Phase B — attempt deferred behavior-preserving refactors (CI golden-gated; revert on drift, never `make test-update`):**

| ID | Title | Status |
|----|-------|--------|
| TD-18 | extract generateTuple/generateUnion | todo |
| TD-17 | evalFunc handler registry | todo |
| TD-15 | split evalBinary | todo |
| TD-19 | valueComparator consolidation | todo |
| TD-14 | semantic dispatch map | todo |
| TD-16 | coherence matchers | todo |

**Deferred — supervised only (genuine behavior/golden change or unbounded lint fallout):**
TD-06 (parser err typing, msg regression), TD-20 (SWIFT, golden drift), TD-21 (EntityRows, speculative),
TD-22 (cmd err wrap), TD-23 (YAML alias error, behavior change), TD-24 (Close errors),
TD-25 (cmd sort), TD-26 (nil Object contract), TD-27..TD-30 (linter enablement, repo-wide fallout).

## Log

- Worktree + branch created from `main` @ ac5b85c.
- Baseline `make ci` green, coverage 89.9%.
- Expert analysis workflow done (17 agents). Plan reviewed; TD-06 demoted.
