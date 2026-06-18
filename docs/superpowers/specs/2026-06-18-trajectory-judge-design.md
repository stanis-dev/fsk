# Trajectory-aware judge: deterministic checks + single-pass expectations

Date: 2026-06-18
Status: approved (design)

## Summary

Replace the judge's source-only model (regex gate + source rubric) with a **trajectory-aware** one. The judge gains access to the run's trajectory (`transcript.jsonl`, `mcp-telemetry.jsonl`) and evaluates two layers per scenario:

1. **Deterministic checks** — bounded, declarative, per-scenario assertions over the trajectory (docs fetched, tools called, MCP telemetry, grounding order). This is the new gate.
2. **Expectations** — user-authored natural-language criteria the judge evaluates in a single trajectory-aware Claude call, for what determinism can't infer. Evolves the existing rubric.

## Decisions

- Expectation engine: **single-pass, trajectory-aware** (not a tool-using agent). Keep the existing citation/anti-gaming guard and conservative-fail philosophy.
- Deterministic checks: **bounded declarative per scenario** (a fixed set of parameterized check types the user fills in).
- Migration: **replace** the source-regex gate and source rubric. One judge model going forward.
- `where` on an expectation is a model hint; the citation check validates the evidence quote against the whole trajectory.
- Telemetry reaches the LLM as a compact summary, not raw JSONL.

## Scenario schema (`scenario.json` `judge` block)

