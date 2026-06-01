# Enhancement Roadmap

This document captures candidate enhancements for `datjitgo`, evaluated by a multi-expert review panel (spanning CLI/DX, Go-library API, data-realism, reliability/testing, scale/perf, integrations, and privacy/compliance lenses). Each candidate was scored 1–5 on **User Value** and **Feasibility** by three independent judges; the **Score** below is `value × feasibility` from the averaged means. Every proposal was checked against the project's hard invariants — determinism (same schema + seed → same bytes), public-API stability, and hexagonal direction (`core/*` imports no adapter). Items that would breach those invariants are flagged and pushed to the deferred section.

## Summary ranking

| Rank | Enhancement | User Value | Feasibility | Score | Effort |
|---|---|---|---|---|---|
| 1 | Richer `inspect` with field detail + cardinality/feasibility analysis | 4.0 | 4.0 | 16.0 | M |
| 2 | Schema introspection: export, diff, dependency-graph | 3.67 | 4.0 | 14.67 | M |
| 3 | Typed row accessors + Go struct codegen | 4.0 | 3.67 | 14.67 | M |
| 4 | Structured error types + batch validation with suggestions | 4.0 | 3.67 | 14.67 | M |
| 5 | Cardinality-aware (skewed) reference sampling | 3.67 | 4.0 | 14.67 | M |
| 6 | Complete named-type composition (reusable records) | 3.67 | 3.67 | 13.44 | L |
| 7 | Fluent Go schema builder | 3.33 | 4.0 | 13.33 | L |
| 8 | Watch mode + REPL/CLI dev-loop ergonomics | 3.33 | 4.0 | 13.33 | M |
| 9 | Guided schema scaffolding (`datjit init`) | 3.33 | 4.0 | 13.33 | M |
| 10 | Temporal coherence + referential-fidelity decorators | 4.33 | 3.0 | 13.0 | M |
| 11 | Multi-locale corpus with deterministic fallback | 4.67 | 2.67 | 12.44 | L |
| 12 | Native Parquet output writer | 4.0 | 3.0 | 12.0 | M |
| 13 | Schema importers (OpenAPI / JSON-Schema / SQL) | 4.0 | 3.0 | 12.0 | L |
| 14 | Tunable SQL bulk output (batch / multi-row / COPY) | 3.0 | 4.0 | 12.0 | M |
| 15 | PII tagging, masking, validated patterns, audit report | 4.0 | 2.67 | 10.67 | L |
| 16 | Streaming, memory-bounded row generation | 4.33 | 2.33 | 10.11 | L |
| 17 | Determinism audit + reproducibility testing + diagnostics | 3.33 | 3.0 | 10.0 | M |
| 18 | Cross-row rule enforcement | 4.0 | 2.33 | 9.33 | L |
| 19 | Public writer registration + plugin extension points | 3.0 | 3.0 | 9.0 | M |
| 20 | Context-aware Service facade | 3.0 | 2.0 | 6.0 | M |
| 21 | HTTP generation-as-a-service + event-stream sinks | 3.0 | 2.0 | 6.0 | M |
| 22 | Parallel entity generation | 2.33 | 2.33 | 5.44 | M |

---

## Tier 1 — High value, high feasibility (do first)

These are read-only or purely additive, reuse machinery that already exists, and carry no determinism or API risk.

### 1. Richer `inspect` with field detail and pre-generation cardinality/feasibility analysis

**Problem.** `datjit inspect` shows only entity/field counts and dependencies — no field types, decorators, or constraints — so auditing means re-opening the schema. Worse, users can design infeasible runs (1000 `@unique` rows over a 3-variant enum; 100 Posts referencing 5 Authors) and only discover the failure after expensive generation completes.

**Proposal.** Add a `--verbose` flag to list per-entity fields with type, decorators, and constraints in a compact table. Add `Service.EstimateCardinality(doc) *CardinalityReport` (per-field cardinality bounds derived from enum/pattern/distribution analysis, `@unique` reachability, reference target-pool counts vs. configured volumes) surfaced via `datjit inspect --cardinality`, flagging exhaustion risks with confidence levels. Pure static analysis reusing the existing distribution-bounds helpers; no generation.

**Who benefits.** Schema reviewers and designers, CI pipelines catching infeasible fixtures early, teams learning DDL best practices.

**Effort.** M.

**Risks.** Estimates are heuristic for unbounded distributions and combinatorial patterns and must be labeled approximate, not guarantees. Read-only, so determinism and the public API are untouched.

**Scores.** Value 4.0 · Feasibility 4.0 · Score 16.0. *Panel note: unanimous top pick — high-value early-failure detection layered onto an existing command, with the only caveat being honest "approximate, not guarantee" labeling.*

