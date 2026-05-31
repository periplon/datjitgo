# Repository Layout Phases Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reorder datjitgo files toward idiomatic Go conventions while preserving public import paths and making later API/internalization decisions explicit.

**Architecture:** Keep the root `datjit` package as the public facade and preserve all existing package directories during the first implementation pass. Improve scanability through file renames, package comments, and a compatibility audit before any `internal/` move. Optional package internalization is deliberately deferred until the audit is accepted.

**Tech Stack:** Go 1.26.2 module, standard `go` tooling, `gofmt`, `golangci-lint`, Makefile targets, Markdown docs under `docs/superpowers`.

---

## File Structure

Modify only these existing or planned files in the first pass:

- Rename: `datjit.go` -> `service.go`
- Rename: `helpers.go` -> `generate_helpers.go`
- Rename: `convert.go` -> `value_convert.go`
- Modify: `doc.go`
- Modify or create package docs only as needed in:
  - `parser`
  - `generator`
  - `output`
  - `corpus`
  - `llm`
  - `runtime`
  - `datjittest`
  - `core/model`
  - `core/value`
  - `core/ports`
  - `core/errors`
  - `core/plan`
  - `core/rules`
- Create: `docs/superpowers/specs/2026-04-25-public-api-audit.md`

Do not modify behavior files beyond package comments and file renames. Do not move package directories in this plan.

### Task 1: Rename Root Files Without Code Changes

**Files:**
- Rename: `datjit.go` -> `service.go`
- Rename: `helpers.go` -> `generate_helpers.go`
- Rename: `convert.go` -> `value_convert.go`

- [ ] **Step 1: Inspect current files**

Run:

```bash
sed -n '1,220p' datjit.go
sed -n '1,180p' helpers.go
sed -n '1,120p' convert.go
```

Expected: `datjit.go` contains `Service`; `helpers.go` contains `Generate*` helpers; `convert.go` contains value conversion helpers.

- [ ] **Step 2: Rename files with git**

Run:

```bash
git mv datjit.go service.go
git mv helpers.go generate_helpers.go
git mv convert.go value_convert.go
```

Expected: `git status --short` shows three renames and no package-directory moves.

- [ ] **Step 3: Run focused verification**

Run:

```bash
go test ./... -count=1
```

Expected: all packages pass; no import paths change.

- [ ] **Step 4: Run phase gate**

Run:

```bash
make ci
```

Expected: format, lint, tests, fixtures, and build pass.

- [ ] **Step 5: Commit**

Run:

```bash
git add service.go generate_helpers.go value_convert.go
git commit -m "chore: reorder root facade files"
```

Expected: commit contains only the three file renames.

### Task 2: Mark Package Intent With Go Documentation

**Files:**
- Modify: `doc.go`
- Modify or create package docs in public and implementation-facing packages listed in File Structure.

- [ ] **Step 1: Inspect package docs**

Run:

```bash
go list ./... | while read -r pkg; do go doc "$pkg" >>/tmp/datjitgo-docs.txt; done
rg -n '^package |^// Package ' doc.go core parser generator output corpus llm runtime datjittest
```

Expected: identify packages with missing or vague package comments.

- [ ] **Step 2: Update root package overview**

Edit `doc.go` so it clearly states:

```go
// Package datjit is the public facade for deterministic synthetic data
// generation from datjit schemas.
//
// Use the root Service type and Generate* helpers for application code. The
// core packages expose stable model, value, error, and port contracts for
// integrations. Adapter packages provide the default parser, generator, output,
// corpus, and LLM implementations used by NewDefault.
package datjit
```

Keep any existing useful overview text below this wording if it is not redundant.

- [ ] **Step 3: Add package intent comments where missing**

For any package without a clear `// Package name ...` comment, add a `doc.go` file with one of these exact patterns, adjusted only for the package name:

```go
// Package parser contains the default datjit schema parser adapter.
package parser
```

```go
// Package generator contains the default deterministic dataset generator.
package generator
```

```go
// Package output contains writers for JSON, CSV, NDJSON, YAML, and SQL output.
package output
```

```go
// Package datjittest provides test helpers for deterministic datjit fixtures.
package datjittest
```

