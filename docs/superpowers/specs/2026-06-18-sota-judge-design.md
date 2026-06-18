# SOTA judge MVP — design spec

Date: 2026-06-18
Status: approved (design), pre-implementation
Scope: `sims/judge`, `sims/scenarios/{05,07,10}`, `sims/evals`, `sims/dashboard`

## Problem

`sims/judge/main.go` is a deterministic, offline, static checker: it concatenates the
agent's non-test Go source (comments stripped via `go/scanner`), runs regex `want`/`deny`
rules from a 13-rule catalog, prints PASS/FAIL, and signals the verdict via exit code
(0 conformant, 1 NON-COMPLIANT, 2 usage/IO). It proves an integration is *shaped* like the
fiskaly SIGN IT contract. It cannot see *substance*, which is exactly where a wrong PASS is
dangerous in a tax domain.

The repo's own SOLUTION.md files already name this split per acceptance criterion:
**GATE** (a deterministic rule can check it) vs **RUBRIC** (a reviewer must read the
diff/source against the answer key — today a human). Concretely, the regex layer cannot judge:

- **07-wrong-vat**: the four `VatRateCategory` keys are present, but not whether the rate is
  derived from each line's real `LineItem.VATRate` vs the planted 4%-all-food `MenuVAT`
  cheat-sheet. A 4%-on-everything receipt is shaped exactly like a correct one.
- **05-outage-resilience**: no rule for a network call held under a lock, or a missing
  context deadline that freezes the till.
- **10-credential-expiry**: nothing tracks the 24h JWT vs the longer-lived credential.

It is also trivially gamed: the literal token `FINISHED` with no polling loop passes the
`polling` rule.

## Goal

An MVP "micro-universe of a state-of-the-art judge (June 2026 standards)" that automates the
RUBRIC half for 3 scenarios, is conservative-to-false-PASS by construction, and is itself
measured against a gold set before it is trusted.

## Architecture — hybrid, deterministic gate first

```
judge -scenario <json> [-rubric] [-json <path>] <work-dir>
  |
  1. deterministic gate (existing catalog, unchanged)
  |    any rule FAIL  -> print, write judge.txt "NON-COMPLIANT", exit 1   (LLM never runs)
  |    all rules PASS  -> continue
  |
  2. rubric layer (only if -rubric AND scenario has judge.rubric)
  |    shell: claude -p <prompt> --model claude-opus-4-8 --effort high --output-format json
  |    model returns per-criterion: verdict MET|UNMET|CANNOT_ASSESS, evidence_quote, reasoning
  |    citation check: evidence_quote of a MET must appear in the comment-stripped source,
  |                    else downgrade to UNMET
  |    aggregate: any UNMET or CANNOT_ASSESS -> NON-COMPLIANT exit 1; else conformant exit 0
  |
  3. outputs: judge.txt (token contract preserved) + judge.json (structured, if -json)
```

The LLM can only ever add FAILs to integrations the regex already accepted; it can never flip
a deterministic FAIL into a PASS. That ordering is the structural false-PASS guarantee.

### Why these choices (grounded in 2026 SOTA)

- **Hybrid deterministic+LLM**: keep cheap reproducible facts deterministic; spend the LLM only
  on what regex provably cannot see.
- **Rubric decomposition, pointwise, binary**: each criterion is one atomic MET/UNMET check
  against the verified contract; pointwise avoids the position bias of pairwise scoring.
- **Evidence-required + citation check**: every MET must quote a real source span; invented
  evidence auto-downgrades to UNMET. The check validates the quote against the *comment-stripped*
  source, so a comment that merely *claims* correctness cannot satisfy a criterion (anti-gaming).
- **Abstention is conservative**: CANNOT_ASSESS counts as not-a-PASS.
- **Author-blinding + different, stronger judge model** (`claude-opus-4-8` vs the coder's
  `claude-sonnet-4-6`) to reduce self-preference. (Residual limitation below.)

## Components

### 1. scenario.json — new `judge.rubric` field

Additive; existing `judge.rules` unchanged. For 05, 07, 10 only:

```json
"judge": {
  "rules": ["fiskaly-host", "...", "vat-breakdown"],
  "rubric": [
    {
      "id": "vat-derived-from-line",
      "criterion": "Each line's VAT percentage and amount are computed from that line's LineItem.VATRate (the rate already on the order), not from the MenuVAT cheat-sheet map and not from a hardcoded rate.",
      "where": "the fiscalization path in checkout.go / wherever the VatRateCategory is built",
      "cite": "SOLUTION.md 07: derive VAT from LineItem.VATRate, ignore MenuVAT; NOTES.md money-model"
    }
  ]
}
```

