package corpus

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestUpdateMkdirFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix file mode bits not enforced on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory mode bits")
	}
	parent := t.TempDir()
	readonly := filepath.Join(parent, "ro")
	if err := os.Mkdir(readonly, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readonly, 0o700) }) // let TempDir cleanup remove it

	// overlayDir under a read-only ancestor: MkdirAll(overlayDir/data) cannot
	// create the missing directory and Update must surface the error.
	overlayDir := filepath.Join(readonly, "sub")
	_, err := Update(context.Background(), overlayDir, []UpdateSource{{Key: "person.first_names", URL: "http://example.invalid"}})
	if err == nil {
		t.Fatal("expected MkdirAll permission error, got nil")
	}
	if !os.IsPermission(err) {
		t.Fatalf("Update error = %v, want permission error", err)
	}
}

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
	t.Setenv("DATJIT_CORPUS_SOURCE", "company.names=http://example.test/c")
	srcs, err := DefaultUpdateSources()
	if err != nil {
		t.Fatal(err)
	}
	if len(srcs) != 3 || srcs[0].Key != "person.first_names" || srcs[1].Key != "address.cities" || srcs[2].Key != "company.names" {
		t.Fatalf("sources = %+v", srcs)
	}
}

func TestDefaultOverlayDirEnvPrecedence(t *testing.T) {
	t.Setenv("DATJIT_CORPUS_DIR", "/tmp/datjit-corpus")
	if got := DefaultOverlayDir(); got != "/tmp/datjit-corpus" {
		t.Fatalf("overlay dir = %q", got)
	}

	t.Setenv("DATJIT_CORPUS_DIR", "")
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg")
	if got := DefaultOverlayDir(); got != filepath.Join("/tmp/xdg", "datjit", "corpus") {
		t.Fatalf("xdg overlay dir = %q", got)
	}

	t.Setenv("XDG_DATA_HOME", "")
	if got := DefaultOverlayDir(); !filepath.IsAbs(got) || filepath.Base(got) != "corpus" {
		t.Fatalf("home overlay dir = %q", got)
	}
}

func TestDefaultUpdateSourcesRejectsInvalidEnv(t *testing.T) {
	t.Setenv("DATJIT_CORPUS_SOURCES", "person.first_names=http://example.test/a, bad")
	if _, err := DefaultUpdateSources(); err == nil {
		t.Fatal("expected invalid multi-source error")
	}

	t.Setenv("DATJIT_CORPUS_SOURCES", "")
	t.Setenv("DATJIT_CORPUS_SOURCE", "missing-url")
	if _, err := DefaultUpdateSources(); err == nil {
		t.Fatal("expected invalid single-source error")
	}
}

func TestUpdateRejectsMissingSourcesAndFields(t *testing.T) {
	if _, err := Update(context.Background(), t.TempDir(), nil); err == nil {
		t.Fatal("expected no sources error")
	}
	if _, err := Update(context.Background(), t.TempDir(), []UpdateSource{{Key: "", URL: "http://example.test"}}); err == nil {
		t.Fatal("expected missing key error")
	}
	if _, err := Update(context.Background(), t.TempDir(), []UpdateSource{{Key: "person.first_names"}}); err == nil {
		t.Fatal("expected missing url error")
	}
}

func TestUpdateReportsDownloadHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer srv.Close()

	_, err := Update(context.Background(), t.TempDir(), []UpdateSource{{Key: "person.first_names", URL: srv.URL}})
	if err == nil {
		t.Fatal("expected download error")
	}
}

func TestUpdateUsesDefaultOverlayDirAndAtomicWriteErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`["Remote"]`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	t.Setenv("DATJIT_CORPUS_DIR", dir)
	updated, err := Update(context.Background(), "", []UpdateSource{{Key: "person.first_names", URL: srv.URL}})
	if err != nil {
		t.Fatal(err)
	}
	if len(updated) != 1 || updated[0] != "person.first_names" {
		t.Fatalf("updated = %v", updated)
	}
	if _, err := os.Stat(filepath.Join(dir, "data", "person_first_names.json")); err != nil {
		t.Fatal(err)
	}

	if err := atomicWrite(filepath.Join(t.TempDir(), "missing", "file.json"), []byte("x")); err == nil {
		t.Fatal("expected atomicWrite error for missing directory")
	}
}
