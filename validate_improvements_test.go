package datjit

import (
	"strings"
	"testing"
)

func TestValidateChecksReusableTypeFields(t *testing.T) {
	src := `
domain: bad_type
types:
  Profile:
    owner: ->Missing
entities:
  User:
    profile: Profile
`
	svc := NewDefault()
	doc, err := svc.Parse(strings.NewReader(src), "bad.yaml")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	err = svc.Validate(doc)
	if err == nil {
		t.Fatal("Validate succeeded; expected missing reference inside reusable type")
	}
	if !strings.Contains(err.Error(), "Missing") {
		t.Fatalf("error %q does not mention missing reference", err)
	}
}

func TestValidateRejectsUnknownSemanticType(t *testing.T) {
	src := `
domain: typo
entities:
  User:
    email: emali
`
	svc := NewDefault()
	doc, err := svc.Parse(strings.NewReader(src), "bad.yaml")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	err = svc.Validate(doc)
	if err == nil {
		t.Fatal("Validate succeeded; expected unknown semantic type")
	}
	if !strings.Contains(err.Error(), "emali") {
		t.Fatalf("error %q does not mention unknown semantic", err)
	}
}
