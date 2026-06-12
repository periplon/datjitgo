# Plan: generation profiles (edge / hostile)

Spec: `docs/superpowers/specs/2026-06-12-generation-profiles-design.md`
Branch: `claude/feat-profiles`

## Steps

1. **Recon.** `generator/field.go` (`generateField` post-shaping point),
   `generator/engine.go` (how locale is resolved/threaded — decide
   receiver-field vs `generationState` for profile and note the choice),
   `core/ports` GenerateOptions, `options.go` (option validation patterns),
   `datjit_fixtures_test.go` (golden harness capabilities).
2. **Boundary tables** (`generator/profile.go`): edge + hostile constant
   tables per value class, range-aware numeric/time bounds; unit tests.
3. **Eligibility**: static per-field predicate per spec §3; `@profile`
   opt-out decorator; unit-test the matrix.
4. **Hook**: substitution draws in `generateField` per spec §4 — two draws
   per eligible field only when profile active; never conditional on value.
5. **Options/CLI**: `ports.GenerateOptions.Profile`, `datjit.WithProfile`
   (validated), `generate --profile`.
6. **Goldens**: new fixture schema (covered by default harness) + dedicated
   `WithProfile` golden tests (`profile_edge.json`, `profile_hostile.json`)
   per spec §5; writer-robustness tests (csv re-parse, sql escaping, json/
   yaml/ndjson round-trip) over the hostile dataset.
7. **Docs**: README (flag table + short negative-testing note), CHANGELOG.
8. **Gate**: `make ci` green; existing goldens byte-identical.

## Definition of done

- realistic = today, bit-for-bit; edge/hostile deterministic per seed;
  writers emit well-formed output for every hostile value; `make ci` green;
  no new deps.
