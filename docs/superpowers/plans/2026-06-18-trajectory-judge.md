# Trajectory-aware Judge Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the judge's source-only model (regex gate + source rubric) with a trajectory-aware one: deterministic checks over the run's transcript + MCP telemetry (the new gate), plus single-pass LLM expectations evaluated over the trajectory.

**Architecture:** The Go judge (`sims/judge`) gains trajectory parsing and a bounded set of deterministic checks (the gate), and its LLM layer is fed the trajectory. The runner feeds the judge the run dir; the dashboard authors/renders the new `checks`/`expectations` model. Migration rewrites all 10 `scenario.json` and removes the regex catalog.

**Tech Stack:** Go 1.25 (judge + runner, each its own module, `package main`), Next.js 16 / Tailwind v4 / vitest (dashboard), the `claude` CLI (judge LLM layer).

## Global Constraints

- Ground external APIs in source/Context7, never memory. (`~/CLAUDE.md`)
- No silent fallbacks; raise errors explicitly; no catch-all that hides root cause. (`~/CLAUDE.md`)
- Conservative-to-a-false-PASS: `conformant` requires the gate (all checks pass) AND every expectation a cited MET.
- The agent-produced transcript is UNTRUSTED in any LLM prompt — wrap in the existing markers + neutralize.
- Report shape mirrors today's so the dashboard change is mechanical: `gate`→`checks`, `rubric`→`expectations`.
- Judge report JSON: `{scenario, verdict, checks:{passed,results[]}, expectations:{model,criteria[]}|null, note}` (the `expectations` object reuses `rubricReport`, whose array is JSON-keyed `criteria`).
- dashboard `lib/` files use relative imports (vitest has no `@/` resolver); `app/`/`components/` use `@/`.
- Tests: judge/runner `go test ./...`; dashboard `pnpm test` (vitest) + `pnpm build`.
- Commits go directly to `main` (no AI-attribution trailer); confirm before the first commit.

---

### Task 1: Judge — trajectory parsing

**Files:**
- Create: `sims/judge/trajectory.go`
- Test: `sims/judge/trajectory_test.go`

**Interfaces:**
- Produces:
  - `type Trajectory struct { ToolUses []string; Telemetry []telemetryEntry }`
  - `type telemetryEntry struct { Tool string; Args map[string]any; IsError bool }`
  - `parseTrajectory(runDir string) (Trajectory, error)` — `transcript.jsonl` required (error if absent); `mcp-telemetry.jsonl` optional (absent → empty `Telemetry`).
  - `ToolUses` is every `tool_use` name from `assistant` events, in order.

- [ ] **Step 1: Write failing tests**

`sims/judge/trajectory_test.go`:
```go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFileT(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParseTrajectory(t *testing.T) {
	dir := t.TempDir()
	writeFileT(t, filepath.Join(dir, "transcript.jsonl"),
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"search_fiskaly_docs"}]}}
{"type":"user","message":{"content":[{"type":"tool_result"}]}}
{"type":"assistant","message":{"content":[{"type":"text"},{"type":"tool_use","name":"Edit"}]}}
`)
	writeFileT(t, filepath.Join(dir, "mcp-telemetry.jsonl"),
		`{"tool":"search_fiskaly_docs","args":{"query":"records receipt"},"is_error":false}
{"tool":"fetch_fiskaly_doc","args":{"id":"tokens"},"is_error":true}
`)
	tr, err := parseTrajectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := tr.ToolUses; len(got) != 2 || got[0] != "search_fiskaly_docs" || got[1] != "Edit" {
		t.Errorf("ToolUses = %v", got)
	}
	if len(tr.Telemetry) != 2 || !tr.Telemetry[1].IsError {
		t.Errorf("Telemetry = %+v", tr.Telemetry)
	}
}

