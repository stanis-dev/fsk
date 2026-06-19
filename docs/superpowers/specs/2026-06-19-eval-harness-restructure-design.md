# Eval harness restructure: `sims/` → `eval-harness/` with a Go service backend

Date: 2026-06-19
Status: Approved (design); implementation plan pending

## Summary

Rename `sims/` to `eval-harness/` and split it into a front-end (`dashboard/`) and a
single Go service (`backend/`). The dashboard stops reading the host filesystem and
shelling out to the runner; it becomes a thin client to a Go HTTP+SSE API. The
runner and judge collapse into one Go module that runs as a long-lived service with a
worker pool, a live-run registry for cancellation, and reattach-on-boot. Scenario
cases and the `pos` reference seed stay as system-under-test data, outside the
service.

## Context: how the harness works today

One long-lived process exists: the Next.js dashboard (`sims/dashboard`). Everything
Go is one-shot.

- The dashboard reads run artifacts straight off `~/.cache/fiskaly-eval` via
  `node:fs` (`lib/runs.ts`) and reads the scenario library from `../scenarios`
  (`lib/scenarios.ts`). Status is *derived from files*: a run is "running" until
  `judge.txt` appears, "cancelled" if a `cancelled` marker exists, else "done".
- Three Server Actions mutate by side effect (`app/actions.ts`): `runScenario`
  spawns `go run . run <id>` detached against `../runner`; `cancelRun` writes a
  `cancelled` marker then `kill(-pgid)` + `docker kill` from `run.json`;
  `saveScenario` validates then `fs.writeFileSync` into `../scenarios`.
- The runner (`sims/runner`, module `runner`) is a CLI with a single `run`
  subcommand. Per scenario it copies the fixture into a run dir, makes a git baseline
  commit, runs the coder in Docker, then `go build` / `go test` / judge, and writes a
  fixed artifact set. Root discovery is structural: `findSimsRoot`/`isSimsDir` walk
  up looking for a dir holding both `scenarios/` and `judge/` as siblings. Build
  context for `docker build` is `repoRoot = filepath.Dir(simsRoot)`.
- The judge (`sims/judge`, module `judge`) is `package main` with unexported funcs,
  consumed by build-then-exec (`observe.go: buildJudge` + `runJudge`). Verdict is the
  **process exit code**: `0` conformant, `1` NON-COMPLIANT (gate or expectations),
  `2` infra error / misconfiguration. `judge.json` is written in all cases but its
  `verdict` field only carries `conformant` / `NON-COMPLIANT`; infra errors appear as
  `NON-COMPLIANT` with a `Note` prefixed `"infra error ..."`.
- Four independent Go modules (`runner`, `judge`, `pos`, `fiskaly-mcp`), no `go.work`,
  no `replace`, no cross-module imports. The runner is stdlib-only.

### The two implicit contracts

1. **Artifact file set**, written by the runner and read by `lib/runs.ts`, with names
   duplicated as string literals on both sides:
   `meta.json`, `run.json`, `transcript.jsonl`, `claude.err`, `build.txt`,
   `test.txt`, `judge.txt`, `judge.json`, `changes.diff`, `mcp-telemetry.jsonl`, and
   the `cancelled` marker.
2. **Judge verdict**, where the exit code `0/1/2` carries a three-way distinction that
   `judge.json` alone loses.

Making both contracts explicit and single-sourced is the core engineering value of
this restructure.

### System-under-test vs harness

The 10 scenario fixtures (`scenarios/NN-slug/fixture/`, each its own `go.mod`,
`module pos`, stdlib-only) and the `pos` reference seed are the code being
*evaluated* — copied into a run dir and graded. The runner references them only as
path strings; it never imports them. They are not harness code and never become part
of the service. The judge's goldset (`judge/testdata/goldset`, 3 good/bad pairs for
05/07/10) is the meta-eval's own fixtures and stays with the judge.

