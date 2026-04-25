package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
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
	if _, err := parseVolumeFlags([]string{"User="}); err == nil {
		t.Fatal("expected empty volume count error")
	}
	if _, err := parseVolumeFlags([]string{"User=-1"}); err == nil {
		t.Fatal("expected negative volume error")
	}
	emptyVols, err := parseVolumeFlags([]string{"  "})
	if err != nil {
		t.Fatal(err)
	}
	if emptyVols == nil || len(emptyVols) != 0 {
		t.Fatalf("blank volume item should produce empty map, got %v", emptyVols)
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
	buf.Reset()
	if err := printGeneratePlan(&buf, doc, map[string]int{"User": 7}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "User=7") {
		t.Fatalf("expected override volume, got %q", buf.String())
	}
	if got := plannedVolume("Missing", doc, nil); got != 10 {
		t.Fatalf("default planned volume=%d", got)
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
	if _, err := parseSchemaFile(datjit.NewDefault(), filepath.Join(dir, "missing.yaml")); err == nil {
		t.Fatal("expected open error for missing schema")
	}
	if _, _, err := openOutput(filepath.Join(dir, "missing", "out.txt")); err == nil {
		t.Fatal("expected create error for missing parent directory")
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
	if renderVolume(model.VolumeSpec{Exact: 4}) != "4" {
		t.Fatal("exact volume rendering mismatch")
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
	mutable := model.NewEntity("Mutable")
	if got := strings.Join(inferToolSurface(mutable), ","); got != "list,get,create,update,delete" {
		t.Fatalf("default tools: %s", got)
	}

	doc := model.NewDocument()
	doc.Domain = "inspect"
	doc.Version = "1"
	doc.Enums.Set("Status", model.EnumDef{Variants: []model.EnumVariant{{Value: "active"}}})
	user := model.NewEntity("User")
	user.Meta = []model.Decorator{{Name: "readonly"}}
	user.Fields.Set("id", &model.Field{Name: "id", Type: model.Primitive{Kind: model.PrimInt}})
	doc.Entities.Set("User", user)
	doc.Rules = append(doc.Rules, model.Rule{Expr: "User.id > 0", Severity: model.RuleWarn})
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
	if !strings.Contains(buf.String(), "enums (1):") || !strings.Contains(buf.String(), "rules (1):") {
		t.Fatalf("expected enums and rules in inspection output: %q", buf.String())
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`["Remote"]`))
	}))
	defer srv.Close()
	corpusDir := filepath.Join(dir, "corpus")
	if out := run("corpus", "update", "--corpus-dir", corpusDir, "--source", "person.first_names="+srv.URL); !strings.Contains(out, "updated 1 corpus keys") {
		t.Fatalf("corpus update output: %q", out)
	}
}

func TestInternalCobraCommandErrorPaths(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.yaml")
	if err := os.WriteFile(schemaPath, []byte("domain: cli\nentities:\n  User:\n    id: int\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	runErr := func(args ...string) error {
		t.Helper()
		root := newRootCmd()
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs(args)
		return root.Execute()
	}

	for _, tc := range [][]string{
		{"generate"},
		{"generate", schemaPath, "--format", "bogus"},
		{"generate", schemaPath, "--volume", "User=nope"},
		{"inspect"},
		{"validate"},
	} {
		if err := runErr(tc...); err == nil {
			t.Fatalf("%v: expected error", tc)
		}
	}
	if err := runErr("generate", filepath.Join(dir, "missing.yaml")); err == nil {
		t.Fatal("expected missing schema error")
	}
	if err := runErr("inspect", filepath.Join(dir, "missing.yaml")); err == nil {
		t.Fatal("expected inspect missing schema error")
	}
	if err := runErr("validate", filepath.Join(dir, "missing.yaml")); err == nil {
		t.Fatal("expected validate missing schema error")
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

func TestInternalCmdReplRunsScriptedStdin(t *testing.T) {
	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	oldIn, oldOut, oldErr := os.Stdin, os.Stdout, os.Stderr
	os.Stdin, os.Stdout, os.Stderr = inR, outW, errW
	t.Cleanup(func() {
		os.Stdin, os.Stdout, os.Stderr = oldIn, oldOut, oldErr
		_ = inR.Close()
		_ = outR.Close()
		_ = errR.Close()
	})

	if _, err := inW.WriteString("help\nexit\n"); err != nil {
		t.Fatal(err)
	}
	if err := inW.Close(); err != nil {
		t.Fatal(err)
	}

	cmd := cmdRepl()
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	_ = outW.Close()
	_ = errW.Close()
	outBytes, err := io.ReadAll(outR)
	if err != nil {
		t.Fatal(err)
	}
	errBytes, err := io.ReadAll(errR)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(outBytes, []byte("load <path>")) {
		t.Fatalf("expected help output, got stdout=%q stderr=%q", outBytes, errBytes)
	}
	if len(errBytes) != 0 {
		t.Fatalf("unexpected stderr: %q", errBytes)
	}
}
