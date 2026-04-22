package output

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/jmcarbo/datjitgo/core/model"
	"github.com/jmcarbo/datjitgo/core/ports"
	"github.com/jmcarbo/datjitgo/core/value"

	ierrors "github.com/jmcarbo/datjitgo/core/errors"
)

func TestSQL_Format(t *testing.T) {
	if got := NewSQL().Format(); got != "sql" {
		t.Fatalf("Format() = %q", got)
	}
}

func TestSQL_RequiresDocument(t *testing.T) {
	_, ds := NewFixture(t)
	var buf bytes.Buffer
	err := NewSQL().Write(ds, nil, &buf, ports.WriteOptions{})
	var ie *ierrors.Error
	if !errors.As(err, &ie) || ie.Kind != ierrors.KindValidation {
		t.Fatalf("want KindValidation, got %T %v", err, err)
	}
}

func TestSQL_UnknownDialect(t *testing.T) {
	doc, ds := NewFixture(t)
	var buf bytes.Buffer
	err := NewSQL().Write(ds, doc, &buf, ports.WriteOptions{SQLDialect: "oracle"})
	var ie *ierrors.Error
	if !errors.As(err, &ie) || ie.Kind != ierrors.KindValidation {
		t.Fatalf("want KindValidation, got %T %v", err, err)
	}
}

func TestSQL_PostgresDialect(t *testing.T) {
	doc, ds := NewFixture(t)
	var buf bytes.Buffer
	if err := NewSQL().Write(ds, doc, &buf, ports.WriteOptions{SQLDialect: "postgres"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	s := buf.String()
	// Identifier quoting
	if !strings.Contains(s, `CREATE TABLE "User"`) {
		t.Fatalf("postgres quoting missing:\n%s", s)
	}
	if !strings.Contains(s, `"id" UUID`) {
		t.Fatalf("UUID type missing:\n%s", s)
	}
	if !strings.Contains(s, `"active" BOOLEAN`) {
		t.Fatalf("BOOLEAN missing:\n%s", s)
	}
	if !strings.Contains(s, `"balance" NUMERIC(12,2)`) {
		t.Fatalf("NUMERIC missing:\n%s", s)
	}
	if !strings.Contains(s, `"created_at" TIMESTAMPTZ`) {
		t.Fatalf("TIMESTAMPTZ missing:\n%s", s)
	}
	// Bool literal TRUE
	if !strings.Contains(s, "TRUE") {
		t.Fatalf("TRUE literal missing:\n%s", s)
	}
	// NULL rendered as NULL (nickname of row 2).
	if !strings.Contains(s, "NULL") {
		t.Fatalf("NULL literal missing:\n%s", s)
	}
	// Single-quote escaping for "Bob O'Brien".
	if !strings.Contains(s, "'Bob O''Brien'") {
		t.Fatalf("quote escaping missing:\n%s", s)
	}
	// Decimal bare literal.
	if !strings.Contains(s, "1234.56") {
		t.Fatalf("decimal literal missing:\n%s", s)
	}
}

func TestSQL_MySQLDialect(t *testing.T) {
	doc, ds := NewFixture(t)
	var buf bytes.Buffer
	if err := NewSQL().Write(ds, doc, &buf, ports.WriteOptions{SQLDialect: "mysql"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	s := buf.String()
	if !strings.Contains(s, "CREATE TABLE `User`") {
		t.Fatalf("backtick quoting missing:\n%s", s)
	}
	if !strings.Contains(s, "`id` CHAR(36)") {
		t.Fatalf("mysql uuid type missing:\n%s", s)
	}
	if !strings.Contains(s, "`active` TINYINT(1)") {
		t.Fatalf("mysql bool type missing:\n%s", s)
	}
	if !strings.Contains(s, "`balance` DECIMAL(12,2)") {
		t.Fatalf("mysql decimal type missing:\n%s", s)
	}
	// Must NOT contain TRUE/FALSE for mysql.
	if strings.Contains(s, ", TRUE,") || strings.Contains(s, ", FALSE,") {
		t.Fatalf("mysql should not use TRUE/FALSE:\n%s", s)
	}
	// Must contain 1 or 0 for bool.
	if !strings.Contains(s, ", 1,") && !strings.Contains(s, ", 0,") {
		t.Fatalf("mysql bool literal missing:\n%s", s)
	}
}

func TestSQL_SQLiteDialect(t *testing.T) {
	doc, ds := NewFixture(t)
	var buf bytes.Buffer
	if err := NewSQL().Write(ds, doc, &buf, ports.WriteOptions{SQLDialect: "sqlite"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	s := buf.String()
	if !strings.Contains(s, `CREATE TABLE "User"`) {
		t.Fatalf("sqlite quoting missing:\n%s", s)
	}
	if !strings.Contains(s, `"age" INTEGER`) {
		t.Fatalf("sqlite INTEGER type missing:\n%s", s)
	}
	if !strings.Contains(s, `"score" REAL`) {
		t.Fatalf("sqlite REAL type missing:\n%s", s)
	}
}

func TestSQL_InsertBatching(t *testing.T) {
	// Build a minimal doc/dataset with 250 rows for Item(id INT).
	doc := model.NewDocument()
	ent := model.NewEntity("Item")
	ent.Fields.Set("id", &model.Field{Name: "id", Type: model.Primitive{Kind: model.PrimInt}})
	doc.Entities.Set("Item", ent)

	ds := value.NewDataset()
	rows := make([]*value.Object, 0, 250)
	for i := int64(0); i < 250; i++ {
		o := value.NewObject()
		o.Set("id", value.Int(i))
		rows = append(rows, o)
	}
	ds.Entities.Set("Item", rows)

	var buf bytes.Buffer
	if err := NewSQL().Write(ds, doc, &buf, ports.WriteOptions{SQLDialect: "postgres"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	inserts := strings.Count(buf.String(), "INSERT INTO")
	// 250 rows, batches of 100 → 3 INSERT statements.
	if inserts != 3 {
		t.Fatalf("want 3 INSERTs, got %d", inserts)
	}
	// Each batch must terminate with a semicolon.
	semicolons := strings.Count(buf.String(), ");\n")
	// CREATE TABLE closing + 3 batches final row = 4 semicolons minimum.
	if semicolons < 4 {
		t.Fatalf("want at least 4 ');\\n' terminators, got %d", semicolons)
	}
}

func TestSQL_EntityFilter(t *testing.T) {
	doc, ds := NewFixture(t)
	var buf bytes.Buffer
	if err := NewSQL().Write(ds, doc, &buf, ports.WriteOptions{SQLDialect: "postgres", EntityFilter: "Order"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if strings.Contains(buf.String(), `CREATE TABLE "User"`) {
		t.Fatalf("entity filter leaked User:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), `CREATE TABLE "Order"`) {
		t.Fatalf("entity filter dropped Order:\n%s", buf.String())
	}
}
