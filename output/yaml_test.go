package output

import (
	"bytes"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/periplon/datjitgo/core/ports"
)

func TestYAML_Format(t *testing.T) {
	if got := NewYAML().Format(); got != "yaml" {
		t.Fatalf("Format() = %q", got)
	}
}

func TestYAML_RoundTrips(t *testing.T) {
	doc, ds := NewFixture(t)
	var buf bytes.Buffer
	if err := NewYAML().Write(ds, doc, &buf, ports.WriteOptions{}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	var m map[string]any
	if err := yaml.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, buf.String())
	}
	if _, ok := m["User"]; !ok {
		t.Fatalf("User missing from round-trip: %v", m)
	}
	if _, ok := m["Order"]; !ok {
		t.Fatalf("Order missing from round-trip: %v", m)
	}
}

func TestYAML_EntityOrderByByte(t *testing.T) {
	doc, ds := NewFixture(t)
	var buf bytes.Buffer
	if err := NewYAML().Write(ds, doc, &buf, ports.WriteOptions{}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	s := buf.String()
	user := strings.Index(s, "User:")
	ord := strings.Index(s, "Order:")
	if user < 0 || ord < 0 || user > ord {
		t.Fatalf("entity order wrong: User=%d Order=%d\n%s", user, ord, s)
	}
}

func TestYAML_FieldOrderByByte(t *testing.T) {
	doc, ds := NewFixture(t)
	var buf bytes.Buffer
	if err := NewYAML().Write(ds, doc, &buf, ports.WriteOptions{}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	s := buf.String()
	// Check that in the first user record, id appears before name, before age, etc.
	// We look at the substring between "User:" and "Order:".
	seg := s[strings.Index(s, "User:"):strings.Index(s, "Order:")]
	want := []string{"id:", "name:", "age:", "score:", "active:", "created_at:", "balance:", "tags:", "meta:", "nickname:"}
	last := 0
	for _, k := range want {
		i := strings.Index(seg[last:], k)
		if i < 0 {
			t.Fatalf("field %q not found after %d\n%s", k, last, seg)
		}
		last += i + len(k)
	}
}

func TestYAML_BoolAndNullRendering(t *testing.T) {
	doc, ds := NewFixture(t)
	var buf bytes.Buffer
	if err := NewYAML().Write(ds, doc, &buf, ports.WriteOptions{}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	s := buf.String()
	if !strings.Contains(s, "active: true") {
		t.Fatalf("bool true missing:\n%s", s)
	}
	if !strings.Contains(s, "active: false") {
		t.Fatalf("bool false missing:\n%s", s)
	}
	// Null nickname must serialise as null or ~.
	if !strings.Contains(s, "nickname: null") && !strings.Contains(s, "nickname: ~") {
		t.Fatalf("null nickname missing:\n%s", s)
	}
}

func TestYAML_Deterministic(t *testing.T) {
	doc, ds := NewFixture(t)
	var a, b bytes.Buffer
	_ = NewYAML().Write(ds, doc, &a, ports.WriteOptions{})
	_ = NewYAML().Write(ds, doc, &b, ports.WriteOptions{})
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatalf("non-deterministic yaml")
	}
}
