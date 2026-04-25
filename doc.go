// Package datjit is the public entry point for the datjitgo synthetic data
// library. datjitgo turns a declarative YAML schema describing entities,
// fields, semantic types and inter-entity rules into rows of plausible — and,
// when seeded, byte-for-byte reproducible — synthetic data, suitable for
// tests, demos, ML fixtures and database seeding.
//
// # Layered API
//
// The package surface is layered so callers reach for the lightest tool that
// does the job:
//
//   - Service facade: [Service], built via [NewDefault] or [New] with
//     functional [Option]s ([WithSeed], [WithLocale], [WithVolume],
//     [WithCorpus], [WithLLMProvider], [WithWriter], ...). The Service
//     exposes [Service.Parse], [Service.Validate], [Service.Generate],
//     [Service.GenerateFile] and [Service.Write] for full pipeline control.
//   - One-call helpers: for app code that just wants data, the package-level
//     [GenerateMapFile] / [GenerateMapString], [GenerateRowsFile] /
//     [GenerateRowsString], [GenerateJSONFile] / [GenerateJSONString], and
//     [WriteFile] / [WriteJSONFile] helpers wire Service themselves.
//   - Runtime package: [github.com/jmcarbo/datjitgo/runtime] exposes the
//     same generation engine as embeddable operations — single-document,
//     single-entity, single-value — for DSL hosts and rule engines.
//   - Test helpers: [github.com/jmcarbo/datjitgo/datjittest] adds
//     testing.T-aware sugar (MustGenerate, MustRows, AssertGoldenJSON,
//     UpdateGoldenJSON) for writing concise tests.
//
// # Choosing an API
//
// Match the API to the caller:
//
//   - App code that just needs rows or JSON from a schema file or string →
//     reach for the root one-call helpers ([GenerateMapFile],
//     [GenerateRowsFile], [GenerateJSONString], etc.).
//   - Tests that should fail loud on the first error and want golden-file
//     assertions → use the datjittest package.
//   - Custom adapters (alternate parser, custom writer, swapped corpus,
//     live LLM provider) → construct a [Service] via [New] with the
//     relevant [Option]s.
//   - Embedding generation inside a DSL, rule engine, or other host
//     runtime → use the runtime package, which accepts pre-built
//     [github.com/jmcarbo/datjitgo/core/model].Document values and
//     per-call run options.
//
// # Determinism
//
// Generation is deterministic: pass [WithSeed] (or set seed in the schema /
// the runtime call) to get reproducible output across processes and machines.
//
// # See also
//
// The [github.com/jmcarbo/datjitgo/cmd/datjit] CLI is a thin wrapper around
// the Service facade and is the easiest way to try the library without
// writing Go.
package datjit
