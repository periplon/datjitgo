package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/periplon/datjitgo/core/ports"
)

func TestJSON_Format(t *testing.T) {
	if got := NewJSON().Format(); got != "json" {
		t.Fatalf("Format() = %q, want %q", got, "json")
	}
}

func TestJSON_ValidAndRoundTrips(t *testing.T) {
	doc, ds := NewFixture(t)
	var buf bytes.Buffer
	if err := NewJSON().Write(ds, doc, &buf, ports.WriteOptions{}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v (body: %s)", err, buf.String())
	}
	users, ok := out["User"].([]any)
	if !ok {
		t.Fatalf("User not an array; got %T", out["User"])
	}
	if len(users) != 2 {
		t.Fatalf("want 2 users, got %d", len(users))
	}
}

func TestJSON_EntityAndFieldOrdering(t *testing.T) {
	doc, ds := NewFixture(t)
	var buf bytes.Buffer
	if err := NewJSON().Write(ds, doc, &buf, ports.WriteOptions{}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	s := buf.String()

	// User key must appear before Order key.
	userIdx := strings.Index(s, `"User"`)
	orderIdx := strings.Index(s, `"Order"`)
	if userIdx < 0 || orderIdx < 0 || userIdx > orderIdx {
		t.Fatalf("entity order wrong: User=%d Order=%d\n%s", userIdx, orderIdx, s)
	}

	// Field order inside User row: id → name → age → score → active → created_at → balance → tags → meta → nickname
	ordered := []string{`"id"`, `"name"`, `"age"`, `"score"`, `"active"`, `"created_at"`, `"balance"`, `"tags"`, `"meta"`, `"nickname"`}
	last := 0
	for _, key := range ordered {
		i := strings.Index(s[last:], key)
		if i < 0 {
			t.Fatalf("key %s not found after pos %d", key, last)
		}
		last += i + len(key)
	}
}

func TestJSON_PrettyVsCompact(t *testing.T) {
	doc, ds := NewFixture(t)

	var compact bytes.Buffer
	if err := NewJSON().Write(ds, doc, &compact, ports.WriteOptions{Pretty: false}); err != nil {
		t.Fatalf("compact: %v", err)
	}
	var pretty bytes.Buffer
	if err := NewJSON().Write(ds, doc, &pretty, ports.WriteOptions{Pretty: true}); err != nil {
		t.Fatalf("pretty: %v", err)
	}

	// Pretty must contain newline + 2-space indent.
	if !strings.Contains(pretty.String(), "\n  ") {
		t.Fatalf("pretty output missing indent:\n%s", pretty.String())
	}
	// Compact must be a single line (no internal newlines apart from trailing).
	body := strings.TrimRight(compact.String(), "\n")
	if strings.Contains(body, "\n") {
		t.Fatalf("compact output has newlines:\n%s", compact.String())
	}
}

func TestJSON_NullRendering(t *testing.T) {
	doc, ds := NewFixture(t)
	var buf bytes.Buffer
	if err := NewJSON().Write(ds, doc, &buf, ports.WriteOptions{}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	// Bob's nickname is Null.
	if !strings.Contains(buf.String(), `"nickname":null`) {
		t.Fatalf("expected null nickname in compact output:\n%s", buf.String())
	}
}

func TestJSON_DecimalPreservesScale(t *testing.T) {
	doc, ds := NewFixture(t)
	var buf bytes.Buffer
	if err := NewJSON().Write(ds, doc, &buf, ports.WriteOptions{}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !strings.Contains(buf.String(), `"balance":1234.56`) {
		t.Fatalf("decimal not preserved:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), `"total":99.99`) {
		t.Fatalf("decimal total not preserved:\n%s", buf.String())
	}
}

func TestJSON_EntityFilter(t *testing.T) {
	doc, ds := NewFixture(t)
	var buf bytes.Buffer
	if err := NewJSON().Write(ds, doc, &buf, ports.WriteOptions{EntityFilter: "Order"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if strings.Contains(buf.String(), `"User"`) {
		t.Fatalf("entity filter leaked User:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), `"Order"`) {
		t.Fatalf("entity filter dropped Order:\n%s", buf.String())
	}
}

func TestJSON_DeterministicAcrossRuns(t *testing.T) {
	doc, ds := NewFixture(t)
	var a, b bytes.Buffer
	if err := NewJSON().Write(ds, doc, &a, ports.WriteOptions{Pretty: true}); err != nil {
		t.Fatal(err)
	}
	if err := NewJSON().Write(ds, doc, &b, ports.WriteOptions{Pretty: true}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatalf("non-deterministic output:\nA=%s\nB=%s", a.String(), b.String())
	}
}
