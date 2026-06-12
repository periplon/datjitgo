package main_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestGenerateDirtyRateFlag checks that --dirty-rate changes output relative
// to a clean run with the same seed, and that the clean run stays
// deterministic (same seed → same bytes).
func TestGenerateDirtyRateFlag(t *testing.T) {
	path := fixturePath(t, "minimal.yaml")
	run := func(extra ...string) string {
		args := append([]string{"generate", path, "--seed", "42", "-f", "json", "--pretty"}, extra...)
		stdout, stderr, code := runCmd(t, args...)
		if code != 0 {
			t.Fatalf("exit=%d stderr=%q", code, stderr)
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("stdout is not valid JSON: %v", err)
		}
		return stdout
	}
	clean := run()
	if clean != run() {
		t.Fatal("clean generation not deterministic across runs")
	}
	if dirty := run("--dirty-rate", "1"); dirty == clean {
		t.Fatal("--dirty-rate 1 produced output identical to the clean run")
	}
}

// TestGenerateDirtyRateFlagRejectsOutOfRange checks the usage-error path.
func TestGenerateDirtyRateFlagRejectsOutOfRange(t *testing.T) {
	path := fixturePath(t, "minimal.yaml")
	_, stderr, code := runCmd(t, "generate", path, "--dirty-rate", "1.5")
	if code != 2 {
		t.Fatalf("exit=%d, want 2 (usage error); stderr=%q", code, stderr)
	}
	if !strings.Contains(stderr, "dirty-rate") {
		t.Fatalf("stderr should mention dirty-rate: %q", stderr)
	}
}
