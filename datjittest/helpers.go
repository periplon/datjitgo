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
// It fails tb immediately if any pipeline step returns an error.
func MustGenerate(tb testing.TB, schema string, opts ...datjit.Option) *value.Dataset {
	tb.Helper()

	ds, _, err := datjit.GenerateString(schema, opts...)
	if err != nil {
		tb.Fatalf("datjittest: generate schema: %v", err)
	}
	return ds
}

// MustRows returns the generated rows for entity as plain Go maps.
// It fails tb immediately when the schema cannot be generated or the entity
// is absent from the generated dataset.
func MustRows(tb testing.TB, schema string, entity string, opts ...datjit.Option) []map[string]any {
	tb.Helper()

	rows, err := datjit.GenerateRowsString(schema, entity, opts...)
	if err != nil {
		tb.Fatalf("datjittest: generate rows for %q: %v", entity, err)
	}
	return rows
}

// AssertGoldenJSON compares the stable pretty JSON generated from schema with
// the contents of goldenPath.
func AssertGoldenJSON(tb testing.TB, goldenPath string, schema string, opts ...datjit.Option) {
	tb.Helper()

	got := generateJSON(tb, schema, opts...)
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		tb.Fatalf("datjittest: read golden %s: %v", goldenPath, err)
	}
	if !bytes.Equal(got, want) {
		tb.Fatalf("datjittest: JSON golden mismatch for %s\n\ngot:\n%s\nwant:\n%s", goldenPath, got, want)
	}
}

// UpdateGoldenJSON writes the stable pretty JSON generated from schema to
// goldenPath, creating parent directories when needed.
func UpdateGoldenJSON(tb testing.TB, goldenPath string, schema string, opts ...datjit.Option) {
	tb.Helper()

	got := generateJSON(tb, schema, opts...)
	if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
		tb.Fatalf("datjittest: create golden directory for %s: %v", goldenPath, err)
	}
	if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
		tb.Fatalf("datjittest: write golden %s: %v", goldenPath, err)
	}
}

func generateJSON(tb testing.TB, schema string, opts ...datjit.Option) []byte {
	tb.Helper()

	raw, err := datjit.GenerateJSONString(schema, opts...)
	if err != nil {
		tb.Fatalf("datjittest: generate JSON: %v", err)
	}
	return raw
}