## Goals

- Rename `sims/` → `eval-harness/` with a clear FE/BE split.
- A single Go service that owns triggering, cancellation, run reads, and scenario
  edits, exposing an HTTP+SSE API.
- One authoritative artifact/verdict contract instead of two duplicated copies.
- Live run progress over SSE, replacing the dashboard's 10s polling.
- Correct cancellation owned by the service, not cross-process signal hacks.

## Non-goals

- Authentication / multi-tenant access (explicitly out of scope; localhost-bound).
- Converting `pos` or scenario fixtures into anything server-shaped.
- Changing what is evaluated: scenarios, rubric, checks, and the hermetic coder
  container are untouched.
- Remote/distributed execution beyond "FE and BE may run on different hosts."

## Decisions (resolved with the requester)

- **Scope:** full server (not a cosmetic regroup, not a thin proxy).
- **Concurrency:** worker pool, default 1 (N=1 behaves exactly like today's serial
  `runAll`; configurable for fan-out). The shared `fiskaly-eval` image build is
  serialized regardless of N.
- **FE topology:** browser talks to the Go API directly (CORS + dedicated port);
  the browser opens SSE straight to Go. Next becomes a thin client shell.

## Target structure

```
eval-harness/                        (renamed from sims/)
  dashboard/                         Next.js FE — thin client to the Go API (CORS)
  backend/                           ONE Go module (replaces the runner + judge modules)
    go.mod
    cmd/
      eval-harness/main.go           `serve` (HTTP+SSE) + `run [ids]` (headless, same orchestrator)
      judge/main.go                  thin CLI wrapper; preserves exit 0/1/2
    internal/
      api/          router, handlers, SSE, CORS, DTOs (single source of FE shapes)
      orchestrator/ run pipeline + worker pool + live registry + reattach (was runner/*.go)
      judge/        exported judge core (was judge/*.go: checks, rubric, trajectory, buildReport)
      artifacts/    artifact filenames + read/write + summary derivation (ports lib/runs.ts)
      scenarios/    discover/load/validate/save (ports lib/scenarios.ts validation)
    judge_eval/                      meta-eval; imports internal/judge
  scenarios/                         SUT cases — UNCHANGED (NN-slug/{scenario.json,task.md,fixture/})
  pos/                               SUT reference seed — UNCHANGED
  evals/                             Dockerfile + docker-entrypoint.sh — build assets
```

`mcp/`, `research/`, `memo/` stay at the repo root. `mcp/` stays put because the
Dockerfile copies it (`COPY mcp/`) and the build context must remain the repo root.
The per-fixture `go.mod` isolation, the `~/.cache/fiskaly-eval` artifact dir, and the
hermetic coder container are unchanged.

Open item to confirm: keep `mcp/` at repo root (recommended) vs. move it under
`eval-harness/`.

## Backend service

### Module and binaries

One module under `backend/`, stdlib-only (`net/http` for ~9 routes; no framework).

- `cmd/eval-harness serve -addr 127.0.0.1:8090` — the service.
- `cmd/eval-harness run [ids]` — headless/CI path over the same
  `internal/orchestrator`, no server required.
- `cmd/judge` — thin CLI wrapper retained for standalone grading and `judge_eval`.

### Run pipeline

Unchanged in substance from `run.go: runScenario`:

read `task.md` → `prepareRun` (copy fixture, git baseline commit, write `meta.json`)
→ docker build image → docker run coder (the 5–15 min step) → `go build ./...`
→ `go test ./...` → judge (in-process) → git diff against baseline → write artifacts.

These map to SSE phases: `queued → building → coding → grading → done|cancelled|error`.

### Worker pool and image build

`POST /runs` enqueues a job. A pool of N workers drains the queue; N=1 reproduces
today's serial behavior. The `fiskaly-eval` image build is mutex-guarded and built
once at startup, so concurrent runs cannot race the shared image. Each run holds a
`context.Context`; cancellation is context cancel + `docker kill <container>` using
the deterministic container name `fiskaly-eval-<runbase>`.

