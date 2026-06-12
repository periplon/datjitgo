# Plan: schema introspection (export / diff / deps)

Spec: `docs/superpowers/specs/2026-06-12-schema-introspection-design.md`
Branch: `claude/feat-schema-introspection`

## Steps

1. **Model types.** Add `SchemaSummary`, `EntitySummary`, `FieldSummary`,
   `EnumSummary`, `VolumeSummary`, `SchemaDiff`, `SchemaChange`,
   `DependencyGraph`, `DepEdge` to `core/model` (new file
   `core/model/introspect.go`), with godoc on every exported identifier.
2. **Canonical rendering.** Locate existing TypeExpr/decorator string
   rendering (check `inspect.go`, `core/model`); reuse or add a
   deterministic renderer. Unit-test rendering for every TypeExpr kind
   (primitive, semantic w/ params, enum inline, named, reference,
   many-to-many, list, map, tuple, nullable, union/polymorphic).
3. **Facade.** `Service.SchemaSummary`, `Service.DependencyGraph`,
   `DiffSchemaSummaries` in a new root file `introspect.go` (root package
   `datjit`). Diff is pure comparison; no go-cmp dependency in non-test code.
4. **Cycle paths.** Extend `core/plan` topo-sort to return exemplar cycle
   paths; thread into `DependencyGraph.Cycles` and the cyclic-dependency
   validation error message. Keep the success path behavior identical.
5. **CLI.** `cmd/datjit/cmd_schema.go` with `export|diff|deps` per spec;
   tests alongside existing command tests.
6. **Tests + docs.** Table-driven diff tests, determinism test (export
   twice), godoc example, README CLI table row, CHANGELOG Unreleased entry.
7. **Gate.** `make ci` green; goldens untouched.

## Definition of done

- All spec §2/§3/§4 surface implemented and tested.
- `make ci` passes with zero golden drift.
- No new module dependencies.
