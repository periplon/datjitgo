package repl

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	datjit "github.com/jmcarbo/datjitgo"
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
