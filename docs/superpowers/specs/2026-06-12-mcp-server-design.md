# MCP server: `datjit mcp`

Status: approved for autonomous implementation
Date: 2026-06-12
Implements: killer-list #2 (R2-1) in `docs/enhancements-round2.md`.

## 1. Goal

Expose the existing parse→validate→generate pipeline to AI coding agents via
the Model Context Protocol: `datjit mcp` runs an MCP server over stdio so
agents can synthesize deterministic fixtures on demand. No network listener.
No new module dependencies — the protocol layer is hand-rolled JSON-RPC 2.0.

## 2. Protocol scope

Transport: stdio, newline-delimited JSON-RPC 2.0 (one message per line, no
Content-Length framing — per MCP stdio transport).

Methods handled:

- `initialize` → respond with `protocolVersion` (echo the client's requested
  version if it is one we know, else `"2025-03-26"`), `capabilities:
  {tools: {}}`, and `serverInfo: {name: "datjit", version: <build version>}`.
- `notifications/initialized` → no response (notification).
- `ping` → `{}`.
- `tools/list` → the four tools below with JSON Schema `inputSchema`s.
- `tools/call` → dispatch; tool-level failures return
  `{content: [{type:"text", text:<message>}], isError: true}` (JSON-RPC
  errors are reserved for protocol problems: unknown method/tool, invalid
  params, parse errors).
- Unknown methods → JSON-RPC error `-32601`. Malformed JSON → `-32700`.
  Requests with `id` get responses; notifications never do.

Shutdown: EOF on stdin exits 0. The loop must tolerate interleaved
notifications and oversized lines (cap line length at 4 MiB → `-32700`).

## 3. Tools

All schema inputs are YAML strings (the DDL), capped at 512 KiB. All
generation is seeded and offline (LLM stub only — `--llm-live` is
deliberately NOT exposed over MCP).

1. **`generate`** — `{schema (string, required), format ("json"|"csv"|
   "ndjson"|"yaml"|"sql", default "json"), seed (integer), entity (string),
   volumes (object: entity → integer), pretty (boolean), sql_dialect
   ("postgres"|"mysql"|"sqlite")}`.
   Total requested volume capped at 100 000 rows (tool error above).
   Returns generated output as one text content block.
2. **`validate`** — `{schema (string, required)}`. Returns `"schema is
   valid (N entities)"` or the validation/parse error text with location;
   invalid schema is a *successful* tool call with the diagnostic text and
   `isError: false` (the tool answered the question), unless the input is
   not YAML at all → still a normal diagnostic. Keep it simple: always
   `isError: false` for validate.
3. **`inspect`** — `{schema (string, required)}`. Returns the inspection
   summary (entities, field counts, rules, volumes) as pretty JSON text.
   Reuse `Service.Inspect`.
4. **`sample`** — `{semantic (string, required), count (integer 1..100,
   default 5), seed (integer)}`. Returns a JSON array of sampled values for
   one semantic type (e.g. `"email"`, `"person.full"`), via the `runtime`
   package's `GenerateValue` with per-index seeds derived from the request
   seed (deterministic: same seed → same array).

Tool descriptions are written to teach the DDL: `generate`'s description
includes a minimal schema example, so agents can self-serve syntax.

## 4. Package layout

New top-level package `mcp` (mirrors `repl`'s position: depends on the root
`datjit` facade, `runtime`, and the stable `core/model` types — allowed per
the architecture; no adapter imports):

```
mcp/
├── server.go      // Serve(ctx, in io.Reader, out io.Writer, opts Options) error
├── rpc.go         // JSON-RPC 2.0 line codec (request/response/error structs)
├── tools.go       // tool registry: schemas, dispatch
└── *_test.go
```

`cmd/datjit/cmd_mcp.go` adds the `mcp` subcommand (no flags in v1) wiring
stdin/stdout and the build version string.

The `mcp` package is NOT part of the stable public surface yet (same status
as `repl`); say so in the package godoc.

## 5. Determinism

- Same tool call with the same `seed` returns byte-identical text.
- Calls without `seed` default to seed 0 — NOT time-based — so agent
  retries are reproducible; the tool descriptions say "pass seed to vary".

## 6. Tests

- RPC codec: malformed JSON, notification vs request, unknown method.
- Lifecycle: scripted stdin (initialize → initialized → tools/list →
  tools/call ×N → EOF) asserting full response stream, REPL-test style.
- Each tool: happy path + error path (invalid schema text for generate,
  unknown semantic for sample, volume cap exceeded).
- Determinism: generate twice with same seed → identical; different seeds →
  different.
- `make ci` green; no goldens involved.

## 7. Non-goals

- No HTTP/SSE transport, no resources/prompts capabilities, no `--llm-live`,
  no corpus-dir overlay exposure (follow-ups if demanded).
