package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// observeCore runs the shared build/test/judge checks against a work dir.
func observeCore(work, judgeBin, scenarioJSON string) Outcome {
	return Outcome{
		Build: runGoCmd(work, "build", "./..."),
		Test:  runGoCmd(work, "test", "./..."),
		Judge: runJudge(judgeBin, scenarioJSON, work),
	}
}

type scenarioResult struct {
	id                string
	runDir            string
	preflightViolated bool
	preflight         Outcome
	obs               observation
}

// runScenario is the single path: prepare an isolated run dir, assert the
// baseline preflight on the pristine copy, run the agent, observe the
// result, and write the dashboard artifacts. A preflight violation skips the
// agent: an unsound seed is a harness error, not an eval.
func runScenario(s scenario, runsBase, judgeBin string, ag agent, cfg runConfig) (scenarioResult, error) {
	taskBytes, err := os.ReadFile(filepath.Join(s.dir, "task.md"))
	if err != nil {
		return scenarioResult{}, fmt.Errorf("reading task: %w", err)
	}

	rd, err := prepareRun(runsBase, s, cfg)
	if err != nil {
		return scenarioResult{}, fmt.Errorf("prepareRun: %w", err)
	}

	pre := observeCore(rd.work, judgeBin, s.scenarioJSON)
	if !baselineHolds(s, pre) {
		return scenarioResult{id: s.id, runDir: rd.path, preflightViolated: true, preflight: pre}, nil
	}

	if err := ag.run(rd, string(taskBytes), cfg); err != nil {
		return scenarioResult{}, fmt.Errorf("agent: %w", err)
	}

	core := observeCore(rd.work, judgeBin, s.scenarioJSON)
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

// preflightAll runs the baseline preflight (no Docker) across scenarios and
// returns the ids that violate the invariant. Empty means every seed is sound.
func preflightAll(scenarios []scenario, judgeBin string) []string {
	var violated []string
	for _, s := range scenarios {
		work, err := os.MkdirTemp("", "runner-preflight-"+s.id+"-")
		if err != nil {
			violated = append(violated, s.id)
			continue
		}
		dst := filepath.Join(work, "pos")
		if copyDir(s.fixtureDir, dst) != nil || !baselineHolds(s, observeCore(dst, judgeBin, s.scenarioJSON)) {
			violated = append(violated, s.id)
		}
		os.RemoveAll(work)
	}
	return violated
}
