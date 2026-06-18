# MCP telemetry — design

Status: approved (brainstorm), pending implementation plan
Date: 2026-06-18
Roadmap: Step 0 of [`docs/superpowers/plans/2026-06-17-eval-checks-roadmap.md`](../plans/2026-06-17-eval-checks-roadmap.md)
Rationale: [`research/EVAL-CHECKS.md`](../../../research/EVAL-CHECKS.md) §3.6 (telemetry) and the exercise brief ("mcp use will provide insight into the actual work flows. Probably the highest value").

## Context

The eval harness reconstructs almost nothing about *how* the agent used the docs
tools. The brief calls MCP-usage telemetry the highest-value signal. Decided
2026-06-17: capture it as **server-side instrumentation in the MCP server itself**,
not by scraping the agent transcript — the server is the source of truth for its own
tool calls, it captures arguments and latency the transcript flattens, and it works
regardless of which harness or agent drives it.

## Goals

- The MCP server emits one structured record per `tools/call`, to a file sink, when
  telemetry is enabled.
- The eval harness gives each run its own telemetry file alongside the existing
  artifacts.
- The dashboard reads and summarizes that file in the run-detail view.
- The docs tool handlers (`handleSearch`, `handleFetch`) are unchanged.

## Non-goals (YAGNI this round)

- No full OpenTelemetry SDK / exporter. Field names are OTel-ish snake_case;
  adopting the GenAI semconv is a future option (it is still Development-stability).
- No rule→corpus-id "retrieve→resolve" link (deferred; would need a new judge field).
- No new `sims/scenarios/` entry (see Testing — telemetry is harness infra).
- No parallel Go reader; the dashboard (TS) is the only reader. Raw JSONL stays
  available to anything else.

## Architecture

### Instrumentation point

A single **SDK receiving middleware** on the server, not per-handler wrapping.

go-sdk v1.2.0 (verified in source):
- `func (s *Server) AddReceivingMiddleware(middleware ...Middleware)` (`mcp/server.go`)
- `type Middleware func(MethodHandler) MethodHandler` and
  `type MethodHandler func(ctx context.Context, method string, req Request) (Result, error)` (`mcp/shared.go`)
- In the wrapped handler: `method` is the JSON-RPC method (filter `"tools/call"`);
  `req.GetParams()` is `*CallToolParams` (has `Name` and `Arguments`);
  `req.GetSession().ID()` is the session id; the returned `Result` is
  `*CallToolResult` with `IsError bool` and `StructuredContent any` (`mcp/protocol.go`).

The middleware times the inner call, then records an `Event`. Handlers stay pure;
the future `judge_conformance` tool (roadmap A4) is covered automatically.

Tradeoff: at the method layer the result is type-erased to `StructuredContent any`,
so search result-ids are extracted by type-switching/marshaling that value, not by
reading a typed `searchOutput`. Accepted in exchange for handler purity.

### Package `mcp/telemetry`

- `Event` — fields below; JSON-marshaled one per line.
- `Recorder interface { Record(Event) }`.
- `FileRecorder` — append-only writer to a path, guarded by a `sync.Mutex`
  (concurrent `tools/call` is possible). Best-effort (see Error handling).
- `nopRecorder` — used when telemetry is disabled; zero overhead.
- `Middleware(rec Recorder) mcp.Middleware`.

### `main.go` wiring

Read env `FISKALY_MCP_TELEMETRY` (file path).
- Set → open the file, build a `FileRecorder`, call
  `server.AddReceivingMiddleware(telemetry.Middleware(rec))`.
- Unset → no middleware added; server behaves exactly as today. Existing
  `server_test.go` and ad-hoc runs emit nothing unless they opt in.

Confirmed decisions: env var name `FISKALY_MCP_TELEMETRY`; disabled by default.

## Event schema

One JSON object per line (`mcp-telemetry.jsonl`):

| Field | Type | Notes |
| --- | --- | --- |
| `ts` | string | RFC3339, event completion time |
| `session_id` | string | from `req.GetSession().ID()` (may be empty) |
| `tool` | string | `CallToolParams.Name` (e.g. `search_fiskaly_docs`) |
| `args` | object | echoed call arguments — search: `{query, limit}`; fetch: `{id}` |
| `result_count` | int | search: number of results; fetch: 1 if found else 0 |
| `is_error` | bool | `CallToolResult.IsError` |
| `error` | string | error text when `is_error`; omitted otherwise |
| `latency_ms` | int | wall time around the inner handler |

Args carry docs-search queries and ids only — no fiskaly credentials or customer
data ever reach this server, so nothing sensitive is recorded.

## Data flow

