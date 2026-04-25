# datjitgo Go Idiomatic Tidy + Production Readiness Design

Status: approved (autonomous brainstorm; user requested unattended execution)
Date: 2026-04-25

## Purpose

Bring `github.com/jmcarbo/datjitgo` to a Go-idiomatic, production-ready bar
without changing observable library or CLI behavior. The repository already
has a clean hexagonal architecture, full test suite, and a published facade.
The remaining gaps are project hygiene, tooling, runnable godoc examples,
contributor-facing documentation, and minor lint debt.

## Non-Goals

- No public API changes. The `datjit.Service` facade and root helpers remain
  stable.
- No new features, no behavior changes in parser, generator, output, corpus,
  llm, or runtime.
- No file moves between packages, no rename of exported identifiers.
- No new top-level packages (e.g. no `internal/`) — public adapters are
  already part of the API.

## Current Repo State

Verified before brainstorm:

- `go test ./...` passes on all 16 packages.
- `go vet ./...` clean.
- `gofmt -l .` clean.
- `golangci-lint run ./...` reports one issue:
  `llm/provider_test.go:92: SA1012 do not pass a nil Context`.
- All packages have a `Package X` doc comment on at least one file.
- Tests + golden fixtures + race-mode `make ci` all green.

Production-readiness gaps:

1. No `golangci-lint` configuration committed; lint is not part of CI.
2. Single-job CI: no race mode, coverage report, or matrix.
3. No `CHANGELOG.md`, `CONTRIBUTING.md`, or `SECURITY.md`.
4. README has no badges (CI, Go reference, license).
5. No runnable `Example_*` functions for godoc on the public surface.
6. One staticcheck violation in `llm/provider_test.go`.
7. `docs/refactor.md` is a stale prompt template, easy to mistake for a spec.
8. The plan `docs/superpowers/plans/2026-04-25-library-ergonomics-documentation.md`
   is untracked. Decide: commit, prune, or leave for the owner.
9. Module declares `go 1.26.2` — verify this is intentional vs the toolchain.
10. `coverage.out` / `coverage_all.out` artifacts get regenerated locally;
    `.gitignore` already ignores them. No action.

## Architecture

The hexagonal layout stays untouched:

```
cmd/datjit, repl
        |
        v
datjit Service + helpers
        |
        v
core/ports + core/model + core/value + core/errors + core/rules + core/plan
        ^
        |
parser, generator, output, corpus, llm, runtime adapters
datjittest helpers
```

Tidy work only adds files (configs, docs, examples) or rewrites existing
ones in place. No package moves, no import rewrites.

## Workstreams

The work decomposes into four independent workstreams that can run in
parallel inside the same worktree (separate file sets, no merge conflicts).

### A. Lint + CI Infrastructure

- Add `.golangci.yml` enabling: `govet`, `staticcheck`, `errcheck`,
  `ineffassign`, `unused`, `misspell`, `gofmt`, `goimports`, `revive`,
  `unconvert`, `gocritic` (basic rules).
- Fix the staticcheck violation in `llm/provider_test.go:92` by passing
  `context.Background()`.
- Replace `make lint` body with `golangci-lint run ./...` (fall back to
  `go vet ./...` only if the binary is missing).
- Extend `make ci` to also run `go test -race -coverprofile=coverage.out`
  and check coverage threshold (informational only).
- Update `.github/workflows/ci.yml` to:
  - Cache Go modules.
  - Run `golangci-lint` in a separate job.
  - Run `go test -race -coverprofile=coverage.out` in the test job.
  - Upload `coverage.out` as a workflow artifact.
- Add a `.github/workflows/release.yml` that runs on tag push and produces
  `cmd/datjit` binaries for `linux/amd64`, `linux/arm64`, `darwin/amd64`,
  `darwin/arm64` via `go build` (goreleaser is overkill at this stage).

### B. Contributor + Release Docs

- `CHANGELOG.md` — Keep a Changelog format, seed with one `Unreleased`
  section listing the tidy changes.
- `CONTRIBUTING.md` — short: how to run tests, fixtures, lint; commit style
  ("type: subject" is already in use); design doc location; PR expectations.
- `SECURITY.md` — disclosure email, supported version, scope.
- README updates:
  - Add badges: GitHub Actions CI, `pkg.go.dev`, `goreportcard.com`, license.
  - Add a "Project status" line.
  - Link CHANGELOG, CONTRIBUTING, SECURITY at the bottom.
