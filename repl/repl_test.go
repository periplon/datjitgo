package repl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	datjit "github.com/periplon/datjitgo"
)

// fixture returns an absolute path to a testdata fixture. Tests run from the
// repl/ directory so we walk one level up to reach testdata/.
func fixture(t *testing.T, rel string) string {
	t.Helper()
	// filepath.Abs is applied so error messages are readable even when the
	// test harness chdirs.
	abs, err := filepath.Abs(filepath.Join("..", "testdata", "fixtures", rel))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	if _, err := os.Stat(abs); err != nil {
		t.Fatalf("fixture %q not found: %v", rel, err)
	}
	return abs
}

// runSession feeds lines to a fresh Session and returns stdout/stderr.
func runSession(t *testing.T, lines ...string) (string, string) {
	t.Helper()
	script := strings.NewReader(strings.Join(lines, "\n") + "\n")
	var out, errw bytes.Buffer
	svc := datjit.NewDefault()
	sess := New(svc)
	if err := sess.Run(context.Background(), script, &out, &errw); err != nil {
		t.Fatalf("Run: %v", err)
	}
	return out.String(), errw.String()
}

func TestReplLoadGenerate(t *testing.T) {
	minimal := fixture(t, "minimal.yaml")
	out, errw := runSession(t,
		"load "+minimal,
		"generate",
		"exit",
	)
	if !strings.Contains(out, "User") {
		t.Fatalf("expected output to contain \"User\"; got:\n%s\nstderr:\n%s", out, errw)
	}
	// Find the JSON document in out — `load` prints a status line first, so
	// we scan forward to the first `{`.
	idx := strings.Index(out, "{")
	if idx < 0 {
		t.Fatalf("no JSON object in output:\n%s", out)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(out[idx:]), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\nbody:\n%s", err, out[idx:])
	}
	if _, ok := decoded["User"]; !ok {
		t.Fatalf("expected key \"User\" in JSON; got keys %v", keysOf(decoded))
	}
}

// keysOf returns map keys for test diagnostics.
func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestReplInspect(t *testing.T) {
	minimal := fixture(t, "minimal.yaml")
	out, _ := runSession(t,
		"load "+minimal,
		"inspect",
		"exit",
	)
	if !strings.Contains(out, "domain:") {
		t.Fatalf("expected inspect output to contain \"domain:\"; got:\n%s", out)
	}
}

func TestReplInspectInferTools(t *testing.T) {
	entityMeta := fixture(t, "entity_meta.yaml")
	out, _ := runSession(t,
		"load "+entityMeta,
		"inspect --infer-tools",
		"exit",
	)
	for _, want := range []string{
		"tools:",
		"User: list, get, create, update, delete",
		"AuditLog: list, get",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected inspect --infer-tools output to contain %q; got:\n%s", want, out)
		}
	}
}

func TestReplCorpusListUsesServiceKeys(t *testing.T) {
	out, _ := runSession(t,
		"corpus list",
		"exit",
	)
	if !strings.Contains(out, "person.first_names") {
		t.Fatalf("expected real corpus key in output; got:\n%s", out)
	}
	if strings.Contains(out, "person.full") {
		t.Fatalf("expected fake corpus list to be removed; got:\n%s", out)
	}
}

func TestReplCorpusInfoMatchesCLISummary(t *testing.T) {
	out, _ := runSession(t,
		"corpus info",
		"exit",
	)
	for _, want := range []string{"keys:", "entries:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected corpus info to contain %q; got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "TBD") {
		t.Fatalf("expected corpus info TBD message to be removed; got:\n%s", out)
	}
}