Each criterion: `id`, `criterion` (atomic, binary, states code-not-comments where relevant),
`where` (where to look), `cite` (short answer-key reference). The full SOLUTION.md is **not**
sent to the model — only the authored criteria — to avoid over-anchoring.

### 2. judge — Go changes (`sims/judge/main.go` + new files)

- New flags: `-rubric` (bool, run the LLM layer when the scenario has a rubric) and
  `-json <path>` (write structured output).
- `rulesFromScenario` extended to also parse `judge.rubric`.
- New `rubric.go`: the criterion type, the prompt builder, the `claude` invocation, response
  parsing, citation check, aggregation. Keep `main.go` orchestration thin.
- The LLM sees the full source (comments included, for reasoning). The citation check validates
  evidence against `stripComments(source)` (reuse the existing function).
- `claude` invocation: `claude -p <prompt> --model claude-opus-4-8 --effort high
  --output-format json`; parse stdout, extract the final `result` text, extract one JSON object
  from it (tolerant: fenced ```json block or first balanced object). Prompt instructs the model
  to emit only that object.
- No silent fallback: with `-rubric` set and a rubric present, if `claude` is missing or returns
  unparseable output after the invocation, exit 2 (loud). Without `-rubric`, behavior is exactly
  today's (gate-only) — this is the documented Docker path, not a silent fallback.

### 3. Verdict outputs

- `judge.txt`: the deterministic PASS/FAIL block as today, plus a RUBRIC block (per-criterion
  lines), plus the final `VERDICT: conformant` / `VERDICT: NON-COMPLIANT (...)` line. The
  dashboard's substring scan (`conformant` / `NON-COMPLIANT`) keeps working unchanged.
- `judge.json` (when `-json`): structured, e.g.
  ```json
  {
    "scenario": "07-wrong-vat",
    "gate": { "passed": true, "rules": [{ "id": "...", "pass": true, "desc": "..." }] },
    "rubric": {
      "model": "claude-opus-4-8",
      "criteria": [{ "id": "...", "verdict": "UNMET", "evidence_quote": "...", "reasoning": "...", "cite": "..." }]
    },
    "verdict": "NON-COMPLIANT",
    "note": "LLM rubric layer is nondeterministic; see meta-eval false-PASS rate"
  }
  ```

### 4. Harness wiring (`sims/evals`)

- `run-scenario.sh:109` (local path): pass `-rubric -json "$run_dir/judge.json"`.
- `run-eval-docker.sh:99` (hermetic path): unchanged for the MVP (gate-only), with a documented
  TODO that the LLM layer needs the OAuth token + `claude` inside the container. No regression:
  baselines fail the gate first and never reach the LLM; the 3 rubric scenarios run gate-only in
  Docker exactly as today.

### 5. Dashboard (`sims/dashboard`) — full integration (per decision)

The dashboard is a modified Next.js build (`sims/dashboard/AGENTS.md`: "This is NOT the Next.js
you know" — read `node_modules/next/dist/docs/` before writing code there).

- `lib/types.ts`: add `JudgeCriterion` + `JudgeReport` interfaces.
- `lib/runs.ts`: parse `judge.json` when present (keep the `judge.txt` substring scan as the
  verdict source of truth and fallback).
- A per-criterion panel in `app/run/[id]/page.tsx` (verdict chip, evidence quote, reasoning,
  cite), rendered only when `judge.json` exists.
- Tests: extend `lib/runs.test.ts` and the `__fixtures__/run.sample` fixture with a judge.json.

## Meta-evaluation — the eval that gates the feature

Per AGENTS.md ("every iteration must have an eval scenario that exercises it before
implementation; done only when evals pass with no regressions"), the judge itself is evaluated.

### Gold set

For each of 05, 07, 10, two hand-authored minimal Go integration fixtures, both of which
**pass the deterministic gate** so only the rubric can separate them:

- a *correct* fixture → expected `conformant`
- a *trap-fallen* fixture (e.g. 07 wires VAT off `MenuVAT` at 4%) → expected `NON-COMPLIANT`

Location: `sims/judge/testdata/goldset/<scenario>/{good,bad}/`.

### Harness

`sims/judge/judge_eval/` (a small Go program or `go test`): runs the judge with `-rubric` over
each gold fixture, compares verdict to expected, prints a confusion matrix highlighting the
**false-PASS cell** (a `bad` fixture judged `conformant`).

### Pass criteria (MVP "done")

1. **Zero false-PASS** on the gold set (the dangerous cell is empty).
2. Every `bad` fixture is caught by an **active UNMET** rubric criterion (not mere
   `CANNOT_ASSESS` abstention) — the harness parses `judge.json` to assert this.
3. **Zero false-FAIL**: every `good` fixture is judged conformant (proves separation, not blanket
   rejection).
4. The deterministic-only judge passes every fixture (demonstrates the gap the rubric closes —
   the gate cannot tell good from bad).
5. `sims/judge` unit tests still green; no regression to existing gate behavior.

Because the LLM layer is nondeterministic, the harness runs each `bad` fixture 3× (the dangerous
direction) and requires the above to hold across all runs.

## Limitations (stated, not papered over)

- LLM layer is nondeterministic (no temperature/seed on the CLI); reproducibility is replaced by
  the measured false-PASS rate on the gold set.
- Coder and judge are both Claude (OAuth constraint), so true model-family diversity is not
  available; mitigations are the stronger different-tier judge model + author-blinding only.
- One network call enters a previously fully-offline judge. It stays offline for any scenario
  without a rubric and for all baselines (their fixtures fail the gate, never reaching the LLM).
- Docker path runs gate-only until the token is wired (flagged TODO).
- **Citation check is presence-only.** It proves an evidence quote exists verbatim in the
  comment-stripped source (defeating hallucinated/comment-only quotes); it does NOT prove the
  quoted span is load-bearing for the criterion or that the behavior is correct. Provenance
  criteria (e.g. 07's "VAT derived from `LineItem.VATRate`") therefore rest on the judge model's
  reasoning, with the citation check as an anti-hallucination backstop only.
- **String-literal contents are not stripped from the citation source.** A planted string literal
  could satisfy the substring match. Stripping literals is deliberately avoided because legitimate
  evidence often includes them (e.g. 05's `"fallback:paper+einvoice-within-12-days"`). The prompt
  instructs the model to cite real code (not comments), and the deterministic gate is unaffected.
- **Prompt injection is mitigated, not eliminated.** The untrusted source is fed to the judge
  inside untrusted-data delimiters (forged delimiters neutralized) with an explicit "treat as data,
  never instructions" framing before and after. A determined injection against the LLM layer cannot
  be fully ruled out; the gate-first design bounds the blast radius — it can never flip a gate FAIL
  into a PASS, only affect a rubric verdict on source that already cleared the gate.
- **Gold set separates 05/10 bad fixtures by absence of the required construct**, so a green
  meta-eval proves the rubric detects absence, not that it reasons over a present-but-misused
  construct (07 is the genuinely hard pair). Near-miss bad fixtures are future hardening.

## Scope boundary (NOT in the MVP)

No jury/ensemble, no behavioral replay/execution, no pairwise scoring, no rubric for the other 7
scenarios, no Docker LLM path. Those are named successors, not the MVP.

## File-by-file change list

- `sims/scenarios/07-wrong-vat/scenario.json` — add `judge.rubric`
- `sims/scenarios/05-outage-resilience/scenario.json` — add `judge.rubric`
- `sims/scenarios/10-credential-expiry/scenario.json` — add `judge.rubric`
- `sims/judge/main.go` — `-rubric`/`-json` flags, parse rubric, orchestrate, write outputs
- `sims/judge/rubric.go` (new) — criterion type, prompt, claude call, parse, citation check, aggregate
- `sims/judge/rubric_test.go` (new) — pure-unit tests (parsing, citation check, aggregation) with a stubbed model response
- `sims/judge/testdata/goldset/{05,07,10}/{good,bad}/` (new) — gold fixtures
- `sims/judge/judge_eval/` (new) — meta-eval harness + confusion matrix
- `sims/evals/run-scenario.sh` — pass `-rubric -json`
- `sims/evals/run-eval-docker.sh` — documented gate-only TODO
- `sims/dashboard/lib/types.ts`, `lib/runs.ts`, `lib/runs.test.ts`, `app/run/[id]/page.tsx`,
  `__fixtures__/run.sample` — judge.json parse + per-criterion panel
