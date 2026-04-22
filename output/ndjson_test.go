package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/jmcarbo/datjitgo/core/ports"
)

func TestNDJSON_Format(t *testing.T) {
	if got := NewNDJSON().Format(); got != "ndjson" {
		t.Fatalf("Format() = %q, want %q", got, "ndjson")
	}
}

func TestNDJSON_OneObjectPerLine(t *testing.T) {
	doc, ds := NewFixture(t)
	var buf bytes.Buffer
	if err := NewNDJSON().Write(ds, doc, &buf, ports.WriteOptions{}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	// 2 users + 1 order = 3 lines.
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %d:\n%s", len(lines), buf.String())
	}
	for i, ln := range lines {
		var m map[string]any
		if err := json.Unmarshal([]byte(ln), &m); err != nil {
			t.Fatalf("line %d not valid JSON: %v (%q)", i, err, ln)
		}
	}
	// Stream-decode through json.Decoder should also work.
	dec := json.NewDecoder(bytes.NewReader(buf.Bytes()))
	count := 0
	for {
		var m map[string]any
		if err := dec.Decode(&m); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("stream decode: %v", err)
		}
		count++
	}
	if count != 3 {
		t.Fatalf("stream decoded %d, want 3", count)
	}
}

func TestNDJSON_FieldOrderPreserved(t *testing.T) {
	doc, ds := NewFixture(t)
	var buf bytes.Buffer
	if err := NewNDJSON().Write(ds, doc, &buf, ports.WriteOptions{}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	firstLine := strings.SplitN(buf.String(), "\n", 2)[0]
	idx := strings.Index(firstLine, `"id"`)
	nameIdx := strings.Index(firstLine, `"name"`)
	if idx < 0 || nameIdx < 0 || idx > nameIdx {
		t.Fatalf("field order not preserved: %s", firstLine)
	}
}

func TestNDJSON_EntityFilter(t *testing.T) {
	doc, ds := NewFixture(t)
	var buf bytes.Buffer
	if err := NewNDJSON().Write(ds, doc, &buf, ports.WriteOptions{EntityFilter: "Order"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("want 1 line after filter, got %d: %s", len(lines), buf.String())
	}
}
