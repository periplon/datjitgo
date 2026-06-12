# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed
- `@range` now clamps `decimal` fields. Previously `applyRange` handled only
  int and float kinds, so e.g. `decimal(10, 2) @range(0..10000)` could emit
  values up to ~10^8. Bounds are parsed exactly (scale-preserving) and
  exclusive endpoints step inward by one unit of the value's scale, matching
  the time-range nanosecond convention. Goldens for the two fixtures that
  declare ranged decimals (`decorators`, `profiles`) were regenerated for
  this intentional behavior fix; only `rating`/`balance` values changed.

### Added
- Schema introspection. New read-only API over a parsed document:
  `Service.SchemaSummary` exports a stable, ordered, machine-readable schema
  signature (commit it as a CI drift fixture); `DiffSchemaSummaries` compares
  two signatures and classifies each change as breaking or compatible; and
  `Service.DependencyGraph` reports entity reference edges (polymorphic unions
  expand to one edge per target, `<->` is many-to-many) plus exemplar cycle
  paths. The cyclic-dependency validation error now names the path
  (`cyclic dependency: A -> B -> A`). A new `datjit schema export|diff|deps`
  command group surfaces all three: `export` emits JSON (or YAML), `diff`
  accepts schemas or previously exported summaries and supports `--strict`
  (exit 1 on breaking changes), and `deps` prints text or Graphviz `dot`.
- MCP server: `datjit mcp` runs a Model Context Protocol server over stdio
  (newline-delimited JSON-RPC 2.0, no new dependencies) so AI coding agents can
  drive the parse→validate→generate pipeline. Four tools — `generate`,
  `validate`, `inspect`, and `sample` — backed by the root facade and the
  `runtime` package. Generation is offline and seeded: a missing `seed`
  defaults to `0` (never the clock), so the same tool call yields byte-identical
  output. The new `mcp` package is not yet part of the stable public API.
- Dirty-data injection. A seeded corruption layer makes generated data
  realistically messy — typos, case mangling, stray whitespace, unexpected
  nulls, mixed timestamp formats, near-duplicate rows — deterministically:
  the same schema + seed produce the same mess. Opt in per field
  (`@dirty(rate=0.1, typo, whitespace)`), per entity
  (`_meta: "@dirty(rate=0.02, typo, case, null, duplicate)"`), or globally
  via `datjit.WithDirtyRate` / `ports.GenerateOptions.DirtyRate` / CLI
  `generate --dirty-rate R`. Field config wins over entity meta, which wins
  over the global dial. Safety exemptions keep `@primary`/`@auto` fields,
  references and polymorphic discriminators clean under entity/global
  config, and corrupted `@unique` fields fall back to their original value
  on collision. Schemas without `@dirty` are byte-identical to previous
  output. See `docs/superpowers/specs/2026-06-12-dirty-data-design.md`.
- Time-series & stateful sequence decorators. An entity's rows can now form
  an ordered sequence: `@series(start=, interval=, jitter=)` produces
  monotonic timestamps on date/datetime fields, `@walk(start=, drift=,
  volatility=, min=, max=)` cumulative random walks on int/float/decimal
  fields, and `@chain("from>to:prob, ...", start=)` Markov state
  progressions over enum fields. Values are drawn from per-entity seeded
  substreams in row-index order, so output is deterministic and schemas
  without these decorators are byte-identical to before (zero new draws on
  the default path). Validation rejects type mismatches, unknown chain
  states, and stateful decorators on coherence members, references or
  compound types. See
  `docs/superpowers/specs/2026-06-12-time-series-design.md`.
- Generation profiles for negative testing. `datjit generate --profile
  realistic|edge|hostile` (and `datjit.WithProfile` /
  `ports.GenerateOptions.Profile`) bias eligible field values toward boundary
  cases: `edge` substitutes curated extremes (empty/oversized strings,
  multi-byte/RTL/emoji/combining text, numeric min/max honouring declared
  `@range` bounds, epoch dates, all-zeros UUIDs); `hostile` adds adversarial
  payloads (CSV/SQL/spreadsheet injection shapes, 4 KiB strings, mixed-script
  homoglyphs — no NUL bytes). Keys, references, discriminators, coherence
  members, unique/pattern/derived/computed fields, and fields annotated
  `@profile(realistic)` are never substituted. The default profile is
  byte-identical to previous output; edge/hostile are deterministic per
  schema + seed + profile.

## [0.3.5] - 2026-06-03

### Added
- Index definitions. Entities may declare indexes under a reserved `_indexes`
  block (expanded mapping form — index name → `{fields, unique, where, method}`),
  surfaced by the `sql` writer as `CREATE [UNIQUE] INDEX` statements after each
  `CREATE TABLE`. An optional inference pass also derives indexes from schema
  signals (`@unique` fields, references, polymorphic discriminators). Emission
  is controlled by `--sql-indexes` / `WriteOpts.SQLIndexes`:
  `manual` (default, declared only), `auto` (declared + inferred), or `none`.
  Dialect-aware: partial `WHERE` (postgres/sqlite), `USING <method>`
  (postgres/mysql), and over-long index names are deterministically truncated.

## [0.3.0] - 2026-06-02

### Added
- Release automation: pushing a `v*` tag now publishes a GitHub Release with
  cross-compiled `datjit` archives (linux/darwin × amd64/arm64) and a
  `SHA256SUMS` file attached, with notes drawn from this changelog. Previously
  the workflow only uploaded ephemeral build artifacts.
- Polymorphic references now emit a discriminator. A field whose type is a
  union of two or more entity references (e.g. `author: ->User | ->Org`) gains
  a synthetic companion field `<field>_type` recording which target entity each
  generated row's primary key belongs to, across all output formats. Previously
  such a field yielded an untyped primary key with no way to tell which entity
  it referenced.

### Changed
- The `version` subcommand now reports `dev` for unstamped builds; the release
  workflow injects the real semver at link time via `-ldflags -X main.version`.

## [0.2.1] - 2026-06-02

### Fixed
- Foreign-key resolution now honours the `@primary` decorator on the target
  entity instead of the positional first field. A coherence group on an
  FK-target entity previously shadowed its primary key, so references resolved
  to a coherence value (e.g. a city) rather than the key.
- Cross-entity rules are now scoped by field membership. A bare (unqualified)
  rule such as `score >= 0 @strict` is enforced only against entities that
  declare the referenced field, instead of every entity — which previously
  resolved the field to null on unrelated entities and exhausted the `@strict`
  row-retry budget. Entity-qualified rules (`Entity.field`) are unchanged.

## [0.2.0] - 2026-05-31

### Changed
- **Module path moved to `github.com/periplon/datjitgo`** (was
  `github.com/jmcarbo/datjitgo`). Import paths and README badges updated.
  Install with `go install github.com/periplon/datjitgo/cmd/datjit@latest`.
- CI now runs lint, race-enabled tests, and coverage reporting.
- README now carries badges and contributor links.

### Added
- Godoc `Example_*` tests across the root `datjit` package, `runtime`,
  `datjittest`, and `output`.
- `doc.go` for the root package giving an overview of the library.
- `CHANGELOG.md`, `CONTRIBUTING.md`, and `SECURITY.md`.
- `.golangci.yml` configuration for `golangci-lint`.
- Release workflow for `cmd/datjit`.

### Fixed
- SA1012 (passing a nil `context.Context`) flagged by `staticcheck` in
  `llm/provider_test.go`.
