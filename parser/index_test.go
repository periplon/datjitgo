package parser

import (
	"strings"
	"testing"
)

const indexSchema = `
domain: idx_test
entities:
  User:
    id: uuid @primary
    email: email @unique
    org: ->Org
    created_at: datetime
    _indexes:
      by_email:
        fields: [email]
        unique: true
      by_org_recent:
        fields: [org, created_at]
        where: "deleted_at IS NULL"
        method: btree
  Org:
    id: uuid @primary
`

func TestParseIndexes(t *testing.T) {
	doc, err := New().Parse(strings.NewReader(indexSchema), "idx.yaml")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	user, ok := doc.Entities.Get("User")
	if !ok {
		t.Fatal("User entity missing")
	}
	if len(user.Indexes) != 2 {
		t.Fatalf("want 2 indexes, got %d: %+v", len(user.Indexes), user.Indexes)
	}

	a := user.Indexes[0]
	if a.Name != "by_email" || !a.Unique || a.Source != "manual" {
		t.Fatalf("index[0] mismatch: %+v", a)
	}
	if len(a.Fields) != 1 || a.Fields[0] != "email" {
		t.Fatalf("index[0] fields: %+v", a.Fields)
	}

	b := user.Indexes[1]
	if b.Name != "by_org_recent" || b.Unique {
		t.Fatalf("index[1] mismatch: %+v", b)
	}
	if len(b.Fields) != 2 || b.Fields[0] != "org" || b.Fields[1] != "created_at" {
		t.Fatalf("index[1] fields: %+v", b.Fields)
	}
	if b.Where != "deleted_at IS NULL" || b.Method != "btree" {
		t.Fatalf("index[1] where/method: %+v", b)
	}

	// _indexes must not leak into the field map.
	if user.Fields.Has("_indexes") {
		t.Fatal("_indexes leaked into fields")
	}
}

func TestParseIndexesUnknownKey(t *testing.T) {
	const bad = `
domain: idx_bad
entities:
  User:
    id: uuid @primary
    _indexes:
      by_x:
        fields: [id]
        bogus: true
`
	if _, err := New().Parse(strings.NewReader(bad), "bad.yaml"); err == nil {
		t.Fatal("want error for unknown _indexes spec key, got nil")
	}
}

func TestParseIndexesFieldsMustBeList(t *testing.T) {
	const bad = `
domain: idx_bad2
entities:
  User:
    id: uuid @primary
    _indexes:
      by_x:
        fields: id
`
	if _, err := New().Parse(strings.NewReader(bad), "bad2.yaml"); err == nil {
		t.Fatal("want error for scalar fields, got nil")
	}
}
