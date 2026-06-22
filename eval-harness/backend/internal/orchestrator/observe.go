package orchestrator

import (
	"bytes"
	"fmt"
	"os/exec"

	"backend/internal/judge"
)

func runGoCmd(dir string, args ...string) stepResult {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	out, _ := cmd.CombinedOutput()
	return stepResult{Output: string(out)}
}

// runJudge evaluates sourceDir (the agent's work dir) using trajectory files from
// runDir: it runs the deterministic gate, adds the LLM expectation layer for
// scenarios that declare one, and writes the structured verdict to jsonPath.
func runJudge(scenarioJSON, sourceDir, runDir, jsonPath string) stepResult {
	var buf bytes.Buffer
	report, err := judge.Evaluate(judge.Options{
		ScenarioPath:   scenarioJSON,
		RunDir:         runDir,
		IntegrationDir: sourceDir,
		Expect:         true,
	}, &buf)
	if err != nil {
		fmt.Fprintln(&buf, "judge:", err)
	}
	if err := judge.WriteReport(jsonPath, report); err != nil {
		fmt.Fprintln(&buf, "judge: writing report:", err)
	}
	return stepResult{Output: buf.String()}
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
