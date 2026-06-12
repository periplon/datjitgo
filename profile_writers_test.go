package datjit_test

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	datjit "github.com/periplon/datjitgo"
	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/value"
)

// hostilePayloadMarkers are hostile-only boundary strings (kept in sync with
// generator/profile.go's hostileStrings) used to confirm the hostile dataset
// actually carries adversarial payloads into the writers under test.
var hostilePayloadMarkers = []string{
	"comma,separated,value",
	"embedded \"double\" quotes",
	"it's got 'single' quotes",
	"semi;colons; everywhere",
	"line\nbreak",
	"tab\tseparated",
	"=cmd()",
	"Robert'); DROP TABLE x;--",
	strings.Repeat("A", 4096),
	"раypal",
}

// hostileDataset generates the profiles fixture under the hostile profile and
// returns the dataset plus parsed document for writer round-trips.
func hostileDataset(t *testing.T) (*value.Dataset, *model.Document, *datjit.Service) {
	t.Helper()
	schema := profileSchema(t)
	svc, err := datjit.New(datjit.WithSeed(42), datjit.WithProfile("hostile"))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	doc, err := svc.Parse(strings.NewReader(schema), "profiles.yaml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := svc.Validate(doc); err != nil {
		t.Fatalf("validate: %v", err)
	}
	ds, err := svc.Generate(doc)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return ds, doc, svc
}

// TestHostileProfileWriterRobustness drives a hostile dataset through all
// five writers and asserts each emits well-formed output: JSON/NDJSON/YAML
// re-parse, CSV re-parses with encoding/csv, and SQL string literals stay
// lexically balanced. The profile_hostile.json golden doubles as the
// byte-level regression; this test checks structural validity.
func TestHostileProfileWriterRobustness(t *testing.T) {
	ds, doc, svc := hostileDataset(t)

	outputs := map[string]string{}
	for _, format := range []string{"json", "ndjson", "csv", "yaml", "sql"} {
		var buf bytes.Buffer
		if err := svc.Write(ds, doc, format, &buf, datjit.WriteOpts{SQLDialect: "postgres"}); err != nil {
			t.Fatalf("%s writer failed on hostile dataset: %v", format, err)
		}
		outputs[format] = buf.String()
	}

	// The dataset must actually contain adversarial payloads, otherwise the
	// round-trips below prove nothing. JSON escapes \n, \t and " so match
	// against the decoded dataset rather than raw writer bytes.
	if !datasetContainsAny(ds, hostilePayloadMarkers) {
		t.Fatal("hostile dataset contains none of the hostile payload markers; check seed/table wiring")
	}

	t.Run("json", func(t *testing.T) {
		var v any
		if err := json.Unmarshal([]byte(outputs["json"]), &v); err != nil {
			t.Fatalf("json output does not re-parse: %v", err)
		}
	})

	t.Run("ndjson", func(t *testing.T) {
		sc := bufio.NewScanner(strings.NewReader(outputs["ndjson"]))
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		n := 0
		for sc.Scan() {
			line := sc.Bytes()
			if len(bytes.TrimSpace(line)) == 0 {
				continue
			}
			var v any
			if err := json.Unmarshal(line, &v); err != nil {
				t.Fatalf("ndjson line %d does not re-parse: %v", n+1, err)
			}
			n++
		}
		if err := sc.Err(); err != nil {
			t.Fatalf("scan ndjson: %v", err)
		}
		if n == 0 {
			t.Fatal("ndjson output is empty")
		}
	})

	t.Run("csv", func(t *testing.T) {
		r := csv.NewReader(strings.NewReader(outputs["csv"]))
		// Sections have different column counts and are separated by blank
		// lines (which encoding/csv skips); disable the per-record check.
		r.FieldsPerRecord = -1
		records := 0
		for {
			_, err := r.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("csv output does not re-parse: %v", err)
			}
			records++
		}
		if records == 0 {
			t.Fatal("csv output is empty")
		}
	})

	t.Run("yaml", func(t *testing.T) {
		var v any
		if err := yaml.Unmarshal([]byte(outputs["yaml"]), &v); err != nil {
			t.Fatalf("yaml output does not re-parse: %v", err)
		}
	})

	t.Run("sql", func(t *testing.T) {
		out := outputs["sql"]
		if err := checkSQLStringLexing(out); err != nil {
			t.Fatalf("sql output has unbalanced string literals: %v", err)
		}
		// The injection payload's quote must be escaped: a raw occurrence of
		// the payload would terminate the literal at "Robert'" and leave a
		// live statement behind. Doubling is the writers' escape scheme.
		if strings.Contains(out, "DROP TABLE x") && !strings.Contains(out, "Robert''); DROP TABLE x;--") {
			t.Fatal("sql injection payload present but not quote-escaped")
		}
	})
}

