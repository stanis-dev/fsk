package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"fiskaly-mcp/corpus"
	"fiskaly-mcp/telemetry"
)

func connectWithTelemetry(t *testing.T, recPath string) (*mcp.ClientSession, context.Context) {
	t.Helper()
	ctx := context.Background()
	c, err := corpus.Load()
	if err != nil {
		t.Fatalf("corpus.Load: %v", err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "fiskaly", Version: "test"}, nil)
	registerTools(server, c)
	rec, err := telemetry.NewFileRecorder(recPath)
	if err != nil {
		t.Fatalf("NewFileRecorder: %v", err)
	}
	t.Cleanup(func() {
		if err := rec.Close(); err != nil {
			t.Errorf("rec.Close: %v", err)
		}
	})
	server.AddReceivingMiddleware(telemetry.Middleware(rec))

	st, ct := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return session, ctx
}

type teleEvent struct {
	TS          string         `json:"ts"`
	Tool        string         `json:"tool"`
	Args        map[string]any `json:"args"`
	ResultCount int            `json:"result_count"`
	IsError     bool           `json:"is_error"`
	Error       string         `json:"error"`
	LatencyMS   int64          `json:"latency_ms"`
}

func TestTelemetryRecordsToolCalls(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp-telemetry.jsonl")
	session, ctx := connectWithTelemetry(t, path)

	if _, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "search_fiskaly_docs", Arguments: map[string]any{"query": "idempotency key"},
	}); err != nil {
		t.Fatalf("search: %v", err)
	}
	if _, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "fetch_fiskaly_doc", Arguments: map[string]any{"id": "probe:records-flow"},
	}); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if _, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "fetch_fiskaly_doc", Arguments: map[string]any{"id": "does-not-exist"},
	}); err != nil {
		t.Fatalf("fetch-unknown transport: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read telemetry: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 events, got %d: %s", len(lines), b)
	}
	evs := make([]teleEvent, 0, 3)
	for _, l := range lines {
		var e teleEvent
		if err := json.Unmarshal([]byte(l), &e); err != nil {
			t.Fatalf("bad line %q: %v", l, err)
		}
		evs = append(evs, e)
	}

	if evs[0].Tool != "search_fiskaly_docs" || evs[0].Args["query"] != "idempotency key" {
		t.Errorf("ev0 = %+v", evs[0])
	}
	if evs[0].ResultCount < 1 || evs[0].IsError {
		t.Errorf("ev0 count/err = %d/%v", evs[0].ResultCount, evs[0].IsError)
	}
	if evs[1].Tool != "fetch_fiskaly_doc" || evs[1].Args["id"] != "probe:records-flow" {
		t.Errorf("ev1 = %+v", evs[1])
	}
	if evs[1].ResultCount != 1 || evs[1].IsError {
		t.Errorf("ev1 count/err = %d/%v", evs[1].ResultCount, evs[1].IsError)
	}
	if evs[2].Tool != "fetch_fiskaly_doc" || !evs[2].IsError {
		t.Errorf("ev2 = %+v", evs[2])
	}
	if evs[2].ResultCount != 0 || evs[2].Error == "" {
		t.Errorf("ev2 count/err = %d/%q", evs[2].ResultCount, evs[2].Error)
	}
	for i, e := range evs {
		if e.TS == "" || e.LatencyMS < 0 {
			t.Errorf("ev%d ts/latency = %q/%d", i, e.TS, e.LatencyMS)
		}
	}
}
