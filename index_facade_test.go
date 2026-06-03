package datjit_test

import (
	"strings"
	"testing"

	datjit "github.com/periplon/datjitgo"
)

const inferSchema = `
domain: idx_infer
entities:
  User:
    id: uuid @primary
    email: email @unique
    org: ->Org
    owner: "->User | ->Org"
  Org:
    id: uuid @primary
`

func TestInferredIndexes(t *testing.T) {
	svc := datjit.NewDefault()
	doc, err := svc.Parse(strings.NewReader(inferSchema), "infer.yaml")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	user, _ := doc.Entities.Get("User")

	got := map[string]string{} // name -> joined fields
	for _, idx := range user.Indexes {
		if idx.Source != "inferred" {
			t.Fatalf("expected only inferred indexes, got %+v", idx)
		}
		got[idx.Name] = strings.Join(idx.Fields, ",")
	}

	want := map[string]string{
		"idx_user_email_uniq": "email",
		"idx_user_org":        "org",
		"idx_user_owner":      "owner,owner_type", // polymorphic pair
	}
	if len(got) != len(want) {
		t.Fatalf("want %d inferred indexes, got %d: %+v", len(want), len(got), got)
	}
	for name, fields := range want {
		if got[name] != fields {
			t.Fatalf("index %q fields = %q, want %q (all: %+v)", name, got[name], fields, got)
		}
	}
	// id is @primary → not indexed.
	if _, ok := got["idx_user_id"]; ok {
		t.Fatal("primary-key field should not be inferred-indexed")
	}
}

func TestInferredIndexDedupVsManual(t *testing.T) {
	const schema = `
domain: idx_dedup
entities:
  User:
    id: uuid @primary
    email: email @unique
    _indexes:
      my_email:
        fields: [email]
  Org:
    id: uuid @primary
`
	svc := datjit.NewDefault()
	doc, err := svc.Parse(strings.NewReader(schema), "dedup.yaml")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	user, _ := doc.Entities.Get("User")

	emailIdxs := 0
	for _, idx := range user.Indexes {
		if strings.Join(idx.Fields, ",") == "email" {
			emailIdxs++
		}
	}
	if emailIdxs != 1 {
		t.Fatalf("manual index on [email] should suppress the inferred one; got %d email indexes: %+v", emailIdxs, user.Indexes)
	}
	// The surviving one is the manual.
	if user.Indexes[0].Source != "manual" || user.Indexes[0].Name != "my_email" {
		t.Fatalf("manual index should win: %+v", user.Indexes[0])
	}
}

func TestValidateUnknownIndexField(t *testing.T) {
	const schema = `
domain: idx_badfield
entities:
  User:
    id: uuid @primary
    _indexes:
      by_x:
        fields: [nonexistent]
`
	svc := datjit.NewDefault()
	doc, err := svc.Parse(strings.NewReader(schema), "badfield.yaml")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := svc.Validate(doc); err == nil {
		t.Fatal("want validation error for unknown index field, got nil")
	}
}

func TestValidateEmptyIndexFields(t *testing.T) {
	const schema = `
domain: idx_empty
entities:
  User:
    id: uuid @primary
    _indexes:
      by_x:
        fields: []
`
	svc := datjit.NewDefault()
	doc, err := svc.Parse(strings.NewReader(schema), "empty.yaml")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := svc.Validate(doc); err == nil {
		t.Fatal("want validation error for empty index fields, got nil")
	}
}