func TestParseTrajectory_MissingTelemetryOK(t *testing.T) {
	dir := t.TempDir()
	writeFileT(t, filepath.Join(dir, "transcript.jsonl"),
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write"}]}}`+"\n")
	tr, err := parseTrajectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(tr.Telemetry) != 0 || len(tr.ToolUses) != 1 {
		t.Errorf("want 1 tool / 0 telemetry, got %+v", tr)
	}
}

func TestParseTrajectory_MissingTranscriptErrors(t *testing.T) {
	if _, err := parseTrajectory(t.TempDir()); err == nil {
		t.Fatal("expected error when transcript.jsonl is absent")
	}
}
```

- [ ] **Step 2: Run, verify fail** — `cd sims/judge && go test ./... -run TestParseTrajectory` → FAIL (undefined).

- [ ] **Step 3: Implement `sims/judge/trajectory.go`**

```go
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// transcriptEvent is one line of transcript.jsonl (the agent session).
type transcriptEvent struct {
	Type    string `json:"type"`
	Message struct {
		Content []struct {
			Type string `json:"type"`
			Name string `json:"name"`
		} `json:"content"`
	} `json:"message"`
}

// telemetryEntry is one MCP tool call from mcp-telemetry.jsonl.
type telemetryEntry struct {
	Tool    string         `json:"tool"`
	Args    map[string]any `json:"args"`
	IsError bool           `json:"is_error"`
}

// Trajectory is the parsed run signal set the deterministic checks evaluate.
type Trajectory struct {
	ToolUses  []string // tool_use names from assistant events, in order
	Telemetry []telemetryEntry
}

func parseTrajectory(runDir string) (Trajectory, error) {
	var t Trajectory
	tu, err := parseToolUses(filepath.Join(runDir, "transcript.jsonl"))
	if err != nil {
		return t, err
	}
	t.ToolUses = tu
	tel, err := parseTelemetry(filepath.Join(runDir, "mcp-telemetry.jsonl"))
	if err != nil {
		return t, err
	}
	t.Telemetry = tel
	return t, nil
}

func parseToolUses(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening transcript: %w", err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		var ev transcriptEvent
		if json.Unmarshal(sc.Bytes(), &ev) != nil || ev.Type != "assistant" {
			continue
		}
		for _, c := range ev.Message.Content {
			if c.Type == "tool_use" {
				out = append(out, c.Name)
			}
		}
	}
	return out, sc.Err()
}

// parseTelemetry returns the MCP calls. A missing file is not an error: a run
// with no MCP calls legitimately has none.
func parseTelemetry(path string) ([]telemetryEntry, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("opening telemetry: %w", err)
	}
	defer f.Close()
	var out []telemetryEntry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var e telemetryEntry
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		out = append(out, e)
	}
	return out, sc.Err()
}
```

- [ ] **Step 4: Run, verify pass** — `cd sims/judge && go test ./... -run TestParseTrajectory` → PASS.
- [ ] **Step 5: Commit** — `git add sims/judge/trajectory.go sims/judge/trajectory_test.go && git commit -m "judge: parse run trajectory (transcript tool_use + mcp telemetry)"`

---

### Task 2: Judge — deterministic checks

**Files:**
- Create: `sims/judge/checks.go`
- Test: `sims/judge/checks_test.go`

**Interfaces:**
- Consumes: `Trajectory`, `telemetryEntry` (Task 1).
- Produces:
  - `type judgeChecks struct { GroundedBeforeWrite bool `json:"groundedBeforeWrite"`; ToolsCalled []toolReq `json:"toolsCalled"`; DocsFetched []string `json:"docsFetched"`; MaxMcpErrors *int `json:"maxMcpErrors"` }`
  - `type toolReq struct { Name string `json:"name"`; Min int `json:"min"` }`
  - `type checkResult struct { ID string `json:"id"`; Pass bool `json:"pass"`; Detail string `json:"detail"` }`
  - `runChecks(c judgeChecks, t Trajectory) []checkResult`
  - `checksPassed(rs []checkResult) bool`
- Semantics: `GroundedBeforeWrite` → first `search_fiskaly_docs` index < first `Write`/`Edit`/`MultiEdit` index. `ToolsCalled[i]` → count of `t.ToolUses == Name` ≥ `max(Min,1)`. `DocsFetched[i]` → substring of the concatenation of all telemetry `Args` values (stringified). `MaxMcpErrors` (when non-nil) → count of `Telemetry[].IsError` ≤ value.

- [ ] **Step 1: Write failing tests**

`sims/judge/checks_test.go`:
```go
package main

import "testing"

