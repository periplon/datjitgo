// Package datjittest provides test helpers for deterministic datjit fixture
// generation and golden JSON snapshot assertions and updates.
package datjittest

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/periplon/datjitgo"
	"github.com/periplon/datjitgo/core/value"
)

// MustGenerate parses, validates, and generates a dataset from schema.
// It fails t immediately if any pipeline step returns an error.
func MustGenerate(t testing.TB, schema string, opts ...datjit.Option) *value.Dataset {
	t.Helper()

	ds, _, err := datjit.GenerateString(schema, opts...)
	if err != nil {
		t.Fatalf("datjittest: generate schema: %v", err)
	}
	return ds
}

// MustRows returns the generated rows for entity as plain Go maps.
// It fails t immediately when the schema cannot be generated or the entity
// is absent from the generated dataset.
func MustRows(t testing.TB, schema string, entity string, opts ...datjit.Option) []map[string]any {
	t.Helper()

	rows, err := datjit.GenerateRowsString(schema, entity, opts...)
	if err != nil {
		t.Fatalf("datjittest: generate rows for %q: %v", entity, err)
	}
	return rows
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

	raw, err := datjit.GenerateJSONString(schema, opts...)
	if err != nil {
		t.Fatalf("datjittest: generate JSON: %v", err)
	}
	return raw
}