1. `run-scenario.sh` defines `tele="$run_dir/mcp-telemetry.jsonl"` and writes it into
   the server config it already generates:
   `{ "mcpServers": { "fiskaly": { "command": "<mcp_bin>", "env": { "FISKALY_MCP_TELEMETRY": "<tele>" } } } }`.
2. The claude CLI launches the server (`--mcp-config`, `--strict-mcp-config`); the
   server sees the env var and appends one line per `tools/call`.
3. The file lands in `$run_dir` beside `build.txt` / `test.txt` / `judge.txt` /
   `changes.diff` / `transcript.jsonl`, under the shared runs dir
   `~/.cache/fiskaly-eval` the dashboard already reads.
4. The dashboard parses and summarizes it in run-detail.

For parity, `run-eval-docker.sh` gets the same env injection (the file path resolves
inside the run dir it mounts/copies).

### The search-before-edit signal is a dashboard-side join

The MCP server only sees its own tool calls — never the agent's `Write`/`Edit`. So
"did the agent search before its first edit" is computed in the dashboard by joining
MCP search timestamps (this telemetry) with the first edit timestamp from
`transcript.jsonl` (already parsed by `lib/transcript.ts`). It is not a server-side
field. The existing `assert-grounded.sh` order check is unaffected.

## Error handling

Telemetry is **best-effort observability**: if a sink write fails, the
`FileRecorder` logs to stderr (the server already logs via stderr) and the tool call
still returns normally. This is a deliberate, stated choice — observability must not
break the user's call — and is distinct from the "no silent fallbacks" rule: it
masks no business-logic error and is logged, not swallowed. Nothing telemetry-related
ever writes to **stdout**, which is the MCP stdio protocol channel.

## Dashboard reader + view

- `sims/dashboard/lib/telemetry.ts` — parse `mcp-telemetry.jsonl` into `Event[]` and
  derive a summary: total calls, per-tool counts, error count, p50/p95 `latency_ms`,
  distinct queries, distinct docs fetched. Vitest-tested over a fixture, mirroring
  `lib/transcript.ts` / `lib/diff.ts`.
- `lib/runs.ts` / `lib/paths.ts` — resolve and load the telemetry file for a run
  (absent file → empty summary, not an error: not every run has telemetry).
- `components/TelemetryView.tsx` — render the summary in run-detail, alongside
  `TranscriptView` / `DiffView`. The search-before-edit join is computed here from
  the telemetry summary plus the parsed transcript.

## Testing

Per AGENTS.md, the exercising checks are written before implementation (TDD).

- `mcp/telemetry` unit tests: `FileRecorder` produces well-formed JSONL and is
  append-only; `nopRecorder` records nothing; `Event` marshals to the schema above.
- Integration test (mirrors `mcp/server_test.go`): in-memory server **with** the
  middleware and a temp-file recorder → call `search_fiskaly_docs`,
  `fetch_fiskaly_doc`, and `fetch_fiskaly_doc` with an unknown id → assert three
  events with correct `tool`, `args`, `result_count`, `is_error`, and `latency_ms >= 0`.
- Dashboard: vitest over a fixture `mcp-telemetry.jsonl` asserting the summary and
  the search-before-edit join.
- Harness smoke: a scenario run produces a non-empty, well-formed telemetry file.

### Eval-first interpretation

AGENTS.md says every iteration ships an eval scenario before implementation.
Telemetry is harness **infrastructure**, not agent-graded behavior — there is no
rubric for an agent to satisfy. The integration test and the harness smoke check are
the exercising eval here; no `sims/scenarios/` entry is added. Flagged deliberately.

## Affected files

- `mcp/telemetry/telemetry.go` (new), `mcp/telemetry/telemetry_test.go` (new)
- `mcp/main.go` (wire env → middleware)
- `mcp/server_test.go` (add the middleware integration test, or a sibling file)
- `sims/evals/run-scenario.sh` (env block in the generated `mcp.json`)
- `sims/evals/run-eval-docker.sh` (same injection, for parity)
- `sims/dashboard/lib/telemetry.ts` (+ `.test.ts`), `lib/types.ts`,
  `lib/runs.ts` / `lib/paths.ts` (load), `components/TelemetryView.tsx`,
  run-detail page wiring, a `__fixtures__` telemetry sample

## Open items to confirm during planning

- Verify the `--mcp-config` per-server `env` block is honored by the installed claude
  CLI; fallback is exporting `FISKALY_MCP_TELEMETRY` in the subshell that runs
  `claude` (child MCP process inherits it).
- Exact shape of `StructuredContent` for each tool result at the middleware layer, to
  read `result_count` (type-switch vs. re-marshal).
- Whether `req.GetSession().ID()` is populated under the stdio transport, or whether a
  per-process run id is the more useful correlation key.
