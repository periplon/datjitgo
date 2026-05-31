# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