func TestReplValidateFailure(t *testing.T) {
	// Write a schema whose field references a non-existent entity.
	dir := t.TempDir()
	broken := filepath.Join(dir, "broken.yaml")
	body := "" +
		"domain: broken\n" +
		"entities:\n" +
		"  A:\n" +
		"    id: uuid @primary\n" +
		"    missing: ->Ghost\n"
	if err := os.WriteFile(broken, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	// The REPL must keep running after the validation failure until `exit`
	// is received — the `validate` line below is deliberately followed by
	// more commands to prove the loop survived.
	script := strings.NewReader(strings.Join([]string{
		"load " + broken,
		"validate",
		"help",
		"exit",
	}, "\n") + "\n")
	var out, errw bytes.Buffer
	svc := datjit.NewDefault()
	sess := New(svc)
	if err := sess.Run(context.Background(), script, &out, &errw); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(errw.String(), "validation error") {
		t.Fatalf("expected stderr to contain \"validation error\"; got:\n%s", errw.String())
	}
	// The subsequent `help` must have produced output, proving the loop
	// continued past the failure.
	if !strings.Contains(out.String(), "load") {
		t.Fatalf("expected help output after failing validate; stdout:\n%s", out.String())
	}
}

func TestReplHelp(t *testing.T) {
	out, _ := runSession(t, "help", "exit")
	for _, name := range []string{"load", "reload", "show", "set", "generate", "validate", "inspect", "corpus", "formats", "history", "clear", "exit", "quit"} {
		if !strings.Contains(out, name) {
			t.Fatalf("help missing command %q in output:\n%s", name, out)
		}
	}
	out2, _ := runSession(t, "help load", "exit")
	if !strings.Contains(out2, "load <path>") {
		t.Fatalf("expected `help load` to show load docstring; got:\n%s", out2)
	}
}

func TestReplCompletion(t *testing.T) {
	svc := datjit.NewDefault()
	sess := New(svc)
	c := newCompleter(sess)
	candidates, _ := c.Do([]rune("lo"), 2)
	if !containsRune(candidates, "ad") {
		t.Fatalf("expected completion to offer `load` (suffix `ad`); got %v", runeSlicesToStrings(candidates))
	}
}

// containsRune reports whether any candidate equals target.
func containsRune(candidates [][]rune, target string) bool {
	for _, c := range candidates {
		if string(c) == target {
			return true
		}
	}
	return false
}

// runeSlicesToStrings flattens [][]rune into []string for diagnostics.
func runeSlicesToStrings(in [][]rune) []string {
	out := make([]string, len(in))
	for i, r := range in {
		out[i] = string(r)
	}
	return out
}

func TestReplFormatSwitch(t *testing.T) {
	minimal := fixture(t, "minimal.yaml")
	out, errw := runSession(t,
		"load "+minimal,
		"set format yaml",
		"generate",
		"exit",
	)
	// YAML begins with a top-level key like `User:` (possibly preceded by
	// status lines from `load` and `set format`). Scan for the first line
	// starting with an identifier followed by ':' after those status lines.
	if !strings.Contains(out, "User:") {
		t.Fatalf("expected YAML output to contain `User:`; got:\n%s\nstderr:\n%s", out, errw)
	}
}

func TestReplShowSetCorpusFormatsHistoryAndClear(t *testing.T) {
	minimal := fixture(t, "minimal.yaml")
	out, errw := runSession(t,
		"load "+minimal,
		"show schema",
		"show entities",
		"show enums",
		"show rules",
		"show volume",
		"set seed 99",
		"set locale en-US",
		"set pretty on",
		"set sql-dialect sqlite",
		"set entity User",
		"set entity none",
		"set volume User=1",
		"formats",
		"corpus list",
		"corpus info person.full",
		"history",
		"clear",
		"reload",
		"exit",
	)
	for _, want := range []string{"domain:", "User", "seed=99", "locale=en-US", "pretty=true", "sql-dialect=sqlite", "entity=User", "entity=(none)", "json", "person.full", "history"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s\nstderr:\n%s", want, out, errw)
		}
	}
	if errw != "" {
		t.Fatalf("unexpected stderr:\n%s", errw)
	}
}

func TestReplCommandErrorsStayInSession(t *testing.T) {
	out, errw := runSession(t,
		"# ignored comment",
		"   ",
		"unknowncmd",
		"load",
		"load /definitely/missing.yaml",
		"reload",
		"show bad",
		"set seed nope",
		"set format bogus",
		"set pretty maybe",
		"set unknown value",
		"set volume User=-1",
		"corpus",
		"corpus info",
		"corpus nope",
		"help nope",
		"exit",
	)
	for _, want := range []string{"unknown command", "usage: load", "open /definitely/missing.yaml", "nothing to reload", "no schema loaded", "invalid seed", "unknown format", "pretty must", "unknown set option", "usage: corpus", "unknown corpus", "no help"} {
		if !strings.Contains(errw, want) {
			t.Fatalf("stderr missing %q:\n%s\nstdout:\n%s", want, errw, out)
		}
	}
}

func TestReplGenerateWritesConfiguredOutputFile(t *testing.T) {
	minimal := fixture(t, "minimal.yaml")
	outPath := filepath.Join(t.TempDir(), "nested", "users.ndjson")
	out, errw := runSession(t,
		"load "+minimal,
		"set format ndjson",
		"set output "+outPath,
		"generate",
		"exit",
	)
	if errw != "" {
		t.Fatalf("unexpected stderr:\n%s\nstdout:\n%s", errw, out)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read generated output: %v", err)
	}
	if !bytes.Contains(body, []byte(`"id"`)) || !bytes.Contains(body, []byte(`"email"`)) {
		t.Fatalf("generated file missing expected row fields:\n%s", body)
	}
}

func TestReplRunScriptedHonorsCancelledContext(t *testing.T) {
	sess := New(datjit.NewDefault())
	var out, errw bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := sess.Run(ctx, strings.NewReader("help\n"), &out, &errw)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.Len() != 0 || errw.Len() != 0 {
		t.Fatalf("cancelled run produced output=%q err=%q", out.String(), errw.String())
	}
}