### 2. Schema introspection: export, diff, and dependency-graph tooling

**Problem.** Schemas evolve (fields added, types widened, enums and volumes changed) but there is no machine-readable signature to diff versions or visualize cross-entity dependencies. Drift detection requires manual review or full regeneration, and cyclic-dependency errors are terse (`"cyclic dependency"`) with no path.

**Proposal.** Add read-only introspection: `Service.ExportSchemaSig(doc) *model.SchemaSummary` (entity/field signatures with types + decorators, enum variants, rules, volume specs), `Service.DiffSchemaSig(old, new)`, and `Service.ExportDependencyGraph(doc)` returning nodes/edges/cycles with exemplar cycle paths. Surface as `datjit schema export`, `datjit schema diff <old> <new> [--format unified|json] [--strict]` (exit 1 on breaking changes such as field removal or type narrowing), and `datjit schema deps` (ASCII or Graphviz dot). Enrich the existing cyclic-dependency validation error with a `CyclePath []string`. Committed signatures double as CI drift fixtures.

**Who benefits.** Test engineers managing fixture suites, contract-testing teams, schema maintainers, code reviewers, CI drift gates.

**Effort.** M.

**Risks.** All read-only; no determinism or generation impact. `SchemaSummary`/`DependencyGraph` types live in `core/model` or `core/plan` (no hexagonal violation). Diff is pure comparison (go-cmp). The existing topo-sort in `core/plan` already detects cycles, so adding the path is small.

**Scores.** Value 3.67 · Feasibility 4.0 · Score 14.67. *Panel note: genuinely useful for evolving multi-entity schemas and fully read-only; schema-diff is a somewhat niche workflow, and the full export+diff+deps+CLI surface is solidly M.*

### 3. Typed row accessors and Go struct codegen

**Problem.** Generated data returns as `[]map[string]any`, forcing consumers to cast and nil-check every field (`rows[0]["name"].(string)`) with no autocomplete or compile-time column safety. There is also no path between datjit schemas and existing Go struct types.

**Proposal.** Add generic helpers `RowAs[T any](row) (T, error)` and `FieldAs[T](row, key, zero T) T` using reflection plus `datjit:"..."` struct tags. Add `datjit generate-structs --schema schema.yaml --output structs.go` to emit typed structs from entities, and the inverse `codegen.FromGoStruct(reflect.Type) *model.Document` for schema bootstrapping. Reflection-only, no RNG.

**Who benefits.** Test/integration code consuming generated data, backend engineers with schema-first Go APIs, gorm/sqlc users.

**Effort.** M.

**Risks.** Reflection overhead is negligible for test code. Semantic-type inference from field names is heuristic; nullable mappings (`*T`, `sql.NullX` → `T?`) must be documented. The codegen halves are optional and additive; the `RowAs`/`FieldAs` helpers are the high-confidence core.

**Scores.** Value 4.0 · Feasibility 3.67 · Score 14.67. *Panel note: the generics kill pervasive `map[string]any` cast-and-check pain for the large Go-test audience; struct inference heuristics are the only soft spot and stay optional.*

### 4. Structured error types plus batch validation with suggestions

**Problem.** `IsParseError`/`IsValidationError`/`IsGenerationError` return only `bool`, exposing no kind, location, entity, or cause for structured logging. `Validate` returns the first error only, forcing serial fix-and-revalidate. Errors lack actionable hints (typo `emali` → `email`, exhausted uniqueness domains, circular path).

**Proposal.** Expose typed errors backed by the already-internal `core/errors.Error` (which carries `Kind`, `Entity`, `Field`, `Location`, `Cause` and supports `errors.Is`/`As`): `ParseError{Path,Line,Column,Message}`, `ValidationError{Entity,Field,...}`, `GenerationError{Entity,Field}`, populated at the parser/generator sites. Add `Service.ValidateAll(doc) *ValidationReport{Valid, Errors[], Warnings[]}` collecting every issue in one pass with edit-distance `Suggestions()` for unknown semantic types/fields. Keep `Validate()` for the boolean fast path; wire both into CLI `validate` output.

**Who benefits.** Production apps (Sentry/Datadog logging), schema linters and IDE plugins, CLI users debugging schemas, DDL newcomers.

**Effort.** M.

**Risks.** Re-exporting typed errors widens the stable error API — ship with deprecated aliases or in a major bump. `ValidateAll` is purely additive. Suggestions must be deterministic text and prioritize the known-semantics registry over raw edit-distance guesses.

