# MCP telemetry Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Instrument the fiskaly MCP server to record one structured event per `tools/call` to a per-run JSONL file, captured by the eval harness and summarized in the dashboard.

**Architecture:** A single SDK receiving-middleware on the server times each `tools/call` and writes an `Event` (tool, args, result count, error, latency) through a `Recorder` to an append-only JSONL file whose path comes from an env var. The docs tool handlers are untouched. The harness points each run's server at `<run-dir>/mcp-telemetry.jsonl`; the dashboard parses and summarizes that file in run-detail.

**Tech Stack:** Go 1.23 + `github.com/modelcontextprotocol/go-sdk` v1.2.0 (server); bash (harness); Next.js 16.2.9 / React 19 + vitest (dashboard).

Spec: [`docs/superpowers/specs/2026-06-18-mcp-telemetry-design.md`](../specs/2026-06-18-mcp-telemetry-design.md). Roadmap step 0: [`2026-06-17-eval-checks-roadmap.md`](2026-06-17-eval-checks-roadmap.md).

## Global Constraints

- Go: standard library + go-sdk v1.2.0 only. **No new dependencies.**
- Telemetry is **disabled unless** env `FISKALY_MCP_TELEMETRY` is set to a file path.
- **Never write telemetry to stdout** — that is the MCP stdio protocol channel. File sink only; recorder errors go to stderr via `log`.
- Event JSON field names are snake_case: `ts`, `session_id`, `tool`, `args`, `result_count`, `is_error`, `error`, `latency_ms`.
- The handlers `handleSearch` / `handleFetch` in `mcp/tools.go` **must remain unchanged**.
- Dashboard is Next.js 16 with breaking changes — per `sims/dashboard/AGENTS.md`, read the relevant guide in `node_modules/next/dist/docs/` before writing page / server-component code.
- Commits follow repo style `Area: summary`. **No AI attribution, no Co-Authored-By trailer.**

## Scope note

In scope: server instrumentation, per-run capture (`run-scenario.sh` + `run-eval-docker.sh`), dashboard parse/summary/view. **Deferred** (not this plan): the "search-before-first-edit" join (needs transcript timestamps `parseTranscript` does not extract; `assert-grounded.sh` already covers the order signal) and the rule→corpus-id retrieve→resolve link.

## File structure

- `mcp/telemetry/telemetry.go` (new) — `Event`, `Recorder`, `FileRecorder`, `nopRecorder`, `Middleware`, `resultCount`, `contentText`. One responsibility: capture and persist tool-call telemetry.
- `mcp/telemetry/telemetry_test.go` (new) — unit tests for recorders, `resultCount`, `contentText`.
- `mcp/main.go` (modify) — wire env → recorder → middleware.
- `mcp/server_telemetry_test.go` (new, package main) — in-memory integration test exercising the middleware end-to-end.
- `sims/evals/run-scenario.sh` (modify) — env block in the generated `mcp.json`.
- `sims/evals/run-eval-docker.sh` (modify) — same injection for parity.
- `sims/dashboard/lib/types.ts` (modify) — `TelemetryEvent`, `TelemetryToolStat`, `TelemetrySummary`.
- `sims/dashboard/lib/telemetry.ts` (new) — `parseTelemetry`, `summarizeTelemetry`.
- `sims/dashboard/lib/telemetry.test.ts` (new) — vitest.
- `sims/dashboard/lib/runs.ts` (modify) — load + summarize telemetry into `RunDetail`.
- `sims/dashboard/components/TelemetryView.tsx` (new) — render the summary.
- `sims/dashboard/app/run/[id]/page.tsx` (modify) — add the telemetry section.
- `sims/dashboard/__fixtures__/run.sample/mcp-telemetry.jsonl` (new) — sample.

---

### Task 1: Telemetry recorders

