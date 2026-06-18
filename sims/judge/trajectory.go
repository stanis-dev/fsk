package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type transcriptEvent struct {
	Type    string `json:"type"`
	Message struct {
		Content []struct {
			Type string `json:"type"`
			Name string `json:"name"`
		} `json:"content"`
	} `json:"message"`
}

type telemetryEntry struct {
	Tool    string         `json:"tool"`
	Args    map[string]any `json:"args"`
	IsError bool           `json:"is_error"`
}

type Trajectory struct {
	ToolUses  []string // tool_use names from assistant events, in order
	Telemetry []telemetryEntry
}

func parseTrajectory(runDir string) (Trajectory, error) {
	var t Trajectory
	tu, err := parseToolUses(filepath.Join(runDir, "transcript.jsonl"))
	if err != nil {
		return t, err
	}
	t.ToolUses = tu
	tel, err := parseTelemetry(filepath.Join(runDir, "mcp-telemetry.jsonl"))
	if err != nil {
		return t, err
	}
	t.Telemetry = tel
	return t, nil
}

func parseToolUses(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening transcript: %w", err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		var ev transcriptEvent
		// Transcript is heterogeneous — many valid line shapes don't match
		// transcriptEvent; skip non-matching lines intentionally.
		if json.Unmarshal(sc.Bytes(), &ev) != nil || ev.Type != "assistant" {
			continue
		}
		for _, c := range ev.Message.Content {
			if c.Type == "tool_use" {
				out = append(out, c.Name)
			}
		}
	}
	return out, sc.Err()
}

// parseTelemetry tolerates a missing file — a run may legitimately have no MCP calls.
func parseTelemetry(path string) ([]telemetryEntry, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("opening telemetry: %w", err)
	}
	defer f.Close()
	var out []telemetryEntry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var e telemetryEntry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			return nil, fmt.Errorf("malformed telemetry line: %w", err)
		}
		out = append(out, e)
	}
	return out, sc.Err()
}