**Scores.** Value 4.0 · Feasibility 3.67 · Score 14.67. *Panel note: unusually feasible because the rich `Error` type already exists — this is mostly surfacing plus a second collect-all validation loop, not new infrastructure. Only the public re-export needs a deliberate compat plan.*

### 5. Cardinality-aware (skewed) reference sampling

**Problem.** References sample uniformly or via `@count`, so generated entity graphs lack realistic power-law ownership (80% of Orders to 20% of Users; authors with 0..100 posts). Aggregation/analytics tests and graph simulators get unrealistically flat data.

**Proposal.** Allow distribution decorators on reference fields — e.g. `@dist(zipf, s=1.5)` on a `User` reference — so targets are sampled per the distribution instead of uniformly. For many-to-one, apply the distribution across the target pool and combine with existing reverse `@count` cardinality. Extend the distribution spec to handle reference sampling, seeded per-entity to preserve determinism, with graph-topology variance captured in golden tests.

**Who benefits.** Graph-DB and social-network test generators, marketplaces with seller/buyer skew, analytics engineers validating aggregations.

**Effort.** M.

**Risks.** Sampling must stay within the already-generated target set (never reference non-existent targets) and requires careful per-entity seeding. Non-breaking extension of the existing `@dist` decorator, reusing the clean Fisher-Yates reference path already in the generator.

**Scores.** Value 3.67 · Feasibility 4.0 · Score 14.67. *Panel note: contained, deterministic, non-breaking enhancement that reuses existing `@dist` and per-row RNG machinery; serves a somewhat specialized realism need rather than broad daily use.*

---

## Tier 2 — High value, harder

These deliver strong value but touch generation, output shape, corpus content, or evaluation ordering, raising effort or determinism risk.

### 6. Complete named-type composition (real reusable records)

**Problem.** Named types (`types:`) currently expand to stable placeholders, not real generated data, so repeated structures like `Address` or `Contact` (`User.address`, `Order.billing_address`, `Vendor.headquarters`) force duplicated field definitions. This was explicitly deferred.

**Proposal.** Finish the deferred implementation so `types:` can hold full field definitions and generate real data, in two emission modes: nested objects/maps inline for nesting formats (JSON/YAML/NDJSON) and a generated `_types` pseudo-entity referenced by id (`->Address`) for flat formats (CSV/SQL). Tie generation to a sub-seed (parent row seed or a named-type substream). Report named-type field counts/stats in `inspect`. Gate behind `--expand-types` initially.

**Who benefits.** Schema-heavy teams, domain modelers, anyone reducing fixture boilerplate.

**Effort.** L.

**Risks.** Changes `Dataset` shape and output serialization (nested object vs. pseudo-entity), so a spec update, output-writer coordination, and fixture regeneration are mandatory. Determinism holds only with consistent sub-seeding. The optional flag keeps it non-breaking initially.

**Scores.** Value 3.67 · Feasibility 3.67 · Score 13.44. *Panel note: feasibility is higher than billed — `generateNamedType` already emits real seeded nested objects, so the remaining work is mainly flat-format pseudo-entity emission, inspect stats, and fixtures behind the flag.*

### 7. Fluent Go schema builder (no YAML round-trip)

**Problem.** Embedding datjit in Go requires hand-writing YAML strings/files; there is no type-safe way to assemble a `*model.Document`. Tools, rule engines, and code generators must maintain YAML out-of-band, losing IDE support and compile-time checks.

**Proposal.** Add a `builder` package with a fluent API over existing model types: `builder.Document().Domain().Seed(42).Volume("User",10).Entity("User").Field("id", model.TypeInt).Primary().Field("name", model.TypeSemantic("person.full")).Build()`. No parser involvement. Plugs into `Service` via `New(WithDocument(doc))` and `runtime.New`. `Build()` output must be deterministic (stable field/entity ordering).

**Who benefits.** SDK consumers, test-fixture authors, tools synthesizing schemas from config/DSLs, code generators.

**Effort.** L.

**Risks.** New package, purely additive adapter producing `core/model` (no hexagonal violation). Must guarantee byte-for-byte determinism per construction sequence (the existing `OrderedMap` provides stable ordering).

**Scores.** Value 3.33 · Feasibility 4.0 · Score 13.33. *Panel note: clean and additive, but competes with simply writing YAML, and the model surface (compound types, decorators, rules, distributions) is large enough that a complete ergonomic builder is real work; serves the programmatic-embedding segment.*

### 8. Watch mode and REPL/CLI dev-loop ergonomics

**Problem.** Iterating on a schema means manually re-running `generate` after each edit, and REPL state (seed, format, loaded schema, custom corpus) resets every session with no startup config or live corpus reload.

