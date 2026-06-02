# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
