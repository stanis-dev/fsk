# Deterministic and LLM-Judged Checks for the fiskaly SIGN IT Eval Harness

Scope: the agentic-coding eval system under `/Users/stan/code/fsk/sims/` — the runner (`evals/run-scenario.sh`), the deterministic judge (`judge/main.go`, 13 rules), the grounding check (`evals/assert-grounded.sh`), the docs MCP server (`mcp/`), and the 10 scenarios in `scenarios/`. Ground truth for fiskaly facts is `research/api-probes/NOTES.md` (live TEST probe, `X-Api-Version: 2026-02-03`).

Two corrections carried through from verification, stated up front because they change what you should build:

- **`vat-breakdown` does not catch scenario 07's trap.** Confirmed in `judge/main.go:142`: the rule matches the four quoted keys `"percentage`/`"amount`/`"exclusive`/`"inclusive`. It proves the breakdown is *constructed*, not that the *rate value* is correct. An agent that ships `"percentage":"4"` with all four fields present passes the gate and is still wrong. Scenario 07 (wrong-rate) is **rubric-only today and has no implemented check at all.**
- **Money is decimal strings, not integer cents.** `NOTES.md` line 39: `^(-)?\d{1,12}(\.\d{1,8})?$`. The generic "use integer cents" best practice is *wrong* for this API. Any money check must assert the decimal-string pattern and flag float JSON numbers. (Low-risk to state; ground truth is live-probed.)

One naming note for anyone wiring CI: the on-disk directory is `08-amounts-as-strings` (matches `scenario.json` `id`); the README/AUTHORING call it `08-amounts-decimal-strings`. Use the on-disk name in scripts.

---

## 1. The four check layers, and when each is appropriate

| Layer | What it is | Cost / variance | Appropriate when |
|---|---|---|---|
| **Deterministic gate** | Exact pass/fail with zero run-to-run variance given fixed input bytes: compile, test exit code, regex-over-source conformance, patch-applies, format, name-charset. | Cheapest (~free), exact. | The property is structurally decidable from artifacts. Run first; short-circuits everything downstream. The repo's `go build` / `go test` / judge / `assert-grounded` all live here. |
| **Static analysis** | Source-level inspection beyond compile: lint, `go vet`, SAST (gosec/semgrep), govulncheck, license, mutation, the judge's regex pass, annotation-honesty. | Cheap, deterministic. | Quality/safety signals that don't need execution. Distinct from the gate in that findings are usually advisory thresholds, not hard binaries (except where you choose to gate). |
| **Harness metric** | Measured-not-judged signals derived from the run: turns, cost, latency, MCP-usage telemetry, grounding order, isolation invariants, pass@k aggregation, baseline-invariant assertions. | Cheap, deterministic to extract; variance comes only from the *agent*. | Operational tracking, reliability estimation, and behavioral honesty checks that are decidable from the transcript. |
| **LLM judge** | Model-graded rubric over diff + transcript for semantic substance the gate structurally cannot see: idempotency-key lifecycle, blocking-vs-async, rate correctness, credential conflation, faithfulness, hallucination. | Expensive, nondeterministic — needs temperature control + repeat/majority-vote. | Only the "review-caught" tier, and only **after** deterministic gates pass. Never as a first cut; never for a property a regex already decides. |

The ordering contract is **deterministic-gate → static → harness telemetry → LLM judge**, fail-fast. A red build must skip the judge and the rubric; a `deny`-rule that already proves an invented `/refunds` decides the verdict without a model call. This is the 2026 "deterministic gates the judge" consensus and is what the repo already half-implements.

---

## 2. Taxonomy of checks

`Already in repo` legend: **yes** = wired and running; **partial** = a weaker form exists; **no** = absent. "Layer" uses the four above (harness = harness metric).

### Standard checks (apply regardless of deterministic vs. non-deterministic)

