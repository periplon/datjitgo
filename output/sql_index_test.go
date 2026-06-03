package output

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	ierrors "github.com/periplon/datjitgo/core/errors"
	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/ports"
	"github.com/periplon/datjitgo/core/value"
)

// withIndexes attaches a manual + an inferred index to the fixture's User
// entity and returns the doc/dataset pair.
func withIndexes(t *testing.T) (*model.Document, *value.Dataset) {
	t.Helper()
	doc, ds := NewFixture(t)
	user, _ := doc.Entities.Get("User")
	user.Indexes = []model.Index{
		{Name: "by_name", Fields: []string{"name"}, Unique: true, Source: "manual"},
		{Name: "by_age_score", Fields: []string{"age", "score"}, Where: "score IS NOT NULL", Method: "btree", Source: "manual"},
		{Name: "idx_user_meta", Fields: []string{"meta"}, Source: "inferred"},
	}
	return doc, ds
}

func writeSQL(t *testing.T, doc *model.Document, ds *value.Dataset, opts ports.WriteOptions) string {
	t.Helper()
	var buf bytes.Buffer
	if err := NewSQL().Write(ds, doc, &buf, opts); err != nil {
		t.Fatalf("Write: %v", err)
	}
	return buf.String()
}

func TestSQL_IndexesPostgres(t *testing.T) {
	doc, ds := withIndexes(t)
	s := writeSQL(t, doc, ds, ports.WriteOptions{SQLDialect: "postgres"})
	if !strings.Contains(s, `CREATE UNIQUE INDEX "by_name" ON "User" ("name");`) {
		t.Fatalf("postgres unique index missing:\n%s", s)
	}
	if !strings.Contains(s, `CREATE INDEX "by_age_score" ON "User" USING btree ("age", "score") WHERE score IS NOT NULL;`) {
		t.Fatalf("postgres composite/partial/method index missing:\n%s", s)
	}
	// Default mode is manual → inferred index must NOT appear.
	if strings.Contains(s, "idx_user_meta") {
		t.Fatalf("inferred index leaked in manual mode:\n%s", s)
	}
}

func TestSQL_IndexesMySQL(t *testing.T) {
	doc, ds := withIndexes(t)
	s := writeSQL(t, doc, ds, ports.WriteOptions{SQLDialect: "mysql"})
	// MySQL: USING precedes ON; partial WHERE dropped.
	if !strings.Contains(s, "CREATE INDEX `by_age_score` USING btree ON `User` (`age`, `score`);") {
		t.Fatalf("mysql USING-before-ON form missing:\n%s", s)
	}
	if strings.Contains(s, "WHERE") {
		t.Fatalf("mysql should drop partial WHERE:\n%s", s)
	}
}

func TestSQL_IndexesSQLite(t *testing.T) {
	doc, ds := withIndexes(t)
	s := writeSQL(t, doc, ds, ports.WriteOptions{SQLDialect: "sqlite"})
	// SQLite: method dropped, partial WHERE kept.
	if !strings.Contains(s, `CREATE INDEX "by_age_score" ON "User" ("age", "score") WHERE score IS NOT NULL;`) {
		t.Fatalf("sqlite partial index missing:\n%s", s)
	}
	if strings.Contains(s, "USING") {
		t.Fatalf("sqlite should drop USING method:\n%s", s)
	}
}

func TestSQL_IndexModeNone(t *testing.T) {
	doc, ds := withIndexes(t)
	s := writeSQL(t, doc, ds, ports.WriteOptions{SQLDialect: "postgres", SQLIndexes: "none"})
	if strings.Contains(s, "CREATE INDEX") || strings.Contains(s, "CREATE UNIQUE INDEX") {
		t.Fatalf("none mode should suppress all indexes:\n%s", s)
	}
}

func TestSQL_IndexModeAutoIncludesInferred(t *testing.T) {
	doc, ds := withIndexes(t)
	s := writeSQL(t, doc, ds, ports.WriteOptions{SQLDialect: "postgres", SQLIndexes: "auto"})
	if !strings.Contains(s, `CREATE INDEX "idx_user_meta" ON "User" ("meta");`) {
		t.Fatalf("auto mode should include inferred index:\n%s", s)
	}
	if !strings.Contains(s, `CREATE UNIQUE INDEX "by_name"`) {
		t.Fatalf("auto mode should still include manual index:\n%s", s)
	}
}

func TestSQL_IndexUnknownMode(t *testing.T) {
	doc, ds := withIndexes(t)
	var buf bytes.Buffer
	err := NewSQL().Write(ds, doc, &buf, ports.WriteOptions{SQLDialect: "postgres", SQLIndexes: "bogus"})
	var ie *ierrors.Error
	if !errors.As(err, &ie) || ie.Kind != ierrors.KindValidation {
		t.Fatalf("want KindValidation for unknown index mode, got %T %v", err, err)
	}
}

func TestClampIdent(t *testing.T) {
	short := "by_email"
	if got := clampIdent(short, "postgres"); got != short {
		t.Fatalf("short ident should pass through, got %q", got)
	}
	long := strings.Repeat("a", 100)
	got := clampIdent(long, "postgres")
	if len(got) != 63 {
		t.Fatalf("postgres clamp should be 63 bytes, got %d (%q)", len(got), got)
	}
	// Deterministic: same input → same output.
	if clampIdent(long, "postgres") != got {
		t.Fatalf("clampIdent not deterministic")
	}
	// Distinct long names → distinct clamped names (hash suffix differs).
	other := strings.Repeat("b", 100)
	if clampIdent(other, "postgres") == got {
		t.Fatalf("distinct long names collided after clamp")
	}
}
