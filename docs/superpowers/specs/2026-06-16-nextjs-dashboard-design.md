# Design: Next.js eval dashboard (replaces the Go dashboard)

**Date:** 2026-06-16 · **Status:** Designed — not yet implemented · **Replaces:** `dashboard/` (Go `main.go`)

## Context

The eval harness writes run dirs to `~/.cache/fiskaly-eval/run.*` (transcript, judge/build/test logs, diff, meta). The current dashboard is one 474-line Go `html/template` server (`dashboard/main.go`) that lists runs, shows a run detail (judge verdict, build/tests, parsed transcript, colored diff), triggers a new run, and meta-refreshes every 10s. We are replacing it with a Next.js app that keeps feature parity, adds real-time streaming of a running eval, and a polished shadcn/ui UI.

This is a **local, single-user dev tool**: it reads the local filesystem and spawns the eval via `child_process`. It is intentionally not Vercel-deployable.

## Goals / non-goals

**Goals:** feature parity with `dashboard/main.go`; live transcript streaming for a running eval (replacing 10s full-page refresh); polished shadcn/ui UI; a tested parsing layer (the Go version had no tests).

**Non-goals (YAGNI for now):** multi-run comparison / A/B deltas / aggregate charts; auth; remote deployment; persistence beyond the existing run dirs.

## Stack

Next.js (latest, App Router) · TypeScript · pnpm · Tailwind + shadcn/ui · SWR (list polling). Route Handlers pinned to the **Node.js runtime** (`export const runtime = "nodejs"`) because they use `fs` and `child_process`. Exact versions and setup commands are grounded against Context7 / the vercel skills at plan time, not asserted here.

## Architecture — isolated, testable units

Parsing logic is separated from rendering so it can be unit-tested without a browser. `dashboard/main.go` is the behavioral source of truth for these ports.

- `dashboard/lib/runs.ts` — read and summarize a run dir. Port of Go `summarize`/`parseResult`/`readMeta`/`logInfo`: derives `{id, created, status, coder, harness, model, effort, build, tests, judge, turns, cost}`. Rules ported verbatim:
  - model/cwd/ccver from the transcript's `system` init event; `meta.json` is the fallback for harness/coder/model/effort.
  - harness: `cwd == "/work"` → `docker`, else non-empty cwd → `local`, else meta.
  - `judge.txt` absent → status `running`; present → `done`.
  - judge: contains `conformant` → PASS, `NON-COMPLIANT` → FAIL.
  - build: `build.txt` empty → PASS else FAIL. tests: `test.txt` contains `ok` and not `FAIL` → PASS else FAIL.
  - turns/cost from the `result` event (`num_turns`, `total_cost_usd`).
- `dashboard/lib/transcript.ts` — parse `transcript.jsonl` (stream-json) into typed events `thinking | assistant | tool | result | final`, plus the tool one-liner summarizer (Bash/Read/Write/Edit/Grep/Glob/TodoWrite/WebFetch/WebSearch/Task/ToolSearch/default) ported from Go `summarizeTool`. tool_result text truncated to 600 chars, `error:` prefix when `is_error`.
- `dashboard/lib/diff.ts` — classify diff lines (`meta | hunk | add | del | ctx`) ported from Go `renderDiff`.
- `dashboard/lib/paths.ts` — resolve `runsDir` (default `~/.cache/fiskaly-eval`) and the eval `script` (default `<repo>/evals/run-eval-docker.sh`), overridable by env (`FISKALY_RUNS_DIR`, `FISKALY_EVAL_SCRIPT`).

UI:
- `dashboard/app/page.tsx` — run list (Server Component initial render via `lib/runs`), hydrated by `<RunTable>` which keeps it fresh with SWR.
- `dashboard/app/run/[id]/page.tsx` — detail: judge verdict, build/tests/stderr disclosure, `<TranscriptView>`, `<DiffView>`. Validates the id (`run.` prefix, no path traversal — ported from Go `handleDetail`).
- `dashboard/components/` — shadcn-based `RunTable`, `JudgeBadge`, `TranscriptView`, `DiffView`, `TriggerButton`.

API (Route Handlers, Node runtime):
- `app/api/runs/route.ts` — GET list summaries (SWR source for the list).
- `app/api/runs/[id]/stream/route.ts` — GET **SSE**: tails the run's `transcript.jsonl` and emits new events as they land; emits a terminal `status: done` event when `judge.txt` appears, then closes.
- `app/api/trigger/route.ts` — POST: spawns the eval script **detached** (`spawn(..., {detached:true, stdio:'ignore'}).unref()`) so it survives the request, mirroring the Go async trigger; returns immediately.

## Data flow

1. List: Server Component renders initial summaries from `lib/runs`; `<RunTable>` then polls `/api/runs` via SWR (~1s) so running runs update and new runs appear.
2. Detail: Server Component renders the current transcript/logs/diff; if the run is still `running`, `<TranscriptView>` opens an `EventSource` to `/api/runs/[id]/stream` and appends events live, closing on the terminal event. A `done` run renders fully server-side with no stream.
3. Trigger: `<TriggerButton>` POSTs `/api/trigger`; the new run dir appears in the next SWR list tick.

## Live-update mechanism

- **Detail → SSE.** The stream route reads the file, emits existing events, then watches for growth (`fs.watch` with a size-poll fallback for macOS reliability) and emits each new line parsed via `lib/transcript`. Closes when `judge.txt` exists or the client disconnects (abort signal).
- **List → SWR polling** (~1s) of `/api/runs`. Simpler than a second stream; the list only needs status/turns/cost to tick.

## Testing

- Unit-test `lib/runs`, `lib/transcript`, `lib/diff` against a committed fixture run dir (`dashboard/__fixtures__/run.sample/` with a representative `transcript.jsonl`, `judge.txt`, `build.txt`, `test.txt`, `changes.diff`, `meta.json`). Test runner grounded at plan time (Vitest expected).
- Component smoke: `pnpm build` succeeds and `next dev` serves list + detail without runtime errors.

## Replacement & cleanup

- Delete `dashboard/main.go`, `dashboard/go.mod`, and the committed `dashboard/dashboard` binary; `dashboard/` becomes the Next.js app.
- Add `dashboard/.gitignore` (`node_modules`, `.next`, build output).
- Rewrite `evals/dashboard.sh` to `cd dashboard && pnpm install --frozen-lockfile && pnpm dev` (port stays 8080 via Next config or `-p 8080`).
- No other file references the Go dashboard module (only `evals/dashboard.sh` does).

## Risks / open items

- **shadcn can look generic.** Apply a custom theme (mono-accent, the existing GitHub-diff palette) so it doesn't read as a default-AI dashboard; lean on the frontend-design skill at implementation.
- **Tailwind v4 vs v3 and Next version** — pin and verify setup against Context7 at plan time.
- **SSE under `next dev`** — must set Node runtime, disable response buffering, and handle client abort to avoid leaked watchers.
- **Detached spawn** — verify the eval keeps running after the response returns (the Go version used `cmd.Start()`); `unref()` + `stdio:'ignore'`.
- **macOS `fs.watch`** — size-poll fallback for the transcript tail.

## References

- `dashboard/main.go` (behavioral source of truth) · `evals/dashboard.sh` · `evals/run-eval-docker.sh`
- Next.js App Router, shadcn/ui, SWR — grounded via Context7 / vercel skills at plan time.
