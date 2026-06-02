# Coherence shadows primary key for FK resolution

Status: done
Branch: `fix/coherence-fk-pk-shadow`
Date: 2026-06-02

## Problem (rationalization)

Foreign-key resolution in `generator` resolves a reference to the target row's
**positional first field**, assuming it is the primary key:

```go
// generator/reference.go
func firstField(row *value.Object) value.Value {
	keys := row.Keys()
	if len(keys) == 0 { return value.Null() }
	v, _ := row.Get(keys[0])   // keys[0] == "assumed primary key"
	return v
}
```

`@primary` is accepted in the DDL but is **inert** — no code in `parser` or
`generator` consults it for FK resolution (only `generator/llm_stub.go` treats
it as a value-producing decorator). FK identity is therefore decided purely by
**insertion order** into `value.Object`.

Row insertion order is set in `generateRow` (`generator/field.go`):

1. `applyCoherence` runs **first** and `row.Set(...)`s every coherence member
   field.
2. Then fields are generated in declaration order, skipping coherence members.

`value.Object.Set` appends a key on first write (insertion order preserved).
So when an entity that is an **FK target** also carries a coherence group whose
members are **not** the primary key, the coherence fields occupy `keys[0..]`
and the real `@primary id` lands later. `firstField` then returns a coherence
value, and every `->ThatEntity` / `<->ThatEntity` reference points at the wrong
column.

The shipped design example only puts coherence on standalone / source entities
(`coherence_groups.yaml`: Office, Employee self-refs), so the collision is
real but unexercised by fixtures.

Secondary site: `generator/expr.go` `firstListOrEntity` resolves a bare
single-segment entity path (`Entity` used in an expression function) via the
same `firstField`, inheriting the identical positional assumption.

## Fix

Make FK / entity-path resolution honour the `@primary` decorator explicitly,
falling back to positional first-field only when no field is marked `@primary`
(backward compatible — all current fixtures declare `id @primary` first, so
their goldens are unchanged).

- `generator/primarykey.go`: `primaryKeyField(ent)`, `primaryKeyMap(doc)`,
  `referenceValue(row, pkField)` — resolve PK by decorator, position fallback.
- `generationState.pk` + `evalEnv.pk`: entity → pk-field-name, precomputed once
  from the document; threaded into `reference.go`, `expr.go`, `derived.go`,
  `rules.go`.
- `reference.go` / `expr.go`: resolve via `referenceValue(row, pk[target])`
  instead of bare `firstField`.

## Definition of Done

- [x] Root cause documented (this file).
- [x] FK resolution resolves the target entity's `@primary` field by decorator.
- [x] Positional first-field fallback retained when no `@primary` exists.
- [x] `expr.go` bare-entity path resolution consistent with FK resolution.
- [x] Regression test: coherence on an FK-target entity; FK values equal the
      target rows' `@primary` values (fails before fix, passes after) —
      negative control confirmed FK resolved to `"Albuquerque, Arizona"`
      (coherence city/state) without the fix.
- [x] Determinism preserved — only the field read from the chosen row changed;
      RNG call sequence unchanged. Race tests + goldens green.
- [x] All existing golden fixtures unchanged — `make test-fixtures` green
      (every fixture declares `id @primary` first, so resolution is identical).
- [x] `make ci` green (gofmt, lint 0 issues, race tests, fixtures, build).
- [x] Self-review pass + independent reviewer pass (separate lane) — no flaws.

## Ledger

| # | Step | Result |
|---|------|--------|
| 1 | Verified claim 2 against code (firstField positional, coherence-first) | confirmed root cause |
| 2 | Created worktree `../datjitgo-fk-fix`, branch `fix/coherence-fk-pk-shadow` | done |
| 3 | Wrote rationalization + DOD + ledger | done |
| 4 | Added `generator/primarykey.go` (primaryKeyField/Map, referenceValue) | done |
| 5 | Threaded `pk` map through generationState, evalEnv, evalRule, reference.go, expr.go, derived.go | done |
| 6 | Added regression test `TestReferenceResolvesPrimaryKeyUnderCoherence` | passes |
| 7 | Negative control: neutered referenceValue → test fails with FK = "Albuquerque, Arizona" | confirmed guard |
| 8 | `make ci` — lint 0 issues, race tests pass, goldens unchanged, build OK | green |
| 9 | Independent reviewer pass on full diff | no flaws |
