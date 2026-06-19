package artifacts

import (
	"encoding/json"
	"strings"
	"testing"
)

func makeTelemetryJSONL() string {
	lines := []map[string]any{
		{"ts": "t1", "tool": "search_fiskaly_docs", "args": map[string]any{"query": "idempotency"}, "result_count": 3, "is_error": false, "latency_ms": 10},
		{"ts": "t2", "tool": "fetch_fiskaly_doc", "args": map[string]any{"id": "probe:records-flow"}, "result_count": 1, "is_error": false, "latency_ms": 20},
		{"ts": "t3", "tool": "fetch_fiskaly_doc", "args": map[string]any{"id": "missing"}, "result_count": 0, "is_error": true, "error": "no doc", "latency_ms": 30},
	}
	var parts []string
	for _, l := range lines {
		b, _ := json.Marshal(l)
		parts = append(parts, string(b))
	}
	return strings.Join(parts, "\n")
}

func TestParseTelemetryMapsSnakeCase(t *testing.T) {
	evs := ParseTelemetry(makeTelemetryJSONL())
	if len(evs) != 3 {
		t.Fatalf("got %d events, want 3", len(evs))
	}
	if evs[0].Tool != "search_fiskaly_docs" {
		t.Errorf("evs[0].Tool = %q", evs[0].Tool)
	}
	if evs[0].LatencyMs != 10 {
		t.Errorf("evs[0].LatencyMs = %d", evs[0].LatencyMs)
	}
	if !evs[2].IsError {
		t.Errorf("evs[2].IsError = false, want true")
	}
}

func TestParseTelemetrySkipsMalformed(t *testing.T) {
	evs := ParseTelemetry("\n{bad}\n")
	if len(evs) != 0 {
		t.Errorf("got %d events, want 0", len(evs))
	}
}

func TestSummarizeTelemetryAggregates(t *testing.T) {
	s := SummarizeTelemetry(ParseTelemetry(makeTelemetryJSONL()))
	if s.Total != 3 {
		t.Errorf("total = %d, want 3", s.Total)
	}
	if s.Errors != 1 {
		t.Errorf("errors = %d, want 1", s.Errors)
	}
	var fetchCalls int
	for _, bt := range s.ByTool {
		if bt.Tool == "fetch_fiskaly_doc" {
			fetchCalls = bt.Calls
		}
	}
	if fetchCalls != 2 {
		t.Errorf("fetch_fiskaly_doc calls = %d, want 2", fetchCalls)
	}
	if s.P50LatencyMs != 20 {
		t.Errorf("p50 = %d, want 20", s.P50LatencyMs)
	}
	if s.P95LatencyMs != 30 {
		t.Errorf("p95 = %d, want 30", s.P95LatencyMs)
	}
	if len(s.Queries) != 1 || s.Queries[0] != "idempotency" {
		t.Errorf("queries = %v", s.Queries)
	}
	if len(s.DocsFetched) != 2 || s.DocsFetched[0] != "probe:records-flow" || s.DocsFetched[1] != "missing" {
		t.Errorf("docsFetched = %v", s.DocsFetched)
	}
}
