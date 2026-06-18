package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverScenarios_RealCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-scenario discovery in -short mode")
	}
	simsRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	sc, err := discoverScenarios(filepath.Join(simsRoot, "scenarios"))
	if err != nil {
		t.Fatalf("discoverScenarios: %v", err)
	}
	if len(sc) != 10 {
		t.Fatalf("discovered %d scenarios, want 10", len(sc))
	}
}

func TestRunScenario_RealDocker(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real Docker run in -short mode")
	}
	if err := checkBinaries("docker"); err != nil {
		t.Skip("docker not available")
	}

	if err := dockerReachable(dockerContext()); err != nil {
		t.Skipf("docker daemon not reachable: %v", err)
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
