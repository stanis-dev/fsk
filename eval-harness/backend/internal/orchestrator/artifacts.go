package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"backend/internal/artifacts"
	"backend/internal/scenarios"
)

type runDir struct {
	path string
	work string
}

type observation struct {
	outcome
	diff string
}

// prepareRun creates an isolated run dir, copies the fixture, makes a baseline
// commit so the agent's changes can be diffed, and writes meta.json.
func prepareRun(runsBase string, s scenarios.Scenario, cfg runConfig) (runDir, error) {
	if err := os.MkdirAll(runsBase, 0o755); err != nil {
		return runDir{}, fmt.Errorf("creating runs base: %w", err)
	}
	path, err := os.MkdirTemp(runsBase, "run.")
	if err != nil {
		return runDir{}, fmt.Errorf("creating run dir: %w", err)
	}
	rd := runDir{path: path, work: filepath.Join(path, "pos")}
	if err := copyDir(s.FixtureDir, rd.work); err != nil {
		return rd, fmt.Errorf("copying fixture: %w", err)
	}
	if err := gitInitBaseline(rd.work); err != nil {
		return rd, err
	}
	if err := writeMeta(rd.path, s.ID, cfg); err != nil {
		return rd, err
	}
	return rd, nil
}

func gitInitBaseline(work string) error {
	steps := [][]string{
		{"init", "-q"},
		{"add", "-A"},
		{"-c", "user.email=eval@local", "-c", "user.name=eval", "commit", "-qm", "baseline"},
	}
	for _, s := range steps {
		if out, err := exec.Command("git", append([]string{"-C", work}, s...)...).CombinedOutput(); err != nil {
			return fmt.Errorf("git %v: %w\n%s", s, err, out)
		}
	}
	return nil
}

func writeMeta(runPath, scenario string, cfg runConfig) error {
	meta := map[string]string{
		"harness":  "docker",
		"coder":    "claude-code",
		"model":    cfg.model,
		"effort":   cfg.effort,
		"scenario": scenario,
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(runPath, artifacts.MetaFile), append(data, '\n'), 0o644)
}

func writeObserveArtifacts(runPath string, o observation) error {
	files := map[string]string{
		artifacts.BuildFile:    o.Build.Output,
		artifacts.TestFile:     o.Test.Output,
		artifacts.JudgeLogFile: o.Judge.Output,
		artifacts.DiffFile:     o.diff,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(runPath, name), []byte(content), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
	}
	return nil
}