**Proposal.** Add `datjit watch <schema.yaml>` that polls for changes, re-validates, and prints/writes output with `generated N rows in Xms` (staying alive and showing diagnostics on error), plus an `--exec '<cmd>'` post-generate hook. Add REPL QoL: a `~/.datjitrc` / `$DATJIT_CONFIG` startup file and `datjit repl --init <file>`, a `save-session` command, `corpus reload <dir>` for hot reload, and `sample <semantic_type> [--count N]` backed by `Service.SampleCorpus(ctx, semantic, count)`.

**Who benefits.** Schema authors doing TDD-style iteration, REPL power users, content specialists tuning corpus realism, CI fixture automation.

**Effort.** M.

**Risks.** The watch loop must clean up on Ctrl-C and must not auto-run generate from the rc file by default. `SampleCorpus` is a deterministic read-only addition. REPL/CLI are outside the stable API per CLAUDE.md, so changes are low-risk.

**Scores.** Value 3.33 · Feasibility 4.0 · Score 13.33. *Panel note: low-risk additive work that meaningfully tightens the inner loop; value is solid-but-incremental ergonomics rather than a new capability.*

### 9. Guided schema scaffolding (`datjit init` and REPL `create-entity`)

**Problem.** New users hit a friction cliff: the compact DDL requires knowing semantic types, decorators, and declaration order before writing even a small schema. There is no guided creation path.

**Proposal.** Add `datjit init [--domain --entities --locale] <output.yaml>` that scaffolds a minimal, annotated, working schema (demonstrating `@primary`, `@unique`, semantic types, `@range`) and verifies it parses + generates before writing. Add a REPL `create-entity` command that prompts field-by-field and emits insertable YAML, backed by `Service.ScaffoldEntity(ctx, in, out)` for editor/IDE embedding.

**Who benefits.** CLI newcomers, onboarding teams, tutorial/doc authors, IDE/plugin developers.

**Effort.** M.

**Risks.** Purely additive I/O with a validate-before-write check; no determinism or generation impact. Generated YAML is a starting point only. The main effort is curating good annotated templates.

**Scores.** Value 3.33 · Feasibility 4.0 · Score 13.33. *Panel note: lowers the first-use cliff and is safe to build, but it addresses one-time onboarding rather than recurring power-user needs.*

### 10. Temporal coherence and referential-fidelity decorators

**Problem.** Date/time fields generate independently (orders shipped before created; age-misaligned birth/hire dates) and references are skeletal — a generated foreign key may not correspond to any existing target row, producing disconnected FK graphs. Realistic relational and time-series fixtures are impossible without manual post-processing.

**Proposal.** Add `@temporal(anchor=field, offset_days_min, offset_days_max)` (extending `@from`) so timestamps order correctly (`created < shipped < delivered`), evaluated in a deterministic post-pass before derived/compute chains, with a `timeline` coherence-group type. Add `@fidelity(required|permissive, cardinality=N%)` on reference fields so `required` back-fills missing targets and a tunable percentage (e.g. 95% valid / 5% synthetic orphans) is seeded per-row, run as a fidelity pass after all entities generate. Both add new model decorator types without breaking existing schemas.

**Who benefits.** Financial/audit/order-tracking/HR systems needing temporal order, ORM/constraint test-suite builders, graph and data-warehouse fixture authors.

**Effort.** M (the panel notes the `@fidelity` half may push toward L).

**Risks.** Temporal offsets must respect date-only vs. timezone-aware fields and reference-resolution ordering (affects generator field/derived evaluation order). Fidelity needs bidirectional reference tracking and per-row seeded cardinality decisions to stay deterministic. Backward-compatible decorators.

**Scores.** Value 4.33 · Feasibility 3.0 · Score 13.0. *Panel note: exactly what makes relational/time-series fixtures usable without post-processing; `@temporal` extends existing `@from` machinery, but `@fidelity` needs new bidirectional tracking and back-filling, with real ordering-correctness risk.*

### 11. Multi-locale corpus with deterministic fallback chaining

**Problem.** Only an en-US corpus is embedded (`Locales()` literally returns en-US). International teams cannot generate French/German/Japanese names, addresses, postal codes, IBAN/BIC without a fully manual `--corpus-dir` overlay, and there is no controlled fallback or way to mix locales in one run. Explicitly deferred.

**Proposal.** Extend `CorpusProvider` to accept a locale chain (`de-DE → de → en-US`) with weighted, locale-scoped RNG substreams so determinism holds. Auto-discover locale subfolders under `--corpus-dir` (`en-US/`, `de-DE/`) and optionally embed a few additional locales via `//go:embed` behind a build option to limit binary bloat. Add a `--locales` flag for a per-entity locale distribution. Pair with parameterized semantic types (`person.full:de`, `phone:us_only`, `iban:de`) by activating the existing-but-unused `Semantic.Params` for corpus lookup hints with graceful fallback.

