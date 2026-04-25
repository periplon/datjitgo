# Repository Guidelines

## Project Structure & Module Organization

`datjitgo` is a Go module for deterministic synthetic data generation. The root package exposes the public `datjit` API through files such as `service.go`, `options.go`, `generate_helpers.go`, and `value_convert.go`. `cmd/datjit` contains the CLI, while `repl` backs the interactive shell. Core domain types and ports live under `core/` (`model`, `value`, `rules`, `plan`, `ports`, `errors`). Adapters are split into `parser`, `generator`, `output`, `corpus`, `llm`, `runtime`, and `datjittest`. Fixtures and golden outputs live in `testdata/fixtures` and `testdata/golden`; embedded corpus data is under `corpus/data`. Design specs and plans belong in `docs/superpowers/specs` and `docs/superpowers/plans`.

## Build, Test, and Development Commands

- `make build`: builds the CLI binary at `bin/datjit`.
- `make check-build`: runs `go build ./...` across all packages.
- `make test`: runs `go test -race -count=1 ./...`.
- `make test-fixtures`: validates fixture and golden output tests.
- `make test-update`: refreshes fixtures with `-update`; use only for intentional golden changes.
- `make lint`: runs `golangci-lint run ./...` when installed, otherwise `go vet ./...`.
- `make fmt` and `make check-format`: format or verify Go formatting.
- `make ci`: the required local gate before pushing.

## Coding Style & Naming Conventions

Use standard Go formatting (`gofmt`; `goimports` is configured in `.golangci.yml`). Keep CLI code thin and place domain behavior in `core` or adapter packages. Public APIs should remain stable around the root `datjit.Service` facade and functional options such as `WithSeed`; consult the public API audit before moving packages under `internal/`. Prefer deterministic examples and tests; pass `WithSeed` or CLI `--seed` whenever output is asserted.

## Testing Guidelines

Tests use the standard Go testing package, with `go-cmp` available for comparisons. Name files `*_test.go` and tests `TestXxx`. Put integration-like public API coverage near the root package, adapter coverage beside the adapter, and reusable fixture assertions in `datjittest`. For final coverage checks, use `go test ./... -count=1 -coverprofile=coverage.out -covermode=atomic`.

## Commit & Pull Request Guidelines

Commits follow `type: subject`, lowercase and present tense, for example `feat: add csv writer option`, `fix: handle empty volume`, or `docs: update runtime example`. Pull requests should describe the workstream, reference the relevant spec or plan when behavior changes, list verification commands, and call out any intentional fixture or golden updates.
