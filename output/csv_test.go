package output

import (
	"bytes"
	stdcsv "encoding/csv"
	"errors"
	"strings"
	"testing"

	ierrors "github.com/jmcarbo/datjitgo/core/errors"
	"github.com/jmcarbo/datjitgo/core/ports"
)

func TestCSV_Format(t *testing.T) {
	if got := NewCSV().Format(); got != "csv" {
		t.Fatalf("Format() = %q", got)
	}
}

func TestCSV_RequiresDocument(t *testing.T) {
	_, ds := NewFixture(t)
	var buf bytes.Buffer
	err := NewCSV().Write(ds, nil, &buf, ports.WriteOptions{})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	var ie *ierrors.Error
	if !errors.As(err, &ie) || ie.Kind != ierrors.KindValidation {
		t.Fatalf("expected KindValidation, got %T %v", err, err)
	}
}

func TestCSV_HeaderAndValues(t *testing.T) {
	doc, ds := NewFixture(t)
	var buf bytes.Buffer
	if err := NewCSV().Write(ds, doc, &buf, ports.WriteOptions{}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Split the output into sections separated by blank lines.
	text := buf.String()
	sections := strings.Split(strings.TrimRight(text, "\n"), "\n\n")
	if len(sections) != 2 {
		t.Fatalf("want 2 sections, got %d:\n%s", len(sections), text)
	}

	// User section: parse as CSV.
	r := stdcsv.NewReader(strings.NewReader(sections[0] + "\n"))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll User: %v", err)
	}
	if len(records) != 3 { // header + 2 rows
		t.Fatalf("want 3 records, got %d", len(records))
	}
	wantHeader := []string{"id", "name", "age", "score", "active", "created_at", "balance", "tags", "meta", "nickname"}
	if len(records[0]) != len(wantHeader) {
		t.Fatalf("header len = %d, want %d: %v", len(records[0]), len(wantHeader), records[0])
	}
	for i, h := range wantHeader {
		if records[0][i] != h {
			t.Fatalf("header[%d] = %q, want %q", i, records[0][i], h)
		}
	}
	// active true/false
	if records[1][4] != "true" {
		t.Fatalf("active row1 = %q, want true", records[1][4])
	}
	if records[2][4] != "false" {
		t.Fatalf("active row2 = %q, want false", records[2][4])
	}
	// nickname null → empty cell
	if records[2][9] != "" {
		t.Fatalf("null nickname should be empty, got %q", records[2][9])
	}
	// decimal balance unquoted
	if records[1][6] != "1234.56" {
		t.Fatalf("balance row1 = %q", records[1][6])
	}
	// list/object serialised as JSON
	if !strings.HasPrefix(records[1][7], "[") {
		t.Fatalf("tags should be JSON array, got %q", records[1][7])
	}
	if !strings.HasPrefix(records[1][8], "{") {
		t.Fatalf("meta should be JSON object, got %q", records[1][8])
	}
}

func TestCSV_EntityFilter(t *testing.T) {
	doc, ds := NewFixture(t)
	var buf bytes.Buffer
	if err := NewCSV().Write(ds, doc, &buf, ports.WriteOptions{EntityFilter: "Order"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	r := stdcsv.NewReader(&buf)
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(records) != 2 { // header + 1 row
		t.Fatalf("want 2 records, got %d", len(records))
	}
	if records[0][0] != "id" {
		t.Fatalf("header[0] = %q", records[0][0])
	}
}