`rules` and `rubric` are removed; `checks` and `expectations` replace them:
```json
"judge": {
  "checks": {
    "groundedBeforeWrite": true,
    "toolsCalled": [{ "name": "search_fiskaly_docs", "min": 1 }],
    "docsFetched": ["records", "tokens"],
    "maxMcpErrors": 0
  },
  "expectations": [
    { "id": "receipt-flow",
      "expectation": "Issues the receipt as a two-call records flow (INTENTION then TRANSACTION referencing it), not a single POST.",
      "where": "source", "cite": "NOTES.md steps 10-11" }
  ]
}
```
- `checks` (all optional; absent = not asserted):
  - `groundedBeforeWrite` (bool) — first `search_fiskaly_docs` `tool_use` precedes the first `Write`/`Edit`/`MultiEdit` (generalizes today's `checkGrounded`).
  - `toolsCalled` (`[{name, min}]`) — count of `tool_use` events with `name` in the transcript ≥ `min` (default min 1).
  - `docsFetched` (`[string]`) — each string is a substring of the union of telemetry fetched-doc refs + queries.
  - `maxMcpErrors` (number) — count of telemetry entries with `is_error: true` ≤ value.
- `expectations` (`[{id, expectation, where?, cite?}]`) — `where ∈ {source, transcript, telemetry}` (hint only).

## Judge architecture (`sims/judge`)

### Inputs
Add `-run <runDir>` so the judge reads `transcript.jsonl` and `mcp-telemetry.jsonl`. Source still comes from the integration-dir positional arg. (`changes.diff` is out of scope for v1 — the transcript carries the edits.)

### Layer 1 — deterministic checks (new gate)
Pure Go over the parsed trajectory. Each check yields `{id, pass, detail}` (detail is a human reason, like `checkGrounded`'s string today). The gate passes iff every configured check passes. A gate failure → `NON-COMPLIANT`, and Layer 2 is **skipped** (same cost-gating as today's regex gate).

### Layer 2 — single-pass expectation eval
One Claude call (reuse `claudeModel`, `modelFn` seam, retry-on-malformed). The prompt carries the expectation criteria + trajectory context: the transcript, a compact telemetry summary, and the source. Verdicts are `MET | UNMET | CANNOT_ASSESS`; `conformant` requires every expectation MET (conservative — unchanged). Guards retained and extended:
- **Untrusted data:** the transcript is agent-produced, so it is wrapped in the existing untrusted-data markers and neutralized alongside the source (an agent could otherwise inject grading instructions into its own transcript).
- **Citation check:** a MET must quote evidence that actually appears in the trajectory (transcript ∪ comment-stripped source), whitespace-insensitive, with the existing alnum guard. Otherwise downgraded to UNMET.

### Verdict
`conformant` iff **all deterministic checks pass AND all expectations MET**.

### Report (`judge.json`)
```json
{ "scenario": "...", "verdict": "conformant|NON-COMPLIANT",
  "checks": { "passed": true, "results": [{ "id": "...", "pass": true, "detail": "..." }] },
  "expectations": { "model": "...", "verdicts": [{ "id": "...", "verdict": "MET", "evidence_quote": "...", "reasoning": "...", "cite": "..." }] },
  "note": "..." }
```
Mirrors today's shape (`gate`→`checks`, `rubric`→`expectations`) so the dashboard adaptation is mechanical.

### Removed
`catalog`, the `rule` type, `selectRules`, `rulesFromScenario`, `pass`, `defaultRules`, the `-rules`/`-list` flags. `readSource` stays (Layer 2 + source citation).

## Runner wiring (`sims/runner`)
- `runJudge`/`observeCore` pass the run dir to the judge (`-run rd.path`).
- Fold the standalone `checkGrounded` into the judge's `groundedBeforeWrite` check (one owner of trajectory evaluation). Remove `checkGrounded` and the `grounded`/`groundedOK` fields + `grounded.txt` artifact from `observe.go`/`artifacts.go`/`run.go`; the dashboard sources grounding from the judge report's `checks` instead.

## Dashboard (`sims/dashboard`)
- `lib/types.ts`: `ScenarioConfig.judge` becomes `{ checks: JudgeChecks; expectations: Expectation[] }`; add `JudgeChecks`, `ToolCall`, `Expectation`. `JudgeReport` changes `gate`→`checks`, `rubric`→`expectations`.
- `lib/scenarios.ts`: `validateConfig` validates the new `judge` shape.
- `lib/runs.ts`: `parseJudgeReport` reads `checks`/`expectations`; the run-detail data adapts.
- `components/ScenarioEditor.tsx`: replace the "judge rules" list with a **checks** form (grounded toggle, `toolsCalled` name+min list, `docsFetched` list, `maxMcpErrors` number) and an **expectations** editor (`id`, `expectation`, `where` select, `cite`).
- `app/scenarios/page.tsx`: the count column reflects checks/expectations.
- `app/run/[id]/page.tsx`: render check results + expectation verdicts (adapt the existing gate/rubric rendering + `CritVerdict`).

## Migration
Rewrite all 10 `scenario.json` from `rules`/`rubric` → `checks`/`expectations`. The current `judge.rules` map to deterministic source facts that no longer have a home — translate each scenario's intent into the new `checks` (trajectory) + `expectations` (LLM). This is per-scenario authoring, not mechanical; do it with the scenario's `SOLUTION.md` as reference.

## Error handling
- Unknown/malformed `judge` block → judge exits non-zero with a clear message (no silent fallback), consistent with today.
- A scenario with neither checks nor expectations is a config error (nothing to evaluate) → fail loudly.
- Layer 2 model/parse failures stay hard errors (retry only malformed output), as today.

## Testing
- **Judge:** table-driven deterministic checks over synthetic `transcript.jsonl`/`mcp-telemetry.jsonl`; expectation parse + extended citation check with a stub `modelFn`; verdict assembly (gate-fail skips Layer 2; conservative pass).
- **Runner:** judge receives the run dir; grounded folded in (the existing artifacts test updates).
- **Dashboard:** vitest for `validateConfig`/`parseJudgeReport` over the new shape; browser for the editor (checks + expectations) and the run-detail render.

## Out of scope
Tool-using agent-judge; `changes.diff` as a judge input; a free-form predicate DSL; keeping the regex gate; new check types beyond the four.

## Build phases
1. Judge: parse trajectory + deterministic checks + verdict assembly + report schema (with stubbed/empty expectations).
2. Judge: trajectory-aware expectation layer (prompt, untrusted transcript, extended citation).
3. Scenario schema migration (10 files) authored from `SOLUTION.md`.
4. Runner wiring (`-run`, fold `checkGrounded`, drop `grounded.txt`).
5. Dashboard: types + validation + report parsing/rendering.
6. Dashboard: scenario editor (checks + expectations forms).
7. Verify: `go test`, `pnpm build`/`test`, and a live run end-to-end (trigger → trajectory judged → dashboard shows checks + expectations).