| Check | Layer | What it verifies | How applied (artifact/signal) | Already in repo (gap) |
|---|---|---|---|---|
| Compile / build | gate | Diff compiles; for Go this also **is** the typecheck | `go build ./...` exit code → `build.txt` (`run-scenario.sh:97`) | **yes** |
| Test gate (F2P + P2P combined) | gate | Target behavior works AND no prior-green test regressed | `go test ./...` exit code → `test.txt` (`:98`) | **partial** — coarse all-or-nothing; no per-test FAIL_TO_PASS / PASS_TO_PASS status parsing (no named target tests) |
| Patch applies | gate | Agent output is a well-formed patch that applies cleanly | SWE-bench `git apply` fallback chain | **no** — N/A by design: agent edits in-place via Write/Edit; diff reconstructed after via `git diff --cached` (`:100`). An unapplyable patch is structurally impossible here |
| Patch minimality / anti-gaming | hybrid | Diff scoped to the fix; no test-file tampering | `changes.diff` artifact; judge `readSource` excludes `*_test.go` (anti-gaming) | **partial** — minimality is a SOLUTION.md review bar, not a numeric gate |
| Format | gate | Canonically formatted | `gofmt -l .` non-empty = fail | **no** |
| Lint | static | Unused vars, unchecked errors, ineffassign, staticcheck | `golangci-lint run ./...` (v2 default set merged gosimple/stylecheck into staticcheck; exact 5-member set *not verbatim-confirmed* — **low confidence**, verify with `golangci-lint help linters`) | **no** |
| Static analyzers (`go vet`) | static | Printf/format mismatches, lost struct tags, copied locks | `go vet ./...` exit code | **no** |
| Security scan (gosec / semgrep) | static | Hardcoded creds, weak crypto, injection (gosec: 50+ CWE/OWASP rules, AST+SSA — *verified*) | `gosec ./...` | **no** — harness withholds real fiskaly creds from the coder (hygiene, not SAST) |
| Dependency vuln scan | static | Reachable known-vuln deps | `govulncheck ./...` | **no** — modules are stdlib-only, so near-empty regardless |
| License check | static | Allow-listed licenses only | `go-licenses check ./...` | **no** — stdlib-only, near-empty |
| Coverage delta | harness | New logic is exercised | `go test -coverprofile` + threshold | **no** — `go test` runs with no `-cover` |
| Mutation testing | static | Test-suite *strength* (do tests kill mutants) | `gremlins unleash` threshold | **no** |
| Fuzzing | gate | Robustness to malformed input | `go test -fuzz` | **no** — no fuzz targets |
| Golden / snapshot | gate | Output byte-matches checked-in golden | runs under `go test` when present | **no** — none authored |
| Efficiency (turns / cost / latency) | harness | Resource cost of the run | `jq` over final `result` event → summary (`:112-116`) | **yes** — measured, **not gated** against a threshold |
| Self-consistency / variance control | harness | Stability across repeated samples | sample N, majority-vote, order-swap | **no** — every run is N=1; deterministic layers don't need it, but the LLM tier will |
| pass@k / pass^k aggregation | harness | Reliability across k attempts (pass^k = robustness vs. silent traps) | run each scenario k times, aggregate | **no** |
| Agent variance pinning | harness | Model + effort pinned (no temp/seed flag exists in headless) | `--model claude-sonnet-4-6`, `--effort` (low/med/high/xhigh/max; *max is session-only*) (`:88-93`) | **yes** |
| Tool-call correctness (right tool + right args) | hybrid | Call happened with correct args, not just a token present | trajectory match vs. expected tool calls (DeepEval ToolCorrectnessMetric is name-only/deterministic; **ArgumentCorrectnessMetric** is the dedicated arg-match scorer) | **partial** — judge token-presence approximates this without verifying args/back-refs |
| Trajectory / process eval | hybrid | Step sequence is correct (order, no extras) | match modes vs. reference trajectory, or LLM-judged | **partial** — `assert-grounded.sh` is a minimal order check |
| Task completion / goal accuracy | llm-judge | End goal actually achieved (outcome, not process) | LLM judge over diff+final state | **partial** — approximated by build+test+judge+grounded |
| Groundedness / faithfulness | llm-judge | Code reflects facts that only appear in fetched docs | claim-vs-context entailment over fetched snippets + diff | **no** — grounding check is order-only |
| Hallucination detection | llm-judge | No invented API/domain facts contradicting ground truth | LLM compares diff to trusted context (NOTES.md) | **no** — only token-shaped inventions caught (deny rules) |
| Citation / attribution | llm-judge | Each load-bearing decision traces to the doc that justifies it | map decision → cited corpus chunk | **no** — **low-medium confidence** as a single named scorer; usually a rubric |
| Rubric LLM-as-judge (pointwise) | llm-judge | Custom-criteria semantic grade | G-Eval / DAG over diff+transcript keyed to SOLUTION.md | **no** — SOLUTION.md rubrics authored, graded by human today |
| Pairwise / reference-based | llm-judge | A/B model comparison; factuality vs. gold | order-swap + average; factuality grader vs. NOTES.md | **no** — useful for comparing models in `meta.json` |
| Answer relevance | llm-judge | Output addresses the ask | referenceless relevancy | **no** — weak fit (tasks produce code, not prose) |
| Refusal / safety | llm-judge | No harmful/off-policy output, no over-refusal | safety rubric / red-team | **no** — low relevance unless prompt-injection-via-docs scenarios are added |

