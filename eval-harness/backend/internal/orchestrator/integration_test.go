package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"backend/internal/config"
	"backend/internal/scenarios"
)

func TestDiscoverScenarios_RealCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-scenario discovery in -short mode")
	}
	ehRoot, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	sc, err := scenarios.Discover(filepath.Join(ehRoot, "backend", "scenarios"))
	if err != nil {
		t.Fatalf("scenarios.Discover: %v", err)
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

	c, err := config.Load()
	if err != nil {
		t.Skipf("no usable config (.env token): %v", err)
	}
	if err := dockerReachable(c.DockerContext); err != nil {
		t.Skipf("docker daemon not reachable: %v", err)
	}
	sc, err := scenarios.Discover(c.ScenariosDir)
	if err != nil {
		t.Fatal(err)
	}
	ag := dockerAgent{repoRoot: c.RepoRoot, dockerfilePath: c.DockerfilePath, context: c.DockerContext, image: c.Image}
	rc := runConfig{model: c.Model, effort: "low", token: c.Token}

	res, err := runScenario(context.Background(), sc[0], t.TempDir(), ag, rc, nil)
	if err != nil {
		t.Fatalf("runScenario: %v", err)
	}
	for _, name := range []string{"meta.json", "transcript.jsonl", "build.txt", "test.txt", "judge.txt", "judge.json", "changes.diff"} {
		if _, err := os.Stat(filepath.Join(res.runDir, name)); err != nil {
			t.Errorf("missing artifact %s: %v", name, err)
		}
	}
}