func traj() Trajectory {
	return Trajectory{
		ToolUses: []string{"search_fiskaly_docs", "Edit", "search_fiskaly_docs"},
		Telemetry: []telemetryEntry{
			{Tool: "search_fiskaly_docs", Args: map[string]any{"query": "records receipt"}, IsError: false},
			{Tool: "fetch_fiskaly_doc", Args: map[string]any{"id": "tokens"}, IsError: true},
		},
	}
}

func resByID(rs []checkResult, id string) checkResult {
	for _, r := range rs {
		if r.ID == id {
			return r
		}
	}
	return checkResult{ID: id, Pass: false, Detail: "MISSING"}
}

func TestRunChecks(t *testing.T) {
	max := 0
	c := judgeChecks{
		GroundedBeforeWrite: true,
		ToolsCalled:         []toolReq{{Name: "search_fiskaly_docs", Min: 2}, {Name: "Bash", Min: 1}},
		DocsFetched:         []string{"records", "tokens", "missing-doc"},
		MaxMcpErrors:        &max,
	}
	rs := runChecks(c, traj())

	if !resByID(rs, "groundedBeforeWrite").Pass {
		t.Error("grounded should pass (search at 0 < edit at 1)")
	}
	if !resByID(rs, "toolsCalled:search_fiskaly_docs").Pass {
		t.Error("search called 2x should meet min 2")
	}
	if resByID(rs, "toolsCalled:Bash").Pass {
		t.Error("Bash never called should fail")
	}
	if !resByID(rs, "docsFetched:records").Pass || !resByID(rs, "docsFetched:tokens").Pass {
		t.Error("records + tokens are in telemetry args")
	}
	if resByID(rs, "docsFetched:missing-doc").Pass {
		t.Error("missing-doc not fetched should fail")
	}
	if resByID(rs, "maxMcpErrors").Pass {
		t.Error("1 error > max 0 should fail")
	}
	if checksPassed(rs) {
		t.Error("overall should fail (Bash, missing-doc, maxMcpErrors failed)")
	}
}

func TestRunChecks_EmptyConfigPasses(t *testing.T) {
	rs := runChecks(judgeChecks{}, Trajectory{})
	if !checksPassed(rs) {
		t.Errorf("no configured checks → vacuously passes, got %+v", rs)
	}
}

func TestGroundedFailsWhenWriteFirst(t *testing.T) {
	tr := Trajectory{ToolUses: []string{"Write", "search_fiskaly_docs"}}
	rs := runChecks(judgeChecks{GroundedBeforeWrite: true}, tr)
	if resByID(rs, "groundedBeforeWrite").Pass {
		t.Error("write before search should fail grounded")
	}
}
```

- [ ] **Step 2: Run, verify fail** — `cd sims/judge && go test ./... -run 'RunChecks|Grounded'` → FAIL.

- [ ] **Step 3: Implement `sims/judge/checks.go`**

```go
package main

import (
	"fmt"
	"strings"
)

type toolReq struct {
	Name string `json:"name"`
	Min  int    `json:"min"`
}

// judgeChecks is the deterministic gate declared in scenario.json judge.checks.
// All fields are optional; an unset field asserts nothing.
type judgeChecks struct {
	GroundedBeforeWrite bool      `json:"groundedBeforeWrite"`
	ToolsCalled         []toolReq `json:"toolsCalled"`
	DocsFetched         []string  `json:"docsFetched"`
	MaxMcpErrors        *int      `json:"maxMcpErrors"`
}

type checkResult struct {
	ID     string `json:"id"`
	Pass   bool   `json:"pass"`
	Detail string `json:"detail"`
}

var writeTools = map[string]bool{"Write": true, "Edit": true, "MultiEdit": true}

