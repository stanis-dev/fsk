# Eval-Check Notes

Current map of the checks used by the SIGN IT eval workbench. Ground truth for
SIGN IT facts is `research/api-probes/NOTES.md`.

## Implemented Surface

| Layer | Path | What it does |
| --- | --- | --- |
| Runner | `eval-harness/backend/cmd/eval-harness` | Runs scenarios in Docker and serves the dashboard API. |
| Orchestrator | `eval-harness/backend/internal/orchestrator` | Copies a fixture, commits a baseline, runs the agent container, captures diff/build/test/judge artifacts. |
| Judge | `eval-harness/backend/internal/judge` | Applies trajectory checks and, when requested, the expectation layer; invoked in-process by the orchestrator. |
| Artifacts | `eval-harness/backend/internal/artifacts` | Parses run directories for the dashboard. |
| Jobs/API | `eval-harness/backend/internal/jobs`, `eval-harness/backend/internal/api` | Queues dashboard-triggered runs, cancels live containers, streams phase events. |
| Docs MCP | `mcp/` | Serves `search_fiskaly_docs` and `fetch_fiskaly_doc`, with JSONL telemetry when enabled. |
| Scenarios | `eval-harness/backend/scenarios/` | Ten isolated Go modules with task prompt, fixture, and `scenario.json`. |

## Check Layers

| Layer | Signal | Current status |
| --- | --- | --- |
| Build | `go build ./...` over the edited fixture | Implemented, written to `build.txt`. |
| Tests | `go test ./...` over the edited fixture | Implemented, written to `test.txt`. |
| Trajectory gate | `groundedBeforeWrite`, `toolsCalled`, `docsFetched`, `maxMcpErrors` | Implemented from `transcript.jsonl` and `mcp-telemetry.jsonl`. |
| Expectation layer | Citation-checked model verdicts over source plus trajectory | Implemented behind `-expect`; uncited `MET` verdicts are downgraded. |
| Dashboard parsing | Run status, diff, transcript, judge JSON, telemetry summary | Implemented for complete and partial run directories. |

## Known Gaps

- Baseline invariants are documented in scenario authoring, but the runner does
  not yet assert every fixture is build/test green and judge non-compliant before
  Docker.
- The judge checks source shape and trajectory, not live SIGN IT behavior.
- `vat-breakdown` proves the VAT fields exist; it does not prove the selected
  VAT rate is correct.
- Scenario 04 still needs stronger checking for idempotency key lifecycle:
  fresh key per distinct write, same key across retries of that write.
- No `gofmt`, `go vet`, lint, pass@k, or MCP protocol conformance gate is wired
  into the runner.

## Priority Checks To Add

1. Baseline-invariant CI assertion for every scenario fixture.
2. VAT rate correctness check for scenario 07.
3. Idempotency-key lifecycle check for scenario 04.
4. Two-call record back-reference and scoped-subject sequence checks.
5. MCP protocol conformance for tool names, schemas, output shape, and error
   channel behavior.
6. Repeat-run aggregation for nondeterministic agent behavior.
