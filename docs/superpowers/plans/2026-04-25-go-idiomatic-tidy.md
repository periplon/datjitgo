# Go Idiomatic Tidy Implementation Plan

> **For agentic workers:** Use superpowers:subagent-driven-development to
> execute this plan. Tasks A/B/C/D are independent and may run in parallel.

**Goal:** Apply the changes specified in
`docs/superpowers/specs/2026-04-25-go-idiomatic-tidy-design.md` without
breaking any existing test or changing observable library or CLI behavior.

**Branch:** `feature/tidy-idiomatic` in `.worktrees/tidy`.

**Definition of done:**

- `go test ./...` green.
- `go test -race ./...` green.
- `go vet ./...` clean.
- `golangci-lint run ./...` clean against the new `.golangci.yml`.
- `go test ./... -run Example` passes (all godoc examples compile and run).
- New CI jobs configured.
- New docs (CHANGELOG, CONTRIBUTING, SECURITY) committed.
- README contains badges and links to the new docs.
- `docs/refactor.md` archived (renamed to `docs/refactor-prompt.md` with a
  status header) so it cannot be confused with current specs.

---

## Task A: Lint + CI Infrastructure

**Files:**

- Create: `.golangci.yml`
- Modify: `Makefile`
- Modify: `.github/workflows/ci.yml`
- Create: `.github/workflows/release.yml`
- Modify: `llm/provider_test.go`

**Steps:**

- [ ] **A1.** Create `.golangci.yml` enabling: `govet`, `staticcheck`,
      `errcheck`, `ineffassign`, `unused`, `misspell`, `gofmt`, `goimports`,
      `revive`, `unconvert`. Use `version: "2"` schema. Set
      `run.timeout: 5m`. Exclude `testdata/` from analysis.

- [ ] **A2.** Fix `llm/provider_test.go` line ~92: change
      `NewHTTP().Complete(nil, ports.LLMRequest{...})` to
      `NewHTTP().Complete(context.Background(), ports.LLMRequest{...})`.
      Add `"context"` to imports if missing.

- [ ] **A3.** Update `Makefile`:
  - `lint:` should run `golangci-lint run ./...` if available, else
    `go vet ./...` (use `command -v golangci-lint`).
  - Add `cover:` target running
    `go test -race -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | tail -1`.

- [ ] **A4.** Replace `.github/workflows/ci.yml` content with two jobs:
  - `lint`: checkout, setup-go from `go.mod`, run
    `golangci/golangci-lint-action@v6` with `version: latest`.
  - `test`: checkout, setup-go from `go.mod`, run `make ci` (which now
    includes race + cover). Upload `coverage.out` via
    `actions/upload-artifact@v4`.

- [ ] **A5.** Create `.github/workflows/release.yml`:
  - Trigger: `on: push: tags: ['v*']`.
  - Build matrix: `linux/amd64`, `linux/arm64`, `darwin/amd64`,
    `darwin/arm64`.
  - Use `go build -ldflags "-s -w" -o datjit-${{matrix.os}}-${{matrix.arch}} ./cmd/datjit`.
  - Upload binaries as workflow artifacts. Do NOT auto-create a GitHub
    Release in this iteration.

- [ ] **A6.** Verify locally: `make ci`, `golangci-lint run ./...`.

**Acceptance:** `make ci` green; `golangci-lint run ./...` reports zero
issues.

---

## Task B: Contributor + Release Docs

**Files:**

- Create: `CHANGELOG.md`
- Create: `CONTRIBUTING.md`
- Create: `SECURITY.md`
- Modify: `README.md`
- Rename: `docs/refactor.md` → `docs/refactor-prompt.md` (add status header)

**Steps:**

- [ ] **B1.** Create `CHANGELOG.md` in Keep a Changelog format:
  - `## [Unreleased]` section with subsections `Added` (godoc examples,
    CHANGELOG/CONTRIBUTING/SECURITY, golangci-lint config, release
    workflow), `Changed` (CI now runs lint + race + coverage), `Fixed`
    (staticcheck SA1012 in llm test).

- [ ] **B2.** Create `CONTRIBUTING.md` covering:
  - Local prereqs (`Go 1.26.2` per go.mod, `golangci-lint`).
  - `make ci` is the gate.
  - Commit message style (`type: subject` lowercase, present tense).
  - Where designs and plans live (`docs/superpowers/specs`,
    `docs/superpowers/plans`).
  - PR expectations (one workstream per PR, reference the spec).
  - Determinism contract (use `WithSeed`; do not break golden fixtures).

- [ ] **B3.** Create `SECURITY.md`:
  - Supported version: latest tagged release.
  - Disclosure: email `joanmarc.carbo@gmail.com`, 90-day disclosure window.
  - Scope: the library, the CLI, the embedded corpus. Out of scope: third
    party LLM providers users wire in.

