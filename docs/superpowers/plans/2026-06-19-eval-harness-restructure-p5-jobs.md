# Eval harness restructure — Phase 5: jobs (run/cancel engine) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let the server execute runs: `POST /runs` enqueues a scenario onto a worker pool (default 1) that runs it in-process under a cancellable context, `POST /runs/{id}/cancel` stops a live run, and the server reattaches to orphaned containers on boot.

**Architecture:** The orchestrator gains a context-aware `Runner` that builds the judge and Docker image once and runs one scenario per call (ctx cancellation → `docker kill` the deterministic container). A new `internal/jobs` package owns the worker pool, the in-memory live-run registry, enqueue/cancel, and reattach-on-boot, talking to the orchestrator through a small interface so it is unit-testable with a fake (no Docker). `internal/api` adds the two POST handlers; `cmd/eval-harness serve` constructs and starts the service. Until P7 the dashboard still triggers via `go run . run`; server-triggered runs write a pgid-0 `run.json` so the dashboard's cancel can never kill the server.

**Tech Stack:** Go 1.23 stdlib (`context`, `sync`, `os/exec`, `net/http`). Verification of the real path needs Docker + a token (spends time + LLM cost); the job logic is verified with a fake runner (free).

**Spec:** `docs/superpowers/specs/2026-06-19-eval-harness-restructure-design.md` (Sequencing → P5; "Worker pool and image build", "Live registry and reattach-on-boot"). P0–P4 complete.

## Global Constraints

- No AI/Claude attribution in commits, code, or docs. No `Co-Authored-By`/generated-with footer.
- Commit and push directly to `main`. Commit-message style: lowercase prefix (`feat:`/`refactor:`).
- `mcp/`, `research/`, `memo/` stay at repo root; Docker build context stays repo root. SUT data not modified.
- Backend stays stdlib-only (no `require`). Dependency direction: `api → jobs → orchestrator`; `jobs → scenarios`; nothing imports `api`/`jobs` back. The orchestrator's import graph stays HTTP-free (no `net/http` in orchestrator/jobs core logic beyond what's needed; jobs is stdlib).
- **Safety:** server-triggered runs MUST NOT write their real process group to `run.json` (write pgid 0) — the dashboard's cancel does `kill(-pgid)` and would otherwise kill the server. Server binds localhost (already P4); cancel/POST are reachable by anything that hits the port → triggers Docker + spends tokens → localhost is the boundary, auth out of scope (documented).
- **Process discipline:** after edits, confirm `git status` clean AND the committed tree builds; the reviewer treats a diff missing an expected edit as a finding.

## Design decisions (resolved)

- **Build once.** `NewRunner` builds the judge binary and the Docker image a single time; the worker pool reuses them. This is how the spec's "image build is a contended resource" is solved — there is no per-run build. The CLI batch (`orchestrator.Run`) is refactored to use the same `Runner` (build-once), an improvement over today's per-scenario build.
- **Cancel = context + docker kill + marker.** The registry holds a `cancel context.CancelFunc` and the container name per live run. Cancel: call cancel(), `docker kill <container>` (best-effort), write the `cancelled` marker. Idempotent; a finished/unknown run cancels to a no-op (404 if unknown).
- **Worker pool, default 1.** `-workers N` (default 1 ≡ serial). Jobs queue on a buffered channel; N worker goroutines drain it. N=1 reproduces today's serial semantics exactly.
- **Reattach-on-boot, best-effort.** On start, scan the runs dir for run dirs with neither `judge.txt` nor `cancelled`; for each whose deterministic container is alive (`docker ps`), register it as cancellable (the goroutine is gone after a restart, but cancel can still `docker kill` the orphan + mark it). We do not resume the pipeline; file-derived status (P3) covers display.
- **Testability via an interface.** `jobs` depends on a `Runner` interface (`RunScenario(ctx, scenarios.Scenario) (runDir string, err error)`, `ContainerName(runDir) string`, `KillContainer(container) error`, `Resolve(id) (scenarios.Scenario, bool)`), satisfied by the orchestrator's concrete runner. Tests inject a fake → the pool/registry/cancel/reattach logic is exercised without Docker.

## Target layout after this phase

