package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmcarbo/datjitgo"
	"github.com/jmcarbo/datjitgo/core/model"
	"github.com/jmcarbo/datjitgo/repl"
)

func TestInternalGenerateHelpers(t *testing.T) {
	vols, err := parseVolumeFlags([]string{"User=3", "Order=4"})
	if err != nil {
		t.Fatal(err)
	}
	if vols["User"] != 3 || vols["Order"] != 4 {
		t.Fatalf("unexpected volumes: %v", vols)
	}
	if _, err := parseVolumeFlags([]string{"bad"}); err == nil {
		t.Fatal("expected malformed volume error")
	}

	svc := datjit.NewDefault()
	if !formatSupported(svc, "json") {
		t.Fatal("json should be supported")
	}
	if formatSupported(svc, "bogus") {
		t.Fatal("bogus format should not be supported")
	}

	doc := model.NewDocument()
	doc.Entities.Set("User", model.NewEntity("User"))
	doc.Volume["User"] = model.VolumeSpec{Min: 2, Max: 8}
	var buf bytes.Buffer
	if err := printGeneratePlan(&buf, doc, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "User=5") {
		t.Fatalf("expected midpoint volume, got %q", buf.String())
	}
}

func TestInternalOpenOutputAndParseSchemaFile(t *testing.T) {
	w, closeFn, err := openOutput("-")
	if err != nil {
		t.Fatal(err)
	}
	if w == nil {
		t.Fatal("stdout writer should not be nil")
	}
	if err := closeFn(); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.txt")
	w, closeFn, err = openOutput(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("ok")); err != nil {
		t.Fatal(err)
	}
	if err := closeFn(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "ok" {
		t.Fatalf("unexpected file contents %q", b)
	}

	schemaPath := filepath.Join(dir, "schema.yaml")
	if err := os.WriteFile(schemaPath, []byte("domain: cli\nentities:\n  User:\n    id: int\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	doc, err := parseSchemaFile(datjit.NewDefault(), schemaPath)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Domain != "cli" || doc.Entities.Len() != 1 {
		t.Fatalf("unexpected doc: %+v", doc)
	}
}

func TestInternalCorpusHelpers(t *testing.T) {
	keys := resolveCorpusKeys(datjit.NewDefault().Corpus())
	if len(keys) == 0 {
		t.Fatal("expected embedded corpus keys")
	}
	if keys[0] > keys[len(keys)-1] {
		t.Fatalf("keys are not sorted: %v", keys)
	}
	var buf bytes.Buffer
	if err := printCorpusInfo(&buf, datjit.NewDefault().Corpus()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "keys:") || !strings.Contains(buf.String(), "entries:") {
		t.Fatalf("unexpected info output: %q", buf.String())
	}
	if err := printCorpusInfo(&buf, nil); err == nil {
		t.Fatal("expected nil provider error")
	}
}

func TestInternalInspectHelpers(t *testing.T) {
	if severityTag(model.RuleProbabilistic) != "@prob" {
		t.Fatal("probabilistic severity tag mismatch")
	}
	if severityTag(model.RuleWarn) != "@warn" {
		t.Fatal("warn severity tag mismatch")
	}
	if severityTag(model.RuleStrict) != "@strict" {
		t.Fatal("strict severity tag mismatch")
	}
	if renderVolume(model.VolumeSpec{Min: 1, Max: 3}) != "1..3" {
		t.Fatal("range volume rendering mismatch")
	}

	readonly := model.NewEntity("Readonly")
	readonly.Meta = []model.Decorator{{Name: "readonly"}}
	if got := strings.Join(inferToolSurface(readonly), ","); got != "list,get" {
		t.Fatalf("readonly tools: %s", got)
	}
	immutable := model.NewEntity("Immutable")
	immutable.Meta = []model.Decorator{{Name: "immutable"}}
	if got := strings.Join(inferToolSurface(immutable), ","); got != "list,get,create" {
		t.Fatalf("immutable tools: %s", got)
	}

	doc := model.NewDocument()
	doc.Domain = "inspect"
	user := model.NewEntity("User")
	user.Meta = []model.Decorator{{Name: "readonly"}}
	user.Fields.Set("id", &model.Field{Name: "id", Type: model.Primitive{Kind: model.PrimInt}})
	doc.Entities.Set("User", user)
	insp, err := datjit.NewDefault().Inspect(doc)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := printInspection(&buf, doc, insp, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "domain: inspect") || !strings.Contains(buf.String(), "tools:") {
		t.Fatalf("unexpected inspection output: %q", buf.String())
	}
}

func TestInternalRootCommandAndUsageErrors(t *testing.T) {
	root := newRootCmd()
	for _, name := range []string{"generate", "validate", "inspect", "corpus", "repl", "version"} {
		if _, _, err := root.Find([]string{name}); err != nil {
			t.Fatalf("missing command %s: %v", name, err)
		}
	}
	err := usageErrorf("bad %s", "args")
	if exitCodeFor(err) != 2 {
		t.Fatal("usage errors should exit 2")
	}
	if exitCodeFor(errors.New("runtime")) != 1 {
		t.Fatal("runtime errors should exit 1")
	}
	if !strings.Contains(err.Error(), "bad args") {
		t.Fatalf("unexpected usage error: %v", err)
	}
	if errors.Unwrap(err) == nil {
		t.Fatal("usage error should unwrap")
	}
	var out bytes.Buffer
	versionCmd := cmdVersion()
	versionCmd.SetOut(&out)
	if err := versionCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), version) {
		t.Fatalf("version output: %q", out.String())
	}
}

