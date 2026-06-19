# Eval harness restructure — Phase 6: SSE live progress Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stream live run-phase updates over Server-Sent Events so the dashboard (P7) can replace its 10s polling: `GET /runs/stream` (all runs) and `GET /runs/{id}/events` (one run), driven by a pub/sub in the jobs service that emits on every phase transition.

**Architecture:** The jobs `Service` gains a subscriber registry and a non-blocking `publish`, called at the existing phase transitions (queued → running → done/error/cancelled). `internal/api` adds two SSE handlers that subscribe, stream `data:`-framed JSON events, send keepalive heartbeats, and unsubscribe when the client disconnects (request context). No orchestrator change: P6 streams the coarse registry phases the service already tracks (fine coding/grading sub-phases are a deferred enhancement that would need an orchestrator `OnPhase` callback).

**Tech Stack:** Go 1.23 stdlib (`net/http` SSE, `time.Ticker` keepalive, `sync`, `context`). No new deps.

**Spec:** `docs/superpowers/specs/2026-06-19-eval-harness-restructure-design.md` (Sequencing → P6; "SSE"). P0–P5 complete.

## Global Constraints

- No AI/Claude attribution in commits, code, or docs. No `Co-Authored-By`/generated-with footer.
- Commit and push directly to `main`. Commit-message style: lowercase prefix (`feat:`).
- Backend stays stdlib-only (no `require`). Dependency direction unchanged: `api → jobs` (api may import jobs for the event type); jobs imports scenarios/artifacts/stdlib (NOT orchestrator, NOT net/http); the orchestrator is NOT touched this phase.
- **Concurrency safety:** `publish` must never block on a slow subscriber — non-blocking send to buffered channels, drop on full. Subscriber add/remove and broadcast are guarded by a mutex. No goroutine/subscriber leak: a disconnecting client must unsubscribe (its channel removed).
- **SSE hygiene:** `Content-Type: text/event-stream`, `Cache-Control: no-cache`; flush after every event; keepalive comment frames so idle connections / proxies don't drop a 5–15 min run; the CORS `Access-Control-Allow-Origin` header (already applied by the middleware) must be present on the stream response.
- **Process discipline:** after edits, confirm `git status` clean AND the committed tree builds; the reviewer treats a diff missing an expected edit as a finding.

## Design decisions (resolved)

- **Coarse phases only.** Stream the registry's `queued | running | done | error | cancelled` transitions the service already makes. No orchestrator `OnPhase` (deferred) — keeps P6 to jobs + api, lower risk.
- **Event type lives in jobs.** `jobs.Event{RunID, ScenarioID, Phase string}`. `api` imports `jobs` for this type (a legitimate api→jobs dependency; api still does NOT import orchestrator). The api `RunService` interface gains `Subscribe`.
- **Non-blocking broadcast.** Buffered per-subscriber channel; `publish` does `select { case ch <- ev: default: }` so a slow client drops events, never blocks a worker.
- **Unsubscribe on disconnect.** The SSE handler returns (and unsubscribes) when `r.Context().Done()` fires.

## Target layout after this phase

```
eval-harness/backend/
  internal/jobs/
    service.go       MODIFIED: Event type, subs registry, Subscribe()/publish(); publish() at each phase transition
    service_test.go  MODIFIED: subscribe-and-receive-events tests (fake runner)
  internal/api/
    server.go        MODIFIED: register GET /runs/stream + /runs/{id}/events; RunService gains Subscribe
    sse.go           NEW: SSE write helper + the two stream handlers + keepalive
    server_test.go   MODIFIED: SSE handler tests (fake service publishes; read the stream)
```

---

### Task 1: jobs pub/sub — `Event`, `Subscribe`, and `publish` at every phase transition

Add a subscriber registry to the service and emit events on phase changes. Deliverable: `internal/jobs` builds and its tests (fake runner) show subscribers receive the queued→running→done and cancelled events, under `-race`.

**Files:** Modify `internal/jobs/service.go`, `internal/jobs/service_test.go`.

**Interfaces produced (consumed by api in Task 2):**
- `type Event struct { RunID string \`json:"runId"\`; ScenarioID string \`json:"scenarioId"\`; Phase string \`json:"phase"\` }`
- `func (s *Service) Subscribe() (<-chan Event, func())` — returns a buffered receive channel and an unsubscribe func (idempotent).