func TestReplDirectCommandBranches(t *testing.T) {
	svc := datjit.NewDefault()
	doc, err := svc.Parse(strings.NewReader(`domain: direct
volume:
  User: 1..3
enums:
  Status: [active, inactive]
entities:
  User:
    id: int
    status: Status
rules:
  - User.id > 0 @warn
`), "direct.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var out, errw bytes.Buffer
	s := New(svc)
	s.out = &out
	s.errw = &errw
	s.state.Doc = doc
	s.state.Volumes["User"] = 5
	s.history = []string{"one", "two"}

	for _, args := range [][]string{{}, {"schema"}, {"entities"}, {"enums"}, {"rules"}, {"volume"}, {"unknown"}} {
		_ = cmdShow(s, args)
	}
	for _, args := range [][]string{
		{"seed"},
		{"volume", "bad"},
		{"volume", "User=nope"},
		{"output", filepath.Join(t.TempDir(), "out.json")},
	} {
		_ = cmdSet(s, args)
	}
	if _, closer, err := s.openOutput(); err != nil {
		t.Fatal(err)
	} else if closer != nil {
		_ = closer.Close()
	}
	_ = cmdValidate(s, nil)
	_ = cmdInspect(s, []string{"--bad"})
	_ = cmdCorpus(s, []string{"list"})
	_ = cmdCorpus(s, []string{"info", "person.full"})
	_ = cmdFormats(s, nil)
	_ = cmdHistory(s, nil)
	_ = cmdClear(s, nil)
	if err := cmdExit(s, nil); !errors.Is(err, errExit) {
		t.Fatalf("exit err=%v", err)
	}
	if out.Len() == 0 || errw.Len() == 0 {
		t.Fatalf("expected both stdout and stderr, out=%q err=%q", out.String(), errw.String())
	}
}

func TestReplGenerateErrorBranches(t *testing.T) {
	svc := datjit.NewDefault()
	doc, err := svc.Parse(strings.NewReader(miniReplSchema), "mini.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var out, errw bytes.Buffer
	s := New(svc)
	s.out = &out
	s.errw = &errw
	s.state.Doc = doc

	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("not a dir"), 0o600); err != nil {
		t.Fatal(err)
	}
	s.state.Output = filepath.Join(blocker, "out.json")
	_ = cmdGenerate(s, nil)
	if !strings.Contains(errw.String(), "open output") {
		t.Fatalf("expected open output error, got %q", errw.String())
	}

	errw.Reset()
	s.state.Output = ""
	s.state.Format = "bogus"
	_ = cmdGenerate(s, nil)
	if !strings.Contains(errw.String(), "write") {
		t.Fatalf("expected write error, got %q", errw.String())
	}
}

func TestReplCompleterNestedAndPaths(t *testing.T) {
	svc := datjit.NewDefault()
	sess := New(svc)
	doc, err := svc.Parse(strings.NewReader(miniReplSchema), "mini.yaml")
	if err != nil {
		t.Fatal(err)
	}
	sess.state.Doc = doc
	c := newCompleter(sess)

	checkCompletion := func(line, want string) {
		t.Helper()
		got, _ := c.Do([]rune(line), len([]rune(line)))
		if !containsRune(got, want) {
			t.Fatalf("%q: want %q in %v", line, want, runeSlicesToStrings(got))
		}
	}
	checkCompletion("set f", "ormat")
	checkCompletion("set format j", "son")
	checkCompletion("set pretty o", "ff")
	checkCompletion("set sql-dialect s", "qlite")
	checkCompletion("set volume U", "ser=")
	checkCompletion("set entity U", "ser")
	checkCompletion("show e", "ntities")
	checkCompletion("corpus i", "nfo")
	checkCompletion("help lo", "ad")

	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.WriteFile(filepath.Join(dir, "schema.yaml"), []byte(miniReplSchema), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	checkCompletion("load sch", "ema.yaml")
}

func TestSessionAccessorsPromptAndHistoryPath(t *testing.T) {
	svc := datjit.NewDefault()
	sess := New(svc)
	if sess.Service() != svc {
		t.Fatal("service accessor mismatch")
	}
	if sess.State() != sess.state {
		t.Fatal("state accessor mismatch")
	}
	if got := sess.prompt(); got != "datjit> " {
		t.Fatalf("prompt=%q", got)
	}
	doc, err := svc.Parse(strings.NewReader(miniReplSchema), "mini.yaml")
	if err != nil {
		t.Fatal(err)
	}
	sess.state.Doc = doc
	if got := sess.prompt(); got != "datjit[repl_test]> " {
		t.Fatalf("domain prompt=%q", got)
	}
	sess.updatePrompt()
	if isTTY(strings.NewReader("")) {
		t.Fatal("strings.Reader should not be a TTY")
	}
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	hp := historyPath()
	if hp != filepath.Join(dir, "datjit", "history") {
		t.Fatalf("history path=%q", hp)
	}
}

const miniReplSchema = `domain: repl_test
entities:
  User:
    id: int
`
