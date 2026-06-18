# Eval-checks roadmap

Status: not started (telemetry is step 0, planned next)
Date: 2026-06-17
Detail / rationale: [`research/EVAL-CHECKS.md`](../../../research/EVAL-CHECKS.md) — the full taxonomy, gap analysis, and application architecture this roadmap executes.

## Goal

Bring the eval mechanism (`sims/`) from a strong deterministic spine to the target
taxonomy: deterministic gate -> static -> harness telemetry -> LLM judge, fail-fast,
with the gaps closed in highest-impact / lowest-effort order.

Project rule (AGENTS.md): every item ships with an eval/test that exercises it
*before* implementation, and is only done when that check passes with no regression.

## Two standing corrections (carry into every item)

- `vat-breakdown` is **presence-only** (`sims/judge/main.go:142`) — it does not catch
  the sc07 wrong-rate trap. Do not claim it does.
- Money is **decimal strings** (`^(-)?\d{1,12}(\.\d{1,8})?$`, `research/api-probes/NOTES.md:39`),
  not integer cents. The generic int-cents best practice is wrong for this API.
- Script naming: sc08's on-disk dir is `08-amounts-as-strings`.

## Step 0 — Telemetry into the MCP server (NEXT, has its own plan)

Decided 2026-06-17: telemetry is **server-side instrumentation in the MCP server
itself**, not host-side transcript scraping. The MCP server records each tool call
(name, args, result cardinality, latency, error, timestamp, session) to a sink the
harness collects per run. This is the brief's stated highest-value signal.

Detailed plan: `docs/superpowers/plans/2026-06-17-mcp-telemetry.md` (to be written).

Acceptance: a scenario run produces a well-formed per-run telemetry record; the MCP
server has tests proving events are emitted with correct fields; stdout (the MCP
protocol channel) stays uncontaminated.

## Tier A — cheap, deterministic, closes correctness/integrity holes

- [ ] **A1. Baseline-invariant CI assertion** — judge every fixture with no agent,
  assert the README baseline pass/total; fail loudly if a seed drifts to conformant
  (a dead trap). *Highest leverage, lowest effort.*
  Exercised by: a CI script run over all 10 fixtures vs. the recorded baseline table.
- [ ] **A2. `compliance.data` + `compliance.url` on RECEIPT** — new judge gate rule:
  terminal RECEIPT carries the AdE document reference. Direct hit on the
  "a bug looks like success" threat model.
  Exercised by: a scenario seed missing the reference judges NON-COMPLIANT; a correct fix flips it.
- [ ] **A3. Money decimal-string regex** (sc08) — judge gate: amounts match the
  decimal-string pattern; reject float JSON / int-cents.
  Exercised by: sc08 seed fails the rule; correct fix flips it.
- [ ] **A4. Judge as MCP tool** — extract pure `Judge(dir, ruleIDs) -> Report`; keep
  CLI as the gate adapter (exit 0/1/2); add a `judge_conformance` MCP tool as the
  in-loop adapter. Dual MCP-tool + CI-gate surface (the brief).
  Exercised by: in-memory MCP test calling `judge_conformance` returns the same Report bytes as the CLI.
- [ ] **A5. (covered by Step 0)** structured telemetry + retrieve->resolve link.
- [ ] **A6. `--network none` on the Docker variant** + run grounded/telemetry on host
  for parity. Near-free hermeticity (corpus MCP is offline).
  Exercised by: Docker run with no network still passes; egress attempt fails.

## Tier B — deterministic, modestly more work

- [ ] **B1. VAT cent-reconciliation** (sc08) — per-line nets/grosses/VAT sum to
  `total_vat` to the cent. Behavioral check over request log/diff.
- [ ] **B2. Ordering/back-reference behavioral checks** — two-call back-reference
  (sc01/03), scoped-subject sequence + 405 (sc02), commissioning order. The judge's
  presence rules can't see arithmetic or ordering.
- [ ] **B3. Format / `go vet` / lint gates** — `gofmt -l`, `go vet`, golangci-lint
  (verify v2 default linter set before wiring).
- [ ] **B4. MCP-server conformance runner** — lifecycle handshake, tool-name/charset,
  `inputSchema` valid JSON-Schema, `outputSchema`/`structuredContent` conformance,
  error-channel discipline; plus **retrieval gold set** (Recall@k / MRR over the real
  `corpus/index.json`) as a regression guard on the ranker.

## Tier C — non-deterministic, build last with variance control

- [ ] **C1. LLM rubric judge** over diff+transcript keyed to each `SOLUTION.md`, gated
  to run only when deterministic layers pass and only for review-caught scenarios.
  **sc05 (blocking checkout), sc07 (wrong rate), sc10 (credential conflation) have no
  implemented check of any kind today** — this is the biggest semantic-coverage gain.
- [ ] **C2. pass@k / pass^k aggregation + self-consistency / majority-vote** on the
  rubric. Mandatory once C1 exists; turns every N=1 verdict into a reliability estimate.
- [ ] **C3. Grounding evidence-used + faithfulness/hallucination judges.** Higher cost,
  overlaps C1; defer until the rubric harness and its variance controls are proven.

## Application architecture (target end-state)

1. Preflight (harness error = exit 2, distinct from NON-COMPLIANT)
2. Build gate -> 3. Test gate -> 4. Deterministic conformance judge -> 5. Harness
   telemetry (grounding order + MCP-usage) -> 6. LLM rubric (review-caught only)
- Keep rule-level scoring **binary** (necessary conditions); weighting lives one level
  up (deterministic verdict = hard gate; rubric = graded review score).
- One judge core, two adapters (CLI gate + MCP tool).
- Variance control: agent pinned model+effort; judge temp ~0.3, odd N majority vote,
  threshold not free-form score.
- Suite roll-up separates gate-caught pass-rate from review-caught pass-rate.
