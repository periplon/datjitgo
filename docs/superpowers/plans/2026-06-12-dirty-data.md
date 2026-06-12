# Plan: dirty-data injection (`@dirty`)

Spec: `docs/superpowers/specs/2026-06-12-dirty-data-design.md`
Branch: `claude/feat-dirty-data`

## Steps

1. **Recon.** Read `generator/engine.go` (post-pass insertion point after
   `enforceDatasetRules`), `generator/field.go` (decorator access patterns),
   how `_meta @llm(...)` is parsed (`parser/yaml.go`, `findLLM` in
   generator) — entity-level `@dirty` must ride the same mechanism.
2. **Config resolution** (`generator/dirty.go`): per-entity dirty plan —
   field-level decorator > entity meta > `GenerateOptions.DirtyRate`;
   static eligibility + exemptions per spec §4; kind-pool filtering by
   declared type.
3. **Operators** (`generator/dirty_ops.go`): typo/case/whitespace/null/
   format_mix/duplicate as pure (value, substream) functions; table-driven
   unit tests with fixed substreams.
4. **Engine hook**: post-pass per entity per spec §5 — empty plan ⇒ zero
   derivations/draws. Uniqueness re-check fallback for `@unique` fields.
5. **Options/CLI**: `ports.GenerateOptions.DirtyRate` (additive),
   `datjit.WithDirtyRate`, `generate --dirty-rate`; validation (0 ≤ r ≤ 1).
6. **Fixture + golden**: `testdata/fixtures/dirty_data.yaml` with its
   golden (generated once via the documented update path for NEW files
   only); determinism + uniqueness-preservation tests; CLI flag test.
7. **Docs**: README DDL summary + CHANGELOG Unreleased; godoc.
8. **Gate**: `make ci` green; `git diff testdata/golden` shows ONLY the new
   golden file.

## Definition of done

- Spec §2–§5 implemented; default path provably draw-free (existing goldens
  byte-identical); `make ci` green; no new deps.