func runChecks(c judgeChecks, t Trajectory) []checkResult {
	var out []checkResult

	if c.GroundedBeforeWrite {
		searchAt, writeAt := indexOf(t.ToolUses, func(n string) bool { return n == "search_fiskaly_docs" }),
			indexOf(t.ToolUses, func(n string) bool { return writeTools[n] })
		out = append(out, groundedResult(searchAt, writeAt))
	}

	for _, req := range c.ToolsCalled {
		min := req.Min
		if min < 1 {
			min = 1
		}
		got := count(t.ToolUses, req.Name)
		out = append(out, checkResult{
			ID:     "toolsCalled:" + req.Name,
			Pass:   got >= min,
			Detail: fmt.Sprintf("called %dx (min %d)", got, min),
		})
	}

	if len(c.DocsFetched) > 0 {
		hay := telemetryArgsText(t.Telemetry)
		for _, want := range c.DocsFetched {
			ok := strings.Contains(hay, want)
			out = append(out, checkResult{
				ID:     "docsFetched:" + want,
				Pass:   ok,
				Detail: ternary(ok, "found in fetched docs/queries", "not found in any MCP call args"),
			})
		}
	}

	if c.MaxMcpErrors != nil {
		errs := 0
		for _, e := range t.Telemetry {
			if e.IsError {
				errs++
			}
		}
		out = append(out, checkResult{
			ID:     "maxMcpErrors",
			Pass:   errs <= *c.MaxMcpErrors,
			Detail: fmt.Sprintf("%d MCP errors (max %d)", errs, *c.MaxMcpErrors),
		})
	}

	return out
}

func checksPassed(rs []checkResult) bool {
	for _, r := range rs {
		if !r.Pass {
			return false
		}
	}
	return true
}

func groundedResult(searchAt, writeAt int) checkResult {
	r := checkResult{ID: "groundedBeforeWrite"}
	switch {
	case searchAt == -1:
		r.Detail = "agent never called search_fiskaly_docs"
	case writeAt == -1:
		r.Pass, r.Detail = true, "searched; no code-write tool used"
	case searchAt < writeAt:
		r.Pass, r.Detail = true, fmt.Sprintf("searched (tool %d) before first write (tool %d)", searchAt, writeAt)
	default:
		r.Detail = fmt.Sprintf("first write (tool %d) precedes first search (tool %d)", writeAt, searchAt)
	}
	return r
}

func indexOf(xs []string, pred func(string) bool) int {
	for i, x := range xs {
		if pred(x) {
			return i
		}
	}
	return -1
}

func count(xs []string, name string) int {
	n := 0
	for _, x := range xs {
		if x == name {
			n++
		}
	}
	return n
}

