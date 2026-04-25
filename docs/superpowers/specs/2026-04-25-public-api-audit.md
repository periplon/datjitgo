# datjitgo Public API Audit

Status: proposed
Date: 2026-04-25

## Stable Public Packages

- `github.com/jmcarbo/datjitgo`: main Service facade, options, helpers, error predicates.
- `github.com/jmcarbo/datjitgo/core/model`: schema and inspection model types.
- `github.com/jmcarbo/datjitgo/core/value`: generated value model.
- `github.com/jmcarbo/datjitgo/core/ports`: extension interfaces for parser, generator, writers, corpus, and LLM providers.
- `github.com/jmcarbo/datjitgo/core/errors`: typed parse, validation, generation, and corpus errors.
- `github.com/jmcarbo/datjitgo/datjittest`: testing helpers.
- `github.com/jmcarbo/datjitgo/runtime`: embeddable runtime for host DSLs and rule engines.

## Review Before Marking Public

- `github.com/jmcarbo/datjitgo/corpus`: useful for corpus overlays, but may expose default adapter details.
- `github.com/jmcarbo/datjitgo/llm`: useful for default LLM provider wiring, but provider contracts live in `core/ports`.

## Candidate Internal Packages

- `github.com/jmcarbo/datjitgo/parser`
- `github.com/jmcarbo/datjitgo/generator`
- `github.com/jmcarbo/datjitgo/output`
- `github.com/jmcarbo/datjitgo/core/plan`
- `github.com/jmcarbo/datjitgo/core/rules`

## Compatibility Rule

Do not move candidate packages under `internal/` until the project accepts a compatibility note and changelog entry. If adapter imports are considered supported API, keep the package directories public and document them as extension points instead.
