package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestLineReaderBasic(t *testing.T) {
	lr := newLineReader(strings.NewReader("first\nsecond\n"))
	got, err := lr.readLine()
	if err != nil {
		t.Fatalf("readLine: %v", err)
	}
	if string(got) != "first" {
		t.Fatalf("got %q, want first", got)
	}
	got, err = lr.readLine()
	if err != nil {
		t.Fatalf("readLine: %v", err)
	}
	if string(got) != "second" {
		t.Fatalf("got %q, want second", got)
	}
	if _, err := lr.readLine(); !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestLineReaderUnterminatedFinalLine(t *testing.T) {
	lr := newLineReader(strings.NewReader("only"))
	got, err := lr.readLine()
	if err != nil {
		t.Fatalf("readLine: %v", err)
	}
	if string(got) != "only" {
		t.Fatalf("got %q, want only", got)
	}
	if _, err := lr.readLine(); !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestLineReaderStripsCRLF(t *testing.T) {
	lr := newLineReader(strings.NewReader("crlf\r\n"))
	got, err := lr.readLine()
	if err != nil {
		t.Fatalf("readLine: %v", err)
	}
	if string(got) != "crlf" {
		t.Fatalf("got %q, want crlf", got)
	}
}

func TestLineReaderOversizedLine(t *testing.T) {
	// One line over the cap, then a valid line after the newline.
	big := strings.Repeat("a", maxLineBytes+10) + "\n" + "ok\n"
	lr := newLineReader(strings.NewReader(big))
	if _, err := lr.readLine(); !errors.Is(err, errLineTooLong) {
		t.Fatalf("expected errLineTooLong, got %v", err)
	}
	got, err := lr.readLine()
	if err != nil {
		t.Fatalf("readLine after oversize: %v", err)
	}
	if string(got) != "ok" {
		t.Fatalf("got %q, want ok", got)
	}
}

func TestRequestIsNotification(t *testing.T) {
	var withID request
	if err := json.Unmarshal([]byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`), &withID); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if withID.isNotification() {
		t.Fatal("request with id should not be a notification")
	}
	var noID request
	if err := json.Unmarshal([]byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`), &noID); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !noID.isNotification() {
		t.Fatal("request without id should be a notification")
	}
}

func TestWriteMessageNewlineTerminated(t *testing.T) {
	var buf bytes.Buffer
	if err := writeMessage(&buf, map[string]any{"k": "v"}); err != nil {
		t.Fatalf("writeMessage: %v", err)
	}
	out := buf.String()
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("expected trailing newline, got %q", out)
	}
	if strings.Count(out, "\n") != 1 {
		t.Fatalf("expected exactly one newline, got %q", out)
	}
}

func TestNewErrorResponseNilID(t *testing.T) {
	resp := newErrorResponse(nil, codeParse, "boom")
	if string(resp.ID) != "null" {
		t.Fatalf("expected null id, got %q", resp.ID)
	}
	if resp.Error == nil || resp.Error.Code != codeParse {
		t.Fatalf("unexpected error object: %+v", resp.Error)
	}
}
