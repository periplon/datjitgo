# Plan: time-series & stateful sequences

Spec: `docs/superpowers/specs/2026-06-12-time-series-design.md`
Branch: `claude/feat-time-series`

## Steps

1. **Recon.** `generator/engine.go` (entity loop, where the stateful pass
   slots in before `enforceDatasetRules`), `generator/field.go` (derived
   placeholder pattern to copy for stateful fields), `generator/timeutil.go`
   (existing time parsing), enum resolution (`enumDefs` in state).
2. **Config parsing** (`generator/stateful.go`): parse `@series`/`@walk`/
   `@chain` decorator args into typed configs (durations incl. `Nd` days,
   RFC3339/date starts, transition-table string); unit tests incl. error
   cases.
3. **Validation**: wire config errors + type-compatibility checks (series on
   time fields, walk on numerics, chain states ⊆ enum variants, no stateful
   on coherence members/references/compound elements) into the existing
   validation path (find where field-level validation lives — root
   `validate.go` and helpers).
4. **Stateful pass**: per-entity post-pass per spec §3 with per-field
   substreams (`series:`/`walk:`/`chain:` + field name); position-stable
   draw counts (absorbing chain states still draw; jitter=0 series draw-free
   per row).
5. **Field skip**: stateful fields get null placeholders in `generateRow`
   (alongside the derived/compute branch).
6. **Fixture + golden**: `testdata/fixtures/time_series.yaml` (all three
   decorators + control entity) with new golden; property assertions
   (monotone series, clamped walk, legal chain edges); determinism test.
7. **Docs**: README DDL summary + CHANGELOG Unreleased; godoc on configs.
8. **Gate**: `make ci` green; only the new golden added.

## Definition of done

- Spec §2–§4 implemented with validation; zero draws for entities without
  stateful fields (existing goldens byte-identical); `make ci` green; no new
  deps.