**Files:**
- Create: `mcp/telemetry/telemetry.go`
- Test: `mcp/telemetry/telemetry_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `type Event struct{...}` (snake_case JSON tags); `type Recorder interface { Record(Event) }`; `func NewFileRecorder(path string) (*FileRecorder, error)` with method `Record(Event)` and `Close() error`; `func Nop() Recorder`.

- [ ] **Step 1: Write the failing tests**

Create `mcp/telemetry/telemetry_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd mcp && go test ./telemetry/`
Expected: FAIL — `undefined: NewFileRecorder` / `Event` / `Nop` (package does not compile yet).

- [ ] **Step 3: Write minimal implementation**

Create `mcp/telemetry/telemetry.go`:

```go
// Package telemetry records one structured event per MCP tools/call to a file
// sink, so the eval harness can see how an agent used the docs tools. It never
// writes to stdout, which is the MCP stdio protocol channel.
package telemetry

import (
	"encoding/json"
	"log"
	"os"
	"sync"
)

// Event is one tools/call observation. Field names are the on-disk JSONL schema.
type Event struct {
	TS          string          `json:"ts"`
	SessionID   string          `json:"session_id,omitempty"`
	Tool        string          `json:"tool"`
	Args        json.RawMessage `json:"args,omitempty"`
	ResultCount int             `json:"result_count"`
	IsError     bool            `json:"is_error"`
	Error       string          `json:"error,omitempty"`
	LatencyMS   int64           `json:"latency_ms"`
}

// Recorder persists telemetry events. Implementations must be safe for
// concurrent use.
type Recorder interface {
	Record(Event)
}

// FileRecorder appends one JSON object per line to a file. Writes are
// best-effort: a failure is logged to stderr and never propagates.
type FileRecorder struct {
	mu  sync.Mutex
	f   *os.File
	enc *json.Encoder
}

func NewFileRecorder(path string) (*FileRecorder, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &FileRecorder{f: f, enc: json.NewEncoder(f)}, nil
}

func (r *FileRecorder) Record(e Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.enc.Encode(e); err != nil {
		log.Printf("telemetry: write failed: %v", err)
	}
}

func (r *FileRecorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.f.Close()
}

type nopRecorder struct{}

func (nopRecorder) Record(Event) {}

// Nop returns a Recorder that discards everything.
func Nop() Recorder { return nopRecorder{} }
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd mcp && go test ./telemetry/`
Expected: PASS (`ok  fiskaly-mcp/telemetry`).

- [ ] **Step 5: Commit**

```bash
git add mcp/telemetry/telemetry.go mcp/telemetry/telemetry_test.go
git commit -m "MCP: telemetry recorders (Event, FileRecorder, nop)"
```

---

### Task 2: Middleware + result helpers

**Files:**
- Modify: `mcp/telemetry/telemetry.go`
- Test: `mcp/telemetry/telemetry_test.go`

**Interfaces:**
- Consumes: `Event`, `Recorder` (Task 1); `github.com/modelcontextprotocol/go-sdk/mcp`.
- Produces: `func Middleware(rec Recorder) mcp.Middleware`; helpers `func resultCount(ctr *mcp.CallToolResult) int` and `func contentText(cs []mcp.Content) string`.

- [ ] **Step 1: Write the failing tests**

Append to `mcp/telemetry/telemetry_test.go` (and add `"github.com/modelcontextprotocol/go-sdk/mcp"` to its imports):

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd mcp && go test ./telemetry/`
Expected: FAIL — `undefined: resultCount` / `contentText`.

- [ ] **Step 3: Write minimal implementation**

Append to `mcp/telemetry/telemetry.go` and add `"strings"`, `"time"`, and `"github.com/modelcontextprotocol/go-sdk/mcp"` to its imports:

```go
// Middleware records one Event per tools/call. Other methods pass through
// untouched. The handlers themselves are never modified.
func Middleware(rec Recorder) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method != "tools/call" {
				return next(ctx, method, req)
			}
			start := time.Now()
			res, err := next(ctx, method, req)
			ev := Event{
				TS:        time.Now().UTC().Format(time.RFC3339),
				LatencyMS: time.Since(start).Milliseconds(),
			}
			if sess := req.GetSession(); sess != nil {
				ev.SessionID = sess.ID()
			}
			if p, ok := req.GetParams().(*mcp.CallToolParamsRaw); ok {
				ev.Tool = p.Name
				if len(p.Arguments) > 0 {
					ev.Args = append(json.RawMessage(nil), p.Arguments...)
				}
			}
			switch {
			case err != nil:
				ev.IsError = true
				ev.Error = err.Error()
			case res != nil:
				if ctr, ok := res.(*mcp.CallToolResult); ok {
					ev.IsError = ctr.IsError
					if ctr.IsError {
						ev.Error = contentText(ctr.Content)
					}
					ev.ResultCount = resultCount(ctr)
				}
			}
			rec.Record(ev)
			return res, err
		}
	}
}

// resultCount derives a count from a tool result without importing the server's
// typed output: a list-returning tool exposes a top-level "results" array; a
// single-document tool returns one object.
func resultCount(ctr *mcp.CallToolResult) int {
	if ctr.IsError || ctr.StructuredContent == nil {
		return 0
	}
	b, err := json.Marshal(ctr.StructuredContent)
	if err != nil {
		return 0
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(b, &obj) != nil {
		return 0
	}
	if raw, ok := obj["results"]; ok {
		var arr []json.RawMessage
		if json.Unmarshal(raw, &arr) == nil {
			return len(arr)
		}
	}
	if len(obj) > 0 {
		return 1
	}
	return 0
}

func contentText(cs []mcp.Content) string {
	var b strings.Builder
	for _, c := range cs {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}
```

