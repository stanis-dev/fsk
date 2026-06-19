# Eval harness restructure — Phase 4: read endpoints (HTTP server) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a read-only HTTP server (`internal/api` + `cmd/eval-harness serve`) that exposes runs and scenarios over JSON with CORS, backed by the `internal/artifacts` and `internal/scenarios` packages from P3.

**Architecture:** A stdlib `net/http` server. `internal/api` builds a `http.Handler` (Go 1.23 method+path `ServeMux` patterns) with read handlers that call `artifacts.ListRuns/LoadRun` and `scenarios.List/Load`, JSON-encode the results, and wrap everything in a CORS middleware. `cmd/eval-harness serve` resolves the runs/scenarios dirs (reusing the existing `resolveRoot`) and serves on `127.0.0.1:8090` by default. No writes yet (POST/cancel/PUT are P5). The dashboard still reads the filesystem directly this phase; it switches to this API in P7.

**Tech Stack:** Go 1.23 stdlib (`net/http`, `encoding/json`, `net/http/httptest` for tests). No new deps.

**Spec:** `docs/superpowers/specs/2026-06-19-eval-harness-restructure-design.md` (Sequencing → P4; API surface table). P0–P3 complete.

## Global Constraints

- No AI/Claude attribution in commits, code, or docs. No `Co-Authored-By`/generated-with footer.
- Commit and push directly to `main`. Commit-message style: lowercase prefix (`feat:`/`refactor:`).
- `mcp/`, `research/`, `memo/` stay at repo root; Docker build context stays repo root. SUT data not modified.
- Backend stays stdlib-only (no `require`). `internal/api` may import `internal/artifacts`, `internal/scenarios`, `internal/judge`; it must NOT import `internal/orchestrator` (the read server has no reason to). Keep packages disjoint, no cycles.
- **Security:** bind `127.0.0.1` by default; CORS allowlists exactly one configurable origin (default the dashboard dev origin `http://localhost:8080`). No auth (out of scope, documented).
- **Process discipline:** after edits, confirm `git status` clean AND the committed tree builds; the reviewer treats a diff missing an expected edit as a finding.

## Design decisions (resolved)

