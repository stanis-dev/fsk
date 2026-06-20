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
	out, err := cmd.CombinedOutput()
	return stepResult{OK: err == nil, Output: string(out)}
}

// runJudge evaluates sourceDir (the agent's work dir) with trajectory files read
// from runDir. With expect set (and a scenario that declares judge.expectations),
// the judge adds its LLM expectation layer behind the gate and, when jsonPath is
// given, writes the structured verdict there.
func runJudge(scenarioJSON, sourceDir, runDir string, expect bool, jsonPath string) stepResult {
	var buf bytes.Buffer
	report, code, err := judge.Evaluate(judge.Options{
		ScenarioPath:   scenarioJSON,
		RunDir:         runDir,
		IntegrationDir: sourceDir,
		Expect:         expect,
	}, &buf)
	if err != nil {
		fmt.Fprintln(&buf, "judge:", err)
	}
	if jsonPath != "" {
		if err := judge.WriteReport(jsonPath, report); err != nil {
			fmt.Fprintln(&buf, "judge: writing report:", err)
			return stepResult{OK: false, Output: buf.String()}
		}
	}
	return stepResult{OK: code == 0, Output: buf.String()}
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
