# Library Ergonomics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add ergonomic datjitgo library APIs for app developers, test authors, extension authors, and DSL/rule-engine integrations.

**Architecture:** Keep `datjit.Service` as the core and layer convenience APIs over the existing parse/validate/generate/write pipeline. Add `datjittest` for testing-only helpers and `runtime` for embeddable DSL/rule-engine calls. Keep CLI/REPL untouched.

**Tech Stack:** Go 1.26.2, standard `testing`, existing `core/model`, `core/value`, `core/errors`, and `datjit.Service`.

---

## Files

- Create `convert.go`: root package conversion helpers from `value.Value` / `value.Dataset` to plain Go shapes.
- Create `helpers.go`: root package simple app-facing helpers.
- Create `errors.go`: root package error predicate helpers.
- Create `datjittest/helpers.go` and `datjittest/helpers_test.go`: testing helpers and golden JSON assertions.
- Create `runtime/runtime.go` and `runtime/runtime_test.go`: embeddable runtime API for documents, entities, and single values.
- Modify `README.md`: document simple helpers, testing helpers, and runtime integration.

## Task 1: Plain Conversion Helpers

**Files:**
- Create: `convert.go`
- Create/modify tests: `datjit_test.go`

- [ ] Write tests for `ValueAny`, `ObjectMap`, `DatasetMap`, and entity filtering.
- [ ] Verify tests fail with undefined helpers: `go test . -run 'TestPlainConversion|TestGenerateMap'`.
- [ ] Implement conversion using `value.Value` kinds and ordered object iteration.
- [ ] Verify focused tests pass.
- [ ] Commit: `feat: add plain value conversion helpers`.

## Task 2: Root Convenience Helpers And Error Predicates

**Files:**
- Create: `helpers.go`
- Create: `errors.go`
- Modify tests: `datjit_test.go`

- [ ] Write tests for `GenerateString`, `GenerateMapString`, `GenerateRowsString`, `GenerateJSONString`, `GenerateMapFile`, `GenerateRowsFile`, `GenerateJSONFile`, and error predicates.
- [ ] Verify tests fail with undefined helpers.
- [ ] Implement helpers through `New(opts...)`, `Parse`, `Validate`, `Generate`, `Write`, and conversion helpers.
- [ ] Verify focused tests pass.
- [ ] Commit: `feat: add root library convenience helpers`.

## Task 3: `datjittest` Package

**Files:**
- Create: `datjittest/helpers.go`
- Create: `datjittest/helpers_test.go`

- [ ] Write tests for `MustGenerate`, `MustRows`, `AssertGoldenJSON`, and `UpdateGoldenJSON`.
- [ ] Verify tests fail with undefined package helpers.
- [ ] Implement test helpers with `testing.TB.Helper()`, deterministic pretty JSON, and clear failure messages.
- [ ] Verify focused tests pass.
- [ ] Commit: `feat: add datjittest helpers`.

## Task 4: Runtime Package For DSL Integrations

**Files:**
- Create: `runtime/runtime.go`
- Create: `runtime/runtime_test.go`

- [ ] Write tests for `GenerateDocument`, `GenerateEntity`, semantic `GenerateValue`, primitive `GenerateValue`, decorators on `GenerateValue`, and validation errors.
- [ ] Verify tests fail with undefined runtime package.
- [ ] Implement `Runtime`, `ValueRequest`, `RowsRequest`, `RunOption`, and compiler interfaces.
- [ ] Implement `GenerateValue` by compiling a one-field temporary document and reusing the service generator.
- [ ] Verify focused tests pass.
- [ ] Commit: `feat: add embeddable generation runtime`.

## Task 5: Docs And Full Verification

**Files:**
- Modify: `README.md`

- [ ] Add compact docs for root helpers, `datjittest`, and `runtime`.
- [ ] Run `gofmt` over new files.
- [ ] Run `make ci`.
- [ ] Commit docs/fixes: `docs: document library ergonomics`.

## Self-Review

- Spec coverage: app helpers, test helpers, extension-friendly existing options, runtime APIs, compiler boundary, plain data shapes, and error predicates are covered.
- Placeholder scan: no implementation step relies on TBD behavior.
- Type consistency: `value.Dataset`, `value.Object`, `model.Document`, `model.TypeExpr`, and root `Option` names match existing code.
