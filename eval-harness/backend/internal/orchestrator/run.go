package main

import (
	"fmt"
	"os"
	"path/filepath"
)

type scenarioResult struct {
	id     string
	runDir string
	obs    observation
}

func runScenario(s scenario, runsBase, judgeBin string, ag agent, cfg runConfig) (scenarioResult, error) {
	taskBytes, err := os.ReadFile(filepath.Join(s.dir, "task.md"))
	if err != nil {
		return scenarioResult{}, fmt.Errorf("reading task: %w", err)
	}

	rd, err := prepareRun(runsBase, s, cfg)
	if err != nil {
		return scenarioResult{}, fmt.Errorf("prepareRun: %w", err)
	}
	if err := writeRunHandle(rd.path); err != nil {
		return scenarioResult{}, fmt.Errorf("writeRunHandle: %w", err)
	}

	if err := ag.run(rd, string(taskBytes), cfg); err != nil {
		return scenarioResult{}, fmt.Errorf("agent: %w", err)
	}

	core := outcome{
		Build: runGoCmd(rd.work, "build", "./..."),
		Test:  runGoCmd(rd.work, "test", "./..."),
		Judge: runJudge(judgeBin, s.scenarioJSON, rd.work, rd.path, true, filepath.Join(rd.path, "judge.json")),
	}
	diff, err := gitDiffStaged(rd.work)
	if err != nil {
		return scenarioResult{}, err
	}
	obs := observation{outcome: core, diff: diff}
	if err := writeObserveArtifacts(rd.path, obs); err != nil {
		return scenarioResult{}, err
	}
	return scenarioResult{id: s.id, runDir: rd.path, obs: obs}, nil
}
