# Go Refactor Prompt

Use this prompt when asking an agent to refactor `datjitgo`. It is written to
produce small, verifiable Go changes while preserving behavior, public API
stability, deterministic generation, and the existing hexagonal package
boundaries.

## Prompt

You are refactoring `github.com/jmcarbo/datjitgo`, a Go synthetic-data
generation library and CLI.

Your goal is to improve internal code quality without changing observable
behavior unless a specific behavior change is explicitly requested below.

### Project Context

`datjitgo` is a Go port of `datjit`. It exposes a public library facade in the
root `datjit` package and a `cmd/datjit` CLI. The architecture is hexagonal:

- `core/model`, `core/value`, `core/errors`, and `core/ports` define stable
  domain types, values, typed errors, and adapter interfaces.
- `parser`, `generator`, `output`, and `corpus` are adapters.
- `datjit.Service` wires adapters behind `core/ports`.
- `cmd/datjit` and `repl` are user interfaces over the service facade.

Dependency direction matters:

- `core/*` must not import internal adapters.
- Adapters may import `core/*`.
- The root `datjit` package may wire adapters.
- CLI and REPL code should stay thin and should not duplicate core behavior.

Important behavior to preserve:

- Deterministic output for the same schema and seed.
- YAML/entity/field ordering where it affects output.
- Existing public API compatibility unless explicitly approved.
- Existing CLI flags, formats, and error semantics.
- Golden fixture behavior.

### Refactor Objective

Refactor the following target:

```text
<TARGET PACKAGE, FILES, OR BEHAVIOR HERE>
```

The quality problem to solve is:

```text
<CONCRETE PROBLEM: duplication, tangled responsibilities, poor naming,
hard-to-test function, overlarge file, unclear boundary, etc.>
```

Success means:

```text
<MEASURABLE OUTCOME: smaller focused functions, duplicated logic removed,
new helper with tests, clearer interface boundary, same tests passing, etc.>
```

Do not refactor outside this scope unless it is necessary to complete the
target safely.

### Required Working Method

1. Inspect the current implementation before editing.
2. Identify existing tests that protect the target behavior.
3. If coverage is weak, add focused characterization tests first.
4. Make the smallest coherent refactor that solves the stated quality problem.
5. Keep public exported names, package boundaries, CLI flags, and output formats
   unchanged unless the task explicitly authorizes a change.
6. Run `gofmt` on edited Go files.
7. Run verification commands and report the exact commands used.

### Go Refactor Rules

- Prefer simple functions and explicit data flow over clever abstractions.
- Introduce an interface only at a real boundary or where the existing design
  already uses a port.
- Do not create generic helpers unless they remove meaningful duplication.
- Do not move code across package boundaries without checking imports and the
  architecture rules above.
- Preserve error types and wrapping behavior; callers may rely on
  `errors.Is`/`errors.As`.
- Preserve deterministic RNG substream behavior in `generator`.
- Preserve ordered maps where output order depends on schema order.
- Keep tests close to the package under test.
- Avoid broad formatting-only diffs.
- Avoid unrelated cleanup.

### Verification

Run the narrowest useful test first, then the full suite:

```bash
go test ./<target-package>
go test ./...
```

If the refactor touches generation determinism, parsing, validation, CLI
behavior, output writers, or shared core types, also run:

```bash
go test -race ./...
go vet ./...
```

If any command cannot be run, explain why and state the residual risk.

### Output Format

When done, respond with:

1. A short summary of what changed.
2. The files changed.
3. The tests or checks run, with results.
4. Any behavior or compatibility risks that remain.

Do not claim the refactor is behavior-preserving unless tests or direct
comparison support that claim.

## Optional Scope Fill-Ins

Use one of these when you want a sharper task.

### Generator Refactor

```text
Target: generator package.
Problem: generation responsibilities are hard to follow because planning,
field dispatch, constraints, rules, expressions, and RNG behavior interact.
Success: isolate one responsibility without changing generated rows for
existing fixtures. Add characterization tests before moving logic.
Extra guardrail: do not alter RNG stream derivation or row ordering.
```

### Parser Refactor

```text
Target: parser package.
Problem: parsing logic is difficult to extend or diagnose.
Success: clarify parsing stages or error construction while preserving parsed
model output and location-aware errors.
Extra guardrail: do not loosen syntax acceptance unless explicitly requested.
```

### Output Writer Refactor

```text
Target: output package.
Problem: writer implementations duplicate formatting or entity filtering logic.
Success: share only the duplicated mechanics while keeping each format's
behavior and tests intact.
Extra guardrail: do not change field order, null handling, quoting, SQL
dialect behavior, or pretty-print output.
```

### CLI/REPL Refactor

```text
Target: cmd/datjit and/or repl packages.
Problem: command handling mixes UI concerns with service behavior.
Success: keep commands thin, move reusable behavior behind the service facade
or small unexported helpers, and preserve flags/help/output.
Extra guardrail: do not change command names, flag names, defaults, or exit
status behavior.
```

## Review Checklist

Before accepting a refactor, check:

- Is the diff limited to the requested target?
- Are package boundaries still clean?
- Did any exported API change?
- Is deterministic generation still protected by tests?
- Are typed errors still compatible with `errors.Is`/`errors.As`?
- Did the agent add characterization tests before changing risky behavior?
- Did `gofmt`, targeted tests, and full tests run?
- Is the explanation specific enough to audit the change quickly?
