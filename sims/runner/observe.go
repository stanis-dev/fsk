package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

func runGoCmd(dir string, args ...string) StepResult {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return StepResult{OK: err == nil, Output: string(out)}
}

// runJudge runs the judge against sourceDir (the agent's work dir) with
// trajectory files read from runDir. With expect set (and a scenario that
// declares judge.expectations), the judge adds its LLM expectation layer behind
// the gate and, when jsonPath is given, writes the structured verdict there.
func runJudge(judgeBin, scenarioJSON, sourceDir, runDir string, expect bool, jsonPath string) StepResult {
	args := []string{"-scenario", scenarioJSON, "-run", runDir}
	if expect {
		args = append(args, "-expect")
	}
	if jsonPath != "" {
		args = append(args, "-json", jsonPath)
	}
	args = append(args, sourceDir)
	cmd := exec.Command(judgeBin, args...)
	out, err := cmd.CombinedOutput()
	return StepResult{OK: err == nil, Output: string(out)}
}

func buildJudge(judgeDir, outDir string) (string, error) {
	bin := filepath.Join(outDir, "judge")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = judgeDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("building judge: %w\n%s", err, out)
	}
	return bin, nil
}

// gitDiffStaged stages all changes in work and returns the diff against the
// baseline commit.
func gitDiffStaged(work string) (string, error) {
	if out, err := exec.Command("git", "-C", work, "add", "-A").CombinedOutput(); err != nil {
		return "", fmt.Errorf("git add: %w\n%s", err, out)
	}
	out, err := exec.Command("git", "-C", work, "diff", "--cached").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff: %w\n%s", err, out)
	}
	return string(out), nil
}