**Who benefits.** International SaaS teams, compliance-driven (GDPR/region-specific ID) datasets, multi-currency/locale schemas, regional QA.

**Effort.** L.

**Risks.** Embedded locales grow binary size (mitigate with build flags / overlay-first). Determinism requires consistent locale-scoped substream derivation; the corpus key registry can explode if not gated. The parser must stay backward-compatible for unparameterized semantic types. Corpus is an adapter, so no hexagonal violation.

**Scores.** Value 4.67 · Feasibility 2.67 · Score 12.44. *Panel note: highest raw value on the board (an explicitly deferred wall for all non-US users) but real content-authoring plus substream-wiring effort; `SampleContext` already carries `Locale` while `corpus.Sample` hardcodes en-US.*

### 12. Native Parquet output writer

**Problem.** Data engineers and ML practitioners need columnar Parquet for Spark/Presto/BigQuery/data-lake pipelines, but datjit emits only JSON/CSV/YAML/SQL. Users must generate text then convert externally, adding friction and breaking end-to-end determinism.

**Proposal.** Implement `output.NewParquet()` as a `ports.Writer` using parquet-go, registered as the `parquet` format. Honor `WriteOpts` for compression (snappy/gzip/uncompressed), row-group size, and schema inference from the Document. Enforce fixed compression/row-group defaults so output is byte-stable; add fixtures and goldens matching existing writer patterns; validate decimal/UUID/time serialization against DuckDB/Spark.

**Who benefits.** Data engineers, ML practitioners, analytics teams seeding columnar/lake test datasets.

**Effort.** M (panel notes the determinism-pinning may push toward L).

**Risks.** Adds a third-party dependency to a near-stdlib module. Byte-stable determinism requires pinned compression and row-group settings; decimal/UUID/time encoding must match the Parquet spec exactly and be golden-tested against external readers.

**Scores.** Value 4.0 · Feasibility 3.0 · Score 12.0. *Panel note: genuinely demanded and fits the existing `ports.Writer` slot, but the dependency weight plus byte-stable encoding verification is the cost.*

### 13. Schema importers from OpenAPI/JSON-Schema and live SQL databases

**Problem.** Teams with existing OpenAPI specs, JSON-Schema definitions, or live SQL databases must hand-translate them into datjit DDL, duplicating schema knowledge and inviting drift.

**Proposal.** Add a `parser/importers` subpackage: `FromOpenAPI(r)`, `FromJSONSchema(r)` (mapping JSON-Schema types/formats to datjit primitives and semantic types — `string+format → email/url/uuid`, `integer min/max → @range`, `enum → weighted enum`), and `FromSQLDatabase(ctx, connStr, dialect)` introspecting `information_schema`/`sqlite_master` to map SQL types, detect `PRIMARY KEY` (`@primary`), and infer volumes from row counts. Surface as `datjit import openapi|json-schema|sql ...` emitting parseable, hand-tunable YAML.

**Who benefits.** API-first teams syncing fixtures to contracts, DBAs/backend engineers seeding existing schemas, migration teams keeping fixtures aligned.

**Effort.** L.

**Risks.** Mapping is lossy (JSON-Schema/SQL are more expressive than datjit primitives); semantic types need heuristics or manual post-edit. The SQL importer adds a `database/sql` dependency, live connections, and per-dialect introspection; inferred volumes are stale hints.

**Scores.** Value 4.0 · Feasibility 3.0 · Score 12.0. *Panel note: the OpenAPI/JSON-Schema mappers are feasible pure transforms; the live-SQL path is the heavier, lossier half — consider shipping the file-based importers first.*

### 14. Tunable SQL bulk output (batch size, multi-row INSERT, COPY)

**Problem.** The SQL writer hardcodes 100-row batches (`const sqlBatchSize = 100` in `output/sql.go`), so loading 1M+ rows produces 10K+ INSERT statements and wastes I/O. There is no way to tune batch size or use dialect-native bulk loading (Postgres COPY, MySQL multi-row INSERT).

**Proposal.** Add SQL write options (`BatchSize`, `UseMultiInsert`, `UseCOPY`) to `WriteOptions` with a higher default (~1000). For Postgres optionally emit `COPY FROM STDIN`; for MySQL emit multi-row `INSERT VALUES (...),(...)`. Expose `--sql-batch-size` and `--sql-copy`, document dialect trade-offs, and add benchmarks comparing batch sizes vs. COPY. Row ordering and per-row bytes are unchanged so determinism holds.

