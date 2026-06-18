package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runGoCmd(dir string, args ...string) StepResult {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return StepResult{OK: err == nil, Output: string(out)}
}

func runJudge(judgeBin, scenarioJSON, dir string) StepResult {
	cmd := exec.Command(judgeBin, "-scenario", scenarioJSON, dir)
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

// transcriptEvent is the minimal shape of one Claude stream-json line: a
// tool_use block lives inside an assistant message's content array.
type transcriptEvent struct {
	Message struct {
		Content []struct {
			Type string `json:"type"`
			Name string `json:"name"`
		} `json:"content"`
	} `json:"message"`
}

// checkGrounded reports whether the agent searched the docs before writing
// integration code: the first search_fiskaly_docs tool_use must precede the
// first Write/Edit/MultiEdit tool_use. This is the Go port of assert-grounded.sh,
// parsing events instead of grepping line numbers.
func checkGrounded(transcriptPath string) (bool, string) {
	f, err := os.Open(transcriptPath)
	if err != nil {
		return false, "INCONCLUSIVE: transcript not found"
	}
	defer f.Close()

	searchAt, mutateAt := -1, -1
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024) // transcript lines can be large
	for i := 0; sc.Scan(); i++ {
		var ev transcriptEvent
		if json.Unmarshal(sc.Bytes(), &ev) != nil {
			continue
		}
		for _, c := range ev.Message.Content {
			if c.Type != "tool_use" {
				continue
			}
			if searchAt == -1 && strings.Contains(c.Name, "search_fiskaly_docs") {
				searchAt = i
			}
			if mutateAt == -1 && (c.Name == "Write" || c.Name == "Edit" || c.Name == "MultiEdit") {
				mutateAt = i
			}
		}
	}

	switch {
	case searchAt == -1:
		return false, "NOT GROUNDED: agent never called search_fiskaly_docs"
	case mutateAt == -1:
		return false, "INCONCLUSIVE: agent searched but never wrote integration code"
	case searchAt < mutateAt:
		return true, fmt.Sprintf("GROUNDED: searched (event %d) before first code change (event %d)", searchAt, mutateAt)
	default:
		return false, fmt.Sprintf("NOT GROUNDED: first code change (event %d) precedes first search (event %d)", mutateAt, searchAt)
	}
}

