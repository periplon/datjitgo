# Time-series & stateful sequences: `@series`, `@walk`, `@chain`

Status: approved for autonomous implementation
Date: 2026-06-12
Implements: killer-list #9 (R2-4) in `docs/enhancements-round2.md`.

## 1. Goal

Let an entity's rows form an ordered sequence: monotonic timestamps,
cumulative numeric walks, and Markov state progressions. All values are
drawn from per-entity seeded substreams in row-index order, so output is
deterministic, and schemas without these decorators are byte-identical to
today (zero new draws on the default path).

## 2. DDL surface

Three field decorators (parser already accepts arbitrary `@name(args)`; only
generator-side handling is new):

```
Metric:
  ts:     datetime @series(start=2026-01-01T00:00:00Z, interval=1m, jitter=10s)
  value:  float    @walk(start=100, drift=0.1, volatility=2.5, min=0, max=10000)
  status: status_t @chain("pending>shipped:0.8, pending>cancelled:0.2, shipped>delivered:1.0", start=pending)
```

### `@series` (date/datetime fields)

- `start=<RFC3339 or YYYY-MM-DD>` (required), `interval=<Go duration or Nd
  for days>` (required), `jitter=<duration>` (optional, default 0).
- Row i value: `start + i·interval + u·jitter` where `u` is drawn uniformly
  from [−1, 1) from the field's series substream. With `jitter=0` no draw
  is made for the row. Values remain monotonically non-decreasing as long as
  `jitter < interval/2` — document; do not enforce.

### `@walk` (int / float / decimal fields)

- `start=<number>` (required), `drift=<number per step>` (default 0),
  `volatility=<number>` (default 1), `min=`/`max=` (optional clamps).
- Row i value: `xᵢ = clamp(xᵢ₋₁ + drift + volatility·n)` with `n` a standard
  normal draw from the walk substream; `x₀ = start` exactly (no draw for
  row 0). Int fields round half-away-from-zero after clamping; float/decimal
  round to 2 places (matches existing numeric rounding).

### `@chain` (enum fields — inline or named enums)

- First arg: a quoted transition table `"from>to:prob, from>to:prob, …"`.
  Probabilities out of a state are normalized; a state with no outgoing
  transitions is absorbing (stays put, **no draw** once absorbed... no —
  draws must not depend on runtime state; see Determinism below: absorbing
  states still consume one uniform draw per row so the stream is
  position-stable).
- `start=<state>` (optional; default: first declared enum variant). Row 0 is
  `start` (no draw); row i is sampled from row i−1's outgoing distribution
  (one uniform draw per row, even when absorbing).
- Validation: every state mentioned must be a variant of the field's enum;
  probabilities must be > 0. Violations → validation error (KindValidation,
  entity+field populated).

## 3. Engine integration

Stateful fields are excluded from per-row generation (placeholder null, like
`@derived`) and filled by a **per-entity post-pass** that runs right after
the entity's row loop and *before* `enforceDatasetRules`:

- For each stateful field, derive one substream per field:
  `entSub.Substream("series:"+fname)` / `"walk:"+fname` / `"chain:"+fname`.
- Iterate rows in index order, computing values per §2. State lives in the
  pass, not in `generationState`.
- Because the pass uses its own substreams (not row RNG), rule-retry loops
  and attempt substreams cannot disturb the sequence.

Interactions, documented in godoc and spec:

- Row-level `@strict` rules and `@derived`/`@compute` expressions evaluate
  **before** the stateful pass and therefore see null for stateful fields in
  v1 (limitation; revisit if demanded). `@warn` dataset rules run after, so
  they see final values.
- `@unique`, `@null_rate`, `@dist`, `@range` combined with a stateful
  decorator: the stateful decorator wins; `@range` is honored via min/max
  clamping for `@walk` and ignored for `@series`/`@chain`. Other combos are
  ignored (validation warning not required; document).
- Stateful decorators on coherence-group members, list/map/tuple elements,
  or reference fields are a **validation error** (clear message).

## 4. Determinism rules

- Zero substream derivations and zero draws when an entity has no stateful
  fields → existing goldens unchanged.
- Draw counts must be a function of (row index, config) only — never of
  runtime values (e.g. absorbing chain states still draw) — so any future
  per-row parallelism or resumption stays sound.
- Same schema + seed → byte-identical output (covered by fixture golden).

## 5. Tests

- Unit: series arithmetic incl. day intervals and jitter bounds; walk clamp
  + rounding + int fields; chain normalization, absorbing states, start
  default; config parse errors.
- Validation: bad state names, chain on non-enum field, series on non-time
  field, walk on non-numeric field.
- New fixture `testdata/fixtures/time_series.yaml` + golden exercising all
  three decorators on one entity plus a no-decorator control entity.
- Property-style assertions in tests: series monotone (jitter <
  interval/2), walk within [min,max], chain transitions only along declared
  edges.
- Determinism: generate twice byte-identical; existing goldens untouched.

## 6. Out of scope (v1)

- Cross-entity timeline alignment, seasonality/trend terms beyond linear
  drift, per-group chains (e.g. per-user sessions), stateful fields feeding
  `@derived`/`@strict` rules.
