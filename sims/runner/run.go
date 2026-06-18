package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// observeCore runs build/test/judge against the work dir. expect and jsonPath drive
// the judge's expectation layer and JSON output; runDir holds the trajectory.
func observeCore(work, runDir, judgeBin, scenarioJSON string, expect bool, jsonPath string) Outcome {
	return Outcome{
		Build: runGoCmd(work, "build", "./..."),
		Test:  runGoCmd(work, "test", "./..."),
		Judge: runJudge(judgeBin, scenarioJSON, work, runDir, expect, jsonPath),
	}
}

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

	core := observeCore(rd.work, rd.path, judgeBin, s.scenarioJSON, true, filepath.Join(rd.path, "judge.json"))
	diff, err := gitDiffStaged(rd.work)
	if err != nil {
		return scenarioResult{}, err
	}
	obs := observation{Outcome: core, diff: diff}
	if err := writeObserveArtifacts(rd.path, obs); err != nil {
		return scenarioResult{}, err
	}
	return scenarioResult{id: s.id, runDir: rd.path, obs: obs}, nil
}