**Who benefits.** QA engineers bulk-loading synthetic data into production-like databases, ETL developers, anyone generating >100K rows to SQL.

**Effort.** M.

**Risks.** COPY/multi-row output diverges from pure portable SQL — gate behind a flag. Verify COPY loads in Postgres and multi-row INSERT in MySQL; batching must not alter row ordering or bytes.

**Scores.** Value 3.0 · Feasibility 4.0 · Score 12.0. *Panel note: an easy, well-scoped knob over the hardcoded constant; value is moderate and specific to the SQL-output subset.*

---

## Tier 3 — Nice to have / niche

Lower combined scores: narrower audiences, heavier determinism-sensitive refactors, or items that strain the hard invariants.

### 15. PII tagging, masking, validated patterns, and anonymization audit

**Problem.** Generating fixtures from production-shaped schemas, teams cannot mark which fields are PII, mask/obfuscate them while preserving format and referential consistency, generate validator-passing values (Luhn cards, valid SSN/IBAN), or produce an audit trail proving sanitization — blocking GDPR/HIPAA/SOC2-conscious sharing.

**Proposal.** Add a `@pii(category)` decorator with `datjit inspect --pii-audit`, an opt-in `@pii_mask` / `WithPIIMaskMode` / `--pii-mask` output mode replacing PII with format-preserving seed-deterministic placeholders, `@ref_anonymize(source=Entity.id)` for stable-but-synthetic aliases consistent across references (cached per source value + seed), `@pattern` validated generators (`CC:Luhn`, `SSN:Valid`, `IBAN:DE`) via a pluggable pure-function validator registry, and an optional `--write-anonymization-report` (JSON of transformations, sample before/after, rows affected) written separately from main output.

**Who benefits.** Privacy/compliance teams, organizations sharing test data across legal boundaries, financial-services and healthcare testers, GDPR/SOC2 auditors.

**Effort.** L (panel: leans toward heavy).

**Risks.** Masking and validators must be seed-deterministic; Luhn/IBAN/SSN logic must match external specs with thorough golden tests. `@ref_anonymize` adds a per-source cache to generation state (memory). Additive on the decorator/output side; no hexagonal violation.

**Scores.** Value 4.0 · Feasibility 2.67 · Score 10.67. *Panel note: unlocks compliance-conscious sharing for a real audience, but the breadth (decorator + mask mode + ref-anonymize cache + validator registry + report) plus spec-exact validators makes it sizable.*

### 16. Streaming, memory-bounded row generation for large datasets

**Problem.** `Service.Generate` buffers the entire `value.Dataset` before writing. Generating tens of millions of rows exhausts heap (10M × 10 fields × ~100B ≈ 10GB). There is no iterator or generate-and-write-per-row pipeline, blocking large-scale load/perf testing and warehouse seeding.

**Proposal.** Add a streaming path producing rows on demand into row-oriented writers (NDJSON, CSV, JSON-Lines, chunked JSON array) without buffering, in three shapes: `Service.GenerateStream(ctx, doc, format, w, opts)`; a `stream.Iterator` API (`Next`/`Current`/`Close`) over `*value.Object` / `map[string]any`; and `runtime.GenerateRowsStream` for embedders. CLI `--stream-write` (auto for row-oriented formats). Each row uses a sub-seed derived from entity+index so output stays byte-identical to buffered mode.

**Who benefits.** Performance/load-test engineers, ETL and data-pipeline developers, database seeders, anyone generating datasets larger than RAM.

**Effort.** L.

**Risks.** The generator currently buffers the full `Dataset` and resolves cross-entity back-references and dataset-level rule post-passes, so true streaming forces a single-entity-only carve-out (document the constraint) while still being a large, determinism-critical refactor. Streaming is limited to row-oriented formats (SQL/pretty-JSON need framing). New API is additive.

**Scores.** Value 4.33 · Feasibility 2.33 · Score 10.11. *Panel note: removes a real OOM ceiling on the headline large-dataset use case, but the cross-row/back-ref constraint undercuts the value while keeping the refactor heavy and byte-identical-proof-bound.*

### 17. Determinism audit, reproducibility testing, and constraint-violation diagnostics

**Problem.** Determinism is sacred but there is no way to verify it, trace which fields touched randomness, or get forensics when reproducibility breaks (e.g. x86/ARM endianness/FNV/byte-order drift). Uniqueness-exhaustion and rule-violation errors are minimal, so re-runs are blind.

