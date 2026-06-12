package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// minimalSchema is a small valid DDL schema used across the tool tests.
const minimalSchema = "domain: demo\n" +
	"volume:\n  User: 3\n" +
	"entities:\n" +
	"  User:\n" +
	"    id: uuid @primary\n" +
	"    name: person.full\n" +
	"    email: email\n"

// rawResp is a decoded response with the result kept as a generic map so tests
// can reach into content/isError without bespoke structs.
type rawResp struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
}

// runScript pipes the given JSON-RPC lines through Serve and returns the
// decoded response stream (one per non-notification request).
func runScript(t *testing.T, lines ...string) []rawResp {
	t.Helper()
	in := strings.NewReader(strings.Join(lines, "\n") + "\n")
	var out bytes.Buffer
	if err := Serve(context.Background(), in, &out, Options{Version: "test"}); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	return decodeStream(t, out.String())
}

// decodeStream splits the newline-delimited output into decoded responses.
func decodeStream(t *testing.T, s string) []rawResp {
	t.Helper()
	var resps []rawResp
	for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		if line == "" {
			continue
		}
		var r rawResp
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("decode %q: %v", line, err)
		}
		if r.JSONRPC != "2.0" {
			t.Fatalf("response missing jsonrpc 2.0: %q", line)
		}
		resps = append(resps, r)
	}
	return resps
}

// toolText extracts the first text content block and isError flag from a
// tools/call result.
func toolText(t *testing.T, r rawResp) (string, bool) {
	t.Helper()
	if r.Error != nil {
		t.Fatalf("expected tool result, got rpc error %+v", r.Error)
	}
	var res struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(r.Result, &res); err != nil {
		t.Fatalf("decode tool result: %v", err)
	}
	if len(res.Content) == 0 {
		t.Fatalf("empty content in %s", r.Result)
	}
	return res.Content[0].Text, res.IsError
}

func TestLifecycle(t *testing.T) {
	resps := runScript(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"validate","arguments":{"schema":`+jsonStr(minimalSchema)+`}}}`,
	)
	// initialize, tools/list, ping, tools/call — the notification yields nothing.
	if len(resps) != 4 {
		t.Fatalf("expected 4 responses, got %d: %+v", len(resps), resps)
	}

	// initialize
	var init struct {
		ProtocolVersion string `json:"protocolVersion"`
		Capabilities    struct {
			Tools map[string]any `json:"tools"`
		} `json:"capabilities"`
		ServerInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	if err := json.Unmarshal(resps[0].Result, &init); err != nil {
		t.Fatalf("decode initialize: %v", err)
	}
	if init.ProtocolVersion != "2025-03-26" {
		t.Fatalf("protocol version: got %q", init.ProtocolVersion)
	}
	if init.ServerInfo.Name != "datjit" || init.ServerInfo.Version != "test" {
		t.Fatalf("serverInfo: %+v", init.ServerInfo)
	}
	if init.Capabilities.Tools == nil {
		t.Fatal("expected tools capability")
	}

	// tools/list
	var listRes struct {
		Tools []struct {
			Name        string         `json:"name"`
			Description string         `json:"description"`
			InputSchema map[string]any `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(resps[1].Result, &listRes); err != nil {
		t.Fatalf("decode tools/list: %v", err)
	}
	if len(listRes.Tools) != 4 {
		t.Fatalf("expected 4 tools, got %d", len(listRes.Tools))
	}
	names := map[string]bool{}
	for _, tl := range listRes.Tools {
		names[tl.Name] = true
		if tl.Description == "" || tl.InputSchema == nil {
			t.Fatalf("tool %q missing description/schema", tl.Name)
		}
	}
	for _, want := range []string{"generate", "validate", "inspect", "sample"} {
		if !names[want] {
			t.Fatalf("missing tool %q", want)
		}
	}

	// ping
	if string(resps[2].Result) != "{}" {
		t.Fatalf("ping result: got %q", resps[2].Result)
	}

	// validate tool
	text, isErr := toolText(t, resps[3])
	if isErr {
		t.Fatalf("validate isError unexpectedly true: %q", text)
	}
	if !strings.Contains(text, "schema is valid") {
		t.Fatalf("validate text: %q", text)
	}
}

func TestInitializeUnknownVersionFallsBack(t *testing.T) {
	resps := runScript(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"1999-01-01"}}`,
	)
	var init struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := json.Unmarshal(resps[0].Result, &init); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if init.ProtocolVersion != defaultProtocolVersion {
		t.Fatalf("expected fallback %q, got %q", defaultProtocolVersion, init.ProtocolVersion)
	}
}

func TestUnknownMethod(t *testing.T) {
	resps := runScript(t, `{"jsonrpc":"2.0","id":7,"method":"frobnicate"}`)
	if len(resps) != 1 || resps[0].Error == nil {
		t.Fatalf("expected one error response, got %+v", resps)
	}
	if resps[0].Error.Code != codeMethodNotFound {
		t.Fatalf("expected method-not-found, got %d", resps[0].Error.Code)
	}
}

func TestMalformedJSONIsParseError(t *testing.T) {
	resps := runScript(t, `{not json`)
	if len(resps) != 1 || resps[0].Error == nil {
		t.Fatalf("expected one error response, got %+v", resps)
	}
	if resps[0].Error.Code != codeParse {
		t.Fatalf("expected parse error, got %d", resps[0].Error.Code)
	}
}

func TestNotificationGetsNoResponse(t *testing.T) {
	resps := runScript(t, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if len(resps) != 0 {
		t.Fatalf("expected no responses for notification, got %+v", resps)
	}
}

func TestUnknownToolIsRPCError(t *testing.T) {
	resps := runScript(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"nope","arguments":{}}}`)
	if resps[0].Error == nil || resps[0].Error.Code != codeMethodNotFound {
		t.Fatalf("expected method-not-found rpc error, got %+v", resps[0])
	}
}

