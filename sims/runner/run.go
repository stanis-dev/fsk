package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// observeCore runs the shared build/test/judge checks against a work dir. rubric
// and jsonPath enable the judge's LLM rubric layer and structured output.
func observeCore(work, judgeBin, scenarioJSON string, rubric bool, jsonPath string) Outcome {
	return Outcome{
		Build: runGoCmd(work, "build", "./..."),
		Test:  runGoCmd(work, "test", "./..."),
		Judge: runJudge(judgeBin, scenarioJSON, work, rubric, jsonPath),
	}
}

type scenarioResult struct {
	id     string
	runDir string
	obs    observation
}

// runScenario is the single path: prepare an isolated run dir, run the agent,
// observe the result, and write the dashboard artifacts.
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

	core := observeCore(rd.work, judgeBin, s.scenarioJSON, true, filepath.Join(rd.path, "judge.json"))
	diff, err := gitDiffStaged(rd.work)
	if err != nil {
		return scenarioResult{}, err
	}
	ok, verdict := checkGrounded(filepath.Join(rd.path, "transcript.jsonl"))
	obs := observation{Outcome: core, diff: diff, grounded: verdict, groundedOK: ok}
	if err := writeObserveArtifacts(rd.path, obs); err != nil {
		return scenarioResult{}, err
	}
	return scenarioResult{id: s.id, runDir: rd.path, obs: obs}, nil
}
