# Enhancement Roadmap — Round 2

Second brainstorming pass over `datjitgo`, building on
[`docs/enhancements.md`](enhancements.md) (round 1, 22 scored candidates).
Round 1 covered the obvious axes well — DX (`inspect`, `init`, watch mode),
library API (typed accessors, builder, errors), realism (skewed references,
temporal coherence, locales), formats (Parquet, SQL bulk), and scale
(streaming, parallelism). This round deliberately hunts the space round 1
did **not** touch, then merges both rounds into a single final ranking
("the killer list") at the end.

Method: three iterations.

- **Iteration A — broad sweep.** ~30 raw ideas generated across eight lenses
  (data-quality testing, time-series/eventing, AI-agent tooling, schema
  inference, database integration, web adoption, test ergonomics, format
  long-tail), each grounded against the actual code surface (semantic
  dispatch in `generator/semantic.go`, compute `when:` branches in
  `parser/yaml.go`, the seeded substream model in `generator/rng.go`,
  `ports.Writer` slots in `output/`).
- **Iteration B — dedupe + invariant screen.** Dropped ideas already covered
  by round 1 or by existing features (conditional fields exist via `@compute`
  `when:` branches; value-level generation exists via `runtime.GenerateValue`;
  Avro/custom formats are round-1 #19's writer registration). Screened the
  rest against the hard invariants: every new randomness source must route
  through `core/value`'s seeded RNG; no public-API breaks; no `core/*` →
  adapter imports. Items that strain those moved to the deferred section.
- **Iteration C — scoring + cut.** Survivors scored 1–5 on **User Value** and
  **Feasibility** (same rubric as round 1, `Score = value × feasibility`),
  small items collected as quick wins, and the combined cross-round top list
  assembled.

## Summary ranking (round-2 candidates)

| Rank | Enhancement | User Value | Feasibility | Score | Effort |
|---|---|---|---|---|---|
| R2-1 | MCP server: fixture generation for AI coding agents | 4.0 | 4.0 | 16.0 | M |
| R2-2 | Dirty-data injection (`@dirty`, seeded data-quality chaos) | 4.33 | 3.67 | 15.89 | M |
| R2-3 | Schema fitting from sample data (`datjit fit`) | 4.67 | 3.0 | 14.0 | L |
| R2-4 | Time-series & stateful sequence generation | 4.33 | 3.0 | 13.0 | L |
| R2-5 | Edge-case / hostile-input generation profiles | 3.67 | 3.33 | 12.22 | M |
| R2-6 | JSON-Schema export of the generated shape | 3.0 | 4.0 | 12.0 | S |
| R2-7 | Direct database seeding (`datjit seed --dsn`) + testcontainers helper | 3.67 | 3.0 | 11.0 | L |
| R2-8 | Graph output writers (Cypher / Gremlin) | 3.0 | 3.33 | 10.0 | M |
| R2-9 | WASM browser playground | 3.33 | 3.0 | 10.0 | L |
| R2-10 | Incremental / append-consistent generation (`--append`) | 3.0 | 3.0 | 9.0 | M |
| R2-11 | CDC / mutation event streams | 3.67 | 2.33 | 8.56 | L |

---

## R2-1. MCP server: fixture generation for AI coding agents

**Problem.** AI coding agents (Claude Code, Copilot, Cursor) constantly need
realistic test fixtures while writing tests, and today they either hallucinate
inline literals or shell out to the CLI with no schema feedback loop. datjit
is *exactly* the right tool for this — declarative, deterministic, validated —
but has no agent-native surface.

**Proposal.** Add `datjit mcp` (stdio JSON-RPC, Model Context Protocol)
exposing the existing `Service` pipeline as tools: `generate(schema, seed,
format, volume)`, `validate(schema)` returning structured diagnostics,
`inspect(schema)`, and `sample(semantic_type, count)` for one-off values.
No network listener — stdio only, so none of the attack-surface concerns that
sank round-1 #21 (HTTP service). The tool descriptions double as DDL
documentation the agent reads, which is itself an onboarding channel.

**Who benefits.** Every team using coding agents to write tests; IDE/agent
integrations; datjit adoption generally (agents become a distribution
channel).

**Effort.** M (thin layer over `Service`; the protocol is a small JSON-RPC
loop or one well-maintained SDK dependency).

**Risks.** MCP SDK dependency choice in a near-stdlib module (a hand-rolled
stdio JSON-RPC handler is feasible and dependency-free). Schema input arrives
as a string argument — size-limit it. Determinism is untouched: seeds are
request-scoped exactly as in the CLI.

**Score.** Value 4.0 · Feasibility 4.0 · **16.0**. *Highest-leverage adoption
move on either board: the implementation is a façade over code that already
exists, and the audience is growing monthly.*

## R2-2. Dirty-data injection (`@dirty`, seeded data-quality chaos)

**Problem.** Every generator (datjit included, plus faker/gofakeit/Mockaroo)
produces *suspiciously clean* data. Real pipelines break on the mess: typos,
inconsistent casing, stray whitespace, mixed date formats, near-duplicate
rows, unexpected nulls, mojibake. Teams testing dedupe logic, ETL hardening,
data-quality rules (Great Expectations-style), or ML robustness must corrupt
clean fixtures by hand — and then can't reproduce the corruption.

**Proposal.** Add a seeded corruption layer, applied as a deterministic
post-pass over generated values: field-level `@dirty(rate=0.05,
kinds=[typo, case, whitespace, null, format_mix])` and an entity/dataset-level
dial (`_meta @dirty(...)`, CLI `--dirty-rate`). Kinds: character typos
(swap/drop/double), case mangling, leading/trailing whitespace, format mixing
for dates/phones (e.g. `2026-06-12` vs `06/12/2026`), encoding artifacts,
duplicate-row injection with per-field jitter (the dedupe-testing killer
feature), and over-`null`ing beyond declared `null_rate`. Every corruption
decision draws from a per-row/per-field substream, so the same seed yields
the same mess — corrupted goldens are stable. Optionally emit a companion
`_dirty_report` (row, field, kind) so tests can assert their pipeline caught
everything that was injected.

**Who benefits.** Data engineers testing cleansing/validation pipelines,
dedupe/entity-resolution authors, ML teams testing robustness, anyone running
Great-Expectations-style contracts.

**Effort.** M.

**Risks.** Corruption operators must be pure functions of (value, substream)
to keep determinism; duplicate injection changes row counts so it must adjust
volumes predictably (replace, don't add, by default). Purely additive
decorator — schemas without `@dirty` are byte-identical to today.

**Score.** Value 4.33 · Feasibility 3.67 · **15.89**. *The strongest
differentiator on either board: deterministic mess is something no mainstream
generator offers, and datjit's seeded-substream architecture makes it almost
free.*

## R2-3. Schema fitting from sample data (`datjit fit`)

**Problem.** Round-1 #13 imports *schemas* (OpenAPI, JSON-Schema, SQL DDL),
but most teams start from *data*: a CSV export, an NDJSON dump, a sanitized
production sample. Hand-translating observed columns into DDL — and guessing
distributions — is the single biggest cost of adopting any schema-first
generator.

**Proposal.** Add `datjit fit <sample.csv|sample.ndjson> [--entity Name]
[--rows N] -o schema.yaml`: infer field names and primitive types; match
values against the semantic registry (email/phone/url/uuid/ipv4/iban/… are
already regex-recognizable from `generator/semantic.go`'s domains); detect
low-cardinality columns as weighted enums with observed frequencies; fit
numeric columns against the existing distribution family (normal, lognormal,
zipf, uniform — moment-based fitting, labeled with a goodness note); infer
`@range`, `null_rate` from observed nulls, and `@unique` from distinct
counts; emit annotated, hand-tunable YAML. Round-trip check: the emitted
schema must parse + validate before writing.

**Who benefits.** Every new user with existing data (i.e., almost all of
them); teams replacing production samples with synthetic equivalents for
privacy; load-test engineers matching production shape.

**Effort.** L.

**Risks.** Inference is heuristic and must say so in emitted comments;
distribution fitting needs honest fallbacks (uniform + observed range when
nothing fits). Read-only analysis — no determinism or API exposure. Pairs
naturally with round-1 #15 (PII): fitted schemas can flag likely-PII columns.

**Score.** Value 4.67 · Feasibility 3.0 · **14.0**. *Ties round 1's
multi-locale for highest raw value: it converts "I have data" into "I have a
datjit schema" in one command — the adoption funnel's missing step.*

## R2-4. Time-series & stateful sequence generation

**Problem.** Entities generate as independent rows; there is no way to
produce a *sequence* — metrics with trend/seasonality/noise, monotonic event
timestamps, account balances that accumulate, stock-price random walks,
status fields that follow a state machine (`pending → shipped → delivered`).
Observability, fintech, and ML teams currently post-process or write bespoke
generators.

**Proposal.** Three composable decorators, all evaluated in row-index order
within an entity's existing substream: `@series(start, interval, jitter)`
for monotonic timestamps; `@walk(start, drift, volatility, min, max)`
for cumulative numerics (balances, prices, sensor readings) with optional
`trend` / `seasonality(period, amplitude)` terms; and `@chain(Enum,
transitions={...})` for Markov state progressions over an existing enum.
Because rows already generate in deterministic index order from per-entity
substreams, stateful evaluation is a natural fit — the only ordering rule is
that stateful fields evaluate after independent ones, alongside the existing
derived/compute pass.

**Who benefits.** Observability/metrics teams seeding dashboards and alert
tests, fintech (ledgers, OHLC), IoT/sensor simulation, ML feature-pipeline
testing, anyone whose fixture is "events over time" rather than "rows".

**Effort.** L.

**Risks.** Introduces intra-entity row-order dependence — incompatible with
any future parallel-row generation (round-1 #22 is already bottom-ranked, so
this is a cheap trade) and with single-entity streaming only if state is
carried (it is, trivially). Decorators are additive; schemas without them are
untouched.

**Score.** Value 4.33 · Feasibility 3.0 · **13.0**. *Opens an entire fixture
category (time-ordered data) the tool currently can't express; the seeded
row-index architecture means no new determinism machinery, just disciplined
evaluation order.*

## R2-5. Edge-case / hostile-input generation profiles

**Problem.** Clean realistic data never exercises parser edges: empty
strings, max-length values, RTL and combining Unicode, emoji, homoglyphs,
boundary numerics (`0`, `-1`, `MaxInt64`, subnormal floats), epoch and
far-future dates, CSV-injection-shaped and SQL-metacharacter strings. Teams
fuzz-test inputs with separate tools that know nothing about their schema.

**Proposal.** A generation profile dial: `--profile realistic|edge|hostile`
(default `realistic`, today's behavior). `edge` biases each typed generator
toward its documented boundary set (deterministically sampled from a curated
per-type table); `hostile` adds adversarial-but-safe strings (quote/comma/
newline/NUL-adjacent payloads for CSV/SQL consumers, oversized values,
mixed-script homoglyphs). Per-field opt-out `@profile(realistic)` for keys
that must stay valid (primary keys, references). Generated SQL/CSV output
must itself stay well-formed — the hostility targets the *consumer* of the
data, which doubles as a self-test of datjit's own writers' escaping.

**Who benefits.** API input-validation testing, security-adjacent QA, fuzz
seed-corpus construction, anyone who has shipped a CSV parser.

**Effort.** M.

**Risks.** Boundary tables must be curated per type and versioned (changing
them changes output bytes under `edge`/`hostile` — pin per profile).
References and uniqueness must keep working under extreme values. Default
profile is byte-identical to today.

**Score.** Value 3.67 · Feasibility 3.33 · **12.22**. *Complements R2-2:
`@dirty` is realistic mess, profiles are adversarial extremes; together they
make datjit a negative-testing tool, not just a fixture filler.*

## R2-6. JSON-Schema export of the generated shape

**Problem.** Consumers of generated data (contract tests, API mocks,
downstream validators) have no machine-readable description of what datjit
will emit. Round-1 #2 exports a *signature for diffing*; nothing exports a
*validation contract*.

**Proposal.** `datjit schema export --format json-schema` emitting one JSON
Schema per entity (draft 2020-12): primitives map directly, semantic types
to `type: string` + `format`/`pattern` where standard (`email`, `uuid`,
`uri`, `ipv4`), enums to `enum`, `@range` to `minimum`/`maximum`, nullable
to type unions, references to the target key's type, weighted enums to plain
`enum` (weights have no schema equivalent — documented loss). The natural
inverse of round-1 #13's `FromJSONSchema` importer; sharing one mapping table
keeps the round trip honest.

**Who benefits.** Contract-testing teams, API-mock authors validating against
generated payloads, polyglot consumers who can't import the Go types.

**Effort.** S.

**Risks.** Mapping is lossy (distributions, coherence, rules don't project) —
emit them as `description` annotations. Pure read-only transform; no
determinism or API impact.

**Score.** Value 3.0 · Feasibility 4.0 · **12.0**. *Small, clean, and makes
two round-1 items (#2 export, #13 import) stronger by completing the loop.*

## R2-7. Direct database seeding (`datjit seed --dsn`) + testcontainers helper

**Problem.** The `sql` writer emits text that someone must still pipe through
`psql`/`mysql`, negotiate dialects, and order correctly. Integration-test
suites want one step: schema in, populated database out.

**Proposal.** `datjit seed <schema> --dsn postgres://… [--create-tables]
[--truncate] [--tx]`: connect via `database/sql` with pure-Go drivers (pgx,
go-sql-driver/mysql, modernc sqlite), create tables (reusing the SQL writer's
DDL emission), and insert in the existing topological FK order with batched
prepared statements. Pair with a `datjittest` helper that, given a
testcontainers-go container or any `*sql.DB`, seeds it from a schema —
turning "spin up Postgres with realistic data" into one line of test code.

**Who benefits.** Integration-test authors, local-dev environment seeding,
demo environments, QA seeding staging-like databases.

**Effort.** L (driver matrix and failure-mode handling are the bulk).

**Risks.** Three driver dependencies are heavy for a near-stdlib module —
isolate behind a build tag, a submodule (`seed/`), or keep drivers in
`cmd/datjit` only so library consumers don't inherit them. Live connections
mean retry/timeout/partial-failure semantics (wrap in one transaction by
default). Generation itself is unchanged, so determinism is untouched.

**Score.** Value 3.67 · Feasibility 3.0 · **11.0**. *High convenience, real
dependency cost; the testcontainers pairing is where it earns its keep.*

## R2-8. Graph output writers (Cypher / Gremlin)

**Problem.** datjit's reference graph (`->User`, `<->Tag`, polymorphic
unions, round-1 #5's planned skewed sampling) is exactly the data graph
databases need, but there is no graph-native output — Neo4j/Memgraph users
convert by hand.

**Proposal.** An `output.NewCypher()` writer (`-f cypher`): entities become
node `CREATE`s with labels, reference fields become relationship `CREATE`s
(polymorphic discriminators already name the target entity), `<->` becomes
undirected-pair convention. Batched `UNWIND` form for volume. A Gremlin
variant can follow the same internal traversal model if demand shows.

**Who benefits.** Graph-database teams (Neo4j, Memgraph, Neptune), fraud/
social/recommendation testers — the audience round-1 #5 generates realistic
topology for.

**Effort.** M.

**Risks.** Fits the existing `ports.Writer` slot with zero new dependencies
(text emission, like SQL). Escaping/identifier rules per Cypher spec need
golden coverage. No determinism impact.

**Score.** Value 3.0 · Feasibility 3.33 · **10.0**. *Niche but cheap, and it
compounds with skewed reference sampling: realistic topology + native format.*

## R2-9. WASM browser playground

**Problem.** Trying datjit requires installing a Go toolchain or trusting a
binary. The DDL's expressiveness — the actual selling point — is invisible
until then. Competing tools (Mockaroo, JSON-generator) win adoption on
zero-install playgrounds.

**Proposal.** Compile the parse→validate→generate→write pipeline to
`GOOS=js GOARCH=wasm` (pure Go, no cgo — it already qualifies), embed in a
static docs page: schema editor left, live JSON/CSV/SQL output right, seed
and volume controls, shareable permalink encoding the schema. Examples menu
seeded from `testdata/fixtures/`. Hosted on GitHub Pages from this repo.

**Who benefits.** Evaluation/adoption funnel, documentation (every DDL doc
example becomes runnable), bug reports (permalink = reproduction).

**Effort.** L (the Go side is small; the page and upkeep are the cost).

**Risks.** Binary size (full corpus embeds — acceptable for a docs page,
mitigable with a trimmed corpus build). Determinism is a feature here: the
same permalink shows everyone identical output. No library/API impact at all.

**Score.** Value 3.33 · Feasibility 3.0 · **10.0**. *Pure adoption play;
zero risk to the core, meaningful ongoing hosting/maintenance surface.*

## R2-10. Incremental / append-consistent generation (`--append`)

**Problem.** Growing a fixture from 1K to 10K rows regenerates all 10K —
fine — but the first 1K rows *change* if anything about the run differs, and
there is no way to generate "rows 1001..10000 only" for incremental-load
tests, pagination tests, or topping up a seeded database.

**Proposal.** Derive each row's substream from (entity, row index) — largely
the existing model — and guarantee it as a contract: `--rows-from N` /
`--append` generates rows N..M byte-identical to the same range of a full
run with the same seed. Uniqueness registries for the skipped prefix are
reconstructed deterministically (re-derive values without materializing
rows) or loaded from a `--state` manifest emitted by prior runs.

**Who benefits.** Incremental-ETL testers, pagination/load testing, growing
seeded environments without re-seeding.

**Effort.** M.

**Risks.** The contract constrains future generator refactors (row-index
substream derivation becomes load-bearing API). Uniqueness/reference
reconstruction for skipped prefixes is the tricky half — the manifest path
is the honest fallback. Cross-entity references need the full target pool,
so append applies per-entity with dependencies regenerated or manifest-fed.

**Score.** Value 3.0 · Feasibility 3.0 · **9.0**. *Useful and
architecture-aligned, but the uniqueness-state problem keeps it below the
fold; ship after the determinism-audit work (round-1 #17) hardens the
substream contract.*

## R2-11. CDC / mutation event streams

**Problem.** Teams building sync engines, cache invalidation, CDC consumers
(Debezium-shaped pipelines), or event-sourced systems need *changes over
time*, not snapshots: a deterministic sequence of inserts, updates, and
deletes against a coherent dataset.

**Proposal.** `datjit generate --mutations N [-f cdc|sql-dml]`: after the
base snapshot, emit N seeded mutation events — weighted insert/update/delete
choices, update target and field selection, and new values all drawn from a
mutation substream; updates respect field types/decorators; deletes respect
reference integrity (or deliberately violate it under a flag, for testing
orphan handling). Output as Debezium-style envelopes (`before`/`after`/`op`)
in NDJSON, or as SQL DML.

**Who benefits.** Data-pipeline and CDC-consumer testers, sync/replication
engine authors, event-sourcing teams, cache-invalidation testing.

**Effort.** L.

**Risks.** A second generation phase with its own state (current dataset
evolves as mutations apply) — meaningful new machinery, all seeded so
determinism holds. Schema-shape question (what does `_mutations` output look
like in non-NDJSON formats) needs a spec first. Builds well on R2-4's
stateful groundwork.

**Score.** Value 3.67 · Feasibility 2.33 · **8.56**. *Genuinely novel
capability with a real audience, but the heaviest build on this board —
spec it after R2-4 lands.*

---

## Quick wins (small, additive, low-risk)

- **`datjittest.Seed(t)`** — pick the seed from `-datjit.seed` flag, env, or
  randomize-and-log, so failures print a one-line reproduction (`re-run with
  -datjit.seed=…`). Property-style testing with deterministic replay in ~50
  lines.
- **Relative volumes** — `volume: Order: 20*User` resolved at plan time;
  removes a class of hand-sync errors when scaling fixtures up and down.
- **`datjit explain <schema> <Entity.field>`** — print the resolved pipeline
  for one field (type → decorators in application order → corpus key →
  substream scope). Cheap subset of round-1 #17's audit, immediately useful
  for "why is this value weird".
- **XLSX writer** — testers and business stakeholders ask for spreadsheets;
  one well-maintained dependency, fits the `ports.Writer` slot. Borderline:
  accept only if dependency review passes.

## Deferred / rejected this round

- **Versioned generation compatibility** (`compat: "0.3"` pinning old value
  algorithms so library upgrades never shift downstream goldens) — a real
  pain point, but freezing every generator variant forever is a maintenance
  trap; revisit only if golden-breakage reports actually materialize.
- **Kafka/OTLP/syslog sinks** — same verdict as round-1 #21: stateful
  infrastructure outside the library/CLI shape. The CDC format (R2-11) gets
  the data shape right; delivery stays the user's job.
- **Conditional field presence decorator** — already expressible with
  `@compute` `when:` branches; document the pattern instead of adding syntax.
- **Differential-privacy fitting** (noise-added `datjit fit`) — real research
  surface, wrong tool tier; `fit` + round-1 #15 (PII tagging) covers the
  practical need.
- **Multi-tenant partitioned generation** — expressible today with per-tenant
  seeds and volume overrides; no new machinery earns its keep.

---

## The killer list (rounds 1 + 2 combined)

The cross-round shortlist, ordered by score with ties broken by strategic
compounding (items that make other items better rank up). This is the
recommended build order.

| # | Enhancement | Round | Score | Why it's on the list |
|---|---|---|---|---|
| 1 | Richer `inspect` + cardinality/feasibility analysis | R1-1 | 16.0 | Catches infeasible schemas before generation; pure read-only |
| 2 | MCP server for AI coding agents | R2-1 | 16.0 | Thin façade over existing `Service`; turns agents into a distribution channel |
| 3 | Dirty-data injection (`@dirty`) | R2-2 | 15.89 | The differentiator: deterministic mess, which no mainstream generator offers |
| 4 | Typed row accessors + struct codegen | R1-3 | 14.67 | Kills `map[string]any` casting pain for the core Go-test audience |
| 5 | Structured errors + `ValidateAll` | R1-4 | 14.67 | The rich error type already exists internally; mostly surfacing work |
| 6 | Cardinality-aware (skewed) reference sampling | R1-5 | 14.67 | Realistic graph topology; compounds with #9 and graph writers |
| 7 | Schema fitting from sample data (`datjit fit`) | R2-3 | 14.0 | Converts "I have data" into "I have a schema" — the missing adoption step |
| 8 | Schema introspection: export, diff, deps | R1-2 | 14.67 | CI drift gates + cycle paths; pairs with JSON-Schema export (R2-6) |
| 9 | Time-series & stateful sequences | R2-4 | 13.0 | Opens the entire time-ordered-fixture category |
| 10 | Temporal coherence + referential fidelity | R1-10 | 13.0 | Makes relational/time fixtures correct without post-processing; pairs with #9 |
| 11 | Multi-locale corpus with fallback | R1-11 | 12.44 | Highest raw value anywhere (4.67); unblocks all non-US users |
| 12 | Edge-case / hostile profiles | R2-5 | 12.22 | With #3, completes the negative-testing story |
| 13 | Parquet writer | R1-12 | 12.0 | The most-requested format gap for data/ML pipelines |

Suggested wave structure: **Wave 1 (visibility + trust)** = 1, 5, 8 — all
read-only, no determinism exposure, immediately improve daily use. **Wave 2
(differentiation)** = 2, 3, 12 — the agent surface and the negative-testing
pair nobody else has. **Wave 3 (depth)** = 4, 6, 7, 9, 10 — realism and
adoption funnels that build on hardened substream contracts. **Wave 4
(reach)** = 11, 13 — content-heavy and dependency-heavy expansions.

Every item above is additive, keeps `core/*` adapter-free, and routes all
randomness through the seeded substream model. Each should get its own
design doc under `docs/superpowers/specs/` before implementation, per the
project convention.
