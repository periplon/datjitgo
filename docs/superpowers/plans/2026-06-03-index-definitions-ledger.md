# Ledger — Index Definitions (manual + automated)

Branch: `feat/index-definitions` · Worktree: `../datjitgo-index-defs` · Started: 2026-06-03

Unattended build. One row per step. Status: ☐ todo · ◐ in-progress · ☑ done · ✗ blocked.

## Definition of Done (DOD)
- [x] Spec drafted (expanded mapping syntax) + reviewed clean
- [x] `model.Index` + `Entity.Indexes`
- [x] Parser: `_indexes` expanded-mapping form, key reserved
- [x] Inference pass (gated), tagged `Source: "inferred"`
- [x] Validation: field-exists, dup-name, empty-fields
- [x] `WriteOptions.SQLIndexes` enum + `--sql-indexes` CLI flag
- [x] SQL emit `writeIndexes` with dialect matrix
- [x] Tests: parser, validate, sql ×3 dialects, inference
- [x] Fixture YAML + json golden
- [x] Docs: CHANGELOG, DDL section
- [x] `make ci` green
- [x] Final reviewer pass clean (DIFF CLEAN; 1 latent edge hardened)
- [x] Draft PR opened

## Log
| # | step | status | notes |
|---|------|--------|-------|
| 1 | create worktree + branch | ☑ | `feat/index-definitions` off main |
| 2 | gather impl context | ☑ | model/parser/sql/validate/normalize mapped |
| 3 | draft spec | ☑ | expanded mapping syntax, 10 sections + DOD |
| 4 | spec review #1 | ☑ | reviewer: 0 blocking bugs, 4 clarifications |
| 5 | patch spec (4 findings) | ☑ | WriteOpts facade, reserved key, validate-dedup order, parse-time key rejection |
| 6 | spec re-review #2 | ☑ | SPEC CLEAN; 2 clarity notes folded |
| 7 | impl model+parser | ☑ | model.Index, Entity.Indexes, parseIndexes; build clean |
| 8 | impl inference+validate | ☑ | normalizeIndexes (gated, deduped), checkIndexes; build clean |
| 9 | impl sql emit+opts+cli | ☑ | writeIndexes dialect matrix, clampIdent, SQLIndexes opt, --sql-indexes; build+vet clean, no regression |
| 10 | tests+fixture+docs | ☑ | sql_index_test, parser/index_test, facade+internal tests; indexes.yaml+golden; CHANGELOG; CLAUDE.md DDL+CLI |
| 11 | CLI smoke | ☑ | manual/auto/none × pg/mysql verified end-to-end; dedup + USING-before-ON + WHERE-drop confirmed |
| 12 | DOD gate make ci | ☑ | lint 0 issues, race tests green, fixtures match, build ok |
| 13 | final diff review | ☑ | reviewer: DIFF CLEAN; hardened clampIdent latent edge |
| 14 | commit + push + draft PR | ☑ | see PR link below |
