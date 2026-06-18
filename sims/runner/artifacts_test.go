package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteObserveArtifacts_DashboardContract(t *testing.T) {
	runPath := t.TempDir()
	o := observation{
		Outcome: Outcome{
			Build: StepResult{OK: true, Output: ""},
			Test:  StepResult{OK: true, Output: "ok  \tpos\t0.1s\n"},
			Judge: StepResult{OK: false, Output: "VERDICT: NON-COMPLIANT (5 failures). exit 1\n"},
		},
		diff:       "diff --git a/x b/x\n",
		grounded:   "GROUNDED: searched before first code change",
		groundedOK: true,
	}
	if err := writeObserveArtifacts(runPath, o); err != nil {
		t.Fatalf("writeObserveArtifacts: %v", err)
	}

	// build.txt empty (trim) => dashboard reads PASS
	if b := readFileT(t, runPath, "build.txt"); strings.TrimSpace(b) != "" {
		t.Errorf("build.txt should be empty on PASS, got %q", b)
	}
	// test.txt contains ok and not FAIL => PASS
	tt := readFileT(t, runPath, "test.txt")
	if !strings.Contains(tt, "ok") || strings.Contains(tt, "FAIL") {
		t.Errorf("test.txt not a PASS shape: %q", tt)
	}
	// judge.txt present and NON-COMPLIANT
	if j := readFileT(t, runPath, "judge.txt"); !strings.Contains(j, "NON-COMPLIANT") {
		t.Errorf("judge.txt missing verdict: %q", j)
	}
	if d := readFileT(t, runPath, "changes.diff"); !strings.Contains(d, "diff --git") {
		t.Errorf("changes.diff missing: %q", d)
	}
	if g := readFileT(t, runPath, "grounded.txt"); !strings.Contains(g, "GROUNDED") {
		t.Errorf("grounded.txt missing: %q", g)
	}
}

func TestWriteMeta_Shape(t *testing.T) {
	runPath := t.TempDir()
	if err := writeMeta(runPath, "01-zero-to-receipt", runConfig{model: "m", effort: "e"}); err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(readFileT(t, runPath, "meta.json")), &m); err != nil {
		t.Fatal(err)
	}
	if len(m) != 5 {
		t.Errorf("meta.json has %d keys, want exactly 5", len(m))
	}
	for k, want := range map[string]string{
		"harness": "docker", "coder": "claude-code", "model": "m", "effort": "e", "scenario": "01-zero-to-receipt",
	} {
		if m[k] != want {
			t.Errorf("meta[%q] = %q, want %q", k, m[k], want)
		}
	}
}

func readFileT(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}