```
eval-harness/backend/
  internal/orchestrator/
    docker.go        MODIFIED: agent gains build(ctx)+run(ctx,...); exec.CommandContext; KillContainer
    artifacts.go     MODIFIED: writeRunHandle(runPath, detached bool)  (pgid 0 when !detached)
    run.go/runall.go MODIFIED: context-threaded; CLI uses Runner
    runner.go        NEW: Runner (NewRunner builds judge+image once; RunScenario(ctx,s); ContainerName; Resolve)
  internal/jobs/
    service.go       NEW: Service (worker pool, registry, Enqueue, Cancel, Start, reattach), Runner interface
    service_test.go  NEW: fake-runner unit tests (no Docker)
  internal/api/
    handlers.go      MODIFIED: postRun, cancelRun handlers
    server.go        MODIFIED: register POST routes; CORS allows POST; Config gains the service
  cmd/eval-harness/main.go  MODIFIED: serve constructs+starts the jobs.Service
```

---

### Task 1: Orchestrator — context-aware `Runner` (build-once) + safe handle + container kill

Make the run pipeline cancellable and build-once, and expose a server-callable `Runner`. The CLI batch is refactored onto the same `Runner`. Deliverable: orchestrator builds, unit tests (fake agent incl. a ctx-cancel test) pass, and a Docker e2e still runs.

**Files:** Modify `docker.go`, `artifacts.go`, `run.go`, `runall.go`, `orchestrator.go`; create `runner.go`; update orchestrator tests.

**Interfaces produced (consumed by `jobs` in Task 2 and the CLI):**
- `func ContainerName(runDir string) string` (exported wrapper over the existing deterministic naming).
- `func KillContainer(container, dockerCtx string) error` (`docker kill`, env-pinned).
- `type Runner struct { ... }`
- `func NewRunner(cfg Config) (*Runner, error)` — validates toolchain, loads config, builds judge + image ONCE.
- `func (r *Runner) RunScenario(ctx context.Context, s scenarios.Scenario, detached bool) (runDir string, err error)` — runs one scenario, writes artifacts; ctx cancel kills the container; `detached` controls the `run.json` pgid.
- `func (r *Runner) Resolve(scenariosDir, id string) (scenarios.Scenario, bool)` (discover + find by id) — or expose discovery so jobs can resolve.
- `func (r *Runner) DockerContext() string`.

- [ ] **Step 1: Thread context into the agent + split build/run**

In `docker.go`: change the `agent` interface to:
```go
type agent interface {
	build(ctx context.Context) error
	run(ctx context.Context, rd runDir, task string, cfg runConfig) error
}
```
Split `dockerAgent.run` into `build(ctx)` (the `docker build` portion, using `exec.CommandContext(ctx, "docker", "build", "-f", a.dockerfilePath, "-t", a.image, a.repoRoot)`) and `run(ctx, rd, task, cfg)` (the `docker run --rm --name <container> ...` portion + telemetry rename, using `exec.CommandContext`). Add:
```go
// KillContainer stops a running coder container by its deterministic name.
func KillContainer(container, dockerCtx string) error {
	cmd := exec.Command("docker", "kill", container)
	cmd.Env = dockerEnv(dockerCtx)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker kill %s: %w\n%s", container, err, out)
	}
	return nil
}

// ContainerName is the deterministic container name for a run dir.
func ContainerName(runDir string) string { return containerName(runDir) }
```

- [ ] **Step 2: Safe run handle**

In `artifacts.go`, change `writeRunHandle` to `writeRunHandle(runPath string, detached bool) error`: when `detached`, record the real `os.Getpid()` + `syscall.Getpgid(0)` (today's behavior, used by the CLI `go run` path the dashboard kills); when `!detached` (server in-process), write `PID: 0, PGID: 0` so the dashboard's `pgid > 1` cancel guard skips the process-kill. Always set `Container: containerName(runPath)`.

- [ ] **Step 3: `Runner` (build-once) + context-threaded `runScenario`**

Create `runner.go`. `NewRunner(cfg Config)`: run `checkBinaries`/`dockerReachable`/`loadConfig`/`buildJudge` (as `orchestrator.Run` does today), construct the `dockerAgent`, call `ag.build(context.Background())` ONCE, store everything. `RunScenario(ctx, s, detached)`: the body of today's `runScenario` but (a) takes `ctx`, (b) calls `writeRunHandle(rd.path, detached)`, (c) calls `r.ag.run(ctx, rd, ...)` (no build — image already built), (d) build/test/judge/diff/writeObserveArtifacts unchanged. Add `Resolve` (discover under a scenarios dir + match by exact id) and `DockerContext()`.
Update `run.go`'s `runScenario` to take `ctx context.Context` and call `ag.run(ctx, ...)` (it no longer triggers build — the Runner built the image; for the non-Runner test path, the fake agent's `build` is a no-op). Update `runall.go`'s `runAll` to accept `ctx` and pass it through.

