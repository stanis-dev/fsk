# Eval dashboard

The dashboard is the inspection surface for fiskaly eval runs. It reads run
artifacts produced by `sims/evals/run-scenario.sh` and
`sims/evals/run-eval-docker.sh`, then shows the signals needed to decide what to
change next.

## Run locally

```sh
pnpm install
pnpm dev
```

Open `http://localhost:3000`.

## Configuration

- `FISKALY_RUNS_DIR`: directory containing `run.*` artifacts. Defaults to
  `~/.cache/fiskaly-eval`.
- `FISKALY_EVAL_SCRIPT`: script invoked by the trigger button. Defaults to
  `../evals/run-eval-docker.sh` from this package directory.

## What it reads

Each run directory may contain:

| File | Meaning |
| --- | --- |
| `meta.json` | Harness, model, effort, and scenario metadata. |
| `transcript.jsonl` | Agent transcript in stream-json format. |
| `changes.diff` | Diff from the fixture baseline after the agent run. |
| `build.txt` | `go build ./...` output. |
| `test.txt` | `go test ./...` output. |
| `judge.txt` | Deterministic SIGN IT conformance verdict. |
| `grounded.txt` | Search-before-edit grounding check from the local runner. |
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

`pnpm build` currently passes with a Next.js workspace-root warning on machines
that also have a lockfile above this repository. The build still succeeds.
