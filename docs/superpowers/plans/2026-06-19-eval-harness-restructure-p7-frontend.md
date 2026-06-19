# Eval harness restructure — Phase 7: dashboard rewrite (browser→Go-direct) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Switch the dashboard off the filesystem + Server Actions and onto the Go API directly from the browser — `fetch` for reads/writes and `EventSource` for live updates — completing the FE/BE separation. Add the one missing backend write endpoint (`PUT /scenarios/{id}`) the editor needs.

**Architecture:** The four pages become client components that `fetch` the Go API (`NEXT_PUBLIC_API_URL`); the run list subscribes to `GET /runs/stream` (SSE) instead of 10s `router.refresh()` polling. The three Server Actions are deleted and their callers `fetch` `POST /runs`, `POST /runs/{id}/cancel`, `PUT /scenarios/{id}`. The now-redundant TS derivation (`lib/runs.ts`, `scenarios.ts`, `transcript.ts`, `diff.ts`, `telemetry.ts`, `paths.ts` + their tests) is deleted — the browser consumes already-parsed JSON from Go (P3's work). Presentational components are unchanged. The backend gains `PUT /scenarios/{id}` (validate + write) so editing no longer touches the filesystem from the FE.

**Tech Stack:** Next.js 16.2.9 / React 19 (standard `'use client'`, `NEXT_PUBLIC_*`, `useParams`, `EventSource`); Go 1.23 stdlib for the new endpoint. The dashboard's `AGENTS.md` flags a non-standard Next.js — the bundled docs (`node_modules/next/dist/docs/`) confirm `'use client'`/env-var/EventSource patterns are standard here; read them before writing FE code.

**Spec:** `docs/superpowers/specs/2026-06-19-eval-harness-restructure-design.md` (Sequencing → P7; "Frontend rewrite"). P0–P6 complete (backend fully done).

## Global Constraints

- No AI/Claude attribution in commits, code, or docs. No `Co-Authored-By`/generated-with footer.
- Commit and push directly to `main`. Commit-message style: lowercase prefix (`feat:`/`refactor:`).
- Backend stays stdlib-only (no `require`). SUT data not modified.
- **Non-standard Next.js:** before writing FE code, read the relevant guide under `eval-harness/dashboard/node_modules/next/dist/docs/` (per `dashboard/AGENTS.md`). Use `'use client'`, `useParams()` (client), `NEXT_PUBLIC_API_URL` (browser env, build-time inlined), `new EventSource(...)`.
- **The browser calls the Go API cross-origin** (dashboard origin `http://localhost:8080`, API `http://localhost:8090`); the Go CORS middleware already allows that origin for GET/POST and (after Task 1) PUT.
- **Process discipline:** after edits, confirm `git status` clean AND the committed tree builds (`go build` for Go, `pnpm build` for FE); the reviewer treats a diff missing an expected edit as a finding.

## Design decisions (resolved)

- **API base URL:** `NEXT_PUBLIC_API_URL`, default `http://localhost:8090` (the Go `serve` default addr). The Go CORS origin stays `http://localhost:8080` (the dashboard dev port from `dashboard.sh`).
- **Client-side fetching** (browser→Go-direct, the chosen topology): pages are `'use client'` with `useEffect` fetches + an `EventSource` on the run list. Drop `export const dynamic` / `router.refresh()` — no SSR data layer remains.
- **Backend-down handling:** the fetch client surfaces errors; pages render a clear "backend unreachable (is `eval-harness serve` running?)" state instead of crashing.
- **Move `RunDetail` type:** it currently lives in `lib/runs.ts` (being deleted) — move the interface to `lib/types.ts` first. `ScenarioDetail` is already in `types.ts`.
- **`PUT /scenarios/{id}` validates server-side** via the existing `scenarios.Validate` + `AssignExpectationIds`, then writes `scenario.json` + `task.md` (a new `scenarios.Save`). The FE no longer validates/writes.

## Rewrite surface (from grounding)

Keep: `app/layout.tsx`, all presentational components (`RunTable`, `TranscriptView`, `DiffView`, `TelemetryView`, `JudgeBadge`, `Nav`, `ui/*`), `lib/types.ts`, `lib/utils.ts`.
Rewrite: `app/page.tsx`, `app/run/[id]/page.tsx`, `app/scenarios/page.tsx`, `app/scenarios/[id]/page.tsx`, `components/RunMenu.tsx`, `components/CancelButton.tsx`, `components/ScenarioEditor.tsx`.
Delete: `app/actions.ts`, `components/AutoRefresh.tsx`, `lib/runs.ts`, `lib/scenarios.ts`, `lib/transcript.ts`, `lib/diff.ts`, `lib/telemetry.ts`, `lib/paths.ts`, and the 6 `lib/*.test.ts` for the deleted files.
Create: `lib/api.ts`, `.env.local` (or `.env`).

---

### Task 1: Backend `PUT /scenarios/{id}` (validate + write)

Add the scenario-save endpoint the editor needs. Deliverable: `scenarios.Save` + `PUT /scenarios/{id}` handler, httptest-covered; CORS allows PUT.

**Files:** Modify `internal/scenarios/scenarios.go` (+ test), `internal/api/handlers.go`, `internal/api/server.go` (route + CORS), `internal/api/server_test.go`.

- [ ] **Step 1: `scenarios.Save`**

In `internal/scenarios/scenarios.go`, add `func Save(scenariosDir, id string, config Config, task string) error`: reject if `!IsKnown(scenariosDir, id)` or `config.ID != id` (error); run `AssignExpectationIds(config)`; `Validate` the marshaled config (return its error if non-empty); write `scenario.json` (indented JSON + trailing newline) and `task.md` into `<scenariosDir>/<id>/`. Mirror the old `saveScenario` action's behavior (assign ids → validate → write both files). Add a test in `scenarios_test.go`: save a valid config to a temp scenario dir → files written + reloadable; id mismatch → error; invalid config → error (not written).

- [ ] **Step 2: `PUT /scenarios/{id}` handler + route + CORS PUT**

In `internal/api/handlers.go`, add `putScenario`: decode `{config, task}` (the `scenarioDetail` shape — reuse it; `config *scenarios.Config`, `task string`); 400 on decode error; call `scenarios.Save(cfg.ScenariosDir, r.PathValue("id"), *body.Config, body.Task)`; on error 400 with the message; on success 204. In `server.go`, register `mux.HandleFunc("PUT /scenarios/{id}", cfg.putScenario)` and add `PUT` to the CORS `Access-Control-Allow-Methods` (now `GET, POST, PUT, OPTIONS`). Extend `server_test.go`: PUT a valid scenario → 204 + file updated on disk (temp scenarios dir); PUT invalid → 400; CORS preflight on PUT shows PUT in Allow-Methods.

- [ ] **Step 3: Build, test, commit**

```bash
cd /Users/stan/code/fsk/eval-harness/backend
go build ./... && go test ./internal/scenarios/... ./internal/api/... -short && gofmt -l internal/scenarios internal/api
```
Expected: build green; scenarios + api tests pass incl. the new save/PUT tests; gofmt empty. Then commit (clean-tree discipline):
```bash
cd /Users/stan/code/fsk && git add -A && git status
git commit -m "feat: add PUT /scenarios/{id} (validate + write) and scenarios.Save"
git push origin main
(cd eval-harness/backend && go build ./...) && echo "committed tree builds"
```

---

### Task 2: Dashboard rewrite — fetch + EventSource, delete the fs/actions layer

Switch the FE to the Go API. Deliverable: `pnpm build` succeeds, vitest is green (dead tests removed), and a live smoke (Go `serve` + `pnpm dev`) shows the run list rendering from the API with live SSE updates.

**Files:** per the rewrite surface above.

**FIRST:** read `eval-harness/dashboard/node_modules/next/dist/docs/` guides for `'use client'`, environment variables, and `useParams` before editing (per `dashboard/AGENTS.md`).

- [ ] **Step 1: Move the `RunDetail` type, then create the API client**

Move the `RunDetail` interface from `lib/runs.ts` into `lib/types.ts` (it's consumed by the run-detail page; `ScenarioDetail` is already in `types.ts`). Create `lib/api.ts`:
```ts
import type { Summary, RunDetail, ScenarioConfig, ScenarioDetail } from "./types";

const BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(BASE + path, init);
  if (!res.ok) throw new Error(`${init?.method ?? "GET"} ${path}: ${res.status}`);
  return (res.status === 204 ? undefined : await res.json()) as T;
}

export const apiBase = () => BASE;
export const listRuns = () => req<Summary[]>("/runs");
export const getRun = (id: string) => req<RunDetail>(`/runs/${id}`);
export const listScenarios = () => req<ScenarioConfig[]>("/scenarios");
export const getScenario = (id: string) => req<ScenarioDetail>(`/scenarios/${id}`);
export const postRun = (scenarioId: string) =>
  req<{ runId: string }>("/runs", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ scenarioId }) });
export const cancelRun = (id: string) => req<void>(`/runs/${id}/cancel`, { method: "POST" });
export const saveScenario = (id: string, data: { config: ScenarioConfig; task: string }) =>
  req<void>(`/scenarios/${id}`, { method: "PUT", headers: { "content-type": "application/json" }, body: JSON.stringify(data) });
export const runsStreamURL = () => BASE + "/runs/stream";
```
Create `eval-harness/dashboard/.env.local` with `NEXT_PUBLIC_API_URL=http://localhost:8090`.

- [ ] **Step 2: Rewrite the run-list page (`app/page.tsx`) with SSE**

Make it `'use client'`. On mount, `listRuns()` into state and also `listScenarios()` for the `RunMenu`. Open `new EventSource(runsStreamURL())`; on each `message` (or `onmessage`), re-fetch `listRuns()` (cheap; or merge the event's phase into the row) and update state; close the EventSource on unmount. Render `<RunMenu scenarios>` + `<RunTable runs>` exactly as before. Add a backend-down state: if the initial fetch throws, render a clear "Backend unreachable — start `eval-harness serve`" message. Remove `export const dynamic` and the `<AutoRefresh>` usage.

- [ ] **Step 3: Rewrite the detail/list/editor pages as client components**

- `app/run/[id]/page.tsx`: `'use client'`; `const { id } = useParams<{id:string}>()`; `useEffect` → `getRun(id)`; render the same run-detail UI from the fetched `RunDetail` (it has `summary/judgeLog/judgeReport/buildLog/testLog/err/transcript/diff/telemetry`); on 404/error show a not-found/error state (replace `notFound()` with a rendered message). Optionally subscribe to `GET /runs/{id}/events` for live phase while running (nice-to-have; a re-fetch on event is fine).
- `app/scenarios/page.tsx`: `'use client'`; `useEffect` → `listScenarios()`; same table.
- `app/scenarios/[id]/page.tsx`: `'use client'`; `useParams` + `getScenario(id)` → `<ScenarioEditor detail>`; error state on failure.

- [ ] **Step 4: Rewrite the action-callers to `fetch`**

- `components/RunMenu.tsx`: replace the `runScenario(s.id)` Server Action import with `await postRun(s.id)` from `lib/api`; drop `router.refresh()` (SSE updates the list). Surface a thrown error to the user.
- `components/CancelButton.tsx`: replace `cancelRun(runId)` import with `await cancelRun(runId)` from `lib/api`; drop `router.refresh()`.
- `components/ScenarioEditor.tsx`: replace `saveScenario(config.id, {config, task})` import with `await saveScenario(config.id, {config, task})` from `lib/api`; keep the existing client-side validation feedback if present; drop `router.refresh()`; show save success/error.

- [ ] **Step 5: Delete the dead fs/actions layer**

Delete: `app/actions.ts`, `components/AutoRefresh.tsx`, `lib/runs.ts`, `lib/scenarios.ts`, `lib/transcript.ts`, `lib/diff.ts`, `lib/telemetry.ts`, `lib/paths.ts`, and `lib/{runs,scenarios,transcript,diff,telemetry,paths}.test.ts`. Confirm nothing still imports them (`grep -rn "lib/runs\|lib/scenarios\|lib/transcript\|lib/diff\|lib/telemetry\|lib/paths\|app/actions\|AutoRefresh" app components lib` returns nothing after the rewrites).

- [ ] **Step 6: Build + vitest**

```bash
cd /Users/stan/code/fsk/eval-harness/dashboard
pnpm install --frozen-lockfile
pnpm test run
pnpm build
```
Expected: vitest green (remaining tests, if any — e.g. for kept helpers; the deleted-file tests are gone); `pnpm build` completes with no type errors (the `RunDetail` move + `lib/api.ts` types must line up; no dangling imports of deleted files).

- [ ] **Step 7: Live smoke — both servers (Go API + dashboard)**

```bash
# Terminal-style background; the backend run needs Docker+token only if you POST a run — list/SSE do not.
cd /Users/stan/code/fsk/eval-harness/backend && go run ./cmd/eval-harness serve -addr 127.0.0.1:8090 &
GO=$!; sleep 2
cd /Users/stan/code/fsk/eval-harness/dashboard && (NEXT_PUBLIC_API_URL=http://localhost:8090 pnpm dev -p 8080 &) ; DEVPID=$!; sleep 6
echo "--- dashboard serves HTML ---"; curl -fsS http://localhost:8080/ | head -c 200
echo "--- API reachable from where the browser will call ---"; curl -fsS http://localhost:8090/runs | head -c 120
# stop both:
kill $GO 2>/dev/null; pkill -f 'exe/eval-harness serve' 2>/dev/null; pkill -f 'next dev' 2>/dev/null; echo stopped
```
Expected: the dashboard serves its HTML shell; the API returns the runs JSON. (Full interactive verification — clicking through the run list / triggering a run / watching SSE update the UI — is done in a browser at closeout.) Record actual output; ensure both processes are stopped. SKIPPED note if the environment can't run both.

- [ ] **Step 8: Clean tree + committed build, then commit**

```bash
cd /Users/stan/code/fsk && git add -A && git status
git commit -m "refactor: dashboard fetches the Go API directly (fetch + EventSource); drop fs/Server-Actions layer"
git push origin main
(cd eval-harness/dashboard && pnpm build >/dev/null 2>&1) && echo "committed tree builds (pnpm build)"
```

---

## Self-Review

**Spec coverage (P7):** Spec P7 = "Dashboard → Go-direct client (CORS); replace node:fs and Server Actions with fetch + EventSource; delete moved logic; handle backend-down." Covered: `lib/api.ts` fetch client + `.env.local` (T2 S1), the 4 pages as client components with SSE on the list (T2 S2–S3), the 3 action-callers → fetch (T2 S4), deletion of `actions.ts`/`AutoRefresh`/dead lib+tests (T2 S5), backend-down state (T2 S2–S3). The editor's save path needed the backend `PUT /scenarios/{id}` (T1) — the one missing endpoint from the API table.

**Placeholder scan:** `lib/api.ts` and `.env.local` are complete; the page/component rewrites are specified file-by-file with the exact API calls + the SSE pattern, referencing the kept presentational components for rendering (unchanged). `scenarios.Save` + the PUT handler are specified by behavior mirroring the old `saveScenario` action + the existing `Validate`/`AssignExpectationIds`. The live smoke handles can't-run-both as SKIPPED; full browser verification is the controller's closeout step.

**Type/name consistency:** `lib/api.ts` returns `Summary`/`RunDetail`/`ScenarioConfig`/`ScenarioDetail` from `lib/types.ts` — `RunDetail` is moved there in T2 S1 (it was in the deleted `runs.ts`). The Go JSON shapes match these TS types (P3 built `artifacts.Summary`/`RunDetail`/`scenarios.Config` with matching tags; `RunDetail.judgeReport` is `*judge.Report` = the dashboard's `JudgeReport`). `POST /runs` body `{scenarioId}` and `PUT /scenarios/{id}` body `{config, task}` match the Go handlers (P5 postRun; T1 putScenario reusing `scenarioDetail`). `NEXT_PUBLIC_API_URL=http://localhost:8090` (API) ≠ the CORS origin `http://localhost:8080` (dashboard) — cross-origin, which the Go CORS middleware allows (and T1 adds PUT to the allowed methods).
