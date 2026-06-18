package telemetry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestFileRecorderWritesJSONL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.jsonl")
	rec, err := NewFileRecorder(path)
	if err != nil {
		t.Fatal(err)
	}
	rec.Record(Event{TS: "2026-06-18T00:00:00Z", Tool: "search_fiskaly_docs", ResultCount: 3, LatencyMS: 7})
	rec.Record(Event{TS: "2026-06-18T00:00:01Z", Tool: "fetch_fiskaly_doc", IsError: true, Error: "no doc"})
	if err := rec.Close(); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d: %s", len(lines), b)
	}
	var e0 Event
	if err := json.Unmarshal([]byte(lines[0]), &e0); err != nil {
		t.Fatal(err)
	}
	if e0.Tool != "search_fiskaly_docs" || e0.ResultCount != 3 {
		t.Errorf("got %+v", e0)
	}
	if !strings.Contains(lines[1], `"is_error":true`) {
		t.Errorf("line1 missing is_error: %s", lines[1])
	}
}

func TestNopRecorderRecordsNothing(t *testing.T) {
	Nop().Record(Event{Tool: "x"}) // must not panic
}

func TestResultCount(t *testing.T) {
	search := &mcp.CallToolResult{StructuredContent: map[string]any{"results": []any{1, 2, 3}}}
	if got := resultCount(search); got != 3 {
		t.Errorf("search: got %d, want 3", got)
	}
	fetch := &mcp.CallToolResult{StructuredContent: map[string]any{"id": "x", "text": "y"}}
	if got := resultCount(fetch); got != 1 {
		t.Errorf("fetch: got %d, want 1", got)
	}
	errRes := &mcp.CallToolResult{IsError: true, StructuredContent: map[string]any{"results": []any{1}}}
	if got := resultCount(errRes); got != 0 {
		t.Errorf("error: got %d, want 0", got)
	}
	if got := resultCount(&mcp.CallToolResult{}); got != 0 {
		t.Errorf("empty: got %d, want 0", got)
	}
}

func TestContentText(t *testing.T) {
	r := &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "boom"}}}
	if got := contentText(r.Content); got != "boom" {
		t.Errorf("got %q, want boom", got)
	}
}
