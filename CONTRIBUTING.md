# Contributing to datjitgo

Thanks for your interest in contributing. This document is intentionally short
so it stays accurate. Please skim it before opening a PR.

## Prerequisites

- Go matching the version in [`go.mod`](go.mod) (`go` directive).
- [`golangci-lint`](https://golangci-lint.run/) installed locally.

## Workflow

`make ci` is the gate. Run it from the repository root before pushing:

```bash
make ci
```

CI runs the same target. If `make ci` is green locally, your PR has a good
chance of passing.

## Commit style

Commit messages use the form `type: subject`, lowercase, present tense.
Examples:

- `feat: add X`
- `fix: handle Y`
- `docs: update Z`
- `test: cover edge case in W`
- `chore: bump dependency`
- `refactor: simplify generator pipeline`

## Specs and plans

Design specs live under [`docs/superpowers/specs/`](docs/superpowers/specs/) and
implementation plans live under [`docs/superpowers/plans/`](docs/superpowers/plans/).
New behavior should reference an existing spec or land alongside a new one.

## Pull requests

- One workstream per PR.
- Reference the spec or plan it implements.
- Do not break golden fixtures. If a change is intentional, regenerate the
  goldens explicitly and call that out in the PR description.

## Determinism contract

`datjitgo` is deterministic by design. When generating data in tests or
examples, always use `WithSeed` (or the equivalent `--seed` flag on the CLI) so
output is reproducible.
