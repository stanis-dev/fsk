package main

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeAgent simulates the coder: it grounds, then mutates the work tree so the
// post-agent observe has a real diff and transcript.
type fakeAgent struct{}

func (f fakeAgent) run(rd runDir, task string, cfg runConfig) error {
	tr := evSearch + "\n" + evWrite + "\n"
	if err := os.WriteFile(filepath.Join(rd.path, "transcript.jsonl"), []byte(tr), 0o644); err != nil {
		return err
	}
	// Append a line so changes.diff is non-empty; keep the module building.
	pos := filepath.Join(rd.work, "pos.go")
	// start from empty if the fixture has no pos.go
	b, _ := os.ReadFile(pos)
	return os.WriteFile(pos, append(b, []byte("\n// touched by fake agent\n")...), 0o644)
}

func TestRunScenario_PreflightHoldsAndArtifactsWritten(t *testing.T) {
	if testing.Short() {
		t.Skip("requires building the judge")
	}
	simsRoot, _ := filepath.Abs("..")
	judgeBin, err := buildJudge(filepath.Join(simsRoot, "judge"), t.TempDir())
	if err != nil {
		t.Fatalf("buildJudge: %v", err)
	}
	sc, err := discoverScenarios(filepath.Join(simsRoot, "scenarios"))
	if err != nil {
		t.Fatal(err)
	}
	one := sc[0] // 01-zero-to-receipt

	res, err := runScenario(one, t.TempDir(), judgeBin, fakeAgent{}, runConfig{model: "m", effort: "e"})
	if err != nil {
		t.Fatalf("runScenario: %v", err)
	}
	if res.preflightViolated {
		t.Fatal("pristine seed should hold the baseline preflight")
	}
	for _, name := range []string{"meta.json", "build.txt", "test.txt", "judge.txt", "judge.json", "changes.diff", "grounded.txt", "transcript.jsonl"} {
		if _, err := os.Stat(filepath.Join(res.runDir, name)); err != nil {
			t.Errorf("missing artifact %s: %v", name, err)
		}
	}
}