**Proposal.** Add `Service.Audit(doc) *Audit` (seed genesis, RNG substream lineage per entity/field, per-field randomness category, SHA-256 of the generation path) surfaced as `datjit audit <schema>`. Add a `datjittest.DeterminismSuite` with `AssertDeterministic` (generate 3×, assert byte-identical) and `AssertReproducible` (golden checksum across builds/platforms), wired into `make test-determinism`, plus an `internal/platform` pre-flight verifying FNV-64a, byte order, and RNG state against committed reference checksums. Enrich generation errors with an optional `Diagnostics` map (field, entity, attempted/successful rows, collision_count, cardinality_reached, suggestion).

**Who benefits.** Maintainers validating cross-platform reproducibility and Rust→Go port fidelity, CI catching silent determinism regressions, schema authors debugging constraint failures.

**Effort.** M.

**Risks.** Full `Audit` precision requires instrumenting every RNG substream call — incomplete instrumentation yields silent gaps (require tests on any new randomness source). `Diagnostics` counters add minimal per-failure cost. Reference checksums must be computed on a documented reference platform. All additive; the `Diagnostics` field is backward-compatible.

**Scores.** Value 3.33 · Feasibility 3.0 · Score 10.0. *Panel note: the test helpers and richer exhaustion diagnostics are the easy, high-leverage parts and directly protect the core promise; full RNG-lineage audit is the deep, contributor-facing piece with silent-gap risk.*

### 18. Cross-row rule enforcement with deterministic conflict resolution

