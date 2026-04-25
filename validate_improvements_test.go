package datjit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmcarbo/datjitgo/corpus"
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

func TestValidateAllowsCorpusBackedSemanticType(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "data", "custom_values.json"), []byte(`["one"]`), 0o644); err != nil {
		t.Fatal(err)
	}
	src := `
domain: custom
entities:
  Thing:
    label: custom.values
`
	svc, err := New(WithCorpus(corpus.NewWithOverlay(dir)))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := svc.Parse(strings.NewReader(src), "custom.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Validate(doc); err != nil {
		t.Fatalf("Validate returned error: %v", err)
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
