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
