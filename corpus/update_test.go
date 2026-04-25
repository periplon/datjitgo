package corpus

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateDownloadsAndWritesOverlay(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"name":"Remote","weight":2},"Backup"]`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	updated, err := Update(context.Background(), dir, []UpdateSource{{Key: "person.first_names", URL: srv.URL}})
	if err != nil {
		t.Fatal(err)
	}
	if len(updated) != 1 || updated[0] != "person.first_names" {
		t.Fatalf("updated = %v", updated)
	}
	p := NewWithOverlay(dir)
	entries, err := p.List("en-US", "person.first_names")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Name != "Backup" || entries[1].Name != "Remote" {
		t.Fatalf("entries = %+v", entries)
	}
	if _, err := os.Stat(filepath.Join(dir, "data", "person_first_names.json")); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateRejectsInvalidJSONWithoutWriting(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"name":"not array"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	_, err := Update(context.Background(), dir, []UpdateSource{{Key: "person.first_names", URL: srv.URL}})
	if err == nil {
		t.Fatal("expected error")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "data", "person_first_names.json")); !os.IsNotExist(statErr) {
		t.Fatalf("unexpected output file err=%v", statErr)
	}
}

func TestUpdateRejectsUnsafeKey(t *testing.T) {
	dir := t.TempDir()
	_, err := Update(context.Background(), dir, []UpdateSource{{Key: "../outside", URL: "http://example.test/data.json"}})
	if err == nil {
		t.Fatal("expected error")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "..", "outside.json")); !os.IsNotExist(statErr) {
		t.Fatalf("unexpected escaped file err=%v", statErr)
	}
}

func TestDefaultUpdateSourcesFromEnv(t *testing.T) {
	t.Setenv("DATJIT_CORPUS_SOURCES", "person.first_names=http://example.test/a, address.cities=http://example.test/b")
	srcs, err := DefaultUpdateSources()
	if err != nil {
		t.Fatal(err)
	}
	if len(srcs) != 2 || srcs[0].Key != "person.first_names" || srcs[1].Key != "address.cities" {
		t.Fatalf("sources = %+v", srcs)
	}
}