- [ ] **Step 4: Refactor the CLI batch onto `Runner`**

Rewrite `orchestrator.Run(cfg)` to: `NewRunner(cfg)` (build judge+image once), then loop the discovered/filtered scenarios calling `r.RunScenario(context.Background(), s, true)` and printing the same `runAll` output lines + tallies. Preserve the exit-code contract (2 on pre-batch error, 0/1 from the batch). This removes the per-scenario image build (improvement) while keeping identical user-visible output.

- [ ] **Step 5: Update orchestrator tests + add a cancel test**

Update the fake agent in the tests to implement `build(ctx)` (no-op) and `run(ctx, ...)` (respect ctx: if ctx is cancelled, return its error). Update `run_test.go`/`runall_test.go`/`integration_test.go` call sites for the new signatures (`context.Background()` where a ctx is needed; `detached` arg). Add a focused test: a fake agent whose `run` blocks until ctx is cancelled; cancel the ctx; assert `RunScenario` returns promptly with a context error and writes no `judge.txt`.

- [ ] **Step 6: Build, test, Docker e2e**

```bash
cd /Users/stan/code/fsk/eval-harness/backend
go build ./... && go test ./internal/orchestrator/... -short && gofmt -l internal/orchestrator
go run ./cmd/eval-harness run 01   # Docker + token; confirms build-once Runner path still produces a normal run
```
Expected: build/tests green, gofmt empty, e2e `01 ... judge=...` with a populated run dir. SKIPPED note if Docker/token unavailable (but try — this proves the Runner refactor).

- [ ] **Step 7: Clean tree + committed build, then commit**

```bash
cd /Users/stan/code/fsk
git add -A && git status
git commit -m "refactor: orchestrator Runner with context cancellation and build-once image"
git push origin main
(cd eval-harness/backend && go build ./...) && echo "committed tree builds"
```

---

### Task 2: `internal/jobs` — worker pool, registry, enqueue/cancel, reattach

Build the job service against a `Runner` interface so it is fully unit-testable with a fake. Deliverable: `internal/jobs` builds and its fake-runner tests pass (no Docker).

**Files:** Create `internal/jobs/service.go`, `internal/jobs/service_test.go`.

