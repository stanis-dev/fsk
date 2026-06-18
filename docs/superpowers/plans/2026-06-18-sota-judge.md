# SOTA Judge MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans (inline) or superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an LLM rubric layer behind the existing deterministic gate in `sims/judge`, covering scenarios 05/07/10, plus a gold-set meta-eval that proves zero false-PASS, plus dashboard surfacing.

**Architecture:** The existing regex catalog runs first as a hard pre-gate. Only when it passes does a single `claude` call grade the integration source against an authored per-scenario rubric (binary MET/UNMET/CANNOT_ASSESS, evidence-required). The LLM can only add FAILs, never flip a gate FAIL to PASS.

**Tech Stack:** Go (stdlib only, module `judge`); the `claude` CLI for the model; Next.js (modified build) + Vitest for the dashboard.

## Global Constraints

- Judge model: `claude-opus-4-8` exactly (distinct tier from the coder's `claude-sonnet-4-6`; do not change the coder model). Effort `high`.
- Model invocation: `claude -p <prompt> --model claude-opus-4-8 --effort high --output-format json`.
- `judge.txt` (the judge's stdout) MUST contain the literal `conformant` or `NON-COMPLIANT` (dashboard substring scan).
- Exit codes: 0 conformant, 1 NON-COMPLIANT, 2 usage/IO/model error. No silent fallback: with `-rubric` and a rubric present, a missing/failed `claude` is exit 2, never a gate-only pass.
- Go: stdlib only, no new deps. Comments explain why, not what; docstrings on exported names only.
- No AI/Claude attribution in commits (project CLAUDE.md overrides any harness default).
- Dashboard is a modified Next.js build: read `sims/dashboard/node_modules/next/dist/docs/` before writing dashboard code (`sims/dashboard/AGENTS.md`).

---

### Task 1: Rubric type + scenario parsing + author the 3 rubrics

**Files:**
- Create: `sims/judge/rubric.go`
- Modify: `sims/scenarios/07-wrong-vat/scenario.json`, `sims/scenarios/05-outage-resilience/scenario.json`, `sims/scenarios/10-credential-expiry/scenario.json`
- Test: `sims/judge/rubric_test.go`

**Interfaces:**
- Produces: `type criterion struct { ID, Criterion, Where, Cite string }` (json tags `id`,`criterion`,`where`,`cite`); `func parseScenarioRubric(data []byte) ([]criterion, error)`; `func rubricFromScenario(path string) ([]criterion, error)` (returns nil slice + nil error when no `judge.rubric`).

- [ ] **Step 1: Write the failing test** in `rubric_test.go`:

```go
func TestParseScenarioRubric(t *testing.T) {
	data := []byte(`{"judge":{"rules":["fiskaly-host"],"rubric":[
		{"id":"c1","criterion":"does X","where":"checkout.go","cite":"SOLUTION.md"}]}}`)
	got, err := parseScenarioRubric(data)
	if err != nil { t.Fatal(err) }
	if len(got) != 1 || got[0].ID != "c1" || got[0].Criterion != "does X" {
		t.Fatalf("bad parse: %+v", got)
	}
}

func TestParseScenarioRubricEmpty(t *testing.T) {
	got, err := parseScenarioRubric([]byte(`{"judge":{"rules":["x"]}}`))
	if err != nil || got != nil { t.Fatalf("want nil,nil got %+v %v", got, err) }
}
```

- [ ] **Step 2: Run it, verify it fails** — `cd sims/judge && go test ./... -run TestParseScenarioRubric` → FAIL (undefined `parseScenarioRubric`).

- [ ] **Step 3: Implement** in `rubric.go`:

```go
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type criterion struct {
	ID        string `json:"id"`
	Criterion string `json:"criterion"`
	Where     string `json:"where"`
	Cite      string `json:"cite"`
}

func parseScenarioRubric(data []byte) ([]criterion, error) {
	var s struct {
		Judge struct {
			Rubric []criterion `json:"rubric"`
		} `json:"judge"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing rubric: %w", err)
	}
	return s.Judge.Rubric, nil
}