func TestInternalCobraCommandsExecuteRunEPaths(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.yaml")
	if err := os.WriteFile(schemaPath, []byte("domain: cli\nvolume:\n  User: 1\nentities:\n  User:\n    id: int\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	run := func(cmdName string, args ...string) string {
		t.Helper()
		root := newRootCmd()
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs(append([]string{cmdName}, args...))
		if err := root.Execute(); err != nil {
			t.Fatalf("%s %v: %v\n%s", cmdName, args, err, out.String())
		}
		return out.String()
	}

	if out := run("validate", schemaPath); !strings.Contains(out, "OK") {
		t.Fatalf("validate output: %q", out)
	}
	if out := run("inspect", schemaPath, "--infer-tools"); !strings.Contains(out, "tools:") {
		t.Fatalf("inspect output: %q", out)
	}
	if out := run("generate", schemaPath, "--dry-run"); !strings.Contains(out, "plan:") {
		t.Fatalf("dry-run output: %q", out)
	}
	outPath := filepath.Join(dir, "out.json")
	run("generate", schemaPath, "-o", outPath, "--pretty")
	if b, err := os.ReadFile(outPath); err != nil || !bytes.Contains(b, []byte("User")) {
		t.Fatalf("generate output file: %q %v", b, err)
	}
	if out := run("corpus", "list"); !strings.Contains(out, "person.first_names") {
		t.Fatalf("corpus list output: %q", out)
	}
	if out := run("corpus", "info"); !strings.Contains(out, "entries:") {
		t.Fatalf("corpus info output: %q", out)
	}
	if out := run("corpus", "update"); !strings.Contains(out, "deferred") {
		t.Fatalf("corpus update output: %q", out)
	}
}

func TestInternalReplPreload(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.yaml")
	if err := os.WriteFile(schemaPath, []byte("domain: pre\nentities:\n  User:\n    id: int\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sess := repl.New(datjit.NewDefault())
	preload(sess, schemaPath)
	if sess.State().Doc == nil || sess.State().Doc.Domain != "pre" || sess.State().Path != schemaPath {
		t.Fatalf("preload failed: %+v", sess.State())
	}
	preload(sess, filepath.Join(dir, "missing.yaml"))
}
