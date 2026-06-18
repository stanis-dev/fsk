package telemetry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
