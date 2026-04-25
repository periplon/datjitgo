# datjitgo Repository Layout Phases Design

Status: proposed (phased direction requested by user)
Date: 2026-04-25

## Purpose

Reorder `github.com/jmcarbo/datjitgo` toward idiomatic Go repository
conventions while preserving contributor readability and avoiding avoidable
public import-path churn. This design complements the existing
`2026-04-25-go-idiomatic-tidy-design.md`, but focuses specifically on file and
package layout rather than CI, docs, or release polish.

## Goals

- Make the root package easier to scan.
- Keep the public root API stable: `datjit.Service`, functional options, root
  helper functions, and error predicates remain available.
- Preserve tests next to the packages they validate.
- Use phased changes so each step can be reviewed and reverted independently.
- Run `make ci` after each implementation phase.

## Non-Goals

- No behavior changes in parsing, validation, generation, output, corpus, LLM,
  CLI, REPL, or runtime.
- No exported identifier renames.
- No immediate move of public packages under `internal/`.
- No fixture or golden output changes unless a separate behavior change
  explicitly requires them.

## Current Layout Read

The repository already follows several Go conventions:

- `cmd/datjit` holds the CLI entrypoint.
- Root package `datjit` exposes the main library facade.
- Tests live beside their packages.
- `datjittest` and `runtime` are intentionally public support packages.
- Domain and port packages live under `core/*`.

The main readability issue is not directory structure; it is that root files
mix several public entrypoints and private helpers without a clearly documented
ordering. Some implementation packages may eventually deserve `internal/`, but
that is an API decision and should not be bundled with the first cleanup.

## Phase 1: File-Level Reordering, No Import-Path Changes

This phase is pure organization inside existing packages.

Root package target:

- Keep `doc.go` for the package overview.
- Rename `datjit.go` to `service.go` for `Service`, constructors, service
  methods, and default wiring.
- Keep `options.go` for functional options.
- Rename `helpers.go` to `generate_helpers.go`.
- Rename `convert.go` to `value_convert.go`.
- Keep `errors.go` for public error predicates.
- Keep `validate.go` and `inspect.go` intact in this phase.
- Keep `datjit_corpus.go` intact in this phase.

Tests move only when their subject file is renamed. Broad public API tests stay
at the root; adapter-specific tests stay in adapter packages.

## Phase 2: Package Documentation and Public Surface Marking

Add or tighten package comments so `go doc ./...` communicates intent:

- Public facade: root `datjit`.
- Public extension contracts: `core/model`, `core/value`, `core/ports`,
  `core/errors`.
- Public helpers: `datjittest`, `runtime`.
- Implementation-facing adapters: `parser`, `generator`, `output`, `corpus`,
  `llm`, plus `core/plan` and `core/rules` until their API status is decided.

This phase should not move code. It creates the evidence needed for a later API
audit.

## Phase 3: API Audit Before Any `internal/` Move

Decide which package import paths are supported for external users.

Recommended default:

- Stable public: root `datjit`, `core/model`, `core/value`, `core/ports`,
  `core/errors`, `datjittest`, `runtime`.
- Review individually: `corpus`, `llm`.
- Candidate internal implementation packages: `parser`, `generator`, `output`,
  `core/plan`, `core/rules`.

The audit output should be a short compatibility note in `docs/superpowers`
and, if public paths change, a `CHANGELOG.md` entry.

## Phase 4: Optional Internalization

Only after Phase 3, move selected implementation packages under `internal/`.
Do this in small slices:

1. Move one package family.
2. Update imports.
3. Run focused tests for affected packages.
4. Run `make ci`.
5. Record compatibility impact.

If external adapter imports are considered supported API, skip this phase.

## Verification

For Phase 1 and Phase 2:

- `gofmt` or `make fmt`
- `go test ./... -count=1`
- `make ci`

For Phase 3:

- Documentation review only, plus `make ci` if docs include examples.

For Phase 4:

- Focused package tests after each move.
- `go test ./... -count=1`
- `make ci`

## Risks

- Renaming files is mechanically simple but can obscure `git blame`. Keep
  commits small and avoid mixing renames with behavior edits.
- Moving packages under `internal/` can break external users. Treat it as a
  compatibility decision, not a cleanup default.
- Over-splitting root files can make the package harder to navigate. Split by
  durable responsibility, not by arbitrary file length.