**Problem.** Cross-row business rules (a User's email domain must match its company domain) are parsed but stored only as raw metadata; they are never enforced during generation, so users get syntactically valid data that violates business rules and must post-filter.

**Proposal.** Implement a cross-row rule evaluator that parses rule bodies into a constraint AST (if-then-else over field values), checks each generated row against entity-scoped rules, and on violation applies deterministic conflict resolution (mutate the offending field using a sub-seed from row hash + rule index). Annotate affected rows (`_rules_applied`) for writers and extend `Validate` to catch rule syntax errors early. Start with simple comparisons/ranges before complex conditionals.

**Who benefits.** QA engineers building realistic scenarios, domain-driven teams, anyone with non-trivial business rules.

**Effort.** L.

**Risks.** Adds evaluation latency in the hot generation loop; determinism holds only if mutations are seeded per-rule-per-row. Output gains annotation metadata (shape change) requiring a spec update and writer coordination. The existing per-row retry loop, `evalRule`, and seeded substreams provide a foundation.

**Scores.** Value 4.0 · Feasibility 2.33 · Score 9.33. *Panel note: closes a real correctness gap, but a constraint-AST evaluator with seeded conflict-resolution mutation plus output metadata is a determinism-sensitive L with spec and fixture churn.*

### 19. Public writer registration and runtime plugin extension points

**Problem.** Output is limited to the built-in writers. Applications needing Avro/Parquet/custom-binary/proprietary formats or domain-specific semantic types must reach into core, duplicate serialization, or fork. There is no runtime extension mechanism beyond compile-time `WithWriter`/`WithCorpus`.

**Proposal.** Document and stabilize the already-exported `ports.Writer`/`ports.CorpusProvider` interfaces and add `Service.RegisterWriter(format, writer)` plus dispatch to registered formats, with a documented example. Optionally layer a `plugins` package: `plugins.LoadPlugin(path)` for Go `-buildmode=plugin` `.so` files exporting `NewWriter`/`NewCorpusProvider`, wired via `WithPlugins([...])` and `--plugins a.so,b.so`, plus `docs/plugins.md` and a `make plugin-template` scaffold.

**Who benefits.** SDK consumers needing domain-specific serialization, warehouse/binary-format integrations, enterprises with proprietary formats who cannot vendor.

**Effort.** M.

**Risks.** Exposing the ports publicly locks in their stability (already exported in `core/ports/ports.go`, so low risk if documented). Go plugins work only on Linux/macOS, are unsandboxed (panics/unbounded memory), and are version-fragile — document clearly and keep the static `RegisterWriter` path as the primary, portable option.

**Scores.** Value 3.0 · Feasibility 3.0 · Score 9.0. *Panel note: the static `RegisterWriter` half is a clean small win; the `-buildmode=plugin` half is a maintenance liability and the panel leans toward dropping it.*

### 20. Context-aware Service facade and one-call helpers

**Problem.** `Service.Parse/Validate/Generate` and the one-call helpers accept no `context.Context`. Hosts cannot apply timeouts, cancellation, or tracing, and long-running corpus lookups or live LLM calls cannot be interrupted — despite `LLMProvider.Complete` already taking a context.

**Proposal.** Thread `context.Context` as the first parameter through `Service.Parse/Validate/Generate` and the one-call helpers, honoring cancellation in the parser, generator, and corpus provider. Ship as a major version bump with deprecated thin shims (or a `NewDefault(ctx)` variant) so existing callers keep compiling during migration.

**Who benefits.** Web services with request timeouts, CLI apps, integration tests using `t.Cleanup`, anything needing cancellation or distributed tracing.

**Effort.** M.

**Risks.** **Breaking change to the stable `Service` signature** (a hard invariant). Must be a deliberate major bump with deprecation aliases and a compatibility note; `ports` interfaces largely unchanged.

**Scores.** Value 3.0 · Feasibility 2.0 · Score 6.0. *Panel note: legitimate for live-LLM and host embedding, but an explicit break of the public-API-stability invariant — only justifiable inside a planned major bump.*

### 21. HTTP generation-as-a-service with optional event-stream sinks

**Problem.** Teams want datjit as shared infrastructure decoupled from clients, and event-driven teams want synthetic events delivered straight to a stream. The CLI and library force per-process instantiation, with no network or message-bus path for polyglot tooling.

**Proposal.** Add an HTTP server (`cmd/datjit-server` or `datjit --serve`) on `net/http` exposing `POST /generate`, `POST /validate`, `GET /formats`, `GET /corpus/locales`, with input-size/timeout guards plus `docs/openapi.yaml` and Docker/Helm assets. Optionally add a Kafka sink writer (`output.NewKafka` via kafka-go) registered as the `kafka` format with deterministic consistent-hash partition keys. Seeds are request-scoped so determinism is preserved.

**Who benefits.** QA/CI pipelines generating fresh data per run, polyglot teams, platform engineers running shared generation infra, event-driven/load-test teams.

**Effort.** M (panel: combined surface is well beyond M).

**Risks.** Opens a network attack surface requiring strict input validation, size limits, and generation timeouts. Corpus overlays must be mounted/injected. Kafka is stateful (broker/TLS/auth/failure handling) with deterministic partitioning required — a separate, heavy concern that dilutes the tool's library/CLI focus.

**Scores.** Value 3.0 · Feasibility 2.0 · Score 6.0. *Panel note: helps polyglot/CI consumers but conflicts with the tool's shape and bolts on stateful infra; if pursued, split the HTTP server from the Kafka sink.*

### 22. Parallel entity generation with deterministic substream isolation

**Problem.** Entities generate sequentially in topological order, leaving multi-core machines idle when the dependency graph has independent chains (Products, Orders, Customers). Large multi-entity generation is slower than necessary.

**Proposal.** Analyze the dependency graph for generation-independent entity groups (no cross-references) and generate each group in its own goroutine, each deriving a deterministic RNG substream from the root by group name; merge results in original order so output is byte-identical to sequential. Add `WithParallelism(N)` (default `GOMAXPROCS`). Cross-group rules/references force serialization of those groups.

**Who benefits.** Developers generating large schemas with many independent entities (CRM, marketplace, analytics domains).

**Effort.** M.

**Risks.** Requires correct independence analysis; cross-entity rules spanning groups serialize them. **Determinism is the hard gate** — shared `generationState` (unique sets, generated map, seq counters) and rule post-passes make byte-identical proof against the sequential path the central burden.

**Scores.** Value 2.33 · Feasibility 2.33 · Score 5.44. *Panel note: lowest combined score — generation is rarely the bottleneck for typical fixtures, and the determinism-proof burden outweighs a modest, niche speedup for a tool that prizes correctness over throughput.*

---

## Explicitly out of scope / deferred

The following strain or break the project's hard invariants and should not be pursued without an explicit, called-out decision:

- **Context-aware facade (#20)** and any other signature change to `Service.Parse/Validate/Generate` or the one-call helpers **break public-API stability** and require a deliberate major version bump with deprecation shims — not an incremental enhancement.
- **Parallel generation (#22)** risks the **byte-identical determinism invariant** via shared mutable generation state and cross-entity rules; defer unless a full byte-for-byte equivalence test suite lands first.
- **True streaming (#16)** cannot preserve cross-row back-references and dataset-level rule post-passes; if pursued, it must be scoped to single-entity streaming and the limitation documented.
- **Go `-buildmode=plugin` loading (within #19)** is unsandboxed, OS-limited, and version-fragile; prefer the portable static `RegisterWriter` path and treat plugins as out of scope.
- **Kafka sink and network surface (within #21)** introduce stateful infrastructure and an attack surface outside the library/CLI shape; keep separate from the core tool.
- Any new randomness source must route through `core/value`'s seeded RNG; introducing unrouted randomness is out of scope under the determinism invariant.
