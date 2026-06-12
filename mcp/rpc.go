package mcp

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
)

// JSON-RPC 2.0 error codes used by the server. Protocol-level problems map to
// these; tool-level failures are reported inside a successful result with
// isError:true (see tools.go) rather than as RPC errors.
const (
	// codeParse signals malformed JSON or an oversized line (-32700).
	codeParse = -32700
	// codeInvalidRequest signals a structurally invalid JSON-RPC envelope.
	codeInvalidRequest = -32600
	// codeMethodNotFound signals an unknown method or tool (-32601).
	codeMethodNotFound = -32601
	// codeInvalidParams signals params that fail tool/method validation.
	codeInvalidParams = -32602
	// codeInternal signals an unexpected server-side failure.
	codeInternal = -32603
)

// maxLineBytes caps a single newline-delimited JSON-RPC message at 4 MiB.
// Lines longer than this are rejected with a parse error per the MCP stdio
// transport contract.
const maxLineBytes = 4 << 20

// request is a decoded JSON-RPC 2.0 request or notification. A request with a
// nil ID is a notification and never receives a response.
type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// isNotification reports whether the request carries no id and must therefore
// not be answered.
func (r *request) isNotification() bool {
	return len(r.ID) == 0
}

// response is a JSON-RPC 2.0 response envelope. Exactly one of Result or Error
// is populated. ID echoes the request id (null for errors with no known id).
type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// rpcError is the JSON-RPC 2.0 error object.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// errLineTooLong is returned by readLine when a message exceeds maxLineBytes.
var errLineTooLong = errors.New("mcp: line exceeds 4 MiB cap")

// lineReader reads newline-delimited messages from an underlying stream while
// enforcing the maxLineBytes cap. It is a thin wrapper over bufio.Reader so the
// server loop can distinguish EOF, oversized lines, and decode errors.
type lineReader struct {
	br *bufio.Reader
}

// newLineReader wraps r with the 4 MiB-capped line reader.
func newLineReader(r io.Reader) *lineReader {
	return &lineReader{br: bufio.NewReaderSize(r, 64*1024)}
}

// readLine returns the next line without its trailing newline. It returns
// io.EOF when the stream is exhausted with no pending bytes, and
// errLineTooLong when a line exceeds the cap (the offending bytes are drained
// so the loop can continue). A final unterminated line is returned as-is.
func (lr *lineReader) readLine() ([]byte, error) {
	var buf []byte
	for {
		chunk, err := lr.br.ReadSlice('\n')
		if len(buf)+len(chunk) > maxLineBytes {
			// Drain the rest of the over-long line so the next read starts
			// cleanly on the following message.
			for err == bufio.ErrBufferFull {
				_, err = lr.br.ReadSlice('\n')
			}
			return nil, errLineTooLong
		}
		buf = append(buf, chunk...)
		if err == nil {
			return trimNewline(buf), nil
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		if errors.Is(err, io.EOF) {
			if len(buf) == 0 {
				return nil, io.EOF
			}
			return trimNewline(buf), nil
		}
		return nil, err
	}
}

// trimNewline strips a single trailing "\n" and an optional preceding "\r" so
// CRLF-terminated input is handled.
func trimNewline(b []byte) []byte {
	if n := len(b); n > 0 && b[n-1] == '\n' {
		b = b[:n-1]
		if n := len(b); n > 0 && b[n-1] == '\r' {
			b = b[:n-1]
		}
	}
	return b
}

// isBatch reports whether the line's first non-space byte opens a JSON array
// (a JSON-RPC batch, which this server does not support).
func isBatch(line []byte) bool {
	for _, b := range line {
		switch b {
		case ' ', '\t', '\r', '\n':
			continue
		default:
			return b == '['
		}
	}
	return false
}

// isJSONObject reports whether raw's first non-space byte opens an object.
// JSON null is also accepted: clients may send "arguments": null for a tool
// that needs no arguments.
func isJSONObject(raw json.RawMessage) bool {
	for _, b := range raw {
		switch b {
		case ' ', '\t', '\r', '\n':
			continue
		case '{', 'n':
			return true
		default:
			return false
		}
	}
	return true
}

// writeMessage marshals v and writes it as one newline-terminated line to w.
func writeMessage(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

// newErrorResponse builds an error response for the given id (which may be nil
// for a parse error with no recoverable id).
func newErrorResponse(id json.RawMessage, code int, msg string) response {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	return response{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}}
}

// newResultResponse builds a success response carrying the marshaled result.
func newResultResponse(id json.RawMessage, result any) (response, error) {
	raw, err := json.Marshal(result)
	if err != nil {
		return response{}, err
	}
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	return response{JSONRPC: "2.0", ID: id, Result: raw}, nil
}
