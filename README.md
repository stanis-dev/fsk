# Fiskaly SIGN IT eval workbench

This repository is an end-to-end system for iterating on a coding agent that
integrates fiskaly SIGN IT. The value is the loop, not a standalone MCP server:
change the docs corpus, MCP behavior, judge, scenario, or harness; run the eval;
inspect the result; decide whether the change improved the integration workflow.

The interview task asks for API documentation improvements that move fiskaly's
mission forward, plus a functional prototype. This prototype answers that by
making documentation changes measurable. It tests whether an agent can use
grounded SIGN IT context to implement fiscalization correctly, avoid planted
domain traps, and leave enough telemetry for a developer to understand what
happened.

## The loop

1. Pick a scenario from `sims/scenarios/`.
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
| `memo/OPPORTUNITIES.md` | The opportunity map and strategic answer. |
| `research/` | Evidence base: SIGN IT research, persona, public feedback, API probes, specs, and eval-check analysis. |
| `mcp/` | Go MCP server with embedded SIGN IT docs search/fetch tools and per-call telemetry. |
| `sims/scenarios/` | Ten agent coding exercises with fixtures, prompts, metadata, and answer keys. |
| `sims/judge/` | Deterministic source-level conformance gate for SIGN IT contract shape. |
| `sims/evals/` | Local and Docker eval runners that execute scenarios and collect artifacts. |
| `sims/dashboard/` | Next.js dashboard for browsing eval runs, transcripts, diffs, judge output, and MCP telemetry. |
| `sims/pos/` | The base POS fixture used to build scenario seeds. |

## Current prototype

Implemented:

- Curated local docs MCP with `search_fiskaly_docs` and `fetch_fiskaly_doc`.
- Server-side MCP telemetry controlled by `FISKALY_MCP_TELEMETRY`.
- Ten eval scenarios covering zero-to-receipt, provisioning, cancellation,
  idempotency, outage behavior, polling, VAT, amount encoding, CalVer migration,
  and credential expiry.
- Scenario-aware deterministic judge with rule subsets selected from
  `scenario.json`.
- Local runner with clean HOME, strict MCP config, diff capture, transcript
  capture, grounding check, telemetry capture, build/test gate, and judge gate.
- Docker runner that mounts only the fixture plus the MCP binary.
- Dashboard for listing runs and inspecting run details.

Known limits:

- Several scenarios still require human review of `SOLUTION.md`; the static judge
  is necessary, not sufficient.
- `vat-breakdown` proves the VAT fields are constructed, not that the selected
  VAT rate is correct.
- The judge checks source shape, not live SIGN IT behavior.
- Baseline invariants are documented and can be checked with shell commands, but
  there is not yet a first-class CI script for them.

## Run the checks

Fast package checks:

```sh
cd mcp && go test ./...
cd ../sims/judge && go test ./...
cd ../pos && go test ./...
cd ../dashboard && pnpm test && pnpm lint && pnpm build
```

Verify every scenario seed is green but non-compliant:

```sh
for s in sims/scenarios/[0-9]*; do
  [ -d "$s/fixture" ] || continue
  name="${s##*/}"
  (cd "$s/fixture" && go build ./... && go test ./...)
  (cd sims/judge && go run . -scenario "../scenarios/$name/scenario.json" "../scenarios/$name/fixture") || true
done
```

Run one local eval:

```sh
sims/evals/run-scenario.sh 06-fire-and-forget
```

Run the Docker variant:

```sh
sims/evals/run-eval-docker.sh 06-fire-and-forget
```

Both runners write artifacts under `~/.cache/fiskaly-eval/run.*`.

## Inspect runs

```sh
cd sims/dashboard
pnpm install
pnpm dev
```

Open `http://localhost:3000`. The dashboard reads
`~/.cache/fiskaly-eval` by default. Override paths with:

- `FISKALY_RUNS_DIR`: run artifact directory.
- `FISKALY_EVAL_SCRIPT`: script invoked by the dashboard trigger button.

## Iterating

Use the scenario suite as the guardrail for every change:

- Changing docs context: edit `mcp/corpus/index.json`, run MCP tests, then run one
  or more scenarios that depend on that fact.
- Changing MCP behavior: update `mcp/`, keep telemetry off by default, run MCP
  tests, then run a scenario and inspect telemetry.
- Changing the judge: add or update judge tests first, then verify the affected
  scenario baseline still starts non-compliant and a correct fix would flip it.
- Changing a scenario: keep the seed build/test green, keep the baseline judge
  non-compliant, and update `SOLUTION.md` with the catching signal.
- Changing the dashboard: run `pnpm test`, `pnpm lint`, and `pnpm build`.

The project rule is strict: a change is only done when the eval or test that
exercises it has run in this iteration and passed.
