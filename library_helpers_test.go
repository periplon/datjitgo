package datjit_test

import (
	"encoding/json"
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/periplon/datjitgo"
	derrs "github.com/periplon/datjitgo/core/errors"
	"github.com/periplon/datjitgo/core/value"
)

const helperSchema = `domain: helper
seed: 42
volume:
  User: 2
entities:
  User:
    id: int @primary
    name: person.full
    email: email
`

func TestPlainConversionHelpers(t *testing.T) {
	now := time.Date(2026, 4, 25, 12, 30, 0, 123, time.UTC)
	id := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	obj := value.NewObject()
	obj.Set("nil", value.Null())
	obj.Set("ok", value.Bool(true))
	obj.Set("name", value.Str("Ada"))
	obj.Set("id", value.UUID(id))
	obj.Set("when", value.Time(now))
	obj.Set("price", value.Dec(decimal.RequireFromString("12.30")))
	obj.Set("tags", value.List([]value.Value{value.Str("a"), value.Int(2)}))

	row := datjit.ObjectMap(obj)
	if row["name"] != "Ada" || row["id"] != id.String() || row["price"] != "12.3" {
		t.Fatalf("row = %#v", row)
	}
	if row["when"] != now.Format(time.RFC3339Nano) {
		t.Fatalf("time = %#v", row["when"])
	}
	if row["nil"] != nil || row["ok"] != true {
		t.Fatalf("primitive conversions = %#v", row)
	}
	tags, ok := row["tags"].([]any)
	if !ok || len(tags) != 2 || tags[0] != "a" || tags[1] != int64(2) {
		t.Fatalf("tags = %#v", row["tags"])
	}
	if got := datjit.ValueAny(value.Obj(obj)); got == nil {
		t.Fatal("object value converted to nil")
	}
	if got := datjit.ValueAny(value.Value{}); got != nil {
		t.Fatalf("unknown value converted to %#v, want nil", got)
	}
	if got := datjit.ObjectMap(nil); got != nil {
		t.Fatalf("nil object map = %#v, want nil", got)
	}
	if got := datjit.DatasetMap(nil); got != nil {
		t.Fatalf("nil dataset map = %#v, want nil", got)
	}

	ds := value.NewDataset()
	ds.Entities.Set("User", []*value.Object{obj})
	m := datjit.DatasetMap(ds)
	if len(m["User"]) != 1 || m["User"][0]["name"] != "Ada" {
		t.Fatalf("dataset map = %#v", m)
	}
}

func TestGenerateStringMapRowsAndJSON(t *testing.T) {
	ds, doc, err := datjit.GenerateString(helperSchema, datjit.WithSeed(7))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Domain != "helper" || ds.Entities.Len() != 1 {
		t.Fatalf("doc=%+v ds=%+v", doc, ds)
	}

	m, err := datjit.GenerateMapString(helperSchema, datjit.WithSeed(7))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(m["User"]); got != 2 {
		t.Fatalf("rows = %d", got)
	}

	rows, err := datjit.GenerateRowsString(helperSchema, "User", datjit.WithSeed(7))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0]["name"] == "" {
		t.Fatalf("rows = %#v", rows)
	}

	raw, err := datjit.GenerateJSONString(helperSchema, datjit.WithSeed(7))
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, raw)
	}
	if _, ok := decoded["User"]; !ok {
		t.Fatalf("json = %s", raw)
	}

	_, err = datjit.GenerateRowsString(helperSchema, "Missing", datjit.WithSeed(7))
	if !datjit.IsValidationError(err) {
		t.Fatalf("GenerateRowsString missing entity error = %v, want validation", err)
	}
	_, _, err = datjit.GenerateString("domain: [", datjit.WithSeed(7))
	if !datjit.IsParseError(err) {
		t.Fatalf("GenerateString parse error = %v, want parse", err)
	}
}

func TestGenerateFileHelpers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.yaml")
	if err := os.WriteFile(path, []byte(helperSchema), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := datjit.GenerateMapFile(path, datjit.WithSeed(9))
	if err != nil {
		t.Fatal(err)
	}
	if len(m["User"]) != 2 {
		t.Fatalf("map = %#v", m)
	}

	rows, err := datjit.GenerateRowsFile(path, "User", datjit.WithSeed(9))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %#v", rows)
	}

	raw, err := datjit.GenerateJSONFile(path, datjit.WithSeed(9))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"User"`) {
		t.Fatalf("json = %s", raw)
	}

	_, err = datjit.GenerateRowsFile(path, "Missing", datjit.WithSeed(9))
	if !datjit.IsValidationError(err) {
		t.Fatalf("GenerateRowsFile missing entity error = %v, want validation", err)
	}
	_, _, err = datjit.GenerateFile(filepath.Join(dir, "missing.yaml"))
	if err == nil {
		t.Fatal("GenerateFile missing path returned nil error")
	}
}

func TestWriteFileHelpersValidateAndWrite(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.yaml")
	if err := os.WriteFile(schemaPath, []byte(helperSchema), 0o644); err != nil {
		t.Fatal(err)
	}

	jsonPath := filepath.Join(dir, "out.json")
	if err := datjit.WriteJSONFile(jsonPath, schemaPath, datjit.WithSeed(11)); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"User"`) {
		t.Fatalf("json = %s", raw)
	}

	csvPath := filepath.Join(dir, "out.csv")
	if err := datjit.WriteFile(csvPath, schemaPath, "csv", datjit.WithSeed(11)); err != nil {
		t.Fatal(err)
	}
	csvRaw, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(csvRaw), "id") {
		t.Fatalf("csv = %s", csvRaw)
	}

	invalidPath := filepath.Join(dir, "invalid.yaml")
	if err := os.WriteFile(invalidPath, []byte("domain: bad\nentities:\n  User:\n    field: missing.type\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := datjit.WriteFile(filepath.Join(dir, "bad.json"), invalidPath, "json"); !datjit.IsValidationError(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if err := datjit.WriteFile(filepath.Join(dir, "bad.json"), filepath.Join(dir, "missing.yaml"), "json"); err == nil {
		t.Fatal("WriteFile missing schema returned nil error")
	}
	if err := datjit.WriteFile(filepath.Join(dir, "bad.json"), schemaPath, "unknown"); err == nil {
		t.Fatal("WriteFile unknown format returned nil error")
	}
}

func TestRootErrorPredicates(t *testing.T) {
	if !datjit.IsParseError(derrs.ErrParse) {
		t.Fatal("parse predicate")
	}
	if !datjit.IsValidationError(derrs.ErrValidation) {
		t.Fatal("validation predicate")
	}
	if !datjit.IsGenerationError(derrs.ErrGeneration) {
		t.Fatal("generation predicate")
	}
	if !datjit.IsCorpusError(derrs.ErrCorpusMissing) {
		t.Fatal("corpus predicate")
	}
	if datjit.IsParseError(stderrors.New("plain")) {
		t.Fatal("plain error matched parse")
	}
}