// TestWritersEscapeEveryHostilePayload force-feeds each hostile payload
// through the SQL and CSV writers via a hand-built dataset, so escaping is
// asserted for every payload independent of which entries the seeded
// substitution happens to draw.
func TestWritersEscapeEveryHostilePayload(t *testing.T) {
	doc := model.NewDocument()
	ent := model.NewEntity("Payload")
	ent.Fields.Set("v", &model.Field{Name: "v", Type: model.Primitive{Kind: model.PrimString}})
	doc.Entities.Set("Payload", ent)

	ds := value.NewDataset()
	rows := make([]*value.Object, 0, len(hostilePayloadMarkers))
	for _, m := range hostilePayloadMarkers {
		row := value.NewObject()
		row.Set("v", value.Str(m))
		rows = append(rows, row)
	}
	ds.Entities.Set("Payload", rows)

	svc := datjit.NewDefault()

	t.Run("sql", func(t *testing.T) {
		var buf bytes.Buffer
		if err := svc.Write(ds, doc, "sql", &buf, datjit.WriteOpts{SQLDialect: "postgres"}); err != nil {
			t.Fatalf("sql write: %v", err)
		}
		out := buf.String()
		if err := checkSQLStringLexing(out); err != nil {
			t.Fatalf("sql output has unbalanced string literals: %v", err)
		}
		if !strings.Contains(out, "Robert''); DROP TABLE x;--") {
			t.Fatal("sql injection payload not quote-escaped")
		}
		if strings.Contains(out, "'Robert'); DROP TABLE x;--'") {
			t.Fatal("sql injection payload emitted with a raw unescaped quote")
		}
	})

	t.Run("csv", func(t *testing.T) {
		var buf bytes.Buffer
		if err := svc.Write(ds, doc, "csv", &buf, datjit.WriteOpts{}); err != nil {
			t.Fatalf("csv write: %v", err)
		}
		r := csv.NewReader(strings.NewReader(buf.String()))
		r.FieldsPerRecord = -1
		records, err := r.ReadAll()
		if err != nil {
			t.Fatalf("csv output does not re-parse: %v", err)
		}
		// Header + one row per payload; each cell must round-trip verbatim.
		if len(records) != len(hostilePayloadMarkers)+1 {
			t.Fatalf("csv re-parse produced %d records, want %d", len(records), len(hostilePayloadMarkers)+1)
		}
		for i, m := range hostilePayloadMarkers {
			if got := records[i+1][0]; got != m {
				t.Fatalf("csv cell %d round-tripped to %q, want %q", i, got, m)
			}
		}
	})
}

// datasetContainsAny reports whether any field value (recursively, including
// list elements and nested objects) equals one of the marker strings.
func datasetContainsAny(ds *value.Dataset, markers []string) bool {
	found := false
	var visit func(v value.Value)
	visit = func(v value.Value) {
		if found {
			return
		}
		switch v.Kind {
		case value.KindString:
			for _, m := range markers {
				if v.S == m {
					found = true
					return
				}
			}
		case value.KindList:
			for _, item := range v.L {
				visit(item)
			}
		case value.KindObject:
			v.O.Each(func(_ string, item value.Value) bool {
				visit(item)
				return !found
			})
		default:
			// scalar kinds other than string cannot carry payloads
		}
	}
	ds.Entities.Each(func(_ string, rows []*value.Object) bool {
		for _, row := range rows {
			row.Each(func(_ string, v value.Value) bool {
				visit(v)
				return !found
			})
		}
		return !found
	})
	return found
}

// checkSQLStringLexing scans s with a minimal SQL string lexer ('-quoted
// literals, ” as the escape) and reports an error when the input ends inside
// an open literal — i.e. some value broke out of its quotes.
func checkSQLStringLexing(s string) error {
	inString := false
	for i := 0; i < len(s); i++ {
		if s[i] != '\'' {
			continue
		}
		if inString {
			// Doubled quote inside a literal is an escaped quote.
			if i+1 < len(s) && s[i+1] == '\'' {
				i++
				continue
			}
			inString = false
			continue
		}
		inString = true
	}
	if inString {
		return errors.New("input ends inside a string literal")
	}
	return nil
}