// jsonStr quotes s as a JSON string literal for embedding in scripted requests.
func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestGenerateHappyAndError(t *testing.T) {
	// Happy path: valid schema, json output.
	resps := runScript(t,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"generate","arguments":{"schema":`+jsonStr(minimalSchema)+`,"seed":0}}}`,
	)
	text, isErr := toolText(t, resps[0])
	if isErr {
		t.Fatalf("generate isError: %q", text)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(text), &decoded); err != nil {
		t.Fatalf("generate output not JSON: %v\n%s", err, text)
	}
	if _, ok := decoded["User"]; !ok {
		t.Fatalf("expected User key, got %v", decoded)
	}

	// Error path: invalid schema text → isError true.
	resps = runScript(t,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"generate","arguments":{"schema":"this: : : not valid ddl"}}}`,
	)
	_, isErr = toolText(t, resps[0])
	if !isErr {
		t.Fatal("expected isError true for invalid schema")
	}
}

func TestGenerateVolumeCapExceeded(t *testing.T) {
	resps := runScript(t,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"generate","arguments":{"schema":`+jsonStr(minimalSchema)+`,"volumes":{"User":200000}}}}`,
	)
	text, isErr := toolText(t, resps[0])
	if !isErr {
		t.Fatalf("expected cap error, got success: %q", text)
	}
	if !strings.Contains(text, "exceeds cap") {
		t.Fatalf("expected cap message, got %q", text)
	}
}

func TestValidateInvalidIsSuccessfulCall(t *testing.T) {
	resps := runScript(t,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"validate","arguments":{"schema":"domain: x\nentities:\n  A:\n    ref: ->Missing\n"}}}`,
	)
	text, isErr := toolText(t, resps[0])
	if isErr {
		t.Fatalf("validate should never set isError, got true: %q", text)
	}
	if strings.Contains(text, "schema is valid") {
		t.Fatalf("expected a diagnostic, got %q", text)
	}
}

func TestInspectHappy(t *testing.T) {
	resps := runScript(t,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"inspect","arguments":{"schema":`+jsonStr(minimalSchema)+`}}}`,
	)
	text, isErr := toolText(t, resps[0])
	if isErr {
		t.Fatalf("inspect isError: %q", text)
	}
	var insp map[string]any
	if err := json.Unmarshal([]byte(text), &insp); err != nil {
		t.Fatalf("inspect output not JSON: %v\n%s", err, text)
	}
	if insp["EntityCount"] == nil {
		t.Fatalf("expected EntityCount in %v", insp)
	}
}

func TestSampleHappyAndError(t *testing.T) {
	resps := runScript(t,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"sample","arguments":{"semantic":"email","count":3,"seed":1}}}`,
	)
	text, isErr := toolText(t, resps[0])
	if isErr {
		t.Fatalf("sample isError: %q", text)
	}
	var arr []any
	if err := json.Unmarshal([]byte(text), &arr); err != nil {
		t.Fatalf("sample output not JSON array: %v\n%s", err, text)
	}
	if len(arr) != 3 {
		t.Fatalf("expected 3 values, got %d", len(arr))
	}

	// Error path: unknown semantic type.
	resps = runScript(t,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"sample","arguments":{"semantic":"definitely.not.real"}}}`,
	)
	_, isErr = toolText(t, resps[0])
	if !isErr {
		t.Fatal("expected isError for unknown semantic")
	}
}

func TestSampleCountOutOfRange(t *testing.T) {
	resps := runScript(t,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"sample","arguments":{"semantic":"email","count":1000}}}`,
	)
	text, isErr := toolText(t, resps[0])
	if !isErr {
		t.Fatalf("expected count range error, got %q", text)
	}
}

func TestDeterminism(t *testing.T) {
	call := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"generate","arguments":{"schema":` + jsonStr(minimalSchema) + `,"seed":42}}}`

	first := runScript(t, call)
	second := runScript(t, call)
	t1, _ := toolText(t, first[0])
	t2, _ := toolText(t, second[0])
	if t1 != t2 {
		t.Fatalf("same seed produced different output:\n%s\n---\n%s", t1, t2)
	}

	// A different seed should change the output.
	other := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"generate","arguments":{"schema":` + jsonStr(minimalSchema) + `,"seed":99}}}`
	third := runScript(t, other)
	t3, _ := toolText(t, third[0])
	if t1 == t3 {
		t.Fatal("different seeds produced identical output")
	}
}

func TestSampleDeterminism(t *testing.T) {
	call := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"sample","arguments":{"semantic":"person.full","count":5,"seed":7}}}`
	a := runScript(t, call)
	b := runScript(t, call)
	ta, _ := toolText(t, a[0])
	tb, _ := toolText(t, b[0])
	if ta != tb {
		t.Fatalf("sample not deterministic:\n%s\n---\n%s", ta, tb)
	}
}