- [ ] **B4.** Update `README.md`:
  - Add a top badge row: GitHub Actions CI status, `pkg.go.dev` reference,
    Go report card, license. Use shields.io for the badges.
  - Add a "Project status" sentence: "Active. Public API stable in the
    `datjit` and `core/*` packages."
  - At the bottom, add a "See also" section linking `CHANGELOG.md`,
    `CONTRIBUTING.md`, `SECURITY.md`,
    `docs/superpowers/specs/2026-04-25-library-ergonomics-design.md`.

- [ ] **B5.** Rename `docs/refactor.md` to `docs/refactor-prompt.md` and
      prepend a status header:
      `> Status: prompt template, not a current design or plan. See \`docs/superpowers/specs/\` for active specs.`

**Acceptance:** All four files render correctly on GitHub; README badges
resolve.

---

## Task C: Godoc Examples + doc.go

**Files:**

- Create: `example_test.go` (root package)
- Create: `doc.go` (root package)
- Create: `runtime/example_test.go`
- Create: `datjittest/example_test.go`
- Create: `output/example_test.go`

**Steps:**

- [ ] **C1.** Create root `doc.go`. Move the existing `// Package datjit
      ...` doc block from `datjit.go` to `doc.go` and expand it with:
  - One paragraph summarizing the layered API (Service facade, root
    helpers, runtime, datjittest).
  - A "Choosing an API" section listing four cases: app code, tests,
    extensions, embedding in a DSL — each with the recommended entry
    point.
  - Determinism contract one-liner.
  - Remove the doc block from `datjit.go` (keep the `package datjit`
    line).

- [ ] **C2.** Create root `example_test.go` with:
  - `ExampleGenerateMapString` — a tiny inline schema, deterministic seed,
    `// Output:` asserting the row count.
  - `ExampleGenerateRowsFile` — uses `testdata/fixtures/<smallest>.yaml`
    if available, else inline. Asserts first row's primary key.
  - `ExampleService_Generate` — full pipeline via `NewDefault`.

- [ ] **C3.** Create `runtime/example_test.go` with `ExampleEngine_Run`
      showing how a host application calls the embedded runtime.

- [ ] **C4.** Create `datjittest/example_test.go` with `ExampleRows` using
      the deterministic helpers and asserting via `// Output:`.

- [ ] **C5.** Create `output/example_test.go` with `ExampleNewJSON` and
      `ExampleNewCSV` writing a tiny dataset to `os.Stdout`.

- [ ] **C6.** Run `go test ./... -run Example -v` and confirm every
      example reports PASS. If any `// Output:` proves brittle, drop to an
      unverified `Example_*` form.

**Acceptance:** All examples compile and run. `go doc -all ./...` shows
each example linked to its function.

---

## Task D: Idiomatic Polish

**Files:** any `.go` file under the module, behavior-preserving only.

**Steps:**

- [ ] **D1.** Run `goimports -w .` and commit the result if anything
      changed. If `goimports` is not installed, skip this step (the
      `.golangci.yml` will catch drift in CI).

- [ ] **D2.** Audit exported identifiers in: `errors.go`, `helpers.go`,
      `convert.go`, `validate.go`, `inspect.go`, `datjit_corpus.go`,
      `options.go`. For each missing or non-idiomatic doc comment, add
      one that begins with the identifier name. Do not rephrase comments
      that already follow the convention.

- [ ] **D3.** Audit exported methods in `core/model/orderedmap.go` and
      `core/model/document.go`. Same rule: doc comment starts with
      identifier name; only fix gaps.

- [ ] **D4.** Search for `interface{}` outside of `// `-prefixed comments:
      `grep -RIn 'interface{}' --include='*.go' .`. Replace with `any`
      where it appears in code. Skip vendored or generated files.

- [ ] **D5.** Confirm receiver names are short and consistent within each
      file. Fix the file if a single file mixes (e.g.) `s` and `svc` for
      the same receiver type. Do not rename across files.

- [ ] **D6.** Re-run `go test ./...` and `golangci-lint run ./...` to
      confirm zero diffs in behavior.

**Acceptance:** Tests still green; `golangci-lint` clean; diffs are
comments and import order only — no logic change.

---

## Sequencing

Tasks A, B, C, D operate on disjoint files and may run in parallel as
separate subagents. Task A's `.golangci.yml` must land before Task D's
final lint pass for D's verification to be meaningful, but D can run in
parallel and rebase its verification at the end.

After all tasks complete:

1. Run `make ci` from the worktree root.
2. Run `golangci-lint run ./...`.
3. Run `go test ./... -run Example`.
4. Stage commits per the rollout list in the spec (one commit per
   workstream when feasible).
5. Push branch and open PR (only if user explicitly requests; default is
   to leave the branch ready locally).

## Out of Scope

- Touching parser, generator, output, corpus, llm, or runtime logic.
- Any rename or move of public packages or identifiers.
- Coverage uplift beyond what new examples bring.
- Goreleaser, signed releases, doc site.