- [ ] **Step 1: Subscriber registry + Subscribe/unsubscribe**

In `service.go`: add `Event` (above). Add to `Service`: `subs map[int]chan Event`, `nextSub int`, guarded by the existing `s.mu` (or a dedicated `subMu` — pick one and be consistent). `Subscribe()`: lock, create `ch := make(chan Event, 16)`, assign an id, store, unlock; return `ch` and an unsubscribe closure that locks, deletes the id (if present), and closes the channel (guard against double-close/double-unsubscribe). Initialize `subs` in `NewService`.

- [ ] **Step 2: Non-blocking publish + wire it at every transition**

Add `func (s *Service) publish(ev Event)`: lock, iterate `s.subs`, `select { case ch <- ev: default: }` (drop on full), unlock. Call `publish` at each phase transition (use the constants/strings already used for `liveRun.phase`):
- `Enqueue`: after registering, `publish(Event{id, scenarioID, "queued"})`.
- `runJob`: when setting phase "running", `publish(... "running")`.
- `setPhaseAndDeregister`: `publish(... phase)` for "done"/"error" (capture id+scenarioID before delete).
- `Cancel`: on success, `publish(... "cancelled")`.
- `reattach`: optionally `publish(... "running")` for re-registered in-flight runs (or skip — note which).
Ensure `publish` is NOT called while holding a lock that a subscriber's drain could deadlock on — since `publish` uses non-blocking sends and the handler drains on its own goroutine, calling `publish` under `s.mu` is safe (no blocking). Keep the critical section tight.

- [ ] **Step 3: Tests (fake runner, -race)**

Extend `service_test.go`: subscribe, enqueue a known scenario (fake runner that completes), and assert the subscriber receives `queued` then `running` then `done` events for that run id (drain the channel with a timeout via `select`+`time.After`, not a fixed sleep). A second test: subscribe, enqueue a blocking run, `Cancel`, assert a `cancelled` event arrives. A third: unsubscribe stops delivery (after unsubscribe, no further events; channel closed). Run under `-race`. Use buffered drains / `time.After` timeouts so tests are deterministic and don't hang.

- [ ] **Step 4: Build, test, commit**

```bash
cd /Users/stan/code/fsk/eval-harness/backend
go build ./... && go test ./internal/jobs/... -race -short && gofmt -l internal/jobs
```
Expected: build green; jobs `-race` tests pass incl. the new subscribe/publish tests; gofmt empty. Then commit with the clean-tree discipline:
```bash
cd /Users/stan/code/fsk && git add -A && git status
git commit -m "feat: jobs pub/sub — Subscribe + publish run phase events"
git push origin main
(cd eval-harness/backend && go build ./...) && echo "committed tree builds"
```

---

### Task 2: api SSE handlers (`/runs/stream`, `/runs/{id}/events`) + keepalive

Stream the events over SSE. Deliverable: the two endpoints build, httptest shows events flow as `data:` frames with the right headers, and the server still builds.

**Files:** Create `internal/api/sse.go`; modify `internal/api/server.go` (routes + RunService.Subscribe); extend `internal/api/server_test.go`. (serve already injects the jobs.Service — no cmd change needed since jobs.Service will satisfy the extended interface.)

- [ ] **Step 1: Extend the RunService interface + routes**

In `server.go`: add `Subscribe() (<-chan jobs.Event, func())` to the `RunService` interface (import `backend/internal/jobs`). Register `mux.HandleFunc("GET /runs/stream", cfg.streamRuns)` and `mux.HandleFunc("GET /runs/{id}/events", cfg.streamRun)`. (CORS already covers GET; the middleware sets Allow-Origin on these responses too.)

- [ ] **Step 2: sse.go — write helper + handlers + keepalive**

Create `internal/api/sse.go`:
```go
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"backend/internal/jobs"
)

// stream subscribes to the service and writes events as SSE until the client
// disconnects. keep is the keepalive interval. filter, if non-empty, restricts
// to events for that run id.
func (cfg Config) stream(w http.ResponseWriter, r *http.Request, filter string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch, unsubscribe := cfg.Service.Subscribe()
	defer unsubscribe()

	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case ev, open := <-ch:
			if !open {
				return
			}
			if filter != "" && ev.RunID != filter {
				continue
			}
			data, _ := json.Marshal(ev)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (cfg Config) streamRuns(w http.ResponseWriter, r *http.Request) {
	cfg.stream(w, r, "")
}

func (cfg Config) streamRun(w http.ResponseWriter, r *http.Request) {
	cfg.stream(w, r, r.PathValue("id"))
}
```

