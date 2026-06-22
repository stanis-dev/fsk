package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"backend/internal/artifacts"
	"backend/internal/scenarios"
)

// runConfig is the per-run model, effort, and OAuth token handed to the agent.
type runConfig struct {
	model  string
	effort string
	token  string
}

type scenarioResult struct {
	id     string
	runDir string
	obs    observation
}

func runScenario(ctx context.Context, s scenarios.Scenario, runsBase string, ag agent, cfg runConfig, onStart func(string)) (scenarioResult, error) {
	taskBytes, err := os.ReadFile(filepath.Join(s.Dir, "task.md"))
	if err != nil {
		return scenarioResult{}, fmt.Errorf("reading task: %w", err)
	}

	rd, err := prepareRun(runsBase, s, cfg)
	if err != nil {
		return scenarioResult{}, fmt.Errorf("prepareRun: %w", err)
	}
	if onStart != nil {
		onStart(rd.path)
	}

	if err := ag.run(ctx, rd, string(taskBytes), cfg); err != nil {
		return scenarioResult{}, fmt.Errorf("agent: %w", err)
	}

	core := outcome{
		Build: runGoCmd(rd.work, "build", "./..."),
		Test:  runGoCmd(rd.work, "test", "./..."),
		Judge: runJudge(s.ScenarioJSON, rd.work, rd.path, filepath.Join(rd.path, artifacts.JudgeJSONFile)),
	}
	diff, err := gitDiffStaged(rd.work)
	if err != nil {
		return scenarioResult{}, err
	}
	obs := observation{outcome: core, diff: diff}
	if err := writeObserveArtifacts(rd.path, obs); err != nil {
		return scenarioResult{}, err
	}
	return scenarioResult{id: s.ID, runDir: rd.path, obs: obs}, nil
}
