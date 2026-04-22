# datjitgo

Go port of [datjit](https://github.com/jmcarbo/datjit) — declarative synthetic data generation.

Define a data domain in YAML. Get realistic test data in JSON, CSV, SQL, YAML, or NDJSON.

## Status

Phase 1 (MVP): core library, generator, output writers, embedded corpus, CLI, REPL. LLM-backed generation and live corpus downloaders are deferred to later phases.

Design: [`docs/superpowers/specs/2026-04-22-datjitgo-design.md`](docs/superpowers/specs/2026-04-22-datjitgo-design.md)
Plan: [`docs/superpowers/plans/2026-04-22-datjitgo-phase1.md`](docs/superpowers/plans/2026-04-22-datjitgo-phase1.md)

## Quick start

```bash
go install github.com/jmcarbo/datjitgo/cmd/datjit@latest
datjit generate schema.yaml --seed 42 -f json --pretty
```

Library:

```go
import "github.com/jmcarbo/datjitgo"

svc := datjit.NewDefault()
doc, _ := svc.Parse(f, "schema.yaml")
ds,  _ := svc.Generate(doc)
_ = svc.Write(ds, doc, "json", os.Stdout, datjit.WriteOpts{Pretty: true})
```

## License

MIT
