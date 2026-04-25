# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

`make ci` is the merge gate (gofmt check, `go vet`, race-mode tests, fixture goldens, build). Run it before staging commits.

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

## Architecture

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

## Commit style

`type: subject`, lowercase, present tense. Types in active use: `feat`, `fix`, `docs`, `test`, `chore`, `refactor`, `ci`, `merge`. One workstream per PR; reference the spec it implements. Do not add `Co-Authored-By` trailers (per the contributor's global preference).

## Live integrations

`@llm`, `@llm_values`, and `_meta @llm(...)` use a deterministic offline stub by default. Opt into network calls via `datjit.WithLLMProvider(...)` or CLI `--llm-live`. Built-in HTTP providers cover OpenAI-compatible endpoints (`openai`, `lmstudio`, `vllm`) and Ollama. Tests that hit the network must be skipped by default.

Corpus overlays: JSON arrays of strings or `{name, weight}` objects loaded via `--corpus-dir` or `WithCorpus`. The embedded corpus lives under `corpus/data/`.