- **Read-only this phase.** Only `GET` endpoints + `OPTIONS` preflight. The router registers GET methods; `POST`/`PUT`/cancel land in P5.
- **Reuse P3 types directly as DTOs.** Handlers JSON-encode `artifacts.Summary`/`artifacts.RunDetail`/`scenarios.Config` as-is (their tags already match the dashboard's expectations). The one new DTO is `scenarioDetail{Config *scenarios.Config; Task string}` with tags `config`,`task` (mirrors dashboard `ScenarioDetail`).
- **Log endpoint is allowlisted.** `GET /runs/{id}/logs/{name}` serves a raw artifact file only when `name` is one of the `artifacts.*` filename constants, and `id` passes the same `run.`-prefix/no-traversal guard `LoadRun` uses — defense against path traversal.
- **Config resolution mirrors the CLI.** `serve` reuses `resolveRoot`; `ScenariosDir = <root>/scenarios`; `RunsDir = FISKALY_RUNS_DIR or $HOME/.cache/fiskaly-eval` (same env var the dashboard's `paths.ts` honors, so both point at the same runs).

## Endpoints (this phase)

| Method | Path | Handler returns |
| --- | --- | --- |
| GET | `/healthz` | `200` `{"status":"ok"}` (lets the FE detect backend-up) |
| GET | `/runs` | `[]artifacts.Summary` |
| GET | `/runs/{id}` | `artifacts.RunDetail` (404 if unknown) |
| GET | `/runs/{id}/logs/{name}` | raw artifact bytes (404 if id/name invalid or file absent) |
| GET | `/scenarios` | `[]scenarios.Config` |
| GET | `/scenarios/{id}` | `{config, task}` (404 if unknown) |

## Target layout after this phase

```
eval-harness/backend/
  cmd/eval-harness/main.go     MODIFIED: add `serve` subcommand
  internal/api/                NEW package api
    server.go                  Config, Handler() (router + CORS), json/error helpers
    handlers.go                the six handlers
    server_test.go             httptest tests over a temp runs/scenarios tree
```

---

### Task 1: `internal/api` package (router, handlers, CORS) with httptest tests

Build the HTTP handler and its tests entirely with `httptest` (no live socket). Deliverable: `internal/api` builds and its tests pass.

**Files:**
- Create: `internal/api/server.go`, `internal/api/handlers.go`, `internal/api/server_test.go`

**Interfaces produced (consumed by `serve` in Task 2):**
- `type Config struct { RunsDir, ScenariosDir, CORSOrigin string }`
- `func Handler(cfg Config) http.Handler` — the fully-wired router with CORS applied.

- [ ] **Step 1: server.go — Config, router, CORS, helpers**

Create `internal/api/server.go`:
```go
// Package api serves runs and scenarios over a read-only JSON HTTP API.
package api

import (
	"encoding/json"
	"net/http"
)

// Config is the resolved server configuration.
type Config struct {
	RunsDir      string // dir holding run.* directories (e.g. ~/.cache/fiskaly-eval)
	ScenariosDir string // dir holding NN-slug scenario directories
	CORSOrigin   string // exact allowed browser origin (e.g. http://localhost:8080)
}

// Handler returns the API router with CORS applied. Read-only (GET) this phase.
func Handler(cfg Config) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /runs", cfg.listRuns)
	mux.HandleFunc("GET /runs/{id}", cfg.getRun)
	mux.HandleFunc("GET /runs/{id}/logs/{name}", cfg.getRunLog)
	mux.HandleFunc("GET /scenarios", cfg.listScenarios)
	mux.HandleFunc("GET /scenarios/{id}", cfg.getScenario)
	return cors(cfg.CORSOrigin, mux)
}

// cors allows exactly the configured browser origin and answers preflight.
func cors(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", origin)
		h.Set("Vary", "Origin")
		h.Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
```

- [ ] **Step 2: handlers.go — the read handlers**

Create `internal/api/handlers.go`:
```go
package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"backend/internal/artifacts"
	"backend/internal/scenarios"
)

func (cfg Config) listRuns(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, artifacts.ListRuns(cfg.RunsDir))
}

func (cfg Config) getRun(w http.ResponseWriter, r *http.Request) {
	detail, ok := artifacts.LoadRun(cfg.RunsDir, r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// allowedLogs is the set of raw artifact files servable by name.
var allowedLogs = map[string]bool{
	artifacts.MetaFile: true, artifacts.RunHandleFile: true, artifacts.BuildFile: true,
	artifacts.TestFile: true, artifacts.JudgeLogFile: true, artifacts.DiffFile: true,
	artifacts.TranscriptFile: true, artifacts.CoderErrFile: true,
	artifacts.TelemetryFile: true, artifacts.JudgeJSONFile: true,
}

func (cfg Config) getRunLog(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")
	if !strings.HasPrefix(id, "run.") || strings.Contains(id, "/") || strings.Contains(id, "..") || !allowedLogs[name] {
		writeError(w, http.StatusNotFound, "no such log")
		return
	}
	data, err := os.ReadFile(filepath.Join(cfg.RunsDir, id, name))
	if err != nil {
		writeError(w, http.StatusNotFound, "no such log")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (cfg Config) listScenarios(w http.ResponseWriter, r *http.Request) {
	list, err := scenarios.List(cfg.ScenariosDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

type scenarioDetail struct {
	Config *scenarios.Config `json:"config"`
	Task   string            `json:"task"`
}

func (cfg Config) getScenario(w http.ResponseWriter, r *http.Request) {
	c, task, ok := scenarios.Load(cfg.ScenariosDir, r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "scenario not found")
		return
	}
	writeJSON(w, http.StatusOK, scenarioDetail{Config: c, Task: task})
}
```
Note: confirm the exact signatures of `artifacts.LoadRun`, `scenarios.List`, `scenarios.Load` against the P3 source before finalizing (they are `LoadRun(baseDir, id string) (*RunDetail, bool)`, `List(dir) ([]Config, error)`, `Load(dir, id) (*Config, string, bool)`); adjust if they differ.

- [ ] **Step 3: server_test.go — httptest coverage**

Create `internal/api/server_test.go` with table/unit tests using `httptest.NewServer(Handler(cfg))` (or `httptest.NewRecorder` + `Handler(cfg).ServeHTTP`). Build a temp tree with `t.TempDir()`: a `runs/` dir with one `run.sample/` (write `meta.json`, `judge.txt`, `judge.json` with a conformant verdict, `build.txt` empty, `test.txt` "ok", `transcript.jsonl`) and a `scenarios/` dir with one `01-demo/` (`fixture/` subdir, `scenario.json`, `task.md`). Assert:
- `GET /healthz` → 200, body `{"status":"ok"}`.
- `GET /runs` → 200, JSON array containing a Summary with id `run.sample`, status `done`.
- `GET /runs/run.sample` → 200, RunDetail with the parsed judge report.
- `GET /runs/run.nope` → 404.
- `GET /runs/run.sample/logs/judge.txt` → 200, raw body equals the file; `Content-Type` text/plain.
- `GET /runs/run.sample/logs/secret` (not allowlisted) → 404; `GET /runs/run.sample/logs/..%2f..` style / traversal id → 404.
- `GET /scenarios` → 200, array with `01-demo`.
- `GET /scenarios/01-demo` → 200, `{config, task}` with task text; `GET /scenarios/99-nope` → 404.
- `OPTIONS /runs` → 204 with `Access-Control-Allow-Origin` = configured origin; a GET response carries the CORS header too.

- [ ] **Step 4: Build and test**

```bash
cd /Users/stan/code/fsk/eval-harness/backend
go build ./...
go test ./internal/api/... -v
gofmt -l internal/api
```
Expected: build 0; all api tests pass; gofmt prints nothing.

- [ ] **Step 5: Clean tree + committed build, then commit**

```bash
cd /Users/stan/code/fsk
git add -A && git status
git commit -m "feat: add internal/api read-only HTTP handlers (runs, scenarios, CORS)"
git push origin main
(cd eval-harness/backend && go build ./...) && echo "committed tree builds"
```

---

### Task 2: `serve` subcommand + live smoke test

Wire the server into the CLI and verify it actually serves. Deliverable: `eval-harness serve` runs and answers the endpoints on localhost.

**Files:**
- Modify: `cmd/eval-harness/main.go` (add the `serve` subcommand)

**Interfaces:**
- Consumes: `api.Handler`, `api.Config`, and the existing `resolveRoot` from Task 1 / the CLI.

- [ ] **Step 1: Add the `serve` subcommand**

In `cmd/eval-harness/main.go`, extend `main` to dispatch `serve` in addition to `run` (keep the existing `run` path). Add:
```go
func cmdServe(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:8090", "listen address (bind localhost; auth is out of scope)")
	root := fs.String("root", "", "eval-harness root; default: discovered from cwd")
	corsOrigin := fs.String("cors-origin", "http://localhost:8080", "allowed browser origin for the dashboard")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	ehRoot, err := resolveRoot(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "eval-harness:", err)
		return 2
	}
	runsDir := os.Getenv("FISKALY_RUNS_DIR")
	if runsDir == "" {
		runsDir = filepath.Join(os.Getenv("HOME"), ".cache", "fiskaly-eval")
	}
	h := api.Handler(api.Config{
		RunsDir:      runsDir,
		ScenariosDir: filepath.Join(ehRoot, "scenarios"),
		CORSOrigin:   *corsOrigin,
	})
	fmt.Fprintf(os.Stderr, "eval-harness: serving on http://%s (cors: %s)\n", *addr, *corsOrigin)
	if err := http.ListenAndServe(*addr, h); err != nil {
		fmt.Fprintln(os.Stderr, "eval-harness:", err)
		return 1
	}
	return 0
}
```
Update `main`'s dispatch: accept `os.Args[1] == "serve"` → `os.Exit(cmdServe(os.Args[2:]))`; keep `run`; update the usage string to list both. Add imports `net/http` and `backend/internal/api`.

- [ ] **Step 2: Build**

```bash
cd /Users/stan/code/fsk/eval-harness/backend && go build ./... && go vet ./cmd/eval-harness
```
Expected: exits 0.

- [ ] **Step 3: Live smoke test**

Start the server in the background, hit the endpoints, stop it:
```bash
cd /Users/stan/code/fsk/eval-harness/backend
go run ./cmd/eval-harness serve -addr 127.0.0.1:8099 &
SERVER_PID=$!
sleep 2
echo "--- healthz ---"; curl -fsS http://127.0.0.1:8099/healthz
echo "--- scenarios (expect a JSON array of 10) ---"; curl -fsS http://127.0.0.1:8099/scenarios | head -c 200
echo "--- runs (JSON array; may be [] if no cache) ---"; curl -fsS http://127.0.0.1:8099/runs | head -c 120
echo "--- CORS preflight ---"; curl -fsS -i -X OPTIONS http://127.0.0.1:8099/runs | grep -i 'access-control-allow-origin'
kill $SERVER_PID
```
Expected: `{"status":"ok"}`; a JSON array of scenarios (the 10 real scenarios, since `ScenariosDir` resolves to the repo's `eval-harness/scenarios`); `/runs` returns a JSON array (`[]` or real runs from `~/.cache/fiskaly-eval`); the preflight shows `Access-Control-Allow-Origin: http://localhost:8080`. Record actual output. (No Docker/token needed — these are read endpoints.)

- [ ] **Step 4: Clean tree + committed build, then commit**

```bash
cd /Users/stan/code/fsk
git add -A && git status
git commit -m "feat: add eval-harness serve subcommand (read-only API on localhost)"
git push origin main
(cd eval-harness/backend && go build ./...) && echo "committed tree builds"
```

---

## Self-Review

**Spec coverage (P4):** Spec P4 = "Read endpoints. `GET /runs`, `/runs/{id}`, `/runs/{id}/logs/{name}`, `/scenarios`. Verify against the real cache." Covered: all four (+ `/scenarios/{id}` and `/healthz`) in Task 1 handlers; `serve` wiring + live verification against the real scenarios dir and `~/.cache/fiskaly-eval` in Task 2. CORS + localhost-bind + no-auth per the spec's security note are in `cors`/the default `-addr`.

**Placeholder scan:** `server.go`, `handlers.go`, and the `serve` command are complete code. The test step enumerates concrete assertions (status codes, bodies, CORS header, traversal 404s) rather than "add tests". The live smoke step shows exact commands + expected output and needs no Docker/token.

**Type/name consistency:** `api.Config{RunsDir, ScenariosDir, CORSOrigin}` (server.go) is constructed identically in `cmdServe` (Task 2). Handlers call `artifacts.ListRuns(dir)`, `artifacts.LoadRun(base,id) (*RunDetail,bool)`, `scenarios.List(dir) ([]Config,error)`, `scenarios.Load(dir,id) (*Config,string,bool)` — matching the P3-built signatures (Step 2 flags a re-confirm). `allowedLogs` keys are the `artifacts.*` constants from P3 (single source). `api` imports artifacts/scenarios only — never `orchestrator` — preserving disjointness.