- [ ] **Step 3: httptest coverage**

Extend `server_test.go`. The `fakeService` gains a `Subscribe()` that returns a channel the test controls (so the test can push a `jobs.Event` and assert the handler streams it). Use `httptest.NewServer(Handler(cfg))`, open `GET /runs/stream` with an `http.Client`, push an event through the fake, read from the response body, and assert a `data: {"runId":...,"phase":"running"}` frame appears and `Content-Type` is `text/event-stream`. Use a context/timeout and close the response to end the stream (so the test doesn't hang). Add a `/runs/{id}/events` test asserting the filter drops other runs' events. Keep these deterministic (drive the fake's channel directly; bounded read with a deadline).

- [ ] **Step 4: Build, test, gofmt**

```bash
cd /Users/stan/code/fsk/eval-harness/backend
go build ./... && go test ./... -short && gofmt -l .
```
Expected: all packages green; gofmt empty.

- [ ] **Step 5: Live smoke (no Docker/token needed — SSE is read-side)**

Confirm the stream emits over a real socket. This needs a run to generate events; trigger one and cancel it quickly (Docker + token — or, if avoiding cost, just assert the stream connects + keepalive):
```bash
cd /Users/stan/code/fsk/eval-harness/backend
go run ./cmd/eval-harness serve -addr 127.0.0.1:8099 -workers 1 &
SERVER=$!; sleep 2
# Open the stream in the background, capture a few seconds:
( curl -fsS -N http://127.0.0.1:8099/runs/stream & CURL=$!; sleep 1
  curl -fsS -X POST -H 'content-type: application/json' -d '{"scenarioId":"01-zero-to-receipt"}' http://127.0.0.1:8099/runs >/dev/null
  sleep 4; kill $CURL 2>/dev/null ) | head -c 400
kill $SERVER 2>/dev/null; pkill -f 'exe/eval-harness serve' 2>/dev/null
docker ps --format '{{.Names}}' | grep fiskaly-eval && docker kill $(docker ps -q --filter name=fiskaly-eval) || echo "no orphan"
echo done
```
Expected: the stream shows at least a `data: {... "phase":"queued"}` then `"running"` frame for the triggered run (and/or `: ping` keepalive). CRITICAL: kill the server binary (pkill) and any orphan container. If Docker/token unavailable, instead assert the stream connects and emits a `: ping` within ~21s (no run needed) and record that. Capture actual output.

- [ ] **Step 6: Clean tree + committed build, then commit**

```bash
cd /Users/stan/code/fsk && git add -A && git status
git commit -m "feat: SSE endpoints /runs/stream and /runs/{id}/events with keepalive"
git push origin main
(cd eval-harness/backend && go build ./...) && echo "committed tree builds"
```

---

## Self-Review

**Spec coverage (P6):** Spec P6 = "SSE: /runs/{id}/events + live run-list (/runs/stream) with keepalive." Covered: pub/sub emitting phase events (T1), both SSE endpoints + keepalive + disconnect cleanup (T2). The spec also mentioned tailing transcript/judge lines and fine building/coding/grading phases — explicitly deferred (coarse phases only; log-tailing uses the existing /runs/{id}/logs/{name}); noted in Design decisions so it's not a silent omission.

**Placeholder scan:** `sse.go` is complete code; the jobs pub/sub is specified by exact signatures + the transition points + the non-blocking-send rule, with concrete test assertions (queued/running/done, cancelled, unsubscribe). The live smoke handles Docker-absent by asserting connect + keepalive instead.

**Type/name consistency:** `jobs.Event{RunID,ScenarioID,Phase}` (T1) is what `api.RunService.Subscribe() (<-chan jobs.Event, func())` returns (T2 server.go) and what `sse.go` marshals. The api→jobs import is the only new edge; api still does NOT import orchestrator. `publish` is called at the same transition points where `liveRun.phase` is already set (Enqueue/runJob/setPhaseAndDeregister/Cancel), so the event stream matches the registry state. The buffered-channel + non-blocking-send rule (Global Constraints) is enforced in T1 Step 2 and relied on by the T2 handler draining on its own goroutine.
