package main_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// buildOnce compiles the datjit binary once per test run and returns the
// absolute path to the resulting executable. Subsequent callers reuse the
// same binary — this keeps the exec-based integration tests cheap.
var (
	buildMu   sync.Mutex
	buildPath string
	buildErr  error
)

// buildOnce caches a single `go build -o <tmp> ./cmd/datjit` invocation.
// We cannot use sync.Once because testing.T accepts a fatal hand-off, so
// the guard uses an explicit mutex.
func buildOnce(t *testing.T) string {
	t.Helper()
	buildMu.Lock()
	defer buildMu.Unlock()
	if buildPath != "" || buildErr != nil {
		if buildErr != nil {
			t.Fatalf("buildOnce: %v", buildErr)
		}
		return buildPath
	}
	dir, err := os.MkdirTemp("", "datjit-cli-")
	if err != nil {
		buildErr = err
		t.Fatalf("mkdirtemp: %v", err)
	}
	bin := filepath.Join(dir, "datjit")
	// Build from the module root. The test file lives at cmd/datjit so the
	// repo root is two levels up.
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/datjit")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		buildErr = err
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	buildPath = bin
	return bin
}

// repoRoot returns the module root by walking up until go.mod is found.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}

// runCmd execs the built binary with the given args and captures stdout,
// stderr, and the exit code.
func runCmd(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	bin := buildOnce(t)
	cmd := exec.Command(bin, args...)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	code = 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("cmd.Run: %v", err)
		}
	}
	return so.String(), se.String(), code
}

// fixturePath resolves a file under testdata/fixtures relative to the repo
// root so tests can reference shared YAML fixtures.
func fixturePath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "testdata", "fixtures", name)
}

// localTestdata resolves a file under cmd/datjit/testdata (CLI-local
// fixtures used only by these integration tests).
func localTestdata(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "cmd", "datjit", "testdata", name)
}

func TestVersion(t *testing.T) {
	stdout, stderr, code := runCmd(t, "version")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	got := strings.TrimRight(stdout, "\n")
	if !regexp.MustCompile(`^datjit v0\.1\.0$`).MatchString(got) {
		t.Fatalf("unexpected version output: %q", got)
	}
}

func TestGenerateMinimal(t *testing.T) {
	path := fixturePath(t, "minimal.yaml")
	stdout, stderr, code := runCmd(t, "generate", path, "--seed", "42", "-f", "json", "--pretty")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	if stdout == "" {
		t.Fatal("expected non-empty stdout")
	}
	if !strings.Contains(stdout, "\"User\"") {
		t.Fatalf("expected stdout to contain \"User\", got %q", stdout[:min(200, len(stdout))])
	}
	var any map[string]any
	if err := json.Unmarshal([]byte(stdout), &any); err != nil {
		t.Fatalf("stdout is not valid JSON: %v", err)
	}
}

func TestGenerateDryRun(t *testing.T) {
	path := fixturePath(t, "minimal.yaml")
	stdout, stderr, code := runCmd(t, "generate", path, "--dry-run")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "plan:") {
		t.Fatalf("expected 'plan:' in stdout, got %q", stdout)
	}
	// Make sure we did NOT run the generator — no JSON array should sneak
	// in. A leading '{' or '[' is the easiest tell.
	trimmed := strings.TrimSpace(stdout)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		t.Fatalf("dry-run produced JSON-like output: %q", trimmed)
	}
}

func TestGenerateDryRunReportsResolvedRangeVolume(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "range-volume.yaml")
	if err := os.WriteFile(path, []byte(`
domain: cli_test
volume:
  User: 10..20
entities:
  User:
    id: int
`), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	stdout, stderr, code := runCmd(t, "generate", path, "--dry-run")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "User=15") {
		t.Fatalf("expected midpoint volume in dry-run plan, got %q", stdout)
	}
}

func TestValidateOK(t *testing.T) {
	path := fixturePath(t, "minimal.yaml")
	stdout, stderr, code := runCmd(t, "validate", path)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	if strings.TrimSpace(stdout) != "OK" {
		t.Fatalf("expected 'OK', got %q", stdout)
	}
}

func TestValidateFails(t *testing.T) {
	path := localTestdata(t, "invalid.yaml")
	stdout, stderr, code := runCmd(t, "validate", path)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "error:") {
		t.Fatalf("expected 'error:' in stderr, got %q", stderr)
	}
}

func TestInspect(t *testing.T) {
	path := fixturePath(t, "project_management.yaml")
	stdout, stderr, code := runCmd(t, "inspect", path)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	for _, want := range []string{"domain:", "entities"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected %q in stdout, got %q", want, stdout)
		}
	}
}

func TestCorpusList(t *testing.T) {
	stdout, stderr, code := runCmd(t, "corpus", "list")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "person.first_names") {
		t.Fatalf("expected 'person.first_names' in output, got %q", stdout)
	}
}

func TestGenerateFormatSQL(t *testing.T) {
	path := fixturePath(t, "minimal.yaml")
	stdout, stderr, code := runCmd(t, "generate", path, "--seed", "42", "-f", "sql", "--sql-dialect", "postgres")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	if !strings.HasPrefix(strings.TrimLeft(stdout, "\n"), "CREATE TABLE") {
		t.Fatalf("expected SQL output to start with CREATE TABLE, got %q", stdout[:min(120, len(stdout))])
	}
}

func TestUnknownFormat(t *testing.T) {
	path := fixturePath(t, "minimal.yaml")
	_, stderr, code := runCmd(t, "generate", path, "-f", "xml")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d (stderr=%q)", code, stderr)
	}
}

func TestGenerateUnknownEntityIsUsageError(t *testing.T) {
	path := fixturePath(t, "minimal.yaml")
	stdout, stderr, code := runCmd(t, "generate", path, "--entity", "Ghost")
	if code != 2 {
		t.Fatalf("expected exit 2 for unknown entity, got %d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stderr, `unknown entity "Ghost"`) {
		t.Fatalf("expected unknown entity message, got stderr=%q", stderr)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout for unknown entity, got %q", stdout)
	}
}

func TestCorpusUpdateRequiresSource(t *testing.T) {
	stdout, stderr, code := runCmd(t, "corpus", "update")
	if code != 1 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stderr, "no corpus update sources") {
		t.Fatalf("expected missing source error, got stderr=%q stdout=%q", stderr, stdout)
	}
}

func TestGenerateBadVolume(t *testing.T) {
	path := fixturePath(t, "minimal.yaml")
	_, stderr, code := runCmd(t, "generate", path, "--volume", "bad")
	if code != 2 {
		t.Fatalf("expected exit 2 for bad volume, got %d (stderr=%q)", code, stderr)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
