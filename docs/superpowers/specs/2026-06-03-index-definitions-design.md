# Index definitions — manual declaration + automated inference

Status: design proposed
Date: 2026-06-03
Module: `github.com/periplon/datjitgo`
Implements: index metadata on entities, surfaced in SQL output as `CREATE INDEX`.

## 1. Goal

Let a schema carry **index definitions** so the `sql` output writer can emit
`CREATE INDEX` / `CREATE UNIQUE INDEX` statements alongside `CREATE TABLE`.

Two sources, one model:

1. **Manual** — author declares indexes per entity under a reserved `_indexes`
   key, using an **expanded mapping** form (index name → spec), parallel to the
   existing `_coherence` key.
2. **Automated** — an optional, opt-in inference pass derives indexes from
   schema signals (`@unique` fields, reference fields, polymorphic
   discriminators). Inference is gated and never changes default output.

Non-goals (this iteration): primary-key declaration syntax, foreign-key
constraint emission, covering/INCLUDE columns, index emission in non-SQL
formats, runtime use of indexes during generation (they are output-only
metadata).

## 2. Invariants honoured

- **Determinism**: index ordering and generated names are a pure function of
  the schema (declaration order, field order). No RNG. Same schema + same seed
  + same options → byte-identical SQL.
- **Public API stability**: additive only. New exported `model.Index` type, new
  `Entity.Indexes` field, new `ports.WriteOptions.SQLIndexes` field, and a
  mirrored new `datjit.WriteOpts.SQLIndexes` facade field (the facade struct at
  `service.go:37` mirrors the stable subset of `ports.WriteOptions`; `Service.Write`
  at `service.go:160` copies it into the `ports.WriteOptions` literal). Struct
  fields are keyed, so adding a field is source-compatible. No existing exported
  identifier renamed or removed.
- **Hexagonal direction**: `model.Index` lives in `core/model` (imports
  nothing). Parser/normalize/output adapters read it. Inference runs in the
  root `datjit` package's normalize step (same place as polymorphic
  normalization), not in `core/*`.
- **Backward compatibility**: default `SQLIndexes` mode is `manual`. No fixture
  declares `_indexes` today, so default SQL output is unchanged and all existing
  `output/sql_test.go` assertions keep passing.

## 3. Data model (`core/model`)

New type in `core/model/entity.go`:

```go
// Index is one index definition on an entity, surfaced in SQL output.
// Indexes are output-only metadata; they do not affect generation.
type Index struct {
	Name   string   // index name; for manual indexes this is the _indexes map key
	Fields []string // indexed columns, in order (must be ≥1)
	Unique bool     // emit CREATE UNIQUE INDEX
	Where  string   // optional partial-index predicate (postgres, sqlite)
	Method string   // optional access method: btree|hash|gin|gist (postgres, mysql)
	Source string   // provenance: "manual" | "inferred"
}
```

`Entity` gains an `Indexes []Index` field (declaration-ordered):

```go
type Entity struct {
	Name      string
	Meta      []Decorator
	Fields    *OrderedMap[string, *Field]
	Coherence *OrderedMap[string, []string]
	Indexes   []Index // new; nil when none declared/inferred
}
```

A slice (not `OrderedMap`) is used because index names need not be unique across
the *inferred* set before dedup, ordering is explicit, and downstream code only
iterates. `NewEntity` leaves it nil.

## 4. Manual DDL — expanded mapping syntax

Reserved entity key `_indexes`, a mapping of **index name → spec mapping**.
Mirrors `_coherence`'s structure (named groups), so the DDL stays consistent.
`_indexes` joins the parser's reserved-key set, today `{_meta, _coherence,
_triggers}` (`parser/yaml.go:273`), so it never leaks into the field map.

```yaml
entities:
  User:
    id: uuid @id
    email: email @unique
    org: ->Org
    created_at: datetime
    deleted_at: datetime?
    _indexes:
      by_email:
        fields: [email]
        unique: true
      by_org_recent:
        fields: [org, created_at]
        where: "deleted_at IS NULL"
        method: btree
