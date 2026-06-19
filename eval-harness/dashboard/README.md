# Eval dashboard

The dashboard is the inspection surface for fiskaly eval runs. It reads run
artifacts produced by the Go runner (`sims/runner/`), then shows the signals
needed to decide what to change next.

## Run locally

```sh
pnpm install
pnpm dev
```

Open `http://localhost:3000`.

## Configuration

- `FISKALY_RUNS_DIR`: directory containing `run.*` artifacts. Defaults to
  `~/.cache/fiskaly-eval`.
- `FISKALY_RUNNER_DIR`: path to the Go runner module invoked by the trigger
  button. Defaults to `../runner` from this package directory.
- `FISKALY_SCENARIOS_DIR`: path to the scenario library. Defaults to
  `../scenarios` from this package directory.

## What it reads

Each run directory may contain:

| File | Meaning |
| --- | --- |
| `meta.json` | Harness, model, effort, and scenario metadata. |
| `transcript.jsonl` | Agent transcript in stream-json format. |
| `changes.diff` | Diff from the fixture baseline after the agent run. |
| `build.txt` | `go build ./...` output. |
| `test.txt` | `go test ./...` output. |
| `judge.txt` | Human-readable judge verdict (checks gate + LLM expectations). |
| `judge.json` | Structured judge verdict: check results and expectation criteria. |
| `mcp-telemetry.jsonl` | One MCP tool-call event per line. |
| `claude.err` | Agent stderr. |

Older or partial runs may omit some files. Missing telemetry is shown as an empty
telemetry summary, not as a dashboard error.

## Views

- Run table: scenario, model, effort, build, tests, judge, turns, and cost.
- Run detail: judge log, build/test logs, transcript, MCP telemetry summary, and
  diff.
- Telemetry: total MCP calls, per-tool calls and errors, latency percentiles,
  search queries, and fetched document ids.

## Checks

```sh
pnpm test
pnpm lint
pnpm build
```