- Delete or archive `docs/refactor.md` (stale prompt template).
- Decide on the untracked
  `docs/superpowers/plans/2026-04-25-library-ergonomics-documentation.md`:
  keep tracked under `docs/superpowers/plans/` so future agents see it; this
  spec assumes commit.

### C. Godoc Examples + Polish

Add runnable godoc examples on the root and high-traffic packages. Examples
must be self-contained, compile, and have `// Output:` blocks where output
is deterministic.

- `example_test.go` (root `datjit`):
  - `ExampleGenerateMapString`
  - `ExampleGenerateRowsFile`
  - `ExampleService_Generate` showing `NewDefault` + Parse/Validate/Generate
- `runtime/example_test.go`:
  - `ExampleEngine_Run` showing the embedded runtime API
- `datjittest/example_test.go`:
  - `ExampleRows` showing deterministic fixture generation in a test
- `output/example_test.go`:
  - one writer example each for JSON and CSV

Examples must use small inline schemas and `WithSeed(42)` for determinism.
The deterministic `Output:` line should be a stable subset (e.g. row count,
first id) rather than full JSON to avoid brittle equality.

Also add a top-of-package `doc.go` for the root `datjit` package that
expands beyond the existing `datjit.go` header — covering the layered API
(facade, helpers, runtime, datjittest) and a "Choosing an API" section.
The existing `// Package datjit ...` block on `datjit.go` moves to
`doc.go` and is expanded; nothing else changes in `datjit.go`.

### D. Idiomatic Polish

Small, surgical, behavior-preserving:

- Run `goimports -w .` once and commit any pure-import sort drift.
- `errors.go`: keep the public `Is*` predicates; add a sentence in each
  doc comment naming the underlying typed error so godoc readers can find
  it.
- `helpers.go`, `convert.go`, `validate.go`, `inspect.go`,
  `datjit_corpus.go`, `options.go` — confirm every exported identifier has
  a doc comment that begins with the identifier name. Add the missing ones;
  do not rephrase existing ones.
- `core/model/orderedmap.go` — confirm exported method docs start with
  the method name (Go style); fix only if missing.
- Replace any remaining `interface{}` with `any` (Go 1.18+ idiom). Audit
  shows root + adapters already use `any`; this is just a verification pass.
- Verify receiver names are short and consistent within a file.
- Ensure all new files end with a trailing newline.

No renames, no API breaks, no logic changes.

## Testing Strategy

The bar is "still green, plus examples run":

- `go test ./...` passes.
- `go test -race ./...` passes.
- `go vet ./...` clean.
- `golangci-lint run ./...` clean.
- `go test ./... -run Example` runs all godoc examples and they pass.
- Existing golden fixtures unchanged.

The CI workflow gains a lint job and a race+coverage test job.

## Rollout

All work happens on a single feature branch `feature/tidy-idiomatic` inside
`.worktrees/tidy`. Commits are split by workstream so the PR history is
readable:

1. `chore: add golangci-lint config + fix staticcheck`
2. `ci: add lint job, race tests, coverage upload`
3. `ci: add release workflow for cmd/datjit binaries`
4. `docs: add CHANGELOG, CONTRIBUTING, SECURITY`
5. `docs: add README badges and links`
6. `docs: add godoc examples for root, runtime, datjittest, output`
7. `docs: add doc.go expanding root package overview`
8. `chore: idiomatic polish (goimports, doc comments)`
9. `chore: archive stale refactor prompt`

## Risk

- Examples with `// Output:` can be brittle if generation order varies.
  Mitigation: assert on row count or first key only. If still flaky, drop
  to unverified examples (`Example_X` without `// Output:`).
- `golangci-lint` may surface latent issues outside the staticcheck one
  already known. Plan: fix surgically; if a fix would change behavior,
  disable the specific linter for that file with a comment noting why.
- Release workflow building four binaries on every tag is a small cost;
  no signing or notarization in scope.

## Out of Scope

- Goreleaser, signed releases, homebrew tap.
- Test coverage uplift beyond what `-coverprofile` reports.
- Docs site (the existing godoc + README is sufficient).
- Public API redesign — that lives under the existing
  `2026-04-25-library-ergonomics-design.md` spec, separate work.