```

Spec keys:

| key      | type        | required | default | notes |
|----------|-------------|----------|---------|-------|
| `fields` | list[str]   | yes      | —       | ≥1 entry; each must name a field of the entity |
| `unique` | bool        | no       | `false` | emits `CREATE UNIQUE INDEX` |
| `where`  | string      | no       | `""`    | raw predicate, emitted verbatim; postgres/sqlite only |
| `method` | string      | no       | `""`    | `USING <method>`; postgres/mysql only |

- The map **key is the index name** (`by_email`). Authors control naming; names
  are emitted as-is (after dialect identifier quoting/truncation). Two entries
  with the same key are a YAML quirk: the later silently overwrites the earlier
  before the parser sees them — not a schema error we can detect.
- `fields` accepts the YAML flow form `[a, b]` or block list form.
- `Source` is set to `"manual"` by the parser.
- Unknown spec keys (anything other than `fields`/`unique`/`where`/`method`) are
  rejected **at parse time**, in the same first pass that reads `_indexes` (the
  parser has the raw YAML node there; `_indexes` is never routed through
  `parseField`). The error uses `errors.KindValidation` but is raised by the
  parser, not `validateDoc`. So typos surface, with the YAML line location.

A short single-field flow scalar is **not** supported in this iteration; every
index is a named mapping. (Rationale: one syntax form to validate/teach.
Field-level `@index` shorthand is deferred — see §9.)

## 5. Automated inference

A normalize pass `normalizeIndexes(doc)` added to `normalize.go`, invoked from
`Service.Parse()` immediately after `normalizePolymorphicReferences(doc)` (so it
sees synthetic discriminator fields). Inference only **adds** indexes; it runs
for every parse but its results are filtered at emit time by `SQLIndexes` mode
(§7), so the model always carries both manual and inferred entries and the
writer decides what to emit. Each inferred index is tagged `Source:"inferred"`.

Heuristics, applied per entity in field-declaration order:

1. **`@unique` field** → unique single-column index.
   Name `idx_<table>_<field>_uniq`.
2. **Reference field** (`->X`, `<->X`, non-polymorphic) → non-unique index on
   the join column. Name `idx_<table>_<field>`.
3. **Polymorphic reference** → composite index on `(field, field_type)` where
   `field_type` is the synthetic discriminator. Name `idx_<table>_<field>`.
   (The discriminator alone is low-cardinality; the pair is the useful index.)

Skipped deliberately: primary-key / `@id` fields (assumed covered by a PK
constraint, which this iteration does not emit), coherence groups (query
patterns unknown → noisy).

**Dedup vs manual**: an inferred index is dropped if a manual index on the same
ordered `Fields` already exists (regardless of uniqueness/name). Manual always
wins. Dedup key: the ordered field-name slice joined by `\x00`. This runs inside
`normalizeIndexes` so the model never contains an inferred duplicate of a manual
index.

Inference is **pure**: derived only from field decorators/types and the
discriminator names, both already deterministic.

## 6. Validation (`validate.go`)

In `validateDoc`, per entity, after field checks add an index check loop.
**Only `Source=="manual"` indexes are validated** — the loop `continue`s on any
inferred index. Inferred indexes are correct by construction (their fields are
read from existing fields / the synthetic discriminator), so validating them
could only ever flag a `normalizeIndexes` bug, which is not a schema author's
error to fix. Errors use `errors.KindValidation` with `Entity` set (and `Field`
set to the offending column where applicable):

- `fields` empty → `entity %s index %q: needs at least one field`.
- field name not present in entity (the check runs after polymorphic normalize,
  so synthetic discriminator fields are present and count as valid) →
  `entity %s index %q: unknown field %q`.
- duplicate index **name** among an entity's *manual* indexes →
  `entity %s: duplicate index %q`.

A manual index whose ordered field set duplicates another index is **allowed**
(DBs permit it); not an error.

Ordering guarantee: `Service.Parse` runs `normalizePolymorphicReferences` then
`normalizeIndexes` (dedup included) before the document is returned, and
`Service.Validate` is called on that returned document — so by the time
`validateDoc` runs, inferred duplicates of manual indexes are already removed
and discriminator fields already exist. The dup-name check therefore sees only
manual indexes and cannot collide with an inferred name.

## 7. SQL emission (`output/sql.go`)

### 7.1 Mode gate — `WriteOptions.SQLIndexes`

New field on `core/ports.WriteOptions`:

```go
SQLIndexes string // "" | "none" | "manual" | "auto"
```

Resolution in `SQL.Write` (mirrors the existing `SQLDialect` switch):

| value             | emitted indexes |
|-------------------|-----------------|
| `""` (default)    | same as `manual` |
| `"none"`          | none (suppress all, incl. manual) |
| `"manual"`        | `Source=="manual"` only |
| `"auto"`          | `manual` ∪ `inferred` (deduped) |
| other             | `KindValidation` error `sql writer: unknown index mode %q` |

### 7.2 Emit

After `writeCreateTable` and before the entity's `INSERT`s (so DDL is grouped),
call `writeIndexes(buf, ent, name, dialect, mode)`. For each selected index, in
`Entity.Indexes` order:

```sql
CREATE UNIQUE INDEX "idx_user_email_uniq" ON "user" ("email");
CREATE INDEX "idx_user_org_created_at" ON "user" ("org", "created_at") USING btree WHERE deleted_at IS NULL;
```

- Identifiers quoted via existing `quoteSQLIdent(_, dialect)`.
- Column list quoted per column.
- `UNIQUE` keyword when `Index.Unique`.

### 7.3 Dialect matrix

| feature        | postgres | mysql | sqlite | behaviour when unsupported |
|----------------|----------|-------|--------|----------------------------|
| `CREATE [UNIQUE] INDEX name ON table (cols)` | ✓ | ✓ | ✓ | — |
| `USING <method>` | ✓ (after table) | ✓ | ✗ | sqlite: drop `method` silently |
| partial `WHERE` | ✓ | ✗ | ✓ | mysql: drop `where` silently |

MySQL `USING` placement: `CREATE INDEX name USING btree ON table (cols)` — note
the `USING` precedes `ON` in MySQL, whereas Postgres places `USING method` after
the column list. The writer branches on dialect for clause ordering.

Silent drops are acceptable (the index is still created, just without the
unsupported refinement) and keep output valid SQL. No warning channel exists in
the writer today; a drop does not corrupt results.

### 7.4 Name generation + truncation

Manual names come from the YAML key. Inferred names follow §5 templates,
lowercased. All names pass through `clampIdent(name, dialect)`:

- postgres limit 63 bytes, mysql 64, sqlite effectively unbounded (use 64).
- On overflow, truncate to `limit-9` and append `_` + the FNV-1a-32 hash of the
  full name, formatted as exactly 8 lowercase hex digits (`%08x`, zero-padded).
  Deterministic, collision-resistant enough for index names. Pure, no RNG.

## 8. CLI

`cmd/datjit/cmd_generate.go`: add `--sql-indexes` string flag (default
`"manual"`), plumb into the `datjit.WriteOpts` literal as `SQLIndexes` (the new
facade field from §2; `Service.Write` copies it into `ports.WriteOptions`).
Document in the command help: `manual|auto|none` (auto = include inferred
indexes).

Update `CLI surface` notes in `CLAUDE.md` generate flag list.

## 9. Deferred / future

- Field-level `@index` / `@index(unique)` decorator shorthand.
- Single-field flow scalar in `_indexes` (`by_email: email`).
- Primary-key declaration + emission; foreign-key constraints.
- Covering indexes (`INCLUDE`), expression indexes, descending columns.
- Index metadata in `inspect` output and non-SQL formats.

## 10. Definition of Done

- `model.Index` + `Entity.Indexes` (additive, documented).
- Parser handles `_indexes` expanded mapping; key reserved; unknown spec keys
  rejected; `Source="manual"`.
- `normalizeIndexes` inference pass wired after polymorphic normalize; deduped
  vs manual; `Source="inferred"`.
- `validateDoc` index checks (empty fields, unknown field, dup name).
- `ports.WriteOptions.SQLIndexes` + mirrored `datjit.WriteOpts.SQLIndexes` +
  resolution; `--sql-indexes` CLI flag.
- `writeIndexes` emits correct DDL for all three dialects incl. unique,
  composite, partial `WHERE`, `USING`, and name truncation.
- Tests: parser (`_indexes` round-trip, unknown-key error), validate (3 error
  cases), inference (unique/ref/polymorphic, dedup), sql emit (×3 dialects,
  unique, composite, partial, method, mode gate none/manual/auto, truncation).
- New fixture `testdata/fixtures/indexes.yaml` + regenerated json golden
  (indexes are SQL-only metadata, so json rows are unaffected; the fixture
  exercises parse + the new key without breaking `TestFixtures`).
- Docs: `CHANGELOG.md` `[Unreleased]` entry; `CLAUDE.md` DDL + CLI surface
  notes mentioning `_indexes` and `--sql-indexes`.
- `make ci` green (gofmt, lint, race tests, fixtures, build).