### MCP-server protocol checks (standard, but specific to the docs MCP this repo ships)

These grade the **server** the agent grounds against, not the agent. The entire layer is unbuilt as gates; only Go unit tests exist (`mcp/server_test.go`, `tools_test.go`, `corpus/search_test.go`). This is the **largest unbuilt family by count.** Spec basis: MCP revision 2025-11-25; go-sdk v1.2.0.

| Check | Layer | What it verifies | How applied | Already in repo (gap) |
|---|---|---|---|---|
| Tool-name conformance | gate | Names unique, 1-128 chars, charset `[A-Za-z0-9_.-]` | regex over `tools/list` | **no** — names are legal, nothing enforces it |
| inputSchema is valid JSON Schema | static | Each `inputSchema` is a valid object (2020-12 default) | meta-validate every schema from `tools/list` | **no** — SDK derives schemas; unvalidated |
| outputSchema / structuredContent conformance | static | `structuredContent` conforms to declared `outputSchema`; TextContent mirror present | call each tool, validate result envelope | **partial** — tests string-match result text only |
| Lifecycle handshake + version negotiation | static | `initialize` returns protocolVersion + capabilities + serverInfo; negotiates unsupported versions | drive raw stdio handshake | **no** |
| tools/list capability + completeness | static | Exactly the advertised tool set; `tools` capability declared | assert set + capability after handshake | **partial** — `TestServerListsBothTools` checks names only |
| Input-validation → isError | static | Bad args surface as `isError:true` result, not a throw | empty query / unknown id | **partial** — handler-level tested, not protocol-level |
| Error-channel discipline | static | Unknown-tool = JSON-RPC error; bad-args = `isError` result | unknown tool name vs. bad args | **partial** — no unknown-tool-name protocol-error test |
| No JSON-RPC batching | static | Server rejects batch arrays. **NOTE — corrected:** batching was *added* 2025-03-26 and *removed* 2025-06-18; still removed in 2025-11-25 (**low confidence** on wire behavior; not exercised) | send batch array, assert rejection | **no** |
| read-only annotation honesty | static | `readOnlyHint:true`/`openWorldHint:false` match actual handler behavior (embedded corpus, no net/fs) | annotation vs. source | **no** |
| Retrieval quality (Recall@k / Precision@k / MRR) | static | `search_fiskaly_docs` returns the doc the flipping rule needs | labeled query→doc-id gold set over `index.json` | **partial** — `search_test.go` checks ranking on a 3-doc stub only |
| Snippet / citation fidelity | hybrid | Snippet is a substring of source; id round-trips through fetch | substring + round-trip (deterministic); on-topic (judge) | **partial** — only non-empty checked |
| Tool-description selection quality | llm-judge | Descriptions let a model pick the right tool and chain id→fetch | tool-selection eval | **no** — **medium confidence** (inherently model-judged) |