Do not add new exported identifiers.

- [ ] **Step 4: Format and verify docs**

Run:

```bash
gofmt -w doc.go core parser generator output corpus llm runtime datjittest
go list ./... | while read -r pkg; do go doc "$pkg" >>/tmp/datjitgo-docs-after.txt; done
go test ./... -count=1
```

Expected: docs render; tests pass.

- [ ] **Step 5: Run phase gate**

Run:

```bash
make ci
```

Expected: full gate passes.

- [ ] **Step 6: Commit**

Run:

```bash
git add doc.go core parser generator output corpus llm runtime datjittest
git commit -m "docs: mark package layout intent"
```

Expected: commit contains package comments only.

### Task 3: Write Public API Audit

**Files:**
- Create: `docs/superpowers/specs/2026-04-25-public-api-audit.md`

- [ ] **Step 1: Capture package list**

Run:

```bash
go list ./... > /tmp/datjitgo-packages.txt
cat /tmp/datjitgo-packages.txt
```

Expected: list includes root, `cmd/datjit`, `core/*`, adapters, `runtime`, and `datjittest`.

- [ ] **Step 2: Create audit document**

Create `docs/superpowers/specs/2026-04-25-public-api-audit.md` with this structure:

```markdown
# datjitgo Public API Audit

Status: proposed
Date: 2026-04-25

## Stable Public Packages

- `github.com/periplon/datjitgo`: main Service facade, options, helpers, error predicates.
- `github.com/periplon/datjitgo/core/model`: schema and inspection model types.
- `github.com/periplon/datjitgo/core/value`: generated value model.
- `github.com/periplon/datjitgo/core/ports`: extension interfaces for parser, generator, writers, corpus, and LLM providers.
- `github.com/periplon/datjitgo/core/errors`: typed parse, validation, generation, and corpus errors.
- `github.com/periplon/datjitgo/datjittest`: testing helpers.
- `github.com/periplon/datjitgo/runtime`: embeddable runtime for host DSLs and rule engines.

## Review Before Marking Public

- `github.com/periplon/datjitgo/corpus`: useful for corpus overlays, but may expose default adapter details.
- `github.com/periplon/datjitgo/llm`: useful for default LLM provider wiring, but provider contracts live in `core/ports`.

## Candidate Internal Packages

- `github.com/periplon/datjitgo/parser`
- `github.com/periplon/datjitgo/generator`
- `github.com/periplon/datjitgo/output`
- `github.com/periplon/datjitgo/core/plan`
- `github.com/periplon/datjitgo/core/rules`

## Compatibility Rule

Do not move candidate packages under `internal/` until the project accepts a compatibility note and changelog entry. If adapter imports are considered supported API, keep the package directories public and document them as extension points instead.
```

- [ ] **Step 3: Verify audit against current package list**

Run:

```bash
go list ./... | sed 's#^#- #'
rg -n 'github.com/periplon/datjitgo/(parser|generator|output|core/plan|core/rules)' docs/superpowers/specs/2026-04-25-public-api-audit.md
```

Expected: every candidate internal package is named in the audit.

- [ ] **Step 4: Commit**

Run:

```bash
git add docs/superpowers/specs/2026-04-25-public-api-audit.md
git commit -m "docs: audit public package surface"
```

Expected: commit contains only the audit document.

### Task 4: Final Verification and Handoff

**Files:**
- No planned file modifications.

- [ ] **Step 1: Confirm no accidental package moves**

Run:

```bash
go list ./...
git diff --name-status HEAD~3..HEAD
```

Expected: root file renames, package doc changes, and one audit doc only; no directory moved under `internal/`.

- [ ] **Step 2: Run final gate**

Run:

```bash
make ci
```

Expected: full gate passes.

- [ ] **Step 3: Summarize remaining decision**

Write a final handoff note stating:

```text
Phase 1 and Phase 2 are complete. Phase 3 audit is written. Phase 4 is intentionally blocked on the API decision: keep candidate adapter packages public, or move selected ones under internal with a compatibility note.
```

Expected: no Phase 4 code work is started without explicit approval.
