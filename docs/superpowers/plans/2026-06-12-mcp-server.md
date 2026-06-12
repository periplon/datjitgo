# Plan: MCP server (`datjit mcp`)

Spec: `docs/superpowers/specs/2026-06-12-mcp-server-design.md`
Branch: `claude/feat-mcp-server`

## Steps

1. **RPC codec** (`mcp/rpc.go`): JSON-RPC 2.0 structs (`request`,
   `response`, `rpcError`), line-based read loop with 4 MiB cap, write with
   trailing `\n`. Unit tests for malformed input and notification handling.
2. **Tool layer** (`mcp/tools.go`): four tools per spec §3 — JSON Schema
   maps for `inputSchema`, dispatch functions over `datjit.Service` /
   `runtime`. Input caps (512 KiB schema, 100k rows, count ≤ 100).
3. **Server loop** (`mcp/server.go`): `Serve(ctx, in, out, Options{Version
   string})`; initialize/initialized/ping/tools handling; EOF → nil.
   Package godoc marks the package as not-yet-stable (like `repl`).
4. **CLI** (`cmd/datjit/cmd_mcp.go`): `datjit mcp` wiring stdio + version.
5. **Tests**: scripted lifecycle test, per-tool tests, determinism test
   (same seed twice → identical bytes).
6. **Docs**: README section (CLI table row + short "MCP server" section
   with a client config snippet), CHANGELOG Unreleased entry.
7. **Gate**: `make ci` green; no new dependencies in go.mod.

## Definition of done

- An MCP client speaking newline-delimited JSON-RPC can list and call all
  four tools and gets deterministic output for fixed seeds.
- `make ci` green, goldens untouched, `go.mod` unchanged.
