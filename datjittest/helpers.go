// Package datjittest provides testing helpers for generating deterministic
// datjit fixtures and asserting golden JSON snapshots.
package datjittest

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmcarbo/datjitgo"
	"github.com/jmcarbo/datjitgo/core/value"
)

// MustGenerate parses, validates, and generates a dataset from schema.
// It fails t immediately if any pipeline step returns an error.
func MustGenerate(t testing.TB, schema string, opts ...datjit.Option) *value.Dataset {
	t.Helper()

	svc, err := datjit.New(opts...)
	if err != nil {
		t.Fatalf("datjittest: create service: %v", err)
	}
	doc, err := svc.Parse(strings.NewReader(schema), "datjittest schema")
	if err != nil {
		t.Fatalf("datjittest: parse schema: %v", err)
	}
	if err := svc.Validate(doc); err != nil {
		t.Fatalf("datjittest: validate schema: %v", err)
	}
	ds, err := svc.Generate(doc)
	if err != nil {
		t.Fatalf("datjittest: generate dataset: %v", err)
	}
	return ds
}

// MustRows returns the generated rows for entity as plain Go maps.
// It fails t immediately when the schema cannot be generated or the entity
// is absent from the generated dataset.
func MustRows(t testing.TB, schema string, entity string, opts ...datjit.Option) []map[string]any {
	t.Helper()

	ds := MustGenerate(t, schema, opts...)
	rows, ok := ds.Entities.Get(entity)
	if !ok {
		t.Fatalf("datjittest: entity %q not found", entity)
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		m := make(map[string]any)
		if row != nil {
			row.Each(func(k string, v value.Value) bool {
				m[k] = valueToAny(v)
				return true
			})
		}
		out = append(out, m)
	}
	return out
}

// AssertGoldenJSON compares the stable pretty JSON generated from schema with
// the contents of goldenPath.
func AssertGoldenJSON(t testing.TB, goldenPath string, schema string, opts ...datjit.Option) {
	t.Helper()

	got := generateJSON(t, schema, opts...)
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("datjittest: read golden %s: %v", goldenPath, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("datjittest: JSON golden mismatch for %s\n\ngot:\n%s\nwant:\n%s", goldenPath, got, want)
	}
}

// UpdateGoldenJSON writes the stable pretty JSON generated from schema to
// goldenPath, creating parent directories when needed.
func UpdateGoldenJSON(t testing.TB, goldenPath string, schema string, opts ...datjit.Option) {
	t.Helper()

	got := generateJSON(t, schema, opts...)
	if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
		t.Fatalf("datjittest: create golden directory for %s: %v", goldenPath, err)
	}
	if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
		t.Fatalf("datjittest: write golden %s: %v", goldenPath, err)
	}
}

func generateJSON(t testing.TB, schema string, opts ...datjit.Option) []byte {
	t.Helper()

	svc, err := datjit.New(opts...)
	if err != nil {
		t.Fatalf("datjittest: create service: %v", err)
	}
	doc, err := svc.Parse(strings.NewReader(schema), "datjittest schema")
	if err != nil {
		t.Fatalf("datjittest: parse schema: %v", err)
	}
	if err := svc.Validate(doc); err != nil {
		t.Fatalf("datjittest: validate schema: %v", err)
	}
	ds, err := svc.Generate(doc)
	if err != nil {
		t.Fatalf("datjittest: generate dataset: %v", err)
	}

	var buf bytes.Buffer
	if err := svc.Write(ds, doc, "json", &buf, datjit.WriteOpts{Pretty: true}); err != nil {
		t.Fatalf("datjittest: render JSON: %v", err)
	}
	return buf.Bytes()
}

func valueToAny(v value.Value) any {
	switch v.Kind {
	case value.KindNull:
		return nil
	case value.KindBool:
		return v.B
	case value.KindInt:
		return v.I
	case value.KindFloat:
		return v.F
	case value.KindString:
		return v.S
	case value.KindUUID:
		return v.U.String()
	case value.KindTime:
		return v.T.UTC().Format(time.RFC3339Nano)
	case value.KindDecimal:
		return v.D.String()
	case value.KindList:
		out := make([]any, 0, len(v.L))
		for _, item := range v.L {
			out = append(out, valueToAny(item))
		}
		return out
	case value.KindObject:
		if v.O == nil {
			return nil
		}
		out := make(map[string]any, v.O.Len())
		v.O.Each(func(k string, child value.Value) bool {
			out[k] = valueToAny(child)
			return true
		})
		return out
	default:
		return nil
	}
}
