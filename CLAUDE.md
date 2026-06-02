# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

`make ci` is the merge gate (gofmt check, lint, race-mode tests, fixture goldens, build; `lint` is golangci-lint when installed, else `go vet`). Run it before staging commits.

```bash
make ci                                      # full local gate
make lint                                    # golangci-lint if installed, else go vet
make cover                                   # race tests + coverage summary
make test                                    # race tests, no fixtures, no coverage
make test-fixtures                           # only TestFixtures golden checks
make test-update                             # regenerate golden fixtures (intentional only)

go test -race -count=1 ./parser/...          # one package
go test -race -count=1 -run TestParseDDL ./parser    # one test
go test -count=1 -run Example ./...          # all godoc examples
go test -count=1 -run Example -v .           # root examples specifically

golangci-lint run ./...                      # lint with the committed config
```

CLI smoke: `go run ./cmd/datjit version`.

## CLI surface

Cobra commands under `cmd/datjit` (`cmd_*.go`):

- `generate <schema> [-o -f --seed --locale --volume --entity --sql-dialect --pretty --dry-run --corpus-dir --llm-live]` — formats `json | csv | ndjson | yaml | sql`.
- `validate <schema>` — parse + validate, exit 1 on error.
- `inspect <schema> [--infer-tools]` — entity/field/rule summary.
- `corpus list | info | update` — inspect or refresh embedded/overlay corpus.
- `repl [<schema>]` — interactive shell (`repl` package, chzyer/readline).
- `version`.

## Architecture

Module path is `github.com/periplon/datjitgo`, but the root package is named **`datjit`** (import as `datjit`). The `runtime` package is also named `runtime` and shadows the stdlib — alias it on import (README uses `djruntime`). Requires Go 1.26.2 (matches the `go` directive).

Hexagonal. Imports point inward: adapters depend on `core/*`; `core/*` depends on nothing internal; `cmd/datjit` and `repl` depend on the root `datjit` facade only.

```
cmd/datjit, repl                 user interfaces
       │
       ▼
datjit (root)                    Service facade + functional options + helpers
       │
       ▼  ports.*
core/model, core/value,          stable domain, value, error, port contracts
core/errors, core/ports,
core/rules, core/plan
       ▲
       │  implements ports
parser, generator, output,       adapters — NewDefault wires parser/generator/
corpus, llm, runtime             output/corpus defaults; LLM is opt-in
datjittest                        test helper package
```

Pipeline driven by `datjit.Service`: `Parse → Validate → Generate → Write`. Three API layers exist:

- Root one-call helpers (`GenerateRowsFile`, `GenerateMapString`, `GenerateJSONFile`, …) for application code.
- `datjit.Service` + functional options (`WithSeed`, `WithLocale`, `WithVolume`, `WithCorpus`, `WithLLMProvider`, `WithWriter`) for custom adapters.
- `runtime` package for embedding generation in host DSLs / rule engines (`GenerateValue`, `GenerateDocument`, `GenerateEntity`, `GenerateRows`).

`datjittest` is a testing-only helper package — `MustRows`, `AssertGoldenJSON`, `UpdateGoldenJSON`.

## Invariants

- **Determinism**: same schema + same seed → same output. Use `WithSeed` in tests and examples; do not introduce new sources of randomness without routing through `core/value`'s seeded RNG.
- **Public API stability**: `datjit`, `core/model`, `core/value`, `core/ports`, `core/errors`, `datjittest`, and `runtime` are the stable surface. Do not rename, move, or break their exported identifiers. `corpus` and `llm` need review before being marked stable; `parser`, `generator`, `output`, `core/plan`, and `core/rules` are candidate internal packages. Do not move any of them under `internal/` without the public API audit decision and compatibility note.
- **Hexagonal direction**: `core/*` may not import any adapter. Adapters may import `core/*` only. The root `datjit` package wires adapters and exposes the facade.
- **Golden fixtures**: every file under `testdata/fixtures/` has a matching golden under `testdata/golden/`. `TestFixtures` enforces drift. Regenerate with `make test-update` only for intentional changes.

## Specs and plans

Designs live in `docs/superpowers/specs/YYYY-MM-DD-<topic>-design.md`. Implementation plans live in `docs/superpowers/plans/YYYY-MM-DD-<topic>.md`. New behavior should reference or add a spec. The brainstorming → writing-plans → executing-plans flow (superpowers skills) is the expected path for non-trivial work.

## Schema language (DDL)

Schemas are YAML with a compact DDL for field types: primitives, semantic types (`person.full`, `email`, `address.city`), enums with weighted distributions, references (`->User`, `<->Tag`), compound types (`[T]`, `{K: V}`, `T?`, `T | U`), distributions (`@dist(normal, μ=35, σ=12)`, Zipf, lognormal), pattern templates (`@pattern("SKU-{AA}-{0000}")`), `@derived`/`@compute`/`@default_chain` expressions, cross-entity rules (`@strict`, `@probability(p)`, `@warn`), and coherence groups. Parsing lives in `parser`; full spec in `docs/superpowers/specs/2026-04-22-datjitgo-design.md`.

## Commit style

`type: subject`, lowercase, present tense. Types in active use: `feat`, `fix`, `docs`, `test`, `chore`, `refactor`, `ci`, `merge`. One workstream per PR; reference the spec it implements. Do not add `Co-Authored-By` trailers (per the contributor's global preference).

## Releases

SemVer tags `vMAJOR.MINOR.PATCH`. Pushing a `v*` tag triggers `.github/workflows/release.yml`, which cross-builds `cmd/datjit` for linux/darwin × amd64/arm64 (semver stamped in via `-ldflags -X main.version`), packages each target as a `datjit_<ver>_<os>_<arch>.tar.gz`, and publishes a GitHub Release for the tag with those archives plus a `SHA256SUMS` file attached. The release body is taken from the matching `CHANGELOG.md` `## [x.y.z]` section; tags with a `-` suffix (e.g. `v0.3.0-rc.1`) are marked pre-release. Before tagging: move the `CHANGELOG.md` `[Unreleased]` entries under a new `## [x.y.z] - YYYY-MM-DD` heading (the release notes read from it), tag the commit whose `go.mod` already carries the current module path, then push the tag. The module path moved from `jmcarbo/datjitgo` to `periplon/datjitgo` at v0.2.0 — tags before that point at the old path and are not installable as `periplon`.

## Live integrations

`@llm`, `@llm_values`, and `_meta @llm(...)` use a deterministic offline stub by default. Opt into network calls via `datjit.WithLLMProvider(...)` or CLI `--llm-live`. Built-in HTTP providers cover OpenAI-compatible endpoints (`openai`, `lmstudio`, `vllm`) and Ollama. Tests that hit the network must be skipped by default.

Corpus overlays: JSON arrays of strings or `{name, weight}` objects loaded via `--corpus-dir` or `WithCorpus`. The embedded corpus lives under `corpus/data/`.
