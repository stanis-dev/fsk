package judge

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFileT(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParseTrajectory(t *testing.T) {
	dir := t.TempDir()
	writeFileT(t, filepath.Join(dir, "transcript.jsonl"),
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"search_fiskaly_docs"}]}}
{"type":"user","message":{"content":[{"type":"tool_result"}]}}
{"type":"assistant","message":{"content":[{"type":"text"},{"type":"tool_use","name":"Edit"}]}}
`)
	writeFileT(t, filepath.Join(dir, "mcp-telemetry.jsonl"),
		`{"tool":"search_fiskaly_docs","args":{"query":"records receipt"},"is_error":false}
{"tool":"fetch_fiskaly_doc","args":{"id":"tokens"},"is_error":true}
`)
	tr, err := parseTrajectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := tr.ToolUses; len(got) != 2 || got[0] != "search_fiskaly_docs" || got[1] != "Edit" {
		t.Errorf("ToolUses = %v", got)
	}
	if len(tr.Telemetry) != 2 || !tr.Telemetry[1].IsError {
		t.Errorf("Telemetry = %+v", tr.Telemetry)
	}
}

func TestParseTrajectory_MissingTelemetryOK(t *testing.T) {
	dir := t.TempDir()
	writeFileT(t, filepath.Join(dir, "transcript.jsonl"),
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write"}]}}`+"\n")
	tr, err := parseTrajectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(tr.Telemetry) != 0 || len(tr.ToolUses) != 1 {
		t.Errorf("want 1 tool / 0 telemetry, got %+v", tr)
	}
}

func TestParseTrajectory_MissingTranscriptErrors(t *testing.T) {
	if _, err := parseTrajectory(t.TempDir()); err == nil {
		t.Fatal("expected error when transcript.jsonl is absent")
	}
}

func TestParseTrajectory_MalformedTelemetryErrors(t *testing.T) {
	dir := t.TempDir()
	writeFileT(t, filepath.Join(dir, "transcript.jsonl"),
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write"}]}}`+"\n")
	writeFileT(t, filepath.Join(dir, "mcp-telemetry.jsonl"),
		`{"tool":"search_fiskaly_docs","args":{},"is_error":false}`+"\n"+
			`{ not json`+"\n")
	if _, err := parseTrajectory(dir); err == nil {
		t.Fatal("expected error for malformed telemetry line")
	}
}