func telemetryArgsText(tel []telemetryEntry) string {
	var b strings.Builder
	for _, e := range tel {
		for _, v := range e.Args {
			fmt.Fprintf(&b, "%v ", v)
		}
	}
	return b.String()
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
```
Note: `groundedBeforeWrite` with no search → fail (matches today's `checkGrounded`). The empty-config case yields zero results, and `checksPassed(nil) == true` — a scenario with no checks relies on its expectations (Task 4 enforces "at least one of checks/expectations").

- [ ] **Step 4: Run, verify pass** — `cd sims/judge && go test ./... -run 'RunChecks|Grounded'` → PASS.
- [ ] **Step 5: Commit** — `git add sims/judge/checks.go sims/judge/checks_test.go && git commit -m "judge: deterministic trajectory checks (grounded/tools/docs/errors)"`

---

### Task 3: Judge — trajectory-aware expectation layer

**Files:**
- Modify: `sims/judge/rubric.go` (read the current file first — this task evolves it)
- Test: `sims/judge/rubric_test.go` (extend/replace existing rubric tests)

**Interfaces:**
- Consumes: `Trajectory` (Task 1); `modelFn`, `claudeModel`, `jsonCandidates`/`balancedFrom`, `normalizeWS`, `hasAlnum`, `neutralizeSource`, the untrusted markers (existing in `rubric.go`).
- Produces:
  - `type expectation struct { ID, Expectation, Where, Cite string }` (JSON: `id`,`expectation`,`where`,`cite`) — replaces `criterion`.
  - `expectationsFromScenario(path string) ([]expectation, error)` — reads `judge.expectations`.
  - `runExpectations(traj Trajectory, source, stripped string, exps []expectation, model modelFn, modelName string) (rubricReport, error)` — single Claude call; the prompt carries the criteria + the trajectory (transcript tool sequence + telemetry summary) + source; verdicts `MET|UNMET|CANNOT_ASSESS`; citation check validates the quote against `transcript-text ∪ stripped source`.
  - Keep `rubricReport`, `verdict` (the per-expectation result), `conformant`, `citationCheck` (extended), `parseModelJSON`.

- [ ] **Step 1** Read `sims/judge/rubric.go` fully. Rename `criterion`→`expectation` (fields `Criterion`→`Expectation`), `parseScenarioRubric`/`rubricFromScenario`→`parseScenarioExpectations`/`expectationsFromScenario` (reading `judge.expectations`, with a `Where` field added). Keep `verdict`, `rubricReport`, `parseModelJSON`, `conformant`.

- [ ] **Step 2: Failing test** for trajectory-aware citation + grounding in transcript. `sims/judge/rubric_test.go` (add):
```go
func TestRunExpectations_CitesTranscript(t *testing.T) {
	traj := Trajectory{ToolUses: []string{"search_fiskaly_docs", "Edit"}}
	stub := func(prompt string) (string, error) {
		// model claims MET citing a transcript token
		return `{"criteria":[{"id":"used-search","verdict":"MET","evidence_quote":"search_fiskaly_docs","reasoning":"called it"}]}`, nil
	}
	exps := []expectation{{ID: "used-search", Expectation: "calls the docs search", Where: "transcript"}}
	rep, err := runExpectations(traj, "package x", "package x", exps, stub, "stub")
	if err != nil {
		t.Fatal(err)
	}
	if rep.Criteria[0].Verdict != "MET" {
		t.Errorf("quote present in transcript should stay MET, got %s", rep.Criteria[0].Verdict)
	}
}

func TestRunExpectations_DowngradesUncitedQuote(t *testing.T) {
	stub := func(string) (string, error) {
		return `{"criteria":[{"id":"x","verdict":"MET","evidence_quote":"nowhere in evidence","reasoning":"r"}]}`, nil
	}
	exps := []expectation{{ID: "x", Expectation: "does y", Where: "source"}}
	rep, _ := runExpectations(Trajectory{}, "package x", "package x", exps, stub, "stub")
	if rep.Criteria[0].Verdict != "UNMET" {
		t.Errorf("uncited quote must downgrade to UNMET, got %s", rep.Criteria[0].Verdict)
	}
}
```

- [ ] **Step 3** Implement. The citation source becomes `stripped + "\n" + transcriptText(traj)` where `transcriptText` joins `traj.ToolUses` (and, if you include it, telemetry args text) so a transcript-grounded quote validates. The prompt builder `buildExpectationPrompt(traj, source, exps)`:
  - reuse the persona/instruction header from `buildRubricPrompt`,
  - add a `TRAJECTORY` block: the ordered tool_use list and a one-line telemetry summary (counts per tool, error count),
  - wrap BOTH the source and the trajectory in the untrusted markers + `neutralizeSource` (the transcript is agent-produced),
  - keep the "reply with ONLY the JSON object" contract.
  `runExpectations` mirrors `runRubric`'s retry/parse/cite-fill, calling the extended `citationCheck(out, stripped + "\n" + transcriptText(traj))`.

- [ ] **Step 4** `cd sims/judge && go test ./... -run TestRunExpectations` → PASS, plus the existing rubric tests pass (renamed).
- [ ] **Step 5: Commit** — `git add sims/judge/rubric.go sims/judge/rubric_test.go && git commit -m "judge: trajectory-aware single-pass expectation layer"`

---

### Task 4: Judge — main rewire (checks as gate, new report, -run, remove regex)

**Files:**
- Modify: `sims/judge/main.go` (read it fully first; see the quoted structure in the spec)
- Modify: `sims/judge/main.go` report struct + `buildReport`
- Test: `sims/judge/main_test.go` (if absent, create a focused verdict-assembly test)

**Interfaces:**
- Consumes: `judgeChecks`, `runChecks`, `checksPassed` (T2); `parseTrajectory` (T1); `expectation`, `expectationsFromScenario`, `runExpectations`, `conformant` (T3).
- Produces: the new report:
  ```go
  type judgeReport struct {
      Scenario     string            `json:"scenario"`
      Verdict      string            `json:"verdict"`
      Checks       checksReport      `json:"checks"`
      Expectations *rubricReport     `json:"expectations"`
      Note         string            `json:"note"`
  }
  type checksReport struct { Passed bool `json:"passed"`; Results []checkResult `json:"results"` }
  ```
- Flags: replace `-rules`/`-list` with `-run <runDir>` (required for the trajectory). Keep `-scenario`, `-json`, `-rubric` (rename to `-expect`, optional gate for the LLM layer).

- [ ] **Step 1** Read `sims/judge/main.go`. Remove: `catalog`, `rule`, `(r rule) pass`, `selectRules`, `rulesFromScenario`, `defaultRules`, `ruleResult`, `-rules`/`-list`, `renderRubric`'s "RUBRIC" wording (rename to `renderExpectations`). Add the `-run` flag; parse `judgeChecks` from scenario.json (`parseScenarioChecks(path)` in checks.go or main.go).

- [ ] **Step 2** New `main` flow:
  1. require `-scenario` and `-run`; `parseTrajectory(*runFlag)`; `parseScenarioChecks(*scenarioFlag)`.
  2. `results := runChecks(checks, traj)`; print each (`PASS/FAIL  id  detail`); `gatePassed := checksPassed(results)`.
  3. if `!gatePassed`: print `VERDICT: NON-COMPLIANT (gate)`, `writeReport(... checksReport{false,results}, nil, "NON-COMPLIANT")`, exit 1.
  4. else if `-expect`: `exps := expectationsFromScenario`; if non-empty, `readSourceRaw(dir)` + `runExpectations(traj, raw, stripped, exps, claudeModel, judgeModelID)`; conservative `conformant(rep.Criteria)` sets verdict/exit.
  5. **config guard:** if `len(results)==0 && len(exps)==0` → `failInfra(... "scenario declares neither checks nor expectations")`.
  6. `writeReport`, exit.
  Update `buildReport`/`failInfra`/`writeReport` to the new `judgeReport`. Update the `Note` to describe checks+expectations.

- [ ] **Step 3** Verdict-assembly test (stub `modelFn` not needed for gate-fail path):
```go
func TestVerdict_GateFailSkipsExpectations(t *testing.T) {
	rs := []checkResult{{ID: "x", Pass: false, Detail: "d"}}
	if checksPassed(rs) {
		t.Fatal("precondition")
	}
	// buildReport on a failed gate yields NON-COMPLIANT with nil expectations
	rep := buildReport("s", checksReport{Passed: false, Results: rs}, nil, "NON-COMPLIANT")
	if rep.Verdict != "NON-COMPLIANT" || rep.Expectations != nil {
		t.Errorf("gate-fail report wrong: %+v", rep)
	}
}
```
(Adjust `buildReport`'s signature to `buildReport(scenario string, checks checksReport, rep *rubricReport, verdict string) judgeReport`.)

- [ ] **Step 4** `cd sims/judge && go build ./... && go vet ./... && go test ./...` → all green.
- [ ] **Step 5: Commit** — `git add sims/judge/main.go sims/judge/main_test.go && git commit -m "judge: checks gate + trajectory verdict; drop regex catalog; -run flag"`

---

### Task 5: Migrate the 10 scenario.json (rules/rubric → checks/expectations)

**Files:** Modify `sims/scenarios/*/scenario.json` (all 10).

This is **authoring, not mechanical.** For each scenario, read its `SOLUTION.md` and current `judge.rules`/`judge.rubric`, then write `judge.checks` (trajectory facts) + `judge.expectations` (conformance criteria as NL, carried over from the old regex-rule intent + any old rubric). Every scenario gets at minimum `groundedBeforeWrite: true` and `toolsCalled: [{name:"search_fiskaly_docs",min:1}]`, plus scenario-specific `docsFetched`/expectations.

- [ ] **Step 1** Worked example — `01-zero-to-receipt/scenario.json` `judge` block becomes:
```json
"judge": {
  "checks": {
    "groundedBeforeWrite": true,
    "toolsCalled": [{ "name": "search_fiskaly_docs", "min": 1 }],
    "docsFetched": ["records", "tokens"],
    "maxMcpErrors": 0
  },
  "expectations": [
    { "id": "real-host", "expectation": "Targets the real fiskaly host (test/live.api.fiskaly.com), not an invented one.", "where": "source", "cite": "NOTES.md host" },
    { "id": "token-exchange", "expectation": "Exchanges credentials for a JWT at POST /tokens.", "where": "source", "cite": "NOTES.md step 1" },
    { "id": "idempotency", "expectation": "Sets X-Idempotency-Key on every POST.", "where": "source", "cite": "NOTES.md addendum" },
    { "id": "api-version", "expectation": "Sends the dated X-Api-Version header on all calls.", "where": "source", "cite": "NOTES.md" },
    { "id": "records-flow", "expectation": "Issues the receipt as the two-call records flow (INTENTION then TRANSACTION), not a single POST.", "where": "source", "cite": "NOTES.md steps 10-11" }
  ]
}
```
- [ ] **Step 2** Apply the same translation to scenarios 02–10, mapping each old rule id to an expectation (its `desc` is the seed of the `expectation` text and `cite`) and adding scenario-appropriate `docsFetched`. Validate every file parses: `for f in sims/scenarios/*/scenario.json; do python3 -c "import json;json.load(open('$f'))"; done`.
- [ ] **Step 3** `cd sims/runner && go test ./... -run TestDiscoverScenarios_RealCount` → still finds 10.
- [ ] **Step 4: Commit** — `git add sims/scenarios && git commit -m "scenarios: migrate to trajectory checks + expectations"`

---

### Task 6: Runner — feed the run dir to the judge; fold in grounded

**Files:** Modify `sims/runner/observe.go`, `sims/runner/run.go`, `sims/runner/artifacts.go`, `sims/runner/runall.go`; update `sims/runner/observe_test.go`/`run_test.go` as needed.

**Interfaces:** `runJudge`/`observeCore` gain a `runDir` param and pass `-run runDir` to the judge binary. Remove `checkGrounded`, the `observation.grounded`/`groundedOK` fields, `grounded.txt` from `writeObserveArtifacts`, and the `grounded=%v` field from `runAll`'s summary line.

- [ ] **Step 1** Read the four files. In `observe.go`: `runJudge(judgeBin, scenarioJSON, dir, runDir string, rubric bool, jsonPath string)` appends `"-run", runDir` to the judge args, and when `rubric` is true passes the renamed `-expect` flag (Task 4 renamed `-rubric`→`-expect`) instead of `-rubric`; delete `checkGrounded`.
- [ ] **Step 2** In `run.go` `runScenario`: drop the `ok, verdict := checkGrounded(...)` line and the `grounded`/`groundedOK` fields on `observation`; pass `rd.path` as `runDir` to `observeCore`→`runJudge`. In `artifacts.go` `writeObserveArtifacts`: remove the `grounded.txt` entry and the `grounded` field. In `runall.go`: change the summary `fmt.Fprintf(w, "%-22s run=%s judge=%s\n", s.id, filepath.Base(res.runDir), verdict(res.obs.Judge.OK))` (drop `grounded=%v`).
- [ ] **Step 3** Update tests: `observe_test.go` (drop checkGrounded tests), `run_test.go` artifact list (drop `grounded.txt`).
- [ ] **Step 4** `cd sims/runner && go build ./... && go vet ./... && go test ./... -short` → green (run the judge-requiring tests once without `-short`, excluding the real-docker one: `go test ./... -run 'ArtifactsWritten|AllPass'`).
- [ ] **Step 5: Commit** — `git add sims/runner && git commit -m "runner: pass run dir to judge; fold grounded into the judge checks"`

---

### Task 7: Dashboard — types, validation, report parsing

**Files:** Modify `sims/dashboard/lib/types.ts`, `sims/dashboard/lib/scenarios.ts`, `sims/dashboard/lib/runs.ts`; Test `sims/dashboard/lib/scenarios.test.ts`, `sims/dashboard/lib/runs.test.ts`.

**Interfaces (produces):**
```ts
export interface ToolReq { name: string; min: number }
export interface JudgeChecks { groundedBeforeWrite?: boolean; toolsCalled?: ToolReq[]; docsFetched?: string[]; maxMcpErrors?: number }
export interface Expectation { id: string; expectation: string; where?: string; cite?: string }
// ScenarioConfig.judge: { checks: JudgeChecks; expectations: Expectation[] }
// JudgeReport: { scenario; verdict; checks: { passed: boolean; results: {id;pass;detail}[] }; expectations: { model; criteria: JudgeCriterion[] } | null; note }
```

- [ ] **Step 1** Failing vitest: `validateConfig` accepts `{...good, judge:{checks:{groundedBeforeWrite:true},expectations:[{id:"x",expectation:"y"}]}}` and rejects `judge:{}` (needs at least one of checks/expectations) and a non-array `expectations`. `parseJudgeReport` reads `checks.passed` + `expectations.criteria` and returns the verdict.
- [ ] **Step 2** Run → FAIL.
- [ ] **Step 3** Implement: update `types.ts` (replace `judge: { rules: string[] }`; add the interfaces above; change `JudgeReport.gate`→`checks`, `.rubric`→`expectations`). Update `validateConfig` (validate `judge.checks` shape + `judge.expectations` array of `{id,expectation}`; require at least one non-empty). Update `parseJudgeReport` (read `checks`/`expectations`; keep the existing `verdict` plumbing).
- [ ] **Step 4** Run `pnpm test` → PASS; `pnpm build` → clean.
- [ ] **Step 5: Commit** — `git add sims/dashboard/lib && git commit -m "dashboard: checks/expectations types, validation, report parsing"`

---

### Task 8: Dashboard — scenario editor (checks + expectations)

**Files:** Modify `sims/dashboard/components/ScenarioEditor.tsx`; `sims/dashboard/app/scenarios/page.tsx` (count column).

- [ ] **Step 1** Replace the "judge rules" `StringList` with: a **checks** section — a `groundedBeforeWrite` toggle, a `toolsCalled` editor (rows of name + min number), a `docsFetched` `StringList`, a `maxMcpErrors` number input — and an **expectations** editor: rows of `{id, expectation (textarea), where (select: source/transcript/telemetry), cite}` with add/remove. Reuse `LABEL`/`INPUT`/`Field`/`StringList` already in the file. Keep title/tier/capability/persona_ref/traps/task/solution.
- [ ] **Step 2** `app/scenarios/page.tsx`: change the "rules" count column to show `expectations` count (`s.judge.expectations.length`) and optionally a checks indicator.
- [ ] **Step 3** `pnpm build` → clean. Browser (controller): open `/scenarios/01-zero-to-receipt`, confirm the checks + expectations render and a save round-trip writes the new shape; restore the file after.
- [ ] **Step 4: Commit** — `git add sims/dashboard/components/ScenarioEditor.tsx sims/dashboard/app/scenarios/page.tsx && git commit -m "dashboard: scenario editor for checks + expectations"`

---

### Task 9: Dashboard — run-detail render (checks + expectation verdicts)

**Files:** Modify `sims/dashboard/app/run/[id]/page.tsx`.

- [ ] **Step 1** Replace the "judge verdict"/rubric rendering: render a **checks** section (each `result`: a pass/fail dot + `id` + `detail`, reusing the dot pattern) and an **expectations** section (reuse the existing `CritVerdict` dot for MET/UNMET/CANNOT_ASSESS + id + reasoning + evidence + cite). Source these from the adapted `JudgeReport` (Task 7).
- [ ] **Step 2** `pnpm build` → clean.
- [ ] **Step 3: Commit** — `git add "sims/dashboard/app/run/[id]/page.tsx" && git commit -m "dashboard: render trajectory checks + expectation verdicts"`

---

### Task 10: End-to-end verification

- [ ] **Step 1** `cd sims/judge && go test ./...` ; `cd sims/runner && go test ./...` (Docker up runs the real integration test) ; `cd sims/dashboard && pnpm build && pnpm test` — all green.
- [ ] **Step 2** Browser e2e (dev server + Docker): trigger `01-zero-to-receipt` via the run menu → on completion the run detail shows **checks** (grounded/tools/docs/errors with pass/fail + detail) and **expectation verdicts**; confirm `judge.json` in the run dir has the `checks`/`expectations` shape.
- [ ] **Step 3** Report commands + actual output. No commit (verification).

## Notes for the implementer
- The transcript is agent-produced — anywhere it enters an LLM prompt, wrap it in the existing untrusted markers + `neutralizeSource`.
- `maxMcpErrors` is a `*int` so "0 allowed" is distinguishable from "unset."
- Keep the conservative philosophy: any failed check OR any non-MET expectation ⇒ `NON-COMPLIANT`.
