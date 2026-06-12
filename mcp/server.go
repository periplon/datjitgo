// Package mcp implements a Model Context Protocol server over stdio for the
// datjit pipeline. It speaks newline-delimited JSON-RPC 2.0 (one message per
// line, no Content-Length framing) and exposes four tools — generate, validate,
// inspect, and sample — so AI coding agents can synthesize deterministic
// fixtures on demand. The protocol layer is hand-rolled with the standard
// library; the package depends only on the root datjit façade, the runtime
// package, and the stable core/model types.
//
// This package is NOT yet part of the stable public API surface (same status
// as the repl package): its exported identifiers may change without notice.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	datjit "github.com/periplon/datjitgo"
	djruntime "github.com/periplon/datjitgo/runtime"
)

// supportedProtocolVersions are the MCP protocol revisions this server knows.
// During initialize the client's requested version is echoed back when it is
// one of these; otherwise defaultProtocolVersion is returned.
var supportedProtocolVersions = map[string]struct{}{
	"2025-03-26": {},
	"2024-11-05": {},
	"2025-06-18": {},
}

// defaultProtocolVersion is returned when the client requests an unknown
// protocol version.
const defaultProtocolVersion = "2025-03-26"

// Options configures a Serve call.
type Options struct {
	// Version is the build version string reported in initialize's serverInfo.
	Version string
}

// server holds the per-connection state for one Serve invocation.
type server struct {
	svc  *datjit.Service
	rt   djruntime.Runtime
	reg  *registry
	opts Options
}

// Serve runs the MCP server loop, reading newline-delimited JSON-RPC requests
// from in and writing responses to out. It returns when in reaches EOF (clean
// exit) or ctx is cancelled. Malformed lines and oversized lines yield a parse
// error response rather than aborting the loop.
func Serve(ctx context.Context, in io.Reader, out io.Writer, opts Options) error {
	srv := &server{
		svc:  datjit.NewDefault(),
		rt:   djruntime.NewDefault(),
		reg:  newRegistry(),
		opts: opts,
	}
	lr := newLineReader(in)
	for {
		if ctx.Err() != nil {
			return nil
		}
		line, err := lr.readLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			if errors.Is(err, errLineTooLong) {
				if werr := writeMessage(out, newErrorResponse(nil, codeParse, "request exceeds 4 MiB limit")); werr != nil {
					return werr
				}
				continue
			}
			return err
		}
		if len(line) == 0 {
			continue
		}
		if werr := srv.handleLine(ctx, out, line); werr != nil {
			return werr
		}
	}
}

// handleLine decodes one line and dispatches it, writing any response to out.
// It returns a non-nil error only on a write failure (which is fatal to the
// loop); protocol problems are written as error responses.
func (s *server) handleLine(ctx context.Context, out io.Writer, line []byte) error {
	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		// A JSON array is a batch request: valid JSON, unsupported envelope —
		// invalid request, not a parse error.
		if isBatch(line) {
			return writeMessage(out, newErrorResponse(nil, codeInvalidRequest, "batch requests are not supported"))
		}
		return writeMessage(out, newErrorResponse(nil, codeParse, "parse error: "+err.Error()))
	}
	if req.JSONRPC != "2.0" {
		return writeMessage(out, newErrorResponse(req.ID, codeInvalidRequest, `jsonrpc must be "2.0"`))
	}

	resp, send := s.dispatch(ctx, &req)
	if !send {
		return nil
	}
	return writeMessage(out, resp)
}

// dispatch routes a request to its method handler. The second return value
// reports whether a response should be written (false for notifications).
func (s *server) dispatch(ctx context.Context, req *request) (response, bool) {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "notifications/initialized", "notifications/cancelled":
		// Notifications never receive a response.
		return response{}, false
	case "ping":
		resp, err := newResultResponse(req.ID, map[string]any{})
		if err != nil {
			return newErrorResponse(req.ID, codeInternal, err.Error()), true
		}
		return resp, !req.isNotification()
	case "tools/list":
		resp, err := newResultResponse(req.ID, map[string]any{"tools": s.reg.list()})
		if err != nil {
			return newErrorResponse(req.ID, codeInternal, err.Error()), true
		}
		return resp, !req.isNotification()
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	default:
		if req.isNotification() {
			return response{}, false
		}
		return newErrorResponse(req.ID, codeMethodNotFound, "method not found: "+req.Method), true
	}
}

// handleInitialize answers the initialize handshake, echoing a known protocol
// version or falling back to the default.
func (s *server) handleInitialize(req *request) (response, bool) {
	version := defaultProtocolVersion
	if len(req.Params) > 0 {
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if err := json.Unmarshal(req.Params, &p); err == nil && p.ProtocolVersion != "" {
			if _, ok := supportedProtocolVersions[p.ProtocolVersion]; ok {
				version = p.ProtocolVersion
			}
		}
	}
	result := map[string]any{
		"protocolVersion": version,
		"capabilities":    map[string]any{"tools": map[string]any{}},
		"serverInfo": map[string]any{
			"name":    "datjit",
			"version": s.opts.Version,
		},
	}
	resp, err := newResultResponse(req.ID, result)
	if err != nil {
		return newErrorResponse(req.ID, codeInternal, err.Error()), true
	}
	return resp, !req.isNotification()
}

// toolCallParams is the decoded params for a tools/call request.
type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// handleToolsCall validates the tool name and dispatches to its handler. Tool
// failures (*toolError) become isError:true results; unknown tools and other
// protocol problems become JSON-RPC errors.
func (s *server) handleToolsCall(ctx context.Context, req *request) (response, bool) {
	if req.isNotification() {
		// Notifications never receive a response; tools are pure, so skipping
		// execution entirely is observably identical to running and discarding.
		return response{}, false
	}
	var p toolCallParams
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return newErrorResponse(req.ID, codeInvalidParams, "invalid params: "+err.Error()), true
		}
	}
	if len(p.Arguments) > 0 && !isJSONObject(p.Arguments) {
		return newErrorResponse(req.ID, codeInvalidParams, "invalid params: arguments must be an object"), true
	}
	if p.Name == "" {
		return newErrorResponse(req.ID, codeInvalidParams, "tools/call: missing tool name"), true
	}
	tool := s.reg.lookup(p.Name)
	if tool == nil {
		return newErrorResponse(req.ID, codeMethodNotFound, "unknown tool: "+p.Name), true
	}

	text, err := tool.handle(ctx, s.svc, s.rt, p.Arguments)
	if err != nil {
		var te *toolError
		if errors.As(err, &te) {
			return s.toolResult(req.ID, te.msg, true)
		}
		return newErrorResponse(req.ID, codeInternal, err.Error()), true
	}
	return s.toolResult(req.ID, text, false)
}

// toolResult builds a tools/call result envelope wrapping text in a single
// text content block, with isError set as given.
func (s *server) toolResult(id json.RawMessage, text string, isError bool) (response, bool) {
	result := map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": text},
		},
		"isError": isError,
	}
	resp, err := newResultResponse(id, result)
	if err != nil {
		return newErrorResponse(id, codeInternal, err.Error()), true
	}
	return resp, true
}
