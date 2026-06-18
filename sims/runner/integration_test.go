package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestBaselines_RealScenarios runs the full baselines command against the real
// scenario library: it builds the judge, copies each fixture, and runs
// build/test/judge for real. This is the regression guard that the invariant
// actually holds for every shipped scenario. Skipped under -short.
func TestBaselines_RealScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-scenario integration run in -short mode")
	}
	simsRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	scenariosDir := filepath.Join(simsRoot, "scenarios")
	judgeBin, err := buildJudge(filepath.Join(simsRoot, "judge"), t.TempDir())
	if err != nil {
		t.Fatalf("buildJudge: %v", err)
	}

	sc, err := discoverScenarios(scenariosDir)
	if err != nil {
		t.Fatalf("discoverScenarios: %v", err)
	}
	if len(sc) != 10 {
		t.Fatalf("discovered %d scenarios, want 10", len(sc))
	}

	code := runBaselines(sc, execChecker{judgeBin: judgeBin}, io.Discard)
	if code != 0 {
		// Re-run to stdout so the failing scenario is visible in test output.
		runBaselines(sc, execChecker{judgeBin: judgeBin}, testWriter{t})
		t.Fatalf("baselines exit code = %d, want 0 (a scenario violates the invariant)", code)
	}
}

type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Logf("%s", p)
	return len(p), nil
}

func TestRunScenario_RealDocker(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real Docker run in -short mode")
	}
	if err := checkBinaries("docker"); err != nil {
		t.Skip("docker not available")
	}
	simsRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	repoRoot := filepath.Dir(simsRoot)
	cfg, err := loadConfig(repoRoot, "claude-sonnet-4-6", "low")
	if err != nil {
		t.Skipf("no usable config (.env token): %v", err)
	}
	judgeBin, err := buildJudge(filepath.Join(simsRoot, "judge"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sc, err := discoverScenarios(filepath.Join(simsRoot, "scenarios"))
	if err != nil {
		t.Fatal(err)
	}
	ag := dockerAgent{repoRoot: repoRoot, simsRoot: simsRoot, context: dockerContext(), image: "fiskaly-eval"}

	res, err := runScenario(sc[0], t.TempDir(), judgeBin, ag, cfg)
	if err != nil {
		t.Fatalf("runScenario: %v", err)
	}
	for _, name := range []string{"meta.json", "transcript.jsonl", "build.txt", "test.txt", "judge.txt", "changes.diff", "grounded.txt"} {
		if _, err := os.Stat(filepath.Join(res.runDir, name)); err != nil {
			t.Errorf("missing artifact %s: %v", name, err)
		}
	}
}
