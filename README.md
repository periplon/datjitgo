# datjitgo

Synthetic data generation from declarative YAML schemas — Go port of
[datjit](https://github.com/jmcarbo/datjit).

Define your data domain in a compact DDL (Data Domain Language), get
realistic test data in JSON, CSV, SQL, YAML or NDJSON. Deterministic by
default (seeded). Designed for embedding in Go applications (hexagonal
architecture with port interfaces) or invoking from the CLI / REPL.

## Install

```bash
go install github.com/jmcarbo/datjitgo/cmd/datjit@latest
```

Requires Go 1.26.2, matching the module's `go` directive.

## Quick start

```yaml
# schema.yaml
domain: my_app
seed: 42

volume:
  User: 100

entities:
  User:
    id: uuid @primary
    name: person.full
    email: email @unique
    age: int @range(18..65)
    active: bool
```

```bash
datjit generate schema.yaml                     # JSON to stdout
datjit generate schema.yaml -f csv -o out.csv   # CSV to file
datjit generate schema.yaml -f sql              # SQL INSERTs (Postgres dialect)
datjit generate schema.yaml --seed 42 --pretty  # deterministic, pretty-printed
```

## Library use

```go
import (
    "os"
    "github.com/jmcarbo/datjitgo"
)

svc := datjit.NewDefault()

f, _   := os.Open("schema.yaml")
doc, _ := svc.Parse(f, "schema.yaml")

if err := svc.Validate(doc); err != nil {
    panic(err)
}

ds, _ := svc.Generate(doc)
_ = svc.Write(ds, doc, "json", os.Stdout, datjit.WriteOpts{Pretty: true})
```

All adapters are swappable through functional options:

```go
svc, _ := datjit.New(
    datjit.WithSeed(42),
    datjit.WithLocale("en-US"),
    datjit.WithVolume(map[string]int{"User": 500, "Order": 2000}),
    datjit.WithCorpus(myCustomCorpus),       // implements ports.CorpusProvider
    datjit.WithLLMProvider(myLLMProvider),   // implements ports.LLMProvider
    datjit.WithWriter(myProtoWriter),         // implements ports.Writer
)
```

## CLI reference

| Command                                        | Purpose                                            |
|------------------------------------------------|----------------------------------------------------|
| `datjit generate <schema> [flags]`             | Generate data                                      |
| `datjit validate <schema>`                     | Parse + validate, exit 1 on error                  |
| `datjit inspect  <schema> [--infer-tools]`     | Print entity/field/rule summary                    |
| `datjit corpus list \| info \| update`         | Inspect or refresh embedded/overlay corpus data    |
| `datjit repl [<schema>]`                       | Interactive session                                |
| `datjit version`                               | Print version                                      |

### `generate` flags

| Flag                             | Default    | Meaning                                  |
|----------------------------------|------------|------------------------------------------|
| `-o`, `--output PATH`            | `stdout`   | Output destination                       |
| `-f`, `--format FMT`             | `json`     | `json \| csv \| ndjson \| yaml \| sql`   |
| `--seed N`                       | *schema*   | Override deterministic seed              |
| `--locale BCP47`                 | *schema*   | Override locale                          |
| `--volume Entity=N,...`          | *schema*   | Per-entity volume overrides              |
| `--entity NAME`                  | *all*      | Emit only this entity (deps still gen'd) |
| `--sql-dialect D`                | `postgres` | `postgres \| mysql \| sqlite`            |
| `--pretty`                       | `false`    | 2-space indent for JSON/YAML             |
| `--dry-run`                      | `false`    | Plan only, do not generate               |
| `--corpus-dir DIR`               | embedded   | Use on-disk corpus overlay               |
| `--llm-live`                     | `false`    | Call configured live LLM provider        |

## REPL tour

```text
$ datjit repl
datjit> load schema.yaml
loaded schema.yaml (domain=my_app, entities=1)
datjit[my_app]> set seed 42
datjit[my_app]> set format csv
datjit[my_app]> set volume User=10
datjit[my_app]> generate
id,name,email,age,active
…
datjit[my_app]> inspect
…
datjit[my_app]> exit
```

Full command list: `help`. Tab completion is on for commands, formats, and
the currently-loaded entity names.

## DDL summary

The DDL covers primitives, semantic types (`person.full`, `email`,
`address.city`, …), enums (with weighted distributions), references
(`->User`, `<->Tag`), compound types (`[T]`, `{K: V}`, `T?`, `T | U`),
distributions (`@dist(normal, μ=35, σ=12)`, Zipf, lognormal, weighted
categorical, …), pattern templates (`@pattern("SKU-{AA}-{0000}")`),
`@derived` / `@compute` / `@default_chain` expressions, cross-entity rules
(`@strict`, `@probability(p)`, `@warn`), and coherence groups.

Full language spec: [`docs/superpowers/specs/2026-04-22-datjitgo-design.md`](docs/superpowers/specs/2026-04-22-datjitgo-design.md)

## Architecture

Hexagonal; each adapter implements a port defined in `core/ports`:

```
cmd/datjit   CLI (Cobra)
repl         interactive REPL (chzyer/readline)
datjit       Service facade + Options
────────────────────────────────────────────── ports
core/model   Document, Entity, Field, TypeExpr
core/value   Value, Dataset (ordered)
core/errors  typed Error + sentinels
────────────────────────────────────────────── adapters
parser       YAML + DDL type/decorator parser
generator    engine, plan, primitives, distributions, expr, rules
output       json, ndjson, csv, yaml, sql writers
corpus       embedded name/email/address data
```

Dependencies point inward: adapters depend on `core`, `core` depends on
nothing internal.

## Live integrations

By default, `@llm`, `@llm_values`, and entity-level `_meta @llm(...)` use a
deterministic offline stub so fixture output remains reproducible. Opt into
network calls with `datjit.WithLLMProvider(...)` or CLI `--llm-live`. Built-in
HTTP support covers OpenAI-compatible endpoints (`openai`, `lmstudio`, `vllm`)
and Ollama.

Corpus overlays are JSON arrays of strings or `{ "name": "...", "weight": n }`
objects. Use `datjit corpus update --source key=url --corpus-dir DIR` to
download and validate overlay files, then pass `--corpus-dir DIR` to generation.

## Testing

```bash
make ci                                      # gofmt check, vet, race tests, fixtures, build
go test -race -count=1 ./...                 # race-enabled test suite
go test -count=1 -run TestFixtures ./...     # fixture golden drift check
```

Every fixture from the Rust tree is mirrored under `testdata/fixtures/` with a
matching golden JSON under `testdata/golden/`.

## License

MIT
