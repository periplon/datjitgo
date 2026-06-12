# Killer-Features Program Ledger — 2026-06-12

Program: implement killer-list items **#8, #2, #3, #9, #12** (from
`docs/enhancements-round2.md`, "The killer list") as five separate
auto-merged PRs. Orchestrated autonomously; each PR carries its design
spec, implementation plan, and an updated copy of this ledger.

## Scope

| PR | Item | Feature | Branch | Status |
|---|---|---|---|---|
| A | #8 (R1-2) | Schema introspection: export, diff, deps | `claude/feat-schema-introspection` | in-progress |
| B | #2 (R2-1) | MCP server (`datjit mcp`) | `claude/feat-mcp-server` | in-progress |
| C | #3 (R2-2) | Dirty-data injection (`@dirty`) | `claude/feat-dirty-data` | todo |
| D | #9 (R2-4) | Time-series & stateful sequences | `claude/feat-time-series` | todo |
| E | #12 (R2-5) | Edge-case / hostile generation profiles | `claude/feat-profiles` | todo |

(#9 was added to the program by user request after the initial 2/3/8/12
selection.)

## Method

- **Waves.** Wave 1 = A + B in parallel (additive, no generator surface;
  delegated to Opus subagents per their plans). Wave 2 = C, D, E (all touch
  the generation engine; staggered to avoid cross-PR conflicts in
  `generator/engine.go`).
- **Adversarial loop.** Every implementation gets an independent adversarial
  review pass (separate subagent attacking the diff for correctness,
  determinism leaks, invariant violations, API breaks) before the PR is
  opened; findings fixed and re-reviewed until clean.
- **Gates.** `make ci` green locally before every push; golden fixtures must
  not drift except where a new fixture is intentionally added with its
  golden. PRs auto-merge on green CI.
- **Ledger conflicts.** This file is updated in every PR; later branches
  rebase onto `main` before opening their PR so the ledger merges linearly.

## Invariants honored

- Determinism: schemas not using a new feature produce byte-identical output
  (zero new RNG draws on the default path); all new randomness routes
  through `core/value`-seeded substreams.
- Public API stability: all additions are additive; no renames/moves of
  exported identifiers in `datjit`, `core/*`, `runtime`, `datjittest`.
- Hexagonal direction: `core/*` imports no adapter; new packages depend on
  the root facade or `core/*` only, per their layer.

## Baseline

- `main` @ 47302ce (merge of PR #11), `make ci` green (verified at program
  start), Go 1.26.2.

## Decisions

- **D1.** Single ledger file updated per-PR (rebase-resolved) rather than
  per-feature ledgers — user asked for "a ledger".
- **D2.** Wave-1 features delegated to Opus subagents (lower complexity:
  read-only introspection; protocol façade). Wave-2 features use
  default-model subagents with tighter orchestration and deeper adversarial
  loops (they touch seeded generation paths).
- **D3.** MCP server is hand-rolled JSON-RPC 2.0 over stdio (newline-delimited)
  — no new module dependency in a near-stdlib module.
- **D4.** `@dirty` v1 ships without the `_dirty_report` companion output
  (output-shape change needs its own spec); noted as follow-up.
- **D5.** PR creation is sequential (A→B→C→D→E) even where implementation
  overlapped, so each rebases the ledger cleanly.

## Log

- 2026-06-12: Program started. Worktrees created for A and B off
  `origin/main` @ 47302ce. Baseline `make ci` verified green.
- 2026-06-12: Specs and plans written for all five features. Wave-1 (A, B)
  Opus implementation agents launched in parallel; wave-2 (C, D, E)
  default-model agents staggered behind them.
- 2026-06-12: B (MCP) implemented; adversarial review found 2 major
  (spurious responses to tools/call notifications; 64-bit seed collisions
  via float64 param decoding) + 4 minor findings. All fixed with regression
  tests (json.Number decoding, jsonrpc version check, batch rejection,
  -32602 for non-object arguments); gate green.
- 2026-06-12: A (introspection) implemented; adversarial review found a
  doubled "cyclic dependency:" prefix regression (+ too-weak test), a lossy
  renderReference for programmatic Optional+Many refs, fragile JSON-summary
  sniffing for flow-style YAML, and HTML-escaped JSON export. All fixed:
  exact-message + CLI cyclic tests, trailing-? rendering, decode-with-
  fallback sniffing, SetEscapeHTML(false). Gate green.
- 2026-06-12: D (time-series) implemented; adversarial review verdict
  MERGE-READY (no major findings; draw-position stability and zero-draw
  guarantee verified by code trace + 5× byte comparison). Minor follow-ups
  applied before PR: reject Inf/NaN chain probabilities; fixture comment
  accuracy; spec wording for derived-over-stateful (hard error, pre-existing
  evaluator behavior, not graceful null).
- 2026-06-12: C (@dirty) implemented (zero-draw default path, draw-budget
  content independence, @unique re-check); adversarial review launched.
- 2026-06-12: PR A opened (schema introspection, carries this ledger);
  automerge enabled.
