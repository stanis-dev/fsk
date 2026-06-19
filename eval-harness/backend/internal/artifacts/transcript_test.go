package artifacts

import (
	"encoding/json"
	"strings"
	"testing"
)

func makeTranscriptJSONL() string {
	lines := []map[string]any{
		{"type": "system", "model": "claude-sonnet-4-6", "cwd": "/work"},
		{"type": "assistant", "message": map[string]any{"content": []map[string]any{
			{"type": "thinking", "thinking": "let me look"},
			{"type": "text", "text": "Reading the file"},
			{"type": "tool_use", "name": "Read", "input": map[string]any{"file_path": "pos/checkout.go"}},
		}}},
		{"type": "user", "message": map[string]any{"content": []map[string]any{
			{"type": "tool_result", "content": "file contents here", "is_error": false},
		}}},
		{"type": "result", "result": "done", "num_turns": 12, "total_cost_usd": 1.5},
	}
	var parts []string
	for _, l := range lines {
		b, _ := json.Marshal(l)
		parts = append(parts, string(b))
	}
	return strings.Join(parts, "\n")
}

func TestParseTranscriptOrderedEvents(t *testing.T) {
	evs := ParseTranscript(makeTranscriptJSONL())
	wantKinds := []string{"thinking", "assistant", "tool", "result", "final"}
	if len(evs) != len(wantKinds) {
		t.Fatalf("got %d events, want %d: %v", len(evs), len(wantKinds), evs)
	}
	for i, k := range wantKinds {
		if evs[i].Kind != k {
			t.Errorf("evs[%d].Kind = %q, want %q", i, evs[i].Kind, k)
		}
	}
	if evs[2].Text != "Read  pos/checkout.go" {
		t.Errorf("evs[2].Text = %q", evs[2].Text)
	}
}

func TestParseTranscriptToolResultErrorPrefix(t *testing.T) {
	line, _ := json.Marshal(map[string]any{
		"type": "user",
		"message": map[string]any{"content": []map[string]any{
			{"type": "tool_result", "content": "boom", "is_error": true},
		}},
	})
	evs := ParseTranscript(string(line))
	if len(evs) != 1 || evs[0].Text != "error: boom" {
		t.Errorf("got %v", evs)
	}
}

func TestSummarizeToolBashFormat(t *testing.T) {
	got := SummarizeTool("Bash", map[string]any{"command": "go test ./..."})
	want := "Bash  $ go test ./..."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
