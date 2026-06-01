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
| TD-01 | fmt.Sprintf for IO error messages | err | low | done | 280df93 |
| TD-04 | shared generate() pipeline helper | dup | low | done | 280df93 |
| TD-10 | ValidateString/ValidateFile helpers | api | low | done | 280df93 |
| TD-02 | slices.SortFunc in core/plan | idiom | low | done | e83a3cb |
| TD-03 | requireDocument helper (csv/sql) | dup | low | done | afb921b |
| TD-05 | KindGeneration for unknown-kind errs | err | low | done | afb921b |
| TD-09 | actionable option validator messages | err | low | done | ba27b7f |
| TD-07 | godoc Rows/ValueRequest fields | docs | low | done | 67582ac |
| TD-08 | godoc WithVolume(s)/Service introspection | docs | low | done | 67582ac |
| TD-11 | table tests for output encoders | test | low | done | bf85856 |
| TD-12 | runtime cancel/nil-type tests | test | low | done | 46988dc |
| TD-13 | corpus Update MkdirAll failure test | test | low | done | 245716e |

**Phase B — attempt deferred behavior-preserving refactors (CI golden-gated; revert on drift, never `make test-update`):**

| ID | Title | Status | Commit |
|----|-------|--------|--------|
| TD-18 | extract generateTuple/generateUnion | done | b7e471d |
| TD-15 | split evalBinary | done | 3627fb4 |
| TD-17 | evalFunc handler registry | deferred | — |
| TD-19 | valueComparator consolidation | deferred | — |
| TD-14 | semantic dispatch map | deferred | — |
| TD-16 | coherence matchers | deferred | — |

Attempted the two pure code-motion extractions (TD-18, TD-15) — both verified
behavior-identical (build + generator tests + golden fixtures unchanged, reviewer
confirmed no RNG-order change). TD-14/16/17/19 left deferred: they replace clean
switches with map/interface registries — a speculative restructure, not debt
removal, and the experts flagged them for supervised golden review. Not worth the
determinism exposure unattended.

**Done in later supervised PRs:**
TD-27..TD-30 (linters enabled, PR #2) + legacy exclusion cleanup (PR #3).
TD-20 (SWIFT/BIC now generated from the seeded RNG instead of the hardcoded
`COBADEFFXXX`; goldens regenerated — only `bank_swift` fields changed).

**Verified non-issue (closed without change):**
TD-23 — undefined YAML anchors already fail at `yaml.Unmarshal` decode time
(`unknown anchor referenced`), so `nodeToAny`'s nil-alias branch is unreachable;
there was no silent-null masking to fix.

**Still deferred — supervised only:**
TD-06 (parser err typing, msg regression), TD-14/16/17/19 (speculative restructures),
TD-21 (EntityRows, speculative), TD-22 (cmd err wrap), TD-24 (Close errors),
TD-25 (cmd sort), TD-26 (nil Object contract).

## Outcome

- 14 items shipped across 11 commits (12 Phase-A safe wins + 2 Phase-B extractions).
- `make ci` green at every commit; goldens never regenerated.
- Reviewer pass (separate lane): no findings.
- Coverage: 89.9% → 90.1% (new encoder/runtime/corpus/validate tests added).

## Log

- Worktree + branch created from `main` @ ac5b85c.
- Baseline `make ci` green, coverage 89.9%.
- Expert analysis workflow done (17 agents). Plan reviewed; TD-06 demoted.
- Phase A (12 items) implemented, each its own commit, `make ci` green.
- Phase B: TD-18 + TD-15 extracted and golden-verified; rest held deferred.
- Independent reviewer pass on full diff: clean.