Also add `"context"` to the import block (used by `Middleware`).

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd mcp && go test ./telemetry/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add mcp/telemetry/telemetry.go mcp/telemetry/telemetry_test.go
git commit -m "MCP: telemetry middleware over tools/call"
```

---

### Task 3: In-memory integration test + main.go wiring

**Files:**
- Create: `mcp/server_telemetry_test.go` (package main)
- Modify: `mcp/main.go`

**Interfaces:**
- Consumes: `registerTools` (mcp/tools.go), `corpus.Load` (mcp/corpus), `telemetry.NewFileRecorder` / `telemetry.Middleware` (Task 1-2).
- Produces: nothing for later tasks (terminal for the Go side).

- [ ] **Step 1: Write the failing integration test**

Create `mcp/server_telemetry_test.go`:

```go
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
	t.Cleanup(func() { rec.Close() })
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd mcp && go test -run TestTelemetryRecordsToolCalls .`
Expected: FAIL — `package main imports fiskaly-mcp/telemetry` resolves, but the test fails to compile only if Task 1-2 are missing; if Task 1-2 are done it should already PASS. If it PASSES here, that is correct — the middleware is exercised. Proceed to wire `main.go` so production uses it.

(Note: this test constructs its own server, so it does not depend on `main.go`. It is the deterministic proof the middleware emits correct events.)

- [ ] **Step 3: Wire main.go**

Modify `mcp/main.go` to read the env var and install the middleware. Replace the body of `main` so it reads:

```go
func main() {
	c, err := corpus.Load()
	if err != nil {
		log.Fatal(err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "fiskaly", Version: "v0.1.0"}, nil)
	registerTools(server, c)

	if path := os.Getenv("FISKALY_MCP_TELEMETRY"); path != "" {
		rec, err := telemetry.NewFileRecorder(path)
		if err != nil {
			log.Fatalf("telemetry: %v", err)
		}
		defer rec.Close()
		server.AddReceivingMiddleware(telemetry.Middleware(rec))
	}

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
```

Add `"os"` and `"fiskaly-mcp/telemetry"` to the import block.

- [ ] **Step 4: Run the full Go suite + build**

Run: `cd mcp && go build ./... && go test ./...`
Expected: PASS for `fiskaly-mcp`, `fiskaly-mcp/corpus`, `fiskaly-mcp/telemetry`; binary builds.

- [ ] **Step 5: Commit**

```bash
git add mcp/main.go mcp/server_telemetry_test.go
git commit -m "MCP: emit telemetry when FISKALY_MCP_TELEMETRY is set"
```

---

### Task 4: Harness wiring

**Files:**
- Modify: `sims/evals/run-scenario.sh:68`
- Modify: `sims/evals/run-eval-docker.sh` (the equivalent `mcp.json` generation line)

**Interfaces:**
- Consumes: the built `fiskaly-mcp` binary (already produced by the scripts).
- Produces: `<run-dir>/mcp-telemetry.jsonl` per run.

- [ ] **Step 1: Edit run-scenario.sh**

Replace the `mcp.json` generation at `sims/evals/run-scenario.sh:66-68`:

```bash
# Build the fiskaly MCP and hand it to the coder.
mcp_bin="$run_dir/fiskaly-mcp"
(cd "$repo_root/mcp" && go build -o "$mcp_bin" .)
tele="$run_dir/mcp-telemetry.jsonl"
printf '{ "mcpServers": { "fiskaly": { "command": "%s", "env": { "FISKALY_MCP_TELEMETRY": "%s" } } } }\n' \
  "$mcp_bin" "$tele" >"$run_dir/mcp.json"
```

Then add the telemetry path to the result output near `sims/evals/run-scenario.sh:124`, after the `transcript:` line:

```bash
echo "telemetry:  $run_dir/mcp-telemetry.jsonl"
```

- [ ] **Step 2: Verify the generated config is valid JSON with the env block**

Run (reproduces the printf with placeholder values — no agent, no network):

```bash
printf '{ "mcpServers": { "fiskaly": { "command": "%s", "env": { "FISKALY_MCP_TELEMETRY": "%s" } } } }\n' \
  /tmp/fiskaly-mcp /tmp/run/mcp-telemetry.jsonl | jq '.mcpServers.fiskaly.env'
```

Expected:
```json
{
  "FISKALY_MCP_TELEMETRY": "/tmp/run/mcp-telemetry.jsonl"
}
```

Also run `bash -n sims/evals/run-scenario.sh` — Expected: no output (syntax OK).

- [ ] **Step 3: Mirror the change in run-eval-docker.sh**

The docker variant has no `mcp.json` generation — its MCP config is static in the Dockerfile. Instead, pass the env var into the container on the `docker run` line, pointing at the mounted work dir (the container can only write to the `/work` mount):

```bash
  -e FISKALY_MCP_TELEMETRY="/work/mcp-telemetry.jsonl" \
```

Then, after the container exits and BEFORE the `git add -A` that builds the diff, move the file out of `$work` into `$run_dir` so it sits beside the other artifacts (where the dashboard looks) and does not get swept into `changes.diff`:

```bash
# Telemetry was written inside the mounted work dir; move it beside the other run
# artifacts and out of the tree the diff is computed from.
if [ -f "$work/mcp-telemetry.jsonl" ]; then
  mv "$work/mcp-telemetry.jsonl" "$run_dir/mcp-telemetry.jsonl"
fi
```

Add a `telemetry:` line to the result output too. Run `bash -n sims/evals/run-eval-docker.sh` — Expected: no output.

- [ ] **Step 4: Full-run smoke (manual — needs `CLAUDE_CODE_OAUTH_TOKEN` in `.env`, `claude` CLI, network)**

Run: `sims/evals/run-scenario.sh 01-zero-to-receipt`
Then: `RUN_DIR=$(ls -dt ~/.cache/fiskaly-eval/run.* | head -1); wc -l "$RUN_DIR/mcp-telemetry.jsonl"; head -1 "$RUN_DIR/mcp-telemetry.jsonl" | jq .`
Expected: a non-empty file; the first line parses as JSON with `tool`, `args`, `latency_ms`. If the env block is not honored by the installed CLI, fall back to also exporting it in the agent subshell (`sims/evals/run-scenario.sh:87-88`): add `FISKALY_MCP_TELEMETRY="$tele"` to the `HOME=... CLAUDE_CODE_OAUTH_TOKEN=...` env list so the child MCP process inherits it.

- [ ] **Step 5: Commit**

```bash
git add sims/evals/run-scenario.sh sims/evals/run-eval-docker.sh
git commit -m "Evals: capture per-run MCP telemetry alongside run artifacts"
```

---

### Task 5: Dashboard telemetry lib

**Files:**
- Modify: `sims/dashboard/lib/types.ts`
- Create: `sims/dashboard/lib/telemetry.ts`
- Test: `sims/dashboard/lib/telemetry.test.ts`

**Interfaces:**
- Consumes: nothing.
- Produces: `TelemetryEvent`, `TelemetryToolStat`, `TelemetrySummary` types; `parseTelemetry(jsonl: string): TelemetryEvent[]`; `summarizeTelemetry(events: TelemetryEvent[]): TelemetrySummary`.

- [ ] **Step 1: Add types**

Append to `sims/dashboard/lib/types.ts`:

```ts
export interface TelemetryEvent {
  ts: string;
  sessionId: string;
  tool: string;
  args: Record<string, unknown>;
  resultCount: number;
  isError: boolean;
  error: string;
  latencyMs: number;
}

export interface TelemetryToolStat {
  tool: string;
  calls: number;
  errors: number;
}

export interface TelemetrySummary {
  total: number;
  errors: number;
  byTool: TelemetryToolStat[];
  p50LatencyMs: number;
  p95LatencyMs: number;
  queries: string[];
  docsFetched: string[];
}
```

- [ ] **Step 2: Write the failing test**

Create `sims/dashboard/lib/telemetry.test.ts`:

```ts
import { expect, test } from "vitest";
import { parseTelemetry, summarizeTelemetry } from "./telemetry";

const jsonl = [
  JSON.stringify({ ts: "t1", tool: "search_fiskaly_docs", args: { query: "idempotency" }, result_count: 3, is_error: false, latency_ms: 10 }),
  JSON.stringify({ ts: "t2", tool: "fetch_fiskaly_doc", args: { id: "probe:records-flow" }, result_count: 1, is_error: false, latency_ms: 20 }),
  JSON.stringify({ ts: "t3", tool: "fetch_fiskaly_doc", args: { id: "missing" }, result_count: 0, is_error: true, error: "no doc", latency_ms: 30 }),
].join("\n");

test("parseTelemetry maps snake_case to typed events", () => {
  const evs = parseTelemetry(jsonl);
  expect(evs).toHaveLength(3);
  expect(evs[0].tool).toBe("search_fiskaly_docs");
  expect(evs[0].latencyMs).toBe(10);
  expect(evs[2].isError).toBe(true);
});

test("parseTelemetry skips blank and malformed lines", () => {
  expect(parseTelemetry("\n{bad}\n")).toHaveLength(0);
});

test("summarizeTelemetry aggregates counts, latency, queries, docs", () => {
  const s = summarizeTelemetry(parseTelemetry(jsonl));
  expect(s.total).toBe(3);
  expect(s.errors).toBe(1);
  expect(s.byTool.find((t) => t.tool === "fetch_fiskaly_doc")?.calls).toBe(2);
  expect(s.p50LatencyMs).toBe(20);
  expect(s.p95LatencyMs).toBe(30);
  expect(s.queries).toEqual(["idempotency"]);
  expect(s.docsFetched).toEqual(["probe:records-flow", "missing"]);
});
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd sims/dashboard && pnpm test telemetry`
Expected: FAIL — cannot resolve `./telemetry`.

- [ ] **Step 4: Write the implementation**

Create `sims/dashboard/lib/telemetry.ts`:

```ts
import type { TelemetryEvent, TelemetrySummary, TelemetryToolStat } from "./types";

export function parseTelemetry(jsonl: string): TelemetryEvent[] {
  const out: TelemetryEvent[] = [];
  for (const line of jsonl.split("\n")) {
    const s = line.trim();
    if (!s) continue;
    let m: any;
    try {
      m = JSON.parse(s);
    } catch {
      continue;
    }
    out.push({
      ts: str(m.ts),
      sessionId: str(m.session_id),
      tool: str(m.tool),
      args: m.args && typeof m.args === "object" ? m.args : {},
      resultCount: typeof m.result_count === "number" ? m.result_count : 0,
      isError: m.is_error === true,
      error: str(m.error),
      latencyMs: typeof m.latency_ms === "number" ? m.latency_ms : 0,
    });
  }
  return out;
}

export function summarizeTelemetry(events: TelemetryEvent[]): TelemetrySummary {
  const byTool = new Map<string, TelemetryToolStat>();
  const latencies: number[] = [];
  const queries = new Set<string>();
  const docs = new Set<string>();
  let errors = 0;

  for (const e of events) {
    const st = byTool.get(e.tool) ?? { tool: e.tool, calls: 0, errors: 0 };
    st.calls++;
    if (e.isError) {
      st.errors++;
      errors++;
    }
    byTool.set(e.tool, st);
    latencies.push(e.latencyMs);
    if (e.tool === "search_fiskaly_docs" && typeof e.args.query === "string") queries.add(e.args.query);
    if (e.tool === "fetch_fiskaly_doc" && typeof e.args.id === "string") docs.add(e.args.id);
  }

  latencies.sort((a, b) => a - b);
  return {
    total: events.length,
    errors,
    byTool: [...byTool.values()].sort((a, b) => b.calls - a.calls),
    p50LatencyMs: percentile(latencies, 50),
    p95LatencyMs: percentile(latencies, 95),
    queries: [...queries],
    docsFetched: [...docs],
  };
}

function percentile(sorted: number[], p: number): number {
  if (sorted.length === 0) return 0;
  const idx = Math.min(sorted.length - 1, Math.floor((p / 100) * sorted.length));
  return sorted[idx];
}

function str(v: any): string {
  return typeof v === "string" ? v : "";
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd sims/dashboard && pnpm test telemetry`
Expected: PASS (3 tests).

- [ ] **Step 6: Commit**

```bash
git add sims/dashboard/lib/types.ts sims/dashboard/lib/telemetry.ts sims/dashboard/lib/telemetry.test.ts
git commit -m "Dashboard: parse + summarize MCP telemetry"
```

---

### Task 6: Load telemetry into RunDetail

**Files:**
- Modify: `sims/dashboard/lib/runs.ts`
- Create: `sims/dashboard/__fixtures__/run.sample/mcp-telemetry.jsonl`

**Interfaces:**
- Consumes: `parseTelemetry`, `summarizeTelemetry`, `TelemetrySummary` (Task 5).
- Produces: `RunDetail.telemetry: TelemetrySummary`.

- [ ] **Step 1: Add the fixture**

Create `sims/dashboard/__fixtures__/run.sample/mcp-telemetry.jsonl`:

```
{"ts":"2026-06-18T10:00:00Z","tool":"search_fiskaly_docs","args":{"query":"records flow"},"result_count":4,"is_error":false,"latency_ms":12}
{"ts":"2026-06-18T10:00:01Z","tool":"fetch_fiskaly_doc","args":{"id":"probe:records-flow"},"result_count":1,"is_error":false,"latency_ms":5}
```

- [ ] **Step 2: Wire loadRun**

In `sims/dashboard/lib/runs.ts`:

Add to the imports at the top:

```ts
import { parseTelemetry, summarizeTelemetry } from "./telemetry";
import type { Summary, TranscriptEvent, DiffLine, TelemetrySummary } from "./types";
```

(replace the existing `import type { Summary, TranscriptEvent, DiffLine } from "./types";` line).

Add the field to `RunDetail`:

```ts
export interface RunDetail {
  summary: Summary;
  judgeLog: string;
  buildLog: string;
  testLog: string;
  err: string;
  transcript: TranscriptEvent[];
  diff: DiffLine[];
  telemetry: TelemetrySummary;
}
```

Add to the object returned by `loadRun` (after the `diff:` line):

```ts
    telemetry: summarizeTelemetry(parseTelemetry(readFile(path.join(d, "mcp-telemetry.jsonl")))),
```

(`readFile` already returns `""` for a missing file, so a run without telemetry yields an empty summary — not an error.)

- [ ] **Step 3: Verify the suite still passes**

Run: `cd sims/dashboard && pnpm test`
Expected: PASS (all existing tests + Task 5's), no type errors.

- [ ] **Step 4: Commit**

```bash
git add sims/dashboard/lib/runs.ts sims/dashboard/__fixtures__/run.sample/mcp-telemetry.jsonl
git commit -m "Dashboard: load telemetry summary into run detail"
```

---

### Task 7: Telemetry view in run detail

**Files:**
- Create: `sims/dashboard/components/TelemetryView.tsx`
- Modify: `sims/dashboard/app/run/[id]/page.tsx`

**Interfaces:**
- Consumes: `RunDetail.telemetry` (Task 6); `TelemetrySummary` type.
- Produces: nothing (terminal).

- [ ] **Step 1: Read the Next.js guidance**

Per `sims/dashboard/AGENTS.md`, before editing the page, skim the relevant page/server-component guide under `node_modules/next/dist/docs/`. The page edit here is additive (one import + one `<details>` block, mirroring the existing transcript/diff sections) and introduces no new Next.js APIs.

- [ ] **Step 2: Create the component**

Create `sims/dashboard/components/TelemetryView.tsx` (mirrors `TranscriptView.tsx` style):

```tsx
import type { TelemetrySummary } from "@/lib/types";

export function TelemetryView({ summary }: { summary: TelemetrySummary }) {
  if (summary.total === 0) {
    return <div className="text-xs text-muted-foreground">no MCP telemetry for this run</div>;
  }
  return (
    <div className="space-y-2 font-mono text-xs">
      <div className="flex flex-wrap gap-4">
        <span>calls {summary.total}</span>
        <span>errors {summary.errors}</span>
        <span>p50 {summary.p50LatencyMs}ms</span>
        <span>p95 {summary.p95LatencyMs}ms</span>
      </div>
      <div className="space-y-1">
        {summary.byTool.map((t) => (
          <div key={t.tool} className="text-blue-600">
            {t.tool}: {t.calls} calls{t.errors ? `, ${t.errors} err` : ""}
          </div>
        ))}
      </div>
      {summary.queries.length > 0 && (
        <div className="break-words">queries: {summary.queries.join(" · ")}</div>
      )}
      {summary.docsFetched.length > 0 && (
        <div className="break-words">docs fetched: {summary.docsFetched.join(" · ")}</div>
      )}
    </div>
  );
}
```

- [ ] **Step 3: Add the section to the run page**

In `sims/dashboard/app/run/[id]/page.tsx`:

Add to the imports (after the `DiffView` import):

```tsx
import { TelemetryView } from "@/components/TelemetryView";
```

Add this block immediately after the `session transcript` `<details>` block (before the `diff` block):

```tsx
      <details className="my-2 rounded border p-2" open>
        <summary className="cursor-pointer font-bold">MCP telemetry ({run.telemetry.total} calls)</summary>
        <div className="mt-2"><TelemetryView summary={run.telemetry} /></div>
      </details>
```

- [ ] **Step 4: Verify build + lint**

Run: `cd sims/dashboard && pnpm build && pnpm lint`
Expected: build succeeds; lint clean. (`pnpm test` unchanged from Task 6.)

- [ ] **Step 5: Visual check (optional, manual)**

Run: `cd sims/dashboard && FISKALY_RUNS_DIR="$PWD/__fixtures__" pnpm dev`, open the run for `run.sample`, confirm the "MCP telemetry (2 calls)" section renders the tool breakdown.

- [ ] **Step 6: Commit**

```bash
git add sims/dashboard/components/TelemetryView.tsx sims/dashboard/app/run/[id]/page.tsx
git commit -m "Dashboard: show MCP telemetry in run detail"
```

---

## Self-review

- **Spec coverage:** server middleware (Tasks 1-3) ✓; env-gated, disabled by default (Task 3) ✓; never stdout / file sink (Task 1) ✓; best-effort error handling (Task 1) ✓; per-run capture + Docker parity (Task 4) ✓; event schema fields (Task 1) ✓; dashboard parse/summary/view (Tasks 5-7) ✓; testing incl. integration + harness smoke (Tasks 3-4) ✓. Deferred items (search-before-edit join, retrieve→resolve link) called out in Scope note, matching the spec's non-goals. The spec's "search-before-edit computed in dashboard" is explicitly deferred here — flag to the reader.
- **Type consistency:** Go `Event` JSON tags match the dashboard `parseTelemetry` snake_case keys (`ts`, `session_id`, `tool`, `args`, `result_count`, `is_error`, `error`, `latency_ms`). `resultCount`/`contentText`/`Middleware`/`NewFileRecorder`/`Nop` names consistent across tasks. TS `TelemetrySummary` fields used identically in Tasks 5-7.
- **Placeholder scan:** none — every code/command step is concrete.
