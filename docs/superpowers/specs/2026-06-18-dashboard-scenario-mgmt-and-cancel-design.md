# Dashboard: scenario management, run picker, and run cancellation

Date: 2026-06-18
Status: approved (design)

## Summary

Add three capabilities to the eval dashboard:

1. The hardcoded trigger becomes a **Run** control that lets you pick which scenario to run.
2. **Scenario management** — list scenarios and edit their config (`scenario.json`), task prompt (`task.md`), and reference solution (`SOLUTION.md`).
3. **Cancellation** of in-progress runs, which requires a small change to the Go runner so a run can be traced to a process and a container.

## Decisions

- Management scope: **edit existing only** (no create/delete from the UI).
- Editable surface: `scenario.json` + `task.md` + `SOLUTION.md`.
- Run options: **scenario only** (model/effort use runner defaults).
- On cancel: **mark as cancelled**, keep the run dir.
- Cancel mechanism: **runner self-records `run.json`**; the dashboard reads it to cancel (no `-run-dir` coupling).
- Editor: **structured form** for eval conditions + **markdown textareas** for `task.md`/`SOLUTION.md`.
- Navigation: **slim global header** (`fiskaly eval` brand + `Runs` / `Scenarios` tabs).

## Feature 1 — Run button + scenario picker

- `app/actions.ts`: `triggerRun()` → `runScenario(scenarioId: string)`. It **validates `scenarioId` against `listScenarios()`** (reject unknown ids; never pass unvalidated input to the spawn), then spawns `go run . run <id>` detached, as today.
- `components/RunMenu.tsx` (client): a `Run ▾` dropdown (base-ui Menu) listing scenarios by id + title; selecting one calls `runScenario(id)` then `router.refresh()`.
- `components/TriggerButton.tsx` is removed/replaced by `RunMenu`.

## Feature 2 — Cancellation

### Runner change (`sims/runner`)
- In `runScenario` (after `prepareRun`), derive `runId = base(rd.path)`, `container = "fiskaly-eval-" + runId`, and write `run.json` into the run dir **before** the long docker work:
  ```json
  { "pid": <runner pid>, "pgid": <process group id>, "container": "fiskaly-eval-<runId>", "scenario": "<id>", "startedAt": "<ISO>" }
  ```
  `pgid` via `syscall.Getpgid(0)`. The dashboard spawns the runner with `detached:true`, so the `go run` process is the group leader and `pgid` identifies the whole group (go run + runner binary + docker client).
- `dockerAgent.run`: add `--name <container>` to `docker run` (derived from `rd.path` via a shared `containerName` helper). Container stays `--rm`. Unique run id ⇒ unique name.

### Dashboard
- `app/actions.ts`: `cancelRun(runId: string)`:
  1. Validate `runId` matches `^run\.[A-Za-z0-9.]+$` and the dir exists.
  2. Write a `cancelled` marker file into the run dir (UI status flips immediately).
  3. If `run.json` exists and `pgid > 1`: `process.kill(-pgid, "SIGTERM")` then `SIGKILL` (catch/ignore `ESRCH`). Killing the group stops the runner so it cannot proceed to judge a half-done run.
  4. `docker kill <container>` (best-effort; ignore "no such container").
  - Tolerant of missing `run.json` (pre-existing zombie runs): just write the marker, skip kills.
- `lib/runs.ts`: add `cancelled` to `RunStatus`. In `summarizeRun`, **`cancelled` marker wins** (checked before `judge.txt`): cancelled → "cancelled"; else `judge.txt` → "done"; else "running".
- `components/RunTable.tsx`: running rows render a cancel control (`components/CancelButton.tsx`, client, calls `cancelRun`); cancelled rows show a muted "cancelled" dot.

### Safety
- Never kill `pgid <= 1`. Cancel is idempotent and best-effort. `--rm` + `docker kill` is the authoritative stop for the heavy work; the process-group kill stops the orchestrator.

## Feature 3 — Scenario management + editing

### Read
- `lib/paths.ts`: `scenariosDir()` = `process.env.FISKALY_SCENARIOS_DIR ?? resolve(cwd, "..", "scenarios")`.
- `lib/scenarios.ts`:
  - `listScenarios()`: numeric-prefixed dirs that have `fixture/` + `scenario.json`; parse and sort by id.
  - `loadScenario(id)`: validate id is known; return parsed `scenario.json` + `task.md` + `SOLUTION.md` text.
- Types (`lib/types.ts`): `ScenarioConfig { id, title, tier, capability, persona_ref, traps: string[], judge: { rules: string[] }, baseline: Verdicts, target: Verdicts }`, `Verdicts { build, tests, judge }`.

### Write
- `app/actions.ts`: `saveScenario(id, { config, task, solution })`:
  - Validate id ∈ `listScenarios()` (no path traversal).
  - Validate `config` JSON shape (required fields present, correct types).
  - Write `scenario.json` (2-space JSON + trailing newline), `task.md`, `SOLUTION.md`.
  - Deep validity (e.g. a non-existent judge-rule name) is intentionally **not** duplicated here; the runner's baseline preflight catches it at run time.

### UI
- `app/scenarios/page.tsx`: refined list (id, title, tier, # judge rules, # traps) → rows link to edit.
- `app/scenarios/[id]/page.tsx` + `components/ScenarioEditor.tsx` (client):
  - Structured fields: title, tier, capability, persona_ref, `traps` (add/remove list), `judge.rules` (add/remove list), `baseline`/`target` (build & tests ∈ {PASS, FAIL}; judge ∈ {conformant, NON-COMPLIANT}).
  - Markdown textareas for `task.md` and `SOLUTION.md`.
  - Submit → `saveScenario`; show saved/error state.

## Navigation
- `app/layout.tsx`: slim global header — `fiskaly eval` wordmark + `Runs` / `Scenarios` links with active state. Pages keep their own h1 below it.

## Design language
All new surfaces follow the existing system: Hanken Grotesk UI / IBM Plex Mono for ids+code, uppercase tracked labels, status as colored dots, hairline borders, success/danger/warning tokens, light+dark.

## Error handling
- `runScenario` / `saveScenario` reject unknown ids and malformed input with a visible error; no silent fallbacks.
- `cancelRun` is best-effort and idempotent; missing handle → mark only; already-gone container/process → ignore.

## Out of scope
Create/delete scenarios, model/effort selection at run time, editing fixture code, docker-preflight-on-save.

## Build order (units)
1. `lib/paths` (`scenariosDir`) + `lib/scenarios` (read) + types.
2. Global nav (layout) + `/scenarios` list page.
3. `RunMenu` + `runScenario` action (replace trigger).
4. `ScenarioEditor` + `saveScenario` action.
5. Runner change (`run.json` + container name) + `cancelRun` + `cancelled` status + `CancelButton` + RunTable wiring.
6. Verify: `pnpm build`; browser e2e — run a scenario, cancel a running run, edit a scenario and confirm the file changed.