**Interfaces produced (consumed by `api` in Task 3):**
- `type Runner interface { RunScenario(ctx context.Context, s scenarios.Scenario, detached bool) (string, error); Resolve(id string) (scenarios.Scenario, bool); ContainerName(runDir string) string; KillContainer(container string) error }` (the orchestrator's `*Runner` is adapted to satisfy this — see Task 3 wiring; `Resolve` here is scenarios-dir-bound, `KillContainer` is dockerctx-bound).
- `type Service struct { ... }`
- `func NewService(r Runner, runsBase string, workers int) *Service`
- `func (s *Service) Start()` — launches workers + runs reattach.
- `func (s *Service) Enqueue(scenarioID, model, effort string) (runID string, err error)` — returns error if the scenario is unknown; otherwise queues and returns a provisional id (the run dir base) — note: the run dir is created inside `RunScenario`, so the registry keys on a generated job id and is reconciled to the run dir when the worker starts. (Implementation: generate a job id up front for the registry; record the real run-dir base once `RunScenario` returns/creates it. For cancel-before-start, cancel the queued ctx.)
- `func (s *Service) Cancel(runID string) bool` — true if a live run was found+cancelled.
- `func (s *Service) Active() []ActiveRun` — snapshot for SSE (P6) and tests.

- [ ] **Step 1: Service skeleton + registry**

Create `internal/jobs/service.go` (`package jobs`). Define the `Runner` interface (above), a `liveRun{ id, scenarioID, container string; phase string; cancel context.CancelFunc }`, and `Service{ r Runner; runsBase string; queue chan job; mu sync.Mutex; live map[string]*liveRun }`. `NewService(r, runsBase, workers)` stores fields, sizes the queue, records `workers`.

- [ ] **Step 2: Worker loop + Enqueue**

`Start()` spawns `workers` goroutines each ranging the queue. A `job{ id, scenarioID, model, effort string }`. Worker: build a cancellable ctx, register the liveRun (phase "running"), resolve the scenario, call `r.RunScenario(ctx, s, false)` (server path → detached=false), update phase to done/error, deregister. `Enqueue`: validate via `r.Resolve(scenarioID)` (return error if unknown), generate a job id, push to the queue, return the id. (Use an injected/monotonic id source — do NOT use time/rand directly in tests; a counter is fine.)

- [ ] **Step 3: Cancel**

`Cancel(runID)`: under the mutex, look up the liveRun; if absent return false; call its `cancel()`, then best-effort `r.KillContainer(live.container)`, and write the `cancelled` marker into the run dir (`<runsBase>/<runDir>/cancelled`). Return true. Idempotent (a second cancel finds nothing → false). The container name comes from `r.ContainerName(runDir)` once the run dir is known.

- [ ] **Step 4: Reattach-on-boot**

In `Start()`, before/after launching workers, scan `runsBase` for entries starting `run.` that have neither `judge.txt` nor `cancelled`; for each, derive the container via `r.ContainerName(dir)` and register a liveRun whose `cancel` is a no-op (the goroutine is gone) but whose `container` is set, so `Cancel` can still `docker kill` the orphan + mark it. (Checking `docker ps` liveness is optional in the unit test via the fake; the real liveness check can be a `KillContainer`-style probe — keep it simple: register all in-flight dirs; cancel of a dead container is a harmless best-effort error.)

- [ ] **Step 5: Fake-runner unit tests (no Docker)**

Create `service_test.go` with a `fakeRunner` implementing the `Runner` interface (RunScenario blockable/cancellable via the passed ctx, creates a temp run dir, writes judge.txt on success; Resolve returns a fixed Scenario for known ids; ContainerName/KillContainer record calls). Assert: `Enqueue` of an unknown scenario errors; a known scenario runs to completion (judge.txt written) with workers=1; `Cancel` of a live (blocked) run returns true, calls KillContainer, writes the cancelled marker, and unblocks RunScenario via ctx; `Cancel` of an unknown id returns false; reattach registers an in-flight run dir (no judge.txt/cancelled) so a subsequent `Cancel` finds it. Test workers=1 serializes (a second enqueue doesn't start until the first finishes/cancels).

- [ ] **Step 6: Build, test, commit**

```bash
cd /Users/stan/code/fsk/eval-harness/backend
go build ./... && go test ./internal/jobs/... -race -short && gofmt -l internal/jobs
```
Expected: build green; jobs tests pass under `-race` (this package has real concurrency — run the race detector here); gofmt empty. Then commit with the clean-tree discipline:
```bash
cd /Users/stan/code/fsk && git add -A && git status
git commit -m "feat: add internal/jobs (worker pool, run registry, cancel, reattach)"
git push origin main
(cd eval-harness/backend && go build ./...) && echo "committed tree builds"
```

---

### Task 3: `internal/api` POST handlers + CORS + `serve` wiring + live e2e

Expose the service over HTTP and wire it into `serve`. Deliverable: `POST /runs` triggers a real run and `POST /runs/{id}/cancel` stops one, verified live.

**Files:** Modify `internal/api/server.go`, `internal/api/handlers.go`, `cmd/eval-harness/main.go`; extend `internal/api/server_test.go`.

- [ ] **Step 1: Service hook + POST routes + CORS**

In `api/server.go`: add a `Service` field to `Config` of an interface type `type RunService interface { Enqueue(scenarioID, model, effort string) (string, error); Cancel(runID string) bool }` (so api tests use a fake; jobs.Service satisfies it). Register `mux.HandleFunc("POST /runs", cfg.postRun)` and `mux.HandleFunc("POST /runs/{id}/cancel", cfg.cancelRun)`. Update the CORS `Access-Control-Allow-Methods` to `GET, POST, OPTIONS`.

- [ ] **Step 2: Handlers**

In `api/handlers.go`:
```go
func (cfg Config) postRun(w http.ResponseWriter, r *http.Request) {
	var body struct{ ScenarioID, Model, Effort string }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ScenarioID == "" {
		writeError(w, http.StatusBadRequest, "scenarioId required")
		return
	}
	id, err := cfg.Service.Enqueue(body.ScenarioID, body.Model, body.Effort)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"runId": id})
}

func (cfg Config) cancelRun(w http.ResponseWriter, r *http.Request) {
	if !cfg.Service.Cancel(r.PathValue("id")) {
		writeError(w, http.StatusNotFound, "no live run to cancel")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```
(JSON field names `scenarioId`/`model`/`effort` — add struct tags so decoding matches the dashboard's camelCase.)

- [ ] **Step 3: Wire the service into `serve`**

In `cmd/eval-harness/main.go` `cmdServe`: build the orchestrator runner (`orchestrator.NewRunner(...)` with the resolved Config, server mode), adapt it to the `jobs.Runner` interface (a small adapter binding scenariosDir + dockerctx), `svc := jobs.NewService(adapter, runsDir, *workers)`, `svc.Start()`, then `api.Handler(api.Config{..., Service: svc})`. Add a `-workers` flag (default 1). If `NewRunner` fails (e.g. Docker down), exit 2 with the error (the serve command needs Docker to run jobs; reads alone would work, but jobs require it — fail clearly).

- [ ] **Step 4: api tests with a fake service**

Extend `server_test.go`: a `fakeService` implementing `RunService`. Assert: `POST /runs` with `{"scenarioId":"01-..."}` → 202 + `{"runId":...}`; `POST /runs` with empty/invalid body → 400; `POST /runs` with an unknown scenario (fake returns error) → 400; `POST /runs/{id}/cancel` → 204 when the fake returns true, 404 when false; CORS preflight on `POST /runs` returns the POST method in Allow-Methods.

- [ ] **Step 5: Build + unit tests**

```bash
cd /Users/stan/code/fsk/eval-harness/backend
go build ./... && go test ./... -short && gofmt -l .
```
Expected: all packages green; gofmt empty.

- [ ] **Step 6: Live e2e (Docker + token — spends time + LLM cost)**

```bash
cd /Users/stan/code/fsk/eval-harness/backend
go run ./cmd/eval-harness serve -addr 127.0.0.1:8099 -workers 1 &
SERVER=$!; sleep 2
echo "--- trigger ---"; RUN=$(curl -fsS -X POST -H 'content-type: application/json' -d '{"scenarioId":"01-zero-to-receipt"}' http://127.0.0.1:8099/runs); echo "$RUN"
sleep 5
echo "--- it should appear in /runs as running ---"; curl -fsS http://127.0.0.1:8099/runs | head -c 200
# cancel it (don't pay for a full 10-min run just to prove trigger+cancel):
RUNID=$(echo "$RUN" | sed 's/.*"runId":"\([^"]*\)".*/\1/')
echo "--- cancel ---"; curl -fsS -i -X POST http://127.0.0.1:8099/runs/$RUNID/cancel | head -1
sleep 3
echo "--- it should now show cancelled ---"; curl -fsS http://127.0.0.1:8099/runs | head -c 200
kill $SERVER 2>/dev/null; pkill -f 'exe/eval-harness serve' 2>/dev/null; echo done
```
Expected: POST returns 202 + a runId; the run appears under `/runs`; cancel returns 204/no-content; the run then reports `cancelled`. (Cancelling early avoids paying for a full coder run while still proving trigger + cancel end-to-end.) IMPORTANT: kill the server AND `pkill` the built binary (a prior phase leaked a `go run` child). Record actual output; SKIPPED note if Docker/token unavailable.

- [ ] **Step 7: Clean tree + committed build, then commit**

```bash
cd /Users/stan/code/fsk && git add -A && git status
git commit -m "feat: add POST /runs and cancel; serve wires the jobs service"
git push origin main
(cd eval-harness/backend && go build ./...) && echo "committed tree builds"
```

---

## Self-Review

**Spec coverage (P5):** Spec P5 = "POST /runs (worker pool, build-once, per-run context), POST /runs/{id}/cancel (registry + docker kill), reattach-on-boot." Covered: build-once + context Runner (T1), worker pool + registry + cancel + reattach (T2), the POST endpoints + serve wiring + live trigger/cancel (T3). The pgid-0 safety for server runs (spec: "run.json's pgid go away") is T1 Step 2.

**Placeholder scan:** Concrete code is given for the agent-interface change, KillContainer/ContainerName, the handlers, and the serve hook; the Runner/Service bodies are specified by exact signatures + step-by-step behavior + the fake-based test assertions, with the existing `runScenario` as the line-level reference for the pipeline (it moves into `RunScenario` near-verbatim plus ctx). The two model-dependent steps (T1 e2e, T3 e2e) handle unavailability as SKIPPED and the T3 e2e cancels early to bound cost; the concurrency package is tested under `-race`.

**Type/name consistency:** The `jobs.Runner` interface methods (`RunScenario(ctx,s,detached)`, `Resolve`, `ContainerName`, `KillContainer`) match the orchestrator `Runner` produced in T1 (adapted in T3 Step 3 to bind scenariosDir/dockerctx). `api.RunService` (`Enqueue`, `Cancel`) is satisfied by `jobs.Service` (T2). `writeRunHandle(runPath, detached bool)` (T1 S2) is called with `true` from the CLI batch (T1 S4) and `false` from the service worker (T2 S2). POST JSON fields `scenarioId/model/effort` match the dashboard's camelCase (the FE wires to these in P7). Dependency direction `api → jobs → orchestrator` holds; orchestrator imports neither.