func rubricFromScenario(path string) ([]criterion, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading scenario: %w", err)
	}
	return parseScenarioRubric(data)
}
```

- [ ] **Step 4: Run, verify pass** — `go test ./... -run TestParseScenarioRubric` → PASS.

- [ ] **Step 5: Add `rubric` to the 3 scenario.json files** (inside the existing `judge` object, after `rules`):

07-wrong-vat:
```json
"rubric": [
  { "id": "vat-derived-from-line",
    "criterion": "Each line's VAT percentage and amount are computed from that line's LineItem.VATRate (the rate already on the order). The integration must NOT source the receipt VAT from the MenuVAT cheat-sheet map nor from any hardcoded rate such as 4%. Judge the code, not comments.",
    "where": "the fiscalization path that builds the VatRateCategory (checkout.go and any VAT helper)",
    "cite": "SOLUTION.md 07: derive VAT from LineItem.VATRate, ignore/delete MenuVAT; NOTES.md money-model" }
]
```

05-outage-resilience:
```json
"rubric": [
  { "id": "deadline-bounded",
    "criterion": "The fiskaly network call is bounded by a context deadline/timeout (context.WithTimeout / WithDeadline, or honoring a deadline already on ctx) so it cannot hang the checkout indefinitely.",
    "where": "the fiscalize path in checkout.go",
    "cite": "SOLUTION.md 05: bound the call with a deadline" },
  { "id": "no-lock-across-io",
    "criterion": "The store mutex (s.mu) is NOT held across the fiskaly network round-trip. The lock may guard in-memory Store state, but must be released before the network call. Judge the code, not comments.",
    "where": "CompleteOrder / fiscalize critical section in checkout.go",
    "cite": "SOLUTION.md 05: release the store lock before the network call" },
  { "id": "outage-legal-fallback",
    "criterion": "On a fiskaly/authority outage the code routes to the legal fallback (records the order for a deferred electronic invoice / flags the paper-document path) rather than dropping the sale or marking it completed as if fiscalized.",
    "where": "the outage/error branch of the fiscalize path",
    "cite": "SOLUTION.md 05: 12-day legal fallback (paper doc + e-invoice within 12 days)" }
]
```

10-credential-expiry:
```json
"rubric": [
  { "id": "two-distinct-lifetimes",
    "criterion": "The code treats the 24h JWT and the ~90-day Fisconline credential as different lifetimes. A daily/JWT refresh is NOT used as the answer to the 90-day expiry.",
    "where": "health.go (CredentialHealth) and the auth/token path",
    "cite": "SOLUTION.md 10: 24h JWT vs ~90-day Fisconline are different clocks" },
  { "id": "expiry-alerting",
    "criterion": "CredentialHealth tracks each taxpayer's ~90-day Fisconline credential age/expiry and surfaces an actionable signal (returns/flags at-risk taxpayers with lead time) BEFORE the credential lapses. It is not a no-op and is not wired to 'is the JWT fresh?'.",
    "where": "CredentialHealth in health.go",
    "cite": "SOLUTION.md 10: alert ops ahead of the 90-day lapse" }
]
```

- [ ] **Step 6: Commit** — `git add sims/judge/rubric.go sims/judge/rubric_test.go sims/scenarios/*/scenario.json && git commit -m "Judge: rubric type, scenario parsing, and 3 authored rubrics"`

---

### Task 2: Rubric prompt builder

**Files:** Modify `sims/judge/rubric.go`; Test `sims/judge/rubric_test.go`

**Interfaces:**
- Consumes: `criterion`.
- Produces: `func buildRubricPrompt(source string, crits []criterion) string`.

- [ ] **Step 1: Failing test**:

```go
func TestBuildRubricPrompt(t *testing.T) {
	p := buildRubricPrompt("package main // src", []criterion{{ID: "c1", Criterion: "check X", Where: "foo.go", Cite: "NOTES"}})
	for _, want := range []string{"c1", "check X", "package main // src", "MET", "UNMET", "CANNOT_ASSESS", "evidence_quote", "JSON"} {
		if !strings.Contains(p, want) { t.Errorf("prompt missing %q", want) }
	}
}
```

- [ ] **Step 2: Run, verify fail.**

- [ ] **Step 3: Implement** `buildRubricPrompt`: a prompt that (a) frames the model as a conservative conformance reviewer for an Italian fiscalization integration where a wrong PASS is dangerous; (b) lists each criterion's `id`/`criterion`/`where`/`cite`; (c) includes the integration source in a fenced block; (d) instructs: judge each criterion independently as `MET`/`UNMET`/`CANNOT_ASSESS`, default to UNMET when unsure, every `MET` must include an `evidence_quote` copied verbatim from the source (code, not comments), and reply with ONLY one JSON object `{"criteria":[{"id","verdict","evidence_quote","reasoning"}]}` and no prose. Use a raw string literal; interpolate criteria via `strings.Builder`.

- [ ] **Step 4: Run, verify pass.**

- [ ] **Step 5: Commit** — `git commit -am "Judge: rubric prompt builder"`

---

### Task 3: Tolerant model-JSON parsing

**Files:** Modify `sims/judge/rubric.go`; Test `sims/judge/rubric_test.go`

**Interfaces:**
- Produces: `type verdict struct { ID, Verdict, EvidenceQuote, Reasoning, Cite string }` (json: `id`,`verdict`,`evidence_quote`,`reasoning`,`cite,omitempty`); `func parseModelJSON(text string) ([]verdict, error)`.

- [ ] **Step 1: Failing tests** — covers a bare object, a ```json fenced block, prose-then-object, and malformed (error):

```go
func TestParseModelJSON(t *testing.T) {
	cases := []string{
		`{"criteria":[{"id":"c1","verdict":"MET","evidence_quote":"x","reasoning":"r"}]}`,
		"here:\n```json\n{\"criteria\":[{\"id\":\"c1\",\"verdict\":\"UNMET\",\"evidence_quote\":\"\",\"reasoning\":\"r\"}]}\n```\n",
		"blah {\"criteria\":[{\"id\":\"c1\",\"verdict\":\"CANNOT_ASSESS\",\"evidence_quote\":\"\",\"reasoning\":\"r\"}]} trailing",
	}
	for i, c := range cases {
		got, err := parseModelJSON(c)
		if err != nil { t.Fatalf("case %d: %v", i, err) }
		if len(got) != 1 || got[0].ID != "c1" { t.Fatalf("case %d bad: %+v", i, got) }
	}
	if _, err := parseModelJSON("no json here"); err == nil { t.Fatal("want error on no-json") }
}
```

- [ ] **Step 2: Run, verify fail.**

- [ ] **Step 3: Implement** `parseModelJSON`: strip a leading/trailing ```json fence if present, else scan for the first `{` and the matching balanced `}` (track brace depth, ignore braces inside double-quoted strings with `\` escapes), `json.Unmarshal` that substring into `struct{ Criteria []verdict }`; error if no balanced object or unmarshal fails.

- [ ] **Step 4: Run, verify pass.**

- [ ] **Step 5: Commit** — `git commit -am "Judge: tolerant model-JSON extraction"`

---

### Task 4: Citation check + aggregation

**Files:** Modify `sims/judge/rubric.go`; Test `sims/judge/rubric_test.go`

**Interfaces:**
- Consumes: `verdict`.
- Produces: `func citationCheck(vs []verdict, strippedSource string) []verdict`; `func conformant(vs []verdict) bool`.

- [ ] **Step 1: Failing tests**:

```go
func TestCitationCheckDowngradesAbsentEvidence(t *testing.T) {
	vs := []verdict{{ID: "a", Verdict: "MET", EvidenceQuote: "o.VATRate"}}
	out := citationCheck(vs, "x := o.VATRate * 100")
	if out[0].Verdict != "MET" { t.Fatal("present evidence should stay MET") }
	out = citationCheck([]verdict{{ID: "b", Verdict: "MET", EvidenceQuote: "MenuVAT[item]"}}, "x := o.VATRate")
	if out[0].Verdict != "UNMET" { t.Fatal("absent evidence must downgrade to UNMET") }
}

func TestConformant(t *testing.T) {
	if !conformant([]verdict{{Verdict: "MET"}, {Verdict: "MET"}}) { t.Fatal("all MET => conformant") }
	if conformant([]verdict{{Verdict: "MET"}, {Verdict: "UNMET"}}) { t.Fatal("any UNMET => not") }
	if conformant([]verdict{{Verdict: "CANNOT_ASSESS"}}) { t.Fatal("CANNOT_ASSESS => not") }
}
```

- [ ] **Step 2: Run, verify fail.**

- [ ] **Step 3: Implement**: `citationCheck` — for each `MET` with a non-empty `EvidenceQuote`, if `!strings.Contains(strippedSource, strings.TrimSpace(EvidenceQuote))` set `Verdict="UNMET"` and append a marker to `Reasoning` (e.g. ` [citation not found in source]`); a `MET` with an empty quote also downgrades to `UNMET` (evidence is required). `conformant` returns false if any verdict != `MET`.

- [ ] **Step 4: Run, verify pass.**

- [ ] **Step 5: Commit** — `git commit -am "Judge: citation check + conservative aggregation"`

---

### Task 5: Rubric pipeline (runRubric) with injectable model

**Files:** Modify `sims/judge/rubric.go`; Test `sims/judge/rubric_test.go`

**Interfaces:**
- Consumes: all of the above.
- Produces: `type modelFn func(prompt string) (string, error)`; `type rubricReport struct { Model string; Criteria []verdict }` (json `model`,`criteria`); `func runRubric(source, stripped string, crits []criterion, model modelFn, modelName string) (rubricReport, error)`.

- [ ] **Step 1: Failing test** with a stub model returning a fixed JSON, asserting the report carries the model name, criteria, and the cite copied from the input criterion:

```go
func TestRunRubricStub(t *testing.T) {
	crits := []criterion{{ID: "c1", Criterion: "x", Cite: "CITE1"}}
	stub := func(string) (string, error) {
		return `{"criteria":[{"id":"c1","verdict":"MET","evidence_quote":"keep","reasoning":"ok"}]}`, nil
	}
	rep, err := runRubric("keep this", "keep this", crits, stub, "claude-opus-4-8")
	if err != nil { t.Fatal(err) }
	if rep.Model != "claude-opus-4-8" || len(rep.Criteria) != 1 { t.Fatalf("bad report %+v", rep) }
	if rep.Criteria[0].Cite != "CITE1" { t.Fatal("cite must be copied from criterion") }
	if !conformant(rep.Criteria) { t.Fatal("should be conformant") }
}
```

- [ ] **Step 2: Run, verify fail.**

- [ ] **Step 3: Implement** `runRubric`: build prompt → call `model(prompt)` (return error wrapped on failure) → `parseModelJSON` → fill each verdict's `Cite` from the matching criterion by `ID` → `citationCheck(vs, stripped)` → return `rubricReport{Model: modelName, Criteria: vs}`. If the model returns verdicts whose IDs don't match the rubric, treat a missing criterion as `CANNOT_ASSESS` (conservative) by adding a synthetic verdict for any criterion ID not present in the response.

- [ ] **Step 4: Run, verify pass.**

- [ ] **Step 5: Commit** — `git commit -am "Judge: runRubric pipeline with injectable model"`

---

### Task 6: claude invocation + raw source reader

**Files:** Modify `sims/judge/rubric.go` (claudeModel, claudeArgs), `sims/judge/main.go` (readSourceRaw); Test `sims/judge/rubric_test.go`

**Interfaces:**
- Produces: `func claudeArgs(model, effort string) []string`; `func claudeModel(prompt string) (string, error)`; `func readSourceRaw(dir string) (string, error)`.

- [ ] **Step 1: Failing tests** (pure parts only — the exec is covered by the meta-eval):

```go
func TestClaudeArgs(t *testing.T) {
	a := claudeArgs("claude-opus-4-8", "high")
	joined := strings.Join(a, " ")
	for _, w := range []string{"-p", "--model claude-opus-4-8", "--effort high", "--output-format json"} {
		if !strings.Contains(joined, w) { t.Errorf("args missing %q: %v", w, a) }
	}
}
```

For `readSourceRaw`, a test writing a temp dir with one `.go` file containing a comment and asserting the comment IS retained (unlike `readSource`):

```go
func TestReadSourceRawKeepsComments(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "x.go"), []byte("package p\n// keepme\nvar X = 1\n"), 0o644)
	src, err := readSourceRaw(dir)
	if err != nil { t.Fatal(err) }
	if !strings.Contains(src, "keepme") { t.Fatal("raw reader must keep comments") }
}
```

- [ ] **Step 2: Run, verify fail.**

- [ ] **Step 3: Implement**:
  - `claudeArgs(model, effort)` → `[]string{"-p", "--model", model, "--effort", effort, "--output-format", "json"}` (prompt passed via stdin, not args, to avoid arg-length limits).
  - `claudeModel(prompt)` → `exec.Command("claude", claudeArgs("claude-opus-4-8","high")...)`, write `prompt` to stdin, capture stdout; if `claude` not found or exit != 0, return wrapped error; parse the CLI's `--output-format json` envelope and return its `result` text field (`struct{ Result string `json:"result"` }`).
  - `readSourceRaw(dir)` → like `readSource` (walk non-test `.go`) but append raw bytes (no `stripComments`).

- [ ] **Step 4: Run, verify pass.**

- [ ] **Step 5: Commit** — `git commit -am "Judge: claude CLI model fn + raw source reader"`

---

### Task 7: main.go orchestration + JSON output

**Files:** Modify `sims/judge/main.go`; Test `sims/judge/main_test.go`

**Interfaces:**
- Consumes: gate (`rule.pass`), `rubricFromScenario`, `runRubric`, `claudeModel`, `readSourceRaw`, `readSource`.
- Produces: `func buildReport(scenario string, gate []ruleResult, gatePassed bool, rep *rubricReport, verdict string) judgeReport`; `func renderRubric(rep rubricReport) string`; types `ruleResult{ID,Desc string; Pass bool}` and `judgeReport`.

- [ ] **Step 1: Failing tests** (pure helpers):

```go
func TestBuildReportVerdict(t *testing.T) {
	r := buildReport("07-wrong-vat", []ruleResult{{ID: "x", Pass: true}}, true,
		&rubricReport{Model: "claude-opus-4-8", Criteria: []verdict{{ID: "c1", Verdict: "UNMET"}}}, "NON-COMPLIANT")
	if r.Verdict != "NON-COMPLIANT" || r.Rubric == nil || r.Gate.Passed != true { t.Fatalf("bad: %+v", r) }
}

func TestRenderRubricContainsVerdict(t *testing.T) {
	s := renderRubric(rubricReport{Criteria: []verdict{{ID: "c1", Verdict: "UNMET", Reasoning: "because"}}})
	if !strings.Contains(s, "UNMET") || !strings.Contains(s, "c1") { t.Fatal("render missing fields") }
}
```

- [ ] **Step 2: Run, verify fail.**

- [ ] **Step 3: Implement**:
  - Add flags: `rubricFlag := flag.Bool("rubric", false, ...)`, `jsonFlag := flag.String("json", "", ...)`.
  - Add types `ruleResult` and `judgeReport{ Scenario string; Gate struct{Passed bool; Rules []ruleResult}; Rubric *rubricReport; Verdict, Note string }` with json tags.
  - Refactor the gate loop to also collect `[]ruleResult`; keep printing the existing PASS/FAIL lines.
  - After the gate: if any fail → `verdict="NON-COMPLIANT"`, print existing VERDICT line, write json if `-json`, exit 1.
  - If gate passes: if `*rubricFlag` and `rubricFromScenario` returns a non-empty rubric → read raw source, `runRubric(raw, strippedSrc, crits, claudeModel, "claude-opus-4-8")`; on error print to stderr and exit 2; print `renderRubric(rep)`; `conformant` decides verdict. Else verdict `conformant`.
  - Print the final `VERDICT: conformant` / `VERDICT: NON-COMPLIANT (...)` line (unchanged literal tokens). Write json. Exit 0/1.
  - `buildReport` assembles `judgeReport`; `renderRubric` formats a `RUBRIC` text block; write json via `os.WriteFile(*jsonFlag, json.MarshalIndent(report), 0o644)`.

- [ ] **Step 4: Run, verify pass** — `go test ./...` (all judge tests). Also `go run . -list` still works (regression).

- [ ] **Step 5: Commit** — `git commit -am "Judge: main orchestration, -rubric/-json flags, judge.json output"`

---

### Task 8: Gold-set fixtures (good + bad) for 05/07/10

**Files:** Create `sims/judge/testdata/goldset/{05-outage-resilience,07-wrong-vat,10-credential-expiry}/{good,bad}/integration.go`

**Interfaces:** none (data). Each fixture is a single-file Go integration. Both `good` and `bad` in a scenario MUST contain the tokens that satisfy that scenario's deterministic rules so they pass the gate; they differ only in the rubric-relevant behavior.

Required gate tokens per fixture:
- All 6 fixtures: `https://test.api.fiskaly.com`, `/tokens`, `X-Idempotency-Key`, `X-Api-Version`, `/records`.
- 07 good+bad additionally: the quoted JSON keys `"percentage`, `"amount`, `"exclusive`, `"inclusive` (the `vat-breakdown` rule).

Rubric-distinguishing content:
- **07 good**: builds the breakdown from `line.VATRate` (e.g. `pct := line.VATRate`). **07 bad**: builds it from `MenuVAT[line.Name]` / a literal `4` (e.g. `pct := MenuVAT[line.Name] // 4%`).
- **05 good**: `ctx, cancel := context.WithTimeout(...)`, releases `s.mu` (`s.mu.Unlock()`) before the HTTP call, and an outage branch that flags the deferred e-invoice / paper fallback. **05 bad**: HTTP call inside `s.mu.Lock()`/`defer s.mu.Unlock()` with no timeout and no fallback.
- **10 good**: a `CredentialHealth` that stores a per-taxpayer `expiresAt`/90-day field and returns at-risk taxpayers with lead time, distinct from JWT refresh. **10 bad**: `CredentialHealth` that only checks JWT freshness / daily refresh and never tracks the 90-day clock.

- [ ] **Step 1: Write the 6 fixtures** (compact, plausible Go; they need not compile — the judge only reads source).
- [ ] **Step 2: Build the judge binary** — `cd sims/judge && go build -o judge .`
- [ ] **Step 3: Assert every fixture passes the deterministic gate** (proves the gap the rubric closes):

```bash
for s in 05-outage-resilience 07-wrong-vat 10-credential-expiry; do
  for v in good bad; do
    ./judge -scenario ../scenarios/$s/scenario.json testdata/goldset/$s/$v \
      >/dev/null 2>&1 && echo "$s/$v gate=PASS" || echo "$s/$v gate=FAIL (fix tokens)"
  done
done
```
Expected: all 6 print `gate=PASS`.

- [ ] **Step 4: Commit** — `git add sims/judge/testdata && git commit -m "Judge: gold-set fixtures (good/bad) that all pass the deterministic gate"`

---

### Task 9: Meta-eval harness (confusion matrix, zero false-PASS gate)

**Files:** Create `sims/judge/judge_eval/main.go`

**Interfaces:** standalone `package main` program; builds/uses the judge binary, runs `-rubric` over each gold fixture, compares verdict to expected (`good`→conformant, `bad`→NON-COMPLIANT), prints a confusion matrix, exits non-zero if any false-PASS (a `bad` judged conformant) or any error.

- [ ] **Step 1: Implement** `judge_eval/main.go`: a table of `{scenario, variant, expectConformant}` for the 6 fixtures; for each, run `judge -rubric -scenario <s> testdata/goldset/<s>/<variant>` (reuse the built binary at `../judge` or `go run .` in the judge dir), read exit code (0=conformant, 1=NON-COMPLIANT, 2=error). Optionally run each `bad` fixture `N=3` times and require zero conformant across all reps (nondeterminism guard). Tally a 2x2 matrix (expected x actual); print it; print the false-PASS count; `os.Exit(1)` if false-PASS > 0 or any exit-2.

- [ ] **Step 2: Run it** — `cd sims/judge/judge_eval && go run . ` (requires `claude` authenticated). Expected: zero false-PASS; every `bad` caught with a cited UNMET; matrix printed.

- [ ] **Step 3: Commit** — `git add sims/judge/judge_eval && git commit -m "Judge: meta-eval harness with confusion matrix and zero-false-PASS gate"`

---

### Task 10: Harness wiring

**Files:** Modify `sims/evals/run-scenario.sh` (line ~109), `sims/evals/run-eval-docker.sh` (line ~99)

- [ ] **Step 1:** In `run-scenario.sh`, change the judge invocation to pass `-rubric -json "$run_dir/judge.json"` (keep `-scenario "$scenario_json" "$work"` and the `>"$run_dir/judge.txt" 2>&1` redirect + `&& judge=PASS || judge=FAIL`).
- [ ] **Step 2:** In `run-eval-docker.sh`, leave the judge invocation gate-only but add a one-line comment: `# MVP: rubric layer runs only in run-scenario.sh; Docker stays gate-only until the OAuth token + claude are wired into the container.`
- [ ] **Step 3: Verify** the edited line with `grep -n 'judge' sims/evals/run-scenario.sh`.
- [ ] **Step 4: Commit** — `git commit -am "Evals: run the rubric judge + emit judge.json in run-scenario.sh"`

---

### Task 11: Dashboard surfacing (full integration)

**Files:** Modify `sims/dashboard/lib/types.ts`, `sims/dashboard/lib/runs.ts`, `sims/dashboard/lib/runs.test.ts`, `sims/dashboard/app/run/[id]/page.tsx`, `sims/dashboard/__fixtures__/run.sample/` (add `judge.json`)

- [ ] **Step 1: Read** `sims/dashboard/node_modules/next/dist/docs/` for any relevant guidance, and re-read `lib/runs.ts` / `lib/types.ts` / `app/run/[id]/page.tsx` to match existing patterns.
- [ ] **Step 2: Add types** to `types.ts`:

```ts
export interface JudgeCriterion {
  id: string;
  verdict: "MET" | "UNMET" | "CANNOT_ASSESS";
  evidence_quote: string;
  reasoning: string;
  cite: string;
}
export interface JudgeReport {
  scenario: string;
  verdict: "conformant" | "NON-COMPLIANT";
  gate: { passed: boolean; rules: { id: string; desc: string; pass: boolean }[] };
  rubric: { model: string; criteria: JudgeCriterion[] } | null;
  note: string;
}
```

- [ ] **Step 3: Write a failing test** in `runs.test.ts` for a `parseJudgeReport(json: string): JudgeReport | null` (returns null on absent/garbage) and that RunDetail exposes `judgeReport` when `judge.json` is present in the fixture.
- [ ] **Step 4: Run** `cd sims/dashboard && pnpm test` (or the project's test cmd) → FAIL.
- [ ] **Step 5: Implement** `parseJudgeReport` in `runs.ts`, load `judge.json` from the run dir alongside the existing `judge.txt` read, attach as `RunDetail.judgeReport` (keep the `judge.txt` substring scan as the verdict source of truth).
- [ ] **Step 6: Run tests** → PASS.
- [ ] **Step 7: Render** a per-criterion panel in `app/run/[id]/page.tsx` (verdict chip MET/UNMET/CANNOT_ASSESS, evidence quote in mono, reasoning, cite), shown only when `judgeReport?.rubric` exists. Add a `judge.json` to the `__fixtures__/run.sample` so the detail page renders it.
- [ ] **Step 8: Build** — `pnpm build` (or project build) to confirm the modified Next.js compiles.
- [ ] **Step 9: Commit** — `git add sims/dashboard && git commit -m "Dashboard: parse + render per-criterion judge report"`

---

## Self-Review

- **Spec coverage:** gate-first (T7), rubric type/parse (T1), prompt (T2), parse (T3), citation+aggregate (T4), pipeline (T5), claude+raw (T6), outputs judge.txt/judge.json (T7), 3 rubrics (T1), harness wiring (T10), meta-eval gold set + matrix + zero-false-PASS (T8,T9), dashboard (T11), limitations/Docker note (T10). All spec sections map to a task.
- **Placeholders:** none — concrete code, rubric JSON, fixture token lists, and commands throughout.
- **Type consistency:** `criterion`, `verdict`, `modelFn`, `rubricReport`, `ruleResult`, `judgeReport` defined once and reused; field names (`evidence_quote`, `verdict`, `criteria`, `model`) match between Go json tags and the TS interfaces.

## Verification gate (this turn)

The MVP is "done" only when `sims/judge/judge_eval` reports **zero false-PASS**, every `bad` fixture is caught with a cited UNMET, all 6 fixtures pass the deterministic gate, `go test ./...` in `sims/judge` is green, and the dashboard builds.