This replaces the detached `go run` + `kill(-pgid)` model entirely. `run.json`'s
`pid`/`pgid` are removed; the container name is retained for reattach.

### Live registry and reattach-on-boot

Active runs report live phase from an in-memory registry. On startup the service scans
`~/.cache/fiskaly-eval` for runs without `judge.txt`/`cancelled`, checks `docker ps`
for their deterministic container name, and re-registers live ones so cancel still
works after a restart. Historical runs are file-derived via `internal/artifacts`
(the `lib/runs.ts` logic ported to Go).

### API surface

Browser → Go directly; CORS allowlisted to the dashboard origin.

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/runs` | `[]Summary` |
| GET | `/runs/{id}` | `RunDetail` |
| GET | `/runs/{id}/logs/{name}` | raw blob (transcript, build, etc.); keeps detail JSON small |
| POST | `/runs` | `{scenarioId,model,effort}` → `202 {runId}` |
| POST | `/runs/{id}/cancel` | `204` |
| GET | `/runs/{id}/events` | SSE: phase transitions + log tails |
| GET | `/runs/stream` | SSE: live run-list updates |
| GET | `/scenarios` | `[]ScenarioConfig` |
| GET | `/scenarios/{id}` | `{config, task}` |
| PUT | `/scenarios/{id}` | `{config, task}`; server-side validation; replaces `saveScenario` |

DTOs live in `internal/api` and are the single source of truth for FE shapes,
replacing hand-written `lib/types.ts`. SSE connections send periodic keepalive
heartbeats so long coder runs do not drop behind idle timeouts.

## Judge as a library

Lift the judge core into exported `internal/judge` (`RunChecks`, `RunExpectations`,
`ParseTrajectory`, `BuildReport`, the report struct) so the orchestrator calls it
in-process, dropping the build-then-exec hop and the `judge.json` round-trip on the
service's own runs. `cmd/judge` stays a thin wrapper that maps verdict to exit
`0/1/2`. `judge_eval` imports `internal/judge` for the goldset meta-eval.

The three-way distinction (conformant / NON-COMPLIANT / infra-error) must survive in
both the CLI exit code and the API response body — it must not collapse to a boolean.
The goldset meta-eval is the guard that proves the lift preserved behavior.

## Frontend rewrite

The dashboard drops all `node:fs` and the three Server Actions. The derivation logic
in `lib/runs.ts` and the validation logic in `lib/scenarios.ts` move server-side
(`internal/artifacts`, `internal/scenarios`). The FE keeps thin `fetch` wrappers plus
`EventSource` for SSE, replacing the 10s `AutoRefresh` polling. A new failure mode —
backend unreachable — is handled explicitly in the UI. TS types mirror the Go DTOs;
generating them from the Go structs is preferred, hand-mirroring is acceptable at this
scale.

Constraint: `dashboard/AGENTS.md` states this is a non-standard Next.js with breaking
changes. FE work must read `node_modules/next/dist/docs/` before writing code rather
than relying on training data.

## Rename and Docker fixes

Verified blast radius: 74 token hits across 16 files, ~30 of which are documentation.
Module names are bare and most Go paths resolve dynamically. Three functional breakers,
fixed in the rename step and verified with a real `docker build` plus one end-to-end
run:

1. `evals/Dockerfile:24`: `COPY sims/evals/docker-entrypoint.sh` →
   `COPY eval-harness/evals/docker-entrypoint.sh`. `COPY mcp/` is unchanged; the
   build context stays the repo root.
2. Root discovery: `findSimsRoot`/`isSimsDir` break once `judge/` is no longer a
   sibling of `scenarios/`. Replace them with explicit configuration (scenarios dir,
   repo/build-context root, Dockerfile path) passed to the orchestrator.
3. `.dockerignore` and `.gitignore` stale `sims/*` entries updated to the new paths;
   otherwise renamed dirs silently leak into the build context or stop being ignored.

## Security and config

Bind `127.0.0.1` by default (configurable addr). CORS allowlist the dashboard origin.
Authentication is explicitly out of scope and documented as such: `POST /runs`
triggers Docker and spends LLM tokens, so localhost-only binding is the safety
boundary. The service reads `CLAUDE_CODE_OAUTH_TOKEN` and model/effort defaults from
env/`.env`, matching the runner's `loadConfig` today.

## Testing strategy

- **orchestrator:** keep the existing `agent` interface and fake agent; add tests for
  the worker pool (N=1 and N>1), per-run context cancel, and reattach (scan a fake
  runs dir).
- **judge core:** existing judge tests move with `internal/judge`; `judge_eval` against
  the goldset stays green before and after the lift.
- **artifacts:** port `lib/runs.ts` derivation tests (`runs.test.ts`) to Go table
  tests over fixture run dirs.
- **scenarios:** port validation tests; assert `PUT /scenarios/{id}` rejects invalid
  configs.
- **api:** `httptest` for handlers; SSE verified by reading the stream to a phase
  transition.
- **frontend:** vitest for the API client and SSE handling; delete the FE tests for
  logic that moved server-side.
- **end-to-end:** one real `cmd/eval-harness run 01` through Docker, and `serve` + a
  real `POST /runs`, to confirm the build context and full pipeline.

## Sequencing

Each phase is independently shippable and verified before the next.

- **P0 — Rename.** `sims/` → `eval-harness/`; fix the three Docker breakers. Runner
  and judge keep their current layout (so `isSimsDir` still resolves). Verify
  `docker build` + one end-to-end run.
- **P1 — Consolidate.** Create the `backend` module; move runner into
  `internal/orchestrator` + `cmd/eval-harness`; replace structural discovery with
  explicit config. Verify `go build ./...`, `go test ./...`, and a headless run.
- **P2 — Judge as library.** Export `internal/judge`; reduce `cmd/judge` to a thin
  wrapper; `judge_eval` imports the package. Verify the goldset meta-eval is green.
- **P3 — Artifacts + scenarios packages.** Centralize the file contract and summary
  derivation; move scenario validation server-side. Verify against existing run dirs.
- **P4 — Read endpoints.** `GET /runs`, `/runs/{id}`, `/runs/{id}/logs/{name}`,
  `/scenarios`. Verify against the real cache.
- **P5 — Jobs.** `POST /runs` (worker pool, build-once, per-run context),
  `POST /runs/{id}/cancel` (registry + docker kill), reattach-on-boot. Verify trigger
  and cancel.
- **P6 — SSE.** `/runs/{id}/events` and `/runs/stream` with keepalive. Verify the
  stream.
- **P7 — Frontend rewrite.** Dashboard → Go-direct client (CORS); replace `node:fs`
  and Server Actions with `fetch` + `EventSource`; delete moved logic; handle
  backend-down. Verify in the browser.

P0–P2 are defensible cleanup on their own. The service value lands across P4–P7.

## Tradeoffs accepted

- Two long-lived processes to start and supervise instead of one, plus a new
  FE-up/BE-down failure mode.
- An in-memory job registry with reattach complexity that the stateless file-polling
  model never needed; a crashed service must reattach to orphaned `--rm` containers.
- SSE requires keepalive heartbeats so long coder runs do not drop.
- Merging the deliberately-independent Go modules; `internal/` packages must stay
  disjoint so HTTP/SSE deps never leak into the orchestrator's import graph.
- A larger FE rewrite (SSR + Server Actions → client + CORS).

The payoff: a real FE/BE seam that can run on two hosts, live progress, one
authoritative file/verdict contract, and correct service-owned cancellation.
