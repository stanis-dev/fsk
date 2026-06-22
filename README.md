# Fiskaly SIGN IT eval workbench

This repository is an end-to-end system for iterating on a coding agent that
integrates fiskaly SIGN IT. The value is the loop, not a standalone MCP server:
change the docs corpus, MCP behavior, judge, scenario, or harness; run the eval;
inspect the result; decide whether the change improved the integration workflow.

The system makes documentation changes measurable: it tests whether an agent can
use grounded SIGN IT context to implement fiscalization correctly, avoid planted
domain traps, and leave enough telemetry for a developer to understand what
happened.

## The loop

1. Pick a scenario from `eval-harness/backend/scenarios/`.
2. The runner copies that fixture into an isolated run directory.
3. The agent gets the business task and a strict docs MCP.
4. The MCP serves curated SIGN IT facts and records each tool call as JSONL.
5. The harness captures transcript, diff, build, tests, grounding, telemetry,
   and deterministic judge output.
6. The dashboard shows the run so the next change can be made deliberately.

The current loop is intentionally local and inspectable. It is built to answer
questions such as:

- Did the agent ground itself in the SIGN IT docs before editing code?
- Which docs did it search and fetch?
- Did the result still build and pass the seed tests?
- Which fiskaly contract rules did the deterministic judge catch?
- Did the code change fall for a red herring or silent compliance trap?
- Is a proposed change to the MCP, corpus, or scenario making runs better or
  worse?

## Repository map

| Path | Purpose |
| --- | --- |
| `research/` | Evidence base: SIGN IT research, persona, public feedback, API probes, specs, and eval-check analysis. |
| `mcp/` | Go MCP server with embedded SIGN IT docs search/fetch tools and per-call telemetry. |
| `eval-harness/backend/` | Go server (`cmd/eval-harness`) for the eval workbench; it runs scenarios through the Docker eval on request, applies the deterministic + LLM judge (`internal/judge`), and writes dashboard artifacts for each. |
| `eval-harness/backend/scenarios/` | Ten agent coding exercises with fixtures, prompts, metadata, and answer keys. |
| `eval-harness/backend/scenarios/seed/` | The canonical seed codebase (Go module `pos`) every scenario fixture is derived from. |
| `eval-harness/backend/sandbox/` | Docker sandbox image (Dockerfile and entrypoint) the coder runs inside. |
| `eval-harness/dashboard/` | Next.js dashboard for browsing eval runs, transcripts, diffs, judge output, and MCP telemetry. |

## Implemented system

- Curated local docs MCP with `search_fiskaly_docs` and `fetch_fiskaly_doc`.
- Server-side MCP telemetry controlled by `FISKALY_MCP_TELEMETRY`.
- Ten eval scenarios covering the SIGN IT integration failure spectrum.
- Scenario-aware deterministic judge with rule subsets selected from
  `scenario.json`.
- Local runner with clean HOME, strict MCP config, diff capture, transcript
  capture, grounding check, telemetry capture, build/test gate, and judge gate.
- Docker runner that mounts only the fixture plus the MCP binary.
- Dashboard for listing runs and inspecting run details.

Known limits:

- The deterministic checks are necessary, not sufficient; the LLM expectation
  layer is nondeterministic and downgrades uncited `MET` verdicts.
- `vat-breakdown` proves the VAT fields are constructed, not that the selected
  VAT rate is correct.
- The judge checks source shape, not live SIGN IT behavior.

## Run the checks

Fast package checks:

```sh
cd mcp && go test ./...
cd ../eval-harness/backend && go test ./...
cd scenarios/seed && go test ./...
cd ../../../dashboard && pnpm lint && pnpm build
```

Scenario evals run inside Docker and are launched from the dashboard (below).
For each scenario the server copies the fixture, runs the agent inside Docker,
then collects the transcript, diff, build output, test output, and judge verdict
under `~/.cache/fiskaly-eval/run.*`. Needs Docker and a valid OAuth token in
`.env`.

## Run and inspect scenarios

[Task](https://taskfile.dev) boots the backend (API + Docker runner) and the
dashboard together, streams both logs prefixed, and stops both on Ctrl-C (needs
Docker running and a token in `.env`):

```sh
brew install go-task   # once
task install           # once: dashboard deps
task dev
```

Open the dashboard URL it prints (`http://localhost:8081` by default). It runs on
8081 rather than 8080 so it coexists with anything already on 8080; override with
`task dev DASHBOARD_PORT=<port>`. Run `task` with no arguments to list every task.

To run the two processes by hand instead:

```sh
cd eval-harness/backend && go run ./cmd/eval-harness   # API on :8090
cd eval-harness/dashboard && pnpm dev                  # dashboard on :8080
```

Configuration:

- `FISKALY_RUNS_DIR`: run artifact directory read by the backend.
- `NEXT_PUBLIC_API_URL`: dashboard API URL; defaults to `http://localhost:8090`.
- `CORS_ORIGIN`: browser origin the backend allows; defaults to
  `http://localhost:8080`. `task dev` sets it to match the dashboard port.
- `DASHBOARD_PORT`: dashboard dev port for `task dev`; defaults to 8081.

## Iterating

Use the scenario suite as the guardrail for every change:

- Changing docs context: edit `mcp/corpus/index.json`, run MCP tests, then run one
  or more scenarios that depend on that fact.
- Changing MCP behavior: update `mcp/`, keep telemetry off by default, run MCP
  tests, then run a scenario and inspect telemetry.
- Changing the judge: add or update judge tests first, then verify the affected
  scenario baseline still starts non-compliant and a correct fix would flip it.
- Changing a scenario: keep the seed build/test green, keep the baseline judge
  non-compliant, and encode the catching signal in `judge.checks`/`expectations`.
- Changing the dashboard: run `pnpm lint` and `pnpm build`.

The project rule is strict: a change is only done when the eval or test that
exercises it has run in this iteration and passed.
