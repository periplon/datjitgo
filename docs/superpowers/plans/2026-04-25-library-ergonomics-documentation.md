# Library Ergonomics Documentation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add truthful developer documentation for embedding datjitgo with the implemented layered ergonomics APIs.

**Architecture:** Current `main` contains the root helper layer, `datjittest`, and the `runtime` package. Documentation should show those implemented APIs while keeping examples grounded in the existing `datjit.Service` facade and functional options where lower-level control is needed.

**Tech Stack:** Go modules, Markdown docs, `go test ./...`, `go test ./examples/...` once examples exist.

---

### Task 1: Add Current Library Embedding Guide

**Files:**
- Create: `docs/library-embedding.md`
- Modify: `README.md`

- [ ] **Step 1: Create `docs/library-embedding.md`**

Write a developer guide with these sections:

```markdown
# Library Embedding Guide

## Status

This guide documents the public APIs available on the current branch. The
approved next API layer is specified in
`docs/superpowers/specs/2026-04-25-library-ergonomics-design.md`.

## Supported Surface Today

Use `datjit.Service` from the root `datjit` package for production embedding.
Construct it with `datjit.NewDefault()` for built-in adapters or `datjit.New`
with functional options for custom corpus, LLM, writer, seed, locale, or volume
behavior.

## Pipeline

1. Parse a YAML schema with `Service.Parse`.
2. Validate the document with `Service.Validate`.
3. Generate a `*value.Dataset` with `Service.Generate`.
4. Write output through `Service.Write`.

## Extension Points

- `datjit.WithCorpus` installs a custom `ports.CorpusProvider`.
- `datjit.WithLLMProvider` installs a custom `ports.LLMProvider`.
- `datjit.WithWriter` installs a custom `ports.Writer`.
- `datjit.WithSeed`, `datjit.WithLocale`, and `datjit.WithVolume` override generation settings without mutating the parsed document.

## Implemented Ergonomics Layer

The implemented ergonomics layer includes root one-call helpers, a `datjittest`
package, and a `runtime` package for rule engines and DSLs. Use these APIs in
README and examples when they are the clearest fit.
```

- [ ] **Step 2: Link the guide from README**

In `README.md`, add one short paragraph at the end of the existing `Library use`
section:

```markdown
For embedding guidance, extension points, and the planned convenience API
layers, see [`docs/library-embedding.md`](docs/library-embedding.md).
```

- [ ] **Step 3: Verify Markdown references**

Run:

```bash
test -f docs/library-embedding.md
rg -n "docs/library-embedding.md|2026-04-25-library-ergonomics-design.md" README.md docs/library-embedding.md
```

Expected: both files exist and the README link plus spec link are present.

### Task 2: Add Buildable Current-API Examples

**Files:**
- Create: `examples/library/basic/main.go`
- Create: `examples/library/custom-writer/main.go`

- [ ] **Step 1: Add a basic service-pipeline example**

Create `examples/library/basic/main.go` with a full `Parse -> Validate -> Generate -> Write` flow using an in-memory schema and JSON output.

- [ ] **Step 2: Add a custom writer example**

Create `examples/library/custom-writer/main.go` with a small `ports.Writer`
implementation registered through `datjit.WithWriter`.

- [ ] **Step 3: Verify examples build**

Run:

```bash
go test ./examples/...
```

Expected: both example packages compile.

### Task 3: Final Verification And Commit

**Files:**
- Modify: `docs/superpowers/plans/2026-04-25-library-ergonomics-documentation.md`

- [ ] **Step 1: Run full docs/code verification**

Run:

```bash
go test ./...
git diff --check
```

Expected: tests pass and no whitespace errors are reported.

- [ ] **Step 2: Commit documentation changes**

Run:

```bash
git add README.md docs/library-embedding.md docs/superpowers/plans/2026-04-25-library-ergonomics-documentation.md examples/library/basic/main.go examples/library/custom-writer/main.go
git commit -m "docs: add library embedding documentation"
```

Expected: one docs commit on the worktree branch.
