# Dirty-data injection: `@dirty`

Status: approved for autonomous implementation
Date: 2026-06-12
Implements: killer-list #3 (R2-2) in `docs/enhancements-round2.md`.

## 1. Goal

A seeded corruption layer that makes generated data *realistically messy* —
typos, case mangling, stray whitespace, unexpected nulls, mixed timestamp
formats, near-duplicate rows — deterministically: the same schema + seed
produce the same mess. Schemas that do not opt in are byte-identical to
today (zero new RNG draws on the default path; existing goldens must not
change).

## 2. DDL surface

Field-level decorator:

```
email: email @unique @dirty(rate=0.1, typo, whitespace)
name:  person.full @dirty            # rate defaults to 0.05, kinds default to typo, case, whitespace
```

- `rate=<float in [0,1]>` (ArgKV, default 0.05) — per-row probability that
  this field's value is corrupted.
- Remaining bare-ident args name the corruption kinds to draw from
  (uniformly). Default kind set: `typo, case, whitespace`.

Entity-level meta decorator (applies to every *eligible* field of the
entity that has no field-level `@dirty` of its own; field-level wins):

```
User:
  _meta: "@dirty(rate=0.02, typo, case, null)"
```

(however entity `_meta` decorators are already expressed in this DDL — match
the existing `_meta @llm(...)` mechanism exactly; check `parser/yaml.go` and
fixtures for the concrete form.)

Generation-level dial: `GenerateOptions.DirtyRate float64` (new field,
additive) + `datjit.WithDirtyRate(rate float64)` option + CLI
`--dirty-rate R` on `generate`. When > 0, acts like an entity-level
`@dirty(rate=R)` with default kinds for every entity that has no own config.
Precedence: field decorator > entity meta > global option.

## 3. Corruption kinds (v1)

All operators are pure functions of `(value, substream)`:

| Kind | Eligible value kinds | Effect |
|---|---|---|
| `typo` | string (len ≥ 2) | one of: swap two adjacent chars, drop a char, double a char — position and op chosen from the substream |
| `case` | string (with letters) | one of: UPPER, lower, sWAP first letter case |
| `whitespace` | string | one of: leading space, trailing space, internal double space (at a seeded word boundary; no-op if single word and then fall back to trailing) |
| `null` | any | value becomes null |
| `format_mix` | time | re-render the timestamp as a *string* in one of: `01/02/2006`, `2006-01-02 15:04:05`, `Jan 2, 2006` (type intentionally degrades to string — that is the mess being simulated) |
| `duplicate` | — (entity-level only) | row i becomes a near-copy of row i−1: all fields copied, then 1–2 seeded eligible fields re-corrupted with `typo`/`whitespace` (no-op on row 0) |

A kind that is not applicable to the field's generated value kind is skipped
at *selection* time: the kind pool for a field is filtered statically by the
field's declared type where possible, and dynamically (null/with non-string
values) the corruption is a no-op — the RNG draws still happen so the
decision stream is stable regardless of the value's runtime kind.

## 4. Safety exemptions

Never corrupted, even under entity/global config (a field-level `@dirty`
explicitly placed on such a field wins — the user asked for it):

- `@primary` fields, `@auto`/sequence fields,
- reference fields (`->X`, `<->X`, unions of references) and synthetic
  discriminator fields,
- `@unique` fields under `null`/`duplicate` kinds (corruption must not
  violate the uniqueness the schema promised: `typo`-class corruption of a
  unique field re-checks the uniqueness set and falls back to the original
  value on collision).

`@dirty` on `@internal` fields is pointless (they are stripped) — validation
warning not required, just document.

## 5. Engine integration

A post-pass per entity, after the row loop and `enforceDatasetRules`, before
`ds.Entities.Set` (in `Engine.Generate`):

- Build the entity's dirty plan once (per-field config with precedence
  resolved; static eligibility).
- If the plan is empty → return immediately (**zero substream derivations,
  zero draws** — this is what keeps default output byte-identical).
- Otherwise derive `dirtySub := entSub.Substream("dirty")` and iterate rows
  in index order, fields in declaration order; for each configured field:
  draw `dirtySub.Float()`; if < rate, choose kind index via
  `dirtySub.IntN`, apply operator with `dirtySub` for its internal choices.
- `duplicate` is evaluated first per row (one draw per row when configured
  at entity level), because it replaces the whole row before field-level
  corruption.
- Dirtying happens *after* rule enforcement by design: dirty data may
  violate rules — that is the point. Document this in the spec + godoc.

## 6. Out of scope (v1)

- `_dirty_report` companion output (needs an output-shape spec; follow-up).
- Encoding-artifact (mojibake) kind, numeric jitter kind.
- Cross-field correlated corruption.

## 7. Tests

- Operator unit tests (table-driven, fixed substream).
- Parser/config tests: decorator forms, precedence (field > meta > global),
  default kinds.
- New fixture `testdata/fixtures/dirty_data.yaml` + golden (uses fixed seed;
  exercises field-level, entity-level, null, format_mix, duplicate).
- Determinism: generate twice → identical; **all existing goldens unchanged**
  (`make ci` proves it; never run `make test-update` for existing files).
- Uniqueness preservation test: `@unique @dirty(typo)` field stays unique.
- CLI flag test for `--dirty-rate`.
