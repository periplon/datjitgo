# Polymorphic reference discriminator — plan & ledger

Date: 2026-06-02
Branch: `fix/polymorphic-discriminator`

## Bug

A polymorphic reference — a field whose type is a union of two or more entity
references, e.g. `owner: ->User | ->Org` — is parsed as `model.Union{Variants:
[Reference{User}, Reference{Org}]}`. At generation `generateUnion`
(`generator/field.go`) picks one variant uniformly and `generateReference`
returns a bare primary-key value (`generator/reference.go` →
`referenceValue`). The emitted value is therefore an **untyped foreign key**:
the consumer receives a real PK drawn from *one* of the targets with no
indication of *which* entity it points to, and no FK that could be reconstructed.
Confirmed: no discriminator, type tag, or companion metadata anywhere.

## Fix (chosen approach: sibling discriminator field)

For each polymorphic-reference field `f`, emit a companion scalar field
`f_type` holding the **target entity name** of the variant that was chosen for
that row. Works across all five output formats (json/csv/ndjson/yaml/sql)
because the companion is an ordinary string column. Deterministic: the same
seed picks the same variant, so PK and discriminator always agree.

```
owner:      "org-7c3..."     # PK drawn from Org
owner_type: "Org"            # discriminator
```

### Detection

`polymorphicUnion(t)` returns the union when `t` is a `Union` (or a
`Nullable` wrapping one) whose variants contain **≥2 `Reference` variants**.
A union with 0–1 references (e.g. `->A | string`, `string | int`) is *not*
polymorphic and gets no discriminator. Nested unions inside `List`/`Map`/
`Tuple` are out of scope (a single `_type` column cannot describe per-element
targets) — documented limitation.

### Model (core/model — additive, API-stable)

- `Field.Discriminator string` — on a polymorphic source field, the name of its
  companion discriminator field (empty otherwise).
- `Field.DiscriminatorFor string` — on the companion field, the source field
  name (empty otherwise).
- `OrderedMap.InsertAfter(after, k, v)` — insert keeping declaration order so
  `f_type` lands directly after `f`.

### Normalization (root `datjit` package, run inside `Parse`)

`normalizePolymorphicReferences(doc)`: for every entity field that is a
polymorphic union, choose a free companion name (`f_type`, then `f_type_2`, …
on collision), set `f.Discriminator`, and `InsertAfter` a synthetic string
`Field{Name: companion, Type: Primitive{PrimString}, DiscriminatorFor: f.Name}`.
Idempotent (skips if `f.Discriminator` already set). Runs in `Parse` so
Validate, Generate, Write and Inspect all see a consistent document. Applied to
entities and reusable `types:`.

### Generation (generator package)

- `generateRow`: skip companion fields (`DiscriminatorFor != ""`) in the
  per-field loop; their value is a side effect of the source field. Set null if
  still absent after the source ran (null ref → null discriminator).
- `generateUnion`: when `f.Discriminator != ""` and the chosen variant is a
  `Reference`, `row.Set(f.Discriminator, Str(target))` (resolving `self` →
  entity name); when the chosen variant is not a reference, leave it (loop sets
  null).

## Ledger

- [x] Investigate: confirmed bug, located all sites.
- [x] Decide fix shape (user: sibling discriminator field).
- [x] Worktree `../datjitgo-poly` + branch.
- [x] Write plan/ledger.
- [x] core/model: Field fields + OrderedMap.InsertAfter.
- [x] root: normalize.go + hook into Parse.
- [x] generator: generateUnion + generateRow skip.
- [x] Tests: normalize unit tests, OrderedMap.InsertAfter tests, generation
      determinism + integrity tests, polymorphic.yaml fixture + golden.
- [x] `make ci` green (exit 0).
- [ ] Self-review until no flaws.
- [ ] Commit, push, open PR.

## Verifications
- Manual generate (seed 42): `author` PK + `author_type` ∈ {User, Org};
  referential integrity holds (author_type matches the entity owning the PK).
- csv header + sql DDL/INSERT both carry `author_type`.
- `make ci` exit 0 (gofmt, golangci-lint, race tests, fixtures, build).

## Known limitations (documented, out of scope)
- Polymorphic unions nested inside `[T]`/`{K:V}`/tuples get no discriminator —
  a single `_type` column cannot describe per-element targets.
- Programmatically constructed documents that bypass `Service.Parse` are not
  normalized; they retain the bare-PK behavior unless normalized explicitly.
- Pre-existing (NOT introduced here): `make test-update` uses `PKG=./...` and
  passes `-update` to the `runtime` package, which does not define that flag, so
  the target exits non-zero after the root golden is written. Left untouched.

## Review notes

(filled during review passes)