### Fiskaly-specific checks (map 1:1 to the SIGN IT contract and the 10 traps)

Citations are to `NOTES.md` facts. "Gate" = the judge can token-match it; "rubric/behavioral" = it needs the diff/transcript or an LLM.

| Check | Layer | What it verifies | How applied (artifact/signal) | Already in repo (gap) |
|---|---|---|---|---|
| fiskaly host (test/live) | gate | Targets `test\|live.api.fiskaly.com`, not invented host; **TEST not LIVE** | judge `fiskaly-host` (base) | **yes** |
| Token exchange | gate | Auth via `POST /tokens` | judge `token-exchange` (base) | **yes** |
| JWT under `content.authentication` | hybrid | Reads JWT from the right field, not `access_token` | rubric / behavioral over diff | **no** |
| `X-Idempotency-Key` present | gate | Header appears on writes | judge `idempotency-key` (base) | **yes** (presence only) |
| Idempotency key **lifecycle** (sc04) | hybrid | Fresh key per distinct POST; **same** key on retry | rubric over diff (judge cannot count keys) | **no** — the actual sc04 acceptance bar; no mechanism specified |
| `X-Api-Version` present | gate | Dated header sent | judge `api-version` (base) | **yes** |
| `X-Api-Version` current = `2026-02-03` (sc09) | gate | Header AND current date together | judge `api-version-current` | **yes** |
| Two-call `/records` flow | gate | Issues via `/records`, not single `/receipts` | judge `records-flow` (base) | **yes** (presence; doesn't prove two calls) |
| Two-call back-reference (sc01/03) | hybrid | TRANSACTION `record.id` = INTENTION id | rubric (judge matches `CANCELLATION`/flow token, not the back-ref) | **partial** |
| Poll to FINISHED (sc06) | hybrid | Polls record to terminal `FINISHED`; LIVE is async | judge `polling` (token) + rubric (actual GET loop, gate completion on it) | **partial** — token caught; loop/gating is rubric |
| Cancellation by reference; no `/refunds`, no DELETE (sc03) | hybrid | CANCELLATION record refs original; no invented endpoint | judge deny `no-invented-refunds` + `cancellation-ref` | **yes** (deny is solid; back-ref is rubric) |
| No legacy `/entities` / `/assets` (sc09) | gate | Current resource names only | judge deny `no-legacy-resources` | **yes** |
| Scope-identifier present (sc02) | gate | `X-Scope-Identifier` used | judge `scope-identifier` | **yes** (presence) |
| Scoped-subject **sequence** + 405 (sc02) | hybrid | UNIT org + scoped subject + UNIT token **before** `POST /taxpayers`; GROUP-token taxpayer = 405 | rubric/behavioral (judge can't see order) | **partial** — only header presence |
| Commissioning present (sc02) | gate | `COMMISSIONED` state reached | judge `commissioning` | **yes** (token) |
| Commissioning **order** | hybrid | taxpayer→location→system PATCH order | rubric (judge can't see order) | **no** |
| VAT four-field breakdown constructed (sc07/08) | gate | All of `percentage`/`amount`/`exclusive`/`inclusive` as JSON keys | judge `vat-breakdown` | **yes** |
| VAT **rate correctness** (sc07) | llm-judge | The chosen percentage is legally right for the product | rubric over diff (the API can't validate the rate) | **no** — **the sc07 trap; `vat-breakdown` does NOT cover it** |
| VAT cent-reconciliation (sc08) | hybrid | Per-line nets/grosses/VAT sum to `total_vat` to the cent | rubric (judge can't do arithmetic) | **no** |
| Money = decimal strings (sc08) | gate | Matches `^(-)?\d{1,12}(\.\d{1,8})?$`; reject float JSON / int-cents | log/diff regex (**contradicts the generic brief; NOTES.md wins**) | **no** |
| `compliance.data` + `compliance.url` on RECEIPT | gate | Terminal RECEIPT carries the AdE document reference (proof it reached the authority) | response assertion / diff | **no** — high-value for the "looks like success" threat model |
| Blocking-vs-async checkout (sc05) | llm-judge | Fiscalization is decoupled/timeout-bounded; till never frozen; 12-day fallback | rubric over diff | **no** — sc05 trap, only in the catch-all rubric |
| Credential lifetime: 24h JWT vs 90-day Fisconline (sc10) | llm-judge | Re-auth on JWT expiry ≠ credential rotation; no conflation | rubric over diff/transcript | **no** — sc10 trap, only in the catch-all rubric |
| Grounded: search-before-mutate | gate | A `search_fiskaly_docs` call precedes the first Write/Edit | `assert-grounded.sh` → `grounded.txt` (`:103`) | **yes** (order only) |
| Grounding **evidence used** | llm-judge | The fetched doc's facts actually shaped the code | LLM over fetched snippets + diff | **no** |
| MCP-usage telemetry + retrieve→resolve link | harness | Search/fetch counts, queries, doc-ids; did the agent fetch the doc the flipping rule **cites** before editing | extractor over `transcript.jsonl` → `telemetry.json`; cross-ref `rule.cite` | **partial** — only `tool_use` parsed for display; no structured record |
| Strict-MCP / clean-HOME isolation | harness | Docs MCP is the sole knowledge source; creds withheld | `--strict-mcp-config` + empty `$HOME` (`run-scenario.sh`) | **yes** |
| Hermetic Docker isolation | harness | No repo/research/SOLUTION.md reachable; pinned toolchain | `run-eval-docker.sh`, pinned image | **partial** — no `--network none`; Docker variant skips grounded/telemetry |
| Baseline-invariant assertion | harness | Every seed builds+tests green AND judges NON-COMPLIANT (≥1 selected rule fails) so a fix has something to flip | judge each fixture with no agent vs. README baseline table | **partial** — documented, not asserted in CI |
| Negative-regression (deny-rule trap) | gate | A *wrong* fix flips a passing deny-rule to FAIL | deny rules `no-invented-refunds`, `no-legacy-resources` | **yes** |
| Judge: one core, two adapters | static | CI-gate bytes identical to the agent's in-loop self-check | extract `Judge(dir, ruleIDs)→Report`; CLI adapter + MCP-tool adapter | **partial** — CLI/`go run` only; MCP-tool entrypoint not built |

---

## 3. Application architecture

### 3.1 Layering and fail-fast order

Run cheapest/highest-precision first; each layer can short-circuit the rest:

1. **Preflight (harness, ~free):** tools on PATH, scenario files present, baseline commit, MCP binary built (`run-scenario.sh:42-44, 58-60, 67`). A failure here is a **harness error (exit 2)**, kept distinct from a NON-COMPLIANT verdict so a broken harness never reads as a model regression.
2. **Build gate** (`go build ./...`): red build → emit `BUILD-FAIL`, skip everything downstream.
3. **Test gate** (`go test ./...`): the PASS_TO_PASS analog — seeds are authored green, so a failure is a real regression.
4. **Static conformance judge** (`judge -scenario scenario.json`): the FAIL_TO_PASS analog — a baseline rule engineered to fail must flip to PASS. Includes the negative deny-rule trap.
5. **Harness telemetry** (host-side, deterministic): grounding order + MCP-usage `telemetry.json`; cost/turns/agent_error.
6. **LLM rubric** (expensive, last): only for **review-caught** scenarios, only when 2-4 pass. The gate-caught vs. review-caught field in each `SOLUTION.md` is the **routing key** for whether the model call fires at all.

Anti-patterns to avoid: gating only on layer 4 ships semantic regressions (04/05/07/10 silent bugs are invisible to regex); gating only on layer 6 inherits judge nondeterminism and a five-figure-at-scale bill. The repo's deterministic-heavy instinct is correct.

### 3.2 Deterministic-gate-then-judge

The judge stays one **pure deterministic core** with two thin adapters so the bytes deciding a CI verdict are byte-identical to what an agent self-checks against mid-task:

- Extract `Judge(dir, ruleIDs) → Report` from `main.go` (already pure: no net/time/randomness, proven by `TestReadSourceExcludesTests` / `TestDenyRuleIgnoresComments`).
- CLI `main` = the **gate adapter** (exit 0/1/2).
- A new `judge_conformance` tool on the existing `mcp/` server = the **in-loop adapter** (returns the structured `Report`), letting the agent run conformance on itself before finishing. This is the **dual MCP-tool + CI-gate surface** the brief asks for; today only the CLI path exists.

### 3.3 Variance control for non-deterministic checks

The deterministic layers (build/test/judge/grounded/telemetry) have **zero** run-to-run variance for fixed bytes. All variance is the *agent* producing different diffs, plus any LLM judge.

- **Agent:** pin `--model claude-sonnet-4-6` and `--effort` (both done; `:88-93`). There is **no temperature/seed flag in headless mode**, so true agent determinism is unachievable — repeat-run aggregation is the only honest control. Sample n ≥ 2k-4k completions per task for stable pass@k.
- **LLM judge:** agent at temp 0; judge at ~0.3 for nuanced grading; **repeat the judge call an odd N (3-5) and take majority vote**; gate on an explicit threshold, not a free-form score; swap pair order for any A/B. Self-consistency becomes **mandatory** the moment the rubric layer is added.

### 3.4 Scoring / aggregation to an exit code

- **Keep rule-level scoring binary.** Do NOT add partial-credit weighting inside the judge — these are *necessary conditions*; a "4/5" would imply an integration that omits idempotency is 80% correct, which is false. Verdict = conformant iff all selected rules pass → exit 0; any fail → exit 1; usage/IO → exit 2.
- **Weighting belongs one level up:** the deterministic verdict is the hard binary gate; the LLM rubric produces a graded score for review-caught *substance* (cent-reconciliation, key-lifecycle).
- **Suite roll-up** reports per-scenario `{build, tests, judge, grounded, rubric}` and — critically — **separates gate-caught pass-rate from review-caught pass-rate**, because they carry different reliabilities (gates exact; rubric carries judge variance).
- **pass@k for capability, pass^k for robustness** on silent-bug scenarios (you care that the agent *reliably* avoids the trap, not that it once can).

### 3.5 Regression gating

The seed invariant (`AUTHORING.md`) is the SWE-bench resolution criterion in this domain: every seed builds+tests green **and** judges NON-COMPLIANT, so "conformant" is non-vacuous. To make it a real CI gate:

1. **Assert the baseline in CI** — run the judge against each fixture with **no agent** and assert the recorded baseline pass/total (README table; `scenario.json` baseline/target where present, e.g. 06/08). A scenario that silently drifts to conformant (a dead trap) must fail CI loudly. **This is the single highest-leverage missing harness check for trap integrity.**
2. Gate agent runs on **pass@k over a fixed k**, not a single run, because the agent is nondeterministic.

### 3.6 Telemetry

`transcript.jsonl` (stream-json, one ordered event per line) is the ideal source. Promote the current incidental parsing to a structured per-run `telemetry.json` next to `judge.txt`:

- search/fetch call counts, queries issued, doc-ids fetched, search-before-mutate (existing grounded signal);
- the highest-value derived signal — **did the agent fetch the doc that the flipping rule `cite`s before editing the relevant code** (a deterministic retrieve→resolve link, computable because the corpus is local and each rule carries a `cite`).
- Align attribute names to OpenTelemetry GenAI (`execute_tool`, `gen_ai.tool.name`) but treat the schema as **directional, not frozen** (those attributes are Development-stability — **medium confidence**).

### 3.7 Dual MCP-tool + CI-gate surface

Already covered by 3.2 for the judge. Extend the same principle to the MCP-server conformance family: build a small stdio conformance runner (lifecycle handshake, name/schema/output conformance, error-channel discipline, retrieval Recall@k/MRR against a labeled gold set) that runs in CI against the built `mcp/` binary, and reuse the corpus gold set as a regression guard whenever `index.json` or the BM25 weights change.

---

## 4. Gap analysis and prioritized shortlist

### What this repo already has (solid)

Build gate; test gate (coarse); static conformance judge with 13 binary rules incl. two negative deny-rule traps; grounding order check; strict-MCP + clean-HOME isolation; diff capture against a baseline commit; cost/turns telemetry; pinned model+effort; Docker hermetic variant; the gate-caught/review-caught design split with authored SOLUTION.md rubrics. The deterministic spine is strong and correctly scoped to "necessary, not sufficient."

### What's missing, highest-impact-lowest-effort first

**Tier A — cheap, deterministic, closes correctness or integrity holes:**

1. **Baseline-invariant CI assertion** (harness; ~1 script). Judge every fixture with no agent, assert README baseline pass/total. Protects every trap from silent death. *Highest leverage, lowest effort.*
2. **`compliance.data` + `compliance.url` on RECEIPT** (gate). Direct hit on the core threat model ("a bug looks like success" = no AdE reference). A new judge rule + response assertion.
3. **Money decimal-string regex** (gate, sc08). Cheap; corrects the brief's int-cents error; flags float JSON.
4. **Judge as MCP tool** (extract `Judge()`, add `judge_conformance`). Unlocks in-loop self-check; pure refactor of already-pure code.
5. **Structured MCP-usage `telemetry.json` + retrieve→resolve link** (harness). Deterministic, the brief's stated highest-value signal; data already in `transcript.jsonl`.
6. **`--network none` on the Docker variant + run grounded/telemetry on host for parity** (harness). Corpus MCP is offline, so this is near-free hermeticity.

**Tier B — deterministic, modestly more work:**

7. **VAT cent-reconciliation** (sc08) and **two-call back-reference / scoped-subject sequence / commissioning order** (sc02) as behavioral checks over a request log or diff. The judge's presence rules can't see arithmetic or ordering; these are the actual acceptance bars.
8. **Format / `go vet` / lint** gates. Cheap CI hygiene; not in the core resolved metric but standard.
9. **MCP-server conformance runner** (lifecycle, name/schema/output conformance, error-channel discipline) + **retrieval gold set** (Recall@k/MRR over the real 5-section `index.json`). Largest unbuilt family; validates the surface the whole eval depends on.

**Tier C — non-deterministic, build last with variance control:**

10. **LLM rubric judge** over diff+transcript keyed to each `SOLUTION.md`, gated to run only when deterministic layers pass and only for review-caught scenarios (04 key-lifecycle, **05 blocking-checkout**, **07 wrong-rate**, 08 string-vs-float, **10 credential conflation**). 05/07/10 currently have **no implemented check of any kind** — they live only in the human catch-all. This is the natural automation of the existing rubric tier and the biggest semantic-coverage gain.
11. **pass@k / pass^k aggregation** + **self-consistency / majority-vote** on the rubric. Mandatory the moment (10) exists; turns every N=1 verdict into a reliability estimate.
12. **Grounding evidence-used** and **faithfulness/hallucination** judges. Higher cost, overlap with (10); defer until the rubric harness and its variance controls are proven.

### Caveats carried inline

- `vat-breakdown` is **presence-only** — do not claim it catches sc07's wrong-rate trap (verified at `judge/main.go:142`).
- Money is **decimal strings**, not int-cents — NOTES.md (live-probed) overrides the generic brief.
- The MCP no-batching check's removal date in the source dossier was **inverted**; correct is added 2025-03-26 / removed 2025-06-18 (low confidence on wire behavior — verify before building).
- golangci-lint v2 exact default linter set is **not verbatim-confirmed** this session.
- Scenario 08's on-disk dir is `08-amounts-as-strings`; use that in scripts, not the README's `08-amounts-decimal-strings`.
