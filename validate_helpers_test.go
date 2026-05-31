package datjit_test

import (
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	datjit "github.com/periplon/datjitgo"
	derrs "github.com/periplon/datjitgo/core/errors"
)

func TestValidateStringSucceedsAndReportsErrors(t *testing.T) {
	doc, err := datjit.ValidateString(helperSchema)
	if err != nil {
		t.Fatalf("ValidateString valid schema: %v", err)
	}
	if doc == nil || doc.Domain != "helper" {
		t.Fatalf("ValidateString doc = %+v", doc)
	}

	if _, err := datjit.ValidateString("domain: ["); err == nil {
		t.Fatal("ValidateString malformed schema: want error, got nil")
	}

	// Reference to an undeclared entity is a validation (not parse) error and
	// still returns the parsed document.
	badRef := `domain: badref
volume:
  Order: 1
entities:
  Order:
    id: int @primary
    user: ->Missing
`
	d, err := datjit.ValidateString(badRef)
	if err == nil {
		t.Fatal("ValidateString undeclared reference: want error, got nil")
	}
	if d == nil {
		t.Fatal("ValidateString should return the parsed document on validation error")
	}
	if !stderrors.Is(err, derrs.ErrValidation) {
		t.Fatalf("ValidateString error = %v, want validation kind", err)
	}
}

func TestValidateFileSucceedsAndReportsIOError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.yaml")
	if err := os.WriteFile(path, []byte(helperSchema), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := datjit.ValidateFile(path)
	if err != nil {
		t.Fatalf("ValidateFile valid schema: %v", err)
	}
	if doc == nil || doc.Domain != "helper" {
		t.Fatalf("ValidateFile doc = %+v", doc)
	}

	_, err = datjit.ValidateFile(filepath.Join(dir, "does-not-exist.yaml"))
	if err == nil {
		t.Fatal("ValidateFile missing path: want error, got nil")
	}
	if !stderrors.Is(err, derrs.ErrIO) {
		t.Fatalf("ValidateFile missing path error = %v, want IO kind", err)
	}
}
