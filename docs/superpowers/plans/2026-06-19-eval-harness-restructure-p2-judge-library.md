# Eval harness restructure — Phase 2: Judge as a library Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Lift the judge's evaluation logic into an exported `backend/internal/judge` package behind a single `Evaluate` entry point, reduce `cmd/judge` to a thin exit-code wrapper, and have `judge_eval` import the package instead of building-and-exec'ing the binary — preserving the judge's exact output, `judge.json`, and 0/1/2 exit-code contract.

**Architecture:** All judge logic (`checks.go`, `rubric.go`, `trajectory.go`, and the core of `main.go`) moves into `internal/judge` as `package judge`. The package exposes exactly three new names — `Evaluate`, `Options`, `Report` — and keeps everything else unexported (white-box tests move with it). `cmd/judge/main.go` becomes flag-parsing + `Evaluate` + write-JSON + `os.Exit`. The orchestrator is unchanged: it still `go build` + execs the now-thin `cmd/judge`, so its `judge.txt`/`judge.json` are byte-identical. The orchestrator's own in-process switch is deferred to the server phases (P4/P5).

**Tech Stack:** Go 1.23 (stdlib). The `judge_eval` goldset meta-eval is the correctness gate and runs the real model via the `claude` CLI (verified present: v2.1.183; token in `.env`).

**Spec:** `docs/superpowers/specs/2026-06-19-eval-harness-restructure-design.md` (Sequencing → P2). P0 + P1 complete.

## Global Constraints

- No AI/Claude attribution in commits, code, or docs. No `Co-Authored-By` trailer, no generated-with footer.
- Commit and push directly to `main`. Commit-message style: lowercase prefix (`refactor:`, `docs:`, `cleanup:`).
- `mcp/`, `research/`, `memo/` stay at the repo root; Docker build context stays the repo root.
- System-under-test data is not modified: `scenarios/`, `pos/`, per-fixture `go.mod`.
- The backend module stays stdlib-only (no `require`).
- **The judge's behavior must be byte-preserved:** the human-readable judge output (captured into `judge.txt`), the `judge.json` structure and its field tags, and the exit codes (`0` conformant / `1` NON-COMPLIANT / `2` infra error) are unchanged. The goldset meta-eval is the guard.
- **Process discipline (from the P1 retro):** after the task's edits, the implementer confirms `git status` is clean AND the committed tree builds; the reviewer treats a diff missing an expected edit as a finding rather than reading the working tree to fill it in.

## Design decisions for this phase (resolved, not open)

- **Single public entry, not granular exports.** Export only `Evaluate`, `Options`, `Report`. The spec's prose ("export runChecks/runExpectations/parseTrajectory/buildReport") was a granular guess; every actual consumer (the CLI wrapper, `judge_eval`, the future server) needs only the top-level `Evaluate`. Keep `runChecks`/`runExpectations`/`parseTrajectory`/`buildReport`/types unexported; white-box tests live in `package judge`.
- **`judge_eval` reads criteria through the exported `Report` field.** `Report.Expectations` is `*rubricReport` (unexported type) but its fields are exported, so `report.Expectations.Criteria[i].Verdict` is reachable from `judge_eval` without exporting `rubricReport`/`verdict`. This keeps the export surface minimal.
- **Orchestrator unchanged.** It keeps `buildJudge` + `runJudge` against the thin `cmd/judge` binary. The in-process judge call belongs to the server (P4/P5), where it removes the build-exec hop for the server's own runs.
- **`testdata/goldset` stays at `cmd/judge/testdata/`** (where P1 left it); `judge_eval` keeps resolving it there. Not worth a move this phase.

## Target layout after this phase

```
eval-harness/backend/
  cmd/
    eval-harness/main.go        (unchanged)
    judge/
      main.go                   THIN wrapper: flags -> judge.Evaluate -> write json -> exit
      testdata/goldset/...       (unchanged)
  internal/
    orchestrator/...            (unchanged)
    judge/                      NEW package judge
      checks.go rubric.go trajectory.go   (moved from cmd/judge, package main -> judge)
      report.go                 (the core split out of the old main.go)
      evaluate.go               NEW: Options, Evaluate, infra
      checks_test.go rubric_test.go trajectory_test.go report_test.go  (moved; package judge)
  judge_eval/main.go            (Task 2: imports internal/judge instead of build+exec)
```

---

### Task 1: Extract `internal/judge` and reduce `cmd/judge` to a thin wrapper

Move all judge logic into `internal/judge` behind `Evaluate`; rewrite `cmd/judge/main.go` as a thin wrapper. `judge_eval` is left untouched this task (it still build+execs the now-thin `cmd/judge`, which produces identical output). Deliverable: the backend module builds, judge + orchestrator unit tests pass, and a Docker e2e run produces an unchanged `judge.txt`/`judge.json`.

**Files:**
- Move: `cmd/judge/{checks,rubric,trajectory}.go` + `{checks,rubric,trajectory}_test.go` → `internal/judge/`
- Create: `internal/judge/report.go` (core extracted from `cmd/judge/main.go`)
- Create: `internal/judge/evaluate.go`
- Move: `cmd/judge/main_test.go` → `internal/judge/report_test.go`
- Rewrite: `cmd/judge/main.go` (thin wrapper)

**Interfaces:**
- Produces (consumed by the wrapper now, `judge_eval` in Task 2, the server in P4/P5):
  - `package judge`
  - `type Options struct { ScenarioPath, RunDir, IntegrationDir string; Expect bool }`
  - `type Report struct { Scenario, Verdict string; Checks checksReport; Expectations *rubricReport; Note string }` (JSON tags: `scenario`, `verdict`, `checks`, `expectations`, `note` — unchanged from the old `judgeReport`)
  - `func Evaluate(opts Options, out io.Writer) (Report, int, error)` — writes the human-readable report to `out`; returns the report, exit code (0/1/2), and a non-nil error iff the code is 2 (infra).

- [ ] **Step 1: Move the three logic files and their tests into `internal/judge`**

```bash
cd /Users/stan/code/fsk
mkdir -p eval-harness/backend/internal/judge
git mv eval-harness/backend/cmd/judge/checks.go eval-harness/backend/cmd/judge/checks_test.go \
       eval-harness/backend/cmd/judge/rubric.go eval-harness/backend/cmd/judge/rubric_test.go \
       eval-harness/backend/cmd/judge/trajectory.go eval-harness/backend/cmd/judge/trajectory_test.go \
       eval-harness/backend/internal/judge/
git mv eval-harness/backend/cmd/judge/main_test.go eval-harness/backend/internal/judge/report_test.go
```

- [ ] **Step 2: Create `internal/judge/report.go` from the old `main.go` core**

Move these declarations OUT of `cmd/judge/main.go` and INTO a new file `eval-harness/backend/internal/judge/report.go`, verbatim except (a) the package line is `package judge` and (b) the type `judgeReport` is renamed to `Report`:
- `type checksReport struct {...}` (fields `Passed`/`Results`, tags `passed`/`results`)
- `type judgeReport struct {...}` → rename to `type Report struct {...}` (keep all JSON tags)
- `func buildReport(scenario string, checks checksReport, rep *rubricReport, verdict string) judgeReport` → return type becomes `Report`, and its internal `var r judgeReport` becomes `var r Report`
- `func renderExpectations(rep rubricReport) string`
- `func readSourceRaw(dir string) (string, error)`
- `func stripCommentsKeepLayout(src string) string`

`report.go` needs these imports (carried from `main.go`): `fmt`, `go/scanner`, `go/token`, `os`, `path/filepath`, `strings`.

- [ ] **Step 3: Create `internal/judge/evaluate.go`**

Create `eval-harness/backend/internal/judge/evaluate.go`:
```go
package judge

import (
	"fmt"
	"io"
	"path/filepath"
)

// Options is the fully-resolved input to one judge evaluation.
type Options struct {
	ScenarioPath   string // path to a scenario.json
	RunDir         string // run dir with transcript.jsonl + mcp-telemetry.jsonl; "" = source-only (skip the trajectory gate)
	IntegrationDir string // root of the integration source under review
	Expect         bool   // after the gate passes, run the LLM expectation layer
}

// Evaluate runs the deterministic checks gate and, when Expect is set and the
// scenario declares expectations, the LLM expectation layer. It writes the
// human-readable report to out and returns the structured report plus the
// process exit code: 0 conformant, 1 NON-COMPLIANT, 2 infra error. The returned
// error is non-nil iff the code is 2.
func Evaluate(opts Options, out io.Writer) (Report, int, error) {
	scenarioName := filepath.Base(filepath.Dir(opts.ScenarioPath))

	var traj trajectory
	var results []checkResult
	gatePassed := true
	if opts.RunDir != "" {
		var err error
		traj, err = parseTrajectory(opts.RunDir)
		if err != nil {
			return infra(scenarioName, checksReport{}, err)
		}
		checks, err := parseScenarioChecks(opts.ScenarioPath)
		if err != nil {
			return infra(scenarioName, checksReport{}, err)
		}
		results = runChecks(checks, traj)
		for _, r := range results {
			status := "PASS"
			if !r.Pass {
				status = "FAIL"
			}
			fmt.Fprintf(out, "%-4s  %-30s %s\n", status, r.ID, r.Detail)
		}
		gatePassed = checksPassed(results)
	}

	cr := checksReport{Passed: gatePassed, Results: results}

	if !gatePassed {
		fmt.Fprintln(out, "VERDICT: NON-COMPLIANT (gate). exit 1")
		return buildReport(scenarioName, cr, nil, "NON-COMPLIANT"), 1, nil
	}

	verdict := "conformant"
	exitCode := 0
	var rep *rubricReport
	var exps []expectation

	if opts.Expect {
		var err error
		exps, err = expectationsFromScenario(opts.ScenarioPath)
		if err != nil {
			return infra(scenarioName, cr, err)
		}
		if len(exps) > 0 {
			raw, err := readSourceRaw(opts.IntegrationDir)
			if err != nil {
				return infra(scenarioName, cr, err)
			}
			r, err := runExpectations(traj, raw, stripCommentsKeepLayout(raw), exps, claudeModel, rubricModelID)
			if err != nil {
				return infra(scenarioName, cr, fmt.Errorf("expectation layer: %w", err))
			}
			rep = &r
			fmt.Fprint(out, renderExpectations(r))
			if !conformant(r.Criteria) {
				verdict = "NON-COMPLIANT"
				exitCode = 1
			}
		}
	}

	// A scenario that asserts nothing is a misconfiguration.
	if len(results) == 0 && len(exps) == 0 {
		return infra(scenarioName, cr, fmt.Errorf("scenario declares neither checks nor expectations"))
	}

	if exitCode == 0 {
		fmt.Fprintln(out, "VERDICT: conformant. exit 0")
	} else {
		fmt.Fprintln(out, "VERDICT: NON-COMPLIANT (expectations). exit 1")
	}
	return buildReport(scenarioName, cr, rep, verdict), exitCode, nil
}

// infra builds the shared infra-error result (exit 2): a NON-COMPLIANT report
// whose Note records that no verdict was computed.
func infra(scenario string, cr checksReport, err error) (Report, int, error) {
	rep := buildReport(scenario, cr, nil, "NON-COMPLIANT")
	rep.Note = "infra error (no verdict computed): " + err.Error()
	return rep, 2, err
}
```

- [ ] **Step 4: Change the package line in every moved file**

In all six moved files under `internal/judge/` (`checks.go`, `rubric.go`, `trajectory.go`, `checks_test.go`, `rubric_test.go`, `trajectory_test.go`) and the moved `report_test.go`, change the first non-comment line from `package main` to `package judge`. (`rubric.go` has a leading doc comment before `package main`; keep the comment, change the package line.)

- [ ] **Step 5: Rewrite `cmd/judge/main.go` as a thin wrapper**

Replace the entire contents of `eval-harness/backend/cmd/judge/main.go` with:
```go
// Command judge checks a fiskaly integration against a per-scenario rubric. It
// runs a deterministic checks gate first (trajectory-derived signals from the
// run dir) and, when that passes and -expect is set, an LLM expectation layer.
// The gate is hard: any failing check marks the run NON-COMPLIANT and skips the
// LLM entirely.
//
// Usage: judge -scenario <path> [-run <runDir>] [-expect] [-json <out>] <integrationDir>
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"backend/internal/judge"
)

func main() {
	var (
		scenarioFlag = flag.String("scenario", "", "path to a scenario.json (required)")
		runFlag      = flag.String("run", "", "path to a run dir with transcript.jsonl + mcp-telemetry.jsonl; omit for source-only evaluation")
		expectFlag   = flag.Bool("expect", false, "after the gate passes, run the LLM expectation layer (requires the scenario to declare judge.expectations and the claude CLI)")
		jsonFlag     = flag.String("json", "", "write the structured verdict to this path as JSON")
	)
	flag.Parse()

	if *scenarioFlag == "" {
		fmt.Fprintln(os.Stderr, "judge: -scenario is required")
		os.Exit(2)
	}
	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "judge: missing integration dir")
		os.Exit(2)
	}

	report, code, err := judge.Evaluate(judge.Options{
		ScenarioPath:   *scenarioFlag,
		RunDir:         *runFlag,
		IntegrationDir: flag.Arg(0),
		Expect:         *expectFlag,
	}, os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "judge:", err)
	}
	if *jsonFlag != "" {
		writeReport(*jsonFlag, report)
	}
	os.Exit(code)
}

func writeReport(path string, report judge.Report) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "judge: marshaling report:", err)
		os.Exit(2)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "judge: writing report:", err)
		os.Exit(2)
	}
}
```
This preserves the original ordering: stdout report (written by `Evaluate`), then any stderr error, then `judge.json` is written in every case where `-json` is set (including gate-fail and infra), then exit with the returned code.

- [ ] **Step 6: Build and run unit tests**

```bash
cd /Users/stan/code/fsk/eval-harness/backend
go build ./...
go test ./... -short
```
Expected: `go build` exits 0; `go test` reports `ok` for `backend/internal/judge` (the moved checks/rubric/trajectory/report tests), `backend/internal/orchestrator`, and `backend/cmd/judge` builds (no test files there now). Confirm there is no stray `package main` left under `internal/judge/` and no reference to the old `judgeReport` name (`grep -rn 'judgeReport\|package main' eval-harness/backend/internal/judge` returns nothing).

- [ ] **Step 7: Confirm `judge_eval` still builds (unchanged this task)**

```bash
cd /Users/stan/code/fsk/eval-harness/backend && go vet ./judge_eval
```
Expected: exits 0. `judge_eval` is untouched in Task 1 — it still `go build` + execs `cmd/judge`, which is now the thin wrapper producing identical output.

- [ ] **Step 8: Docker e2e — prove the wrapper's output is unchanged (requires Docker + token)**

The orchestrator build+execs `cmd/judge`; this confirms the thin wrapper produces the same `judge.txt`/`judge.json` the dashboard reads.
```bash
cd /Users/stan/code/fsk/eval-harness/backend && go run ./cmd/eval-harness run 07
```
Expected: `07-wrong-vat ... judge=...` and a run dir under `~/.cache/fiskaly-eval/run.*` whose `judge.txt` contains the `PASS/FAIL` check lines + a `VERDICT:` line (+ `EXPECTATIONS` block, since 07 declares expectations) and whose `judge.json` has the `scenario`/`verdict`/`checks`/`expectations`/`note` fields. If Docker/token unavailable, record SKIPPED with reason; do not claim a pass.

- [ ] **Step 9: Confirm clean tree and that the committed tree builds, then commit**

```bash
cd /Users/stan/code/fsk
git add -A && git status                         # review: only the intended moves/edits staged
git commit -m "refactor: extract internal/judge behind Evaluate; thin cmd/judge wrapper"
git push origin main
git stash list                                   # expect empty (no uncommitted edits left behind)
(cd eval-harness/backend && go build ./...) && echo "committed tree builds"
```
Expected: `git status` clean after commit; the post-commit build exits 0 (guards against the P1 mistake of leaving edits uncommitted).

---

### Task 2: Make `judge_eval` import `internal/judge` and run the goldset gate

Rewrite the meta-eval to call `judge.Evaluate` in-process instead of building and exec'ing the judge binary, then run the goldset meta-eval — P2's correctness gate. Deliverable: `judge_eval` imports `internal/judge`, and the meta-eval passes against the goldset.

**Files:**
- Rewrite: `eval-harness/backend/judge_eval/main.go`

**Interfaces:**
- Consumes: `judge.Evaluate`, `judge.Options`, `judge.Report` from Task 1.

- [ ] **Step 1: Rewrite `judge_eval/main.go` to call `Evaluate` in-process**

Replace the per-case build+exec (the `go build` of the judge, the temp binary, the `exec.Command(bin, ...)`, and the `judge.json` read-back) with a direct `judge.Evaluate` call. Keep the gold-case table, the rep-count logic (good=1, bad=3), the confusion matrix, and the false-PASS/false-FAIL/abstention accounting unchanged. Replace the entire file with:
```go
// Command judge_eval runs the expectation judge against gold fixtures.
//
// Usage: go run ./judge_eval   (from eval-harness/backend)
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"backend/internal/judge"
)

type goldCase struct {
	scenario         string
	variant          string // good | bad
	expectConformant bool
}

var cases = []goldCase{
	{"05-outage-resilience", "good", true},
	{"05-outage-resilience", "bad", false},
	{"07-wrong-vat", "good", true},
	{"07-wrong-vat", "bad", false},
	{"10-credential-expiry", "good", true},
	{"10-credential-expiry", "bad", false},
}

func unmetCount(r judge.Report) int {
	if r.Expectations == nil {
		return 0
	}
	n := 0
	for _, c := range r.Expectations.Criteria {
		if c.Verdict == "UNMET" {
			n++
		}
	}
	return n
}

func main() {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Fprintln(os.Stderr, "locating judge_eval source file")
		os.Exit(2)
	}
	evalDir := filepath.Dir(thisFile)   // .../backend/judge_eval
	backendDir := filepath.Dir(evalDir) // .../backend
	ehRoot := filepath.Dir(backendDir)  // .../eval-harness
	scenariosDir := filepath.Join(ehRoot, "scenarios")
	goldDir := filepath.Join(backendDir, "cmd", "judge", "testdata", "goldset")

	// matrix[expected][actual]; index 0 = conformant, 1 = NON-COMPLIANT.
	var matrix [2][2]int
	falsePass, falseFail, abstentionOnly, errs := 0, 0, 0, 0

	fmt.Printf("meta-eval: %d gold fixtures (good reps=1, bad reps=3)\n\n", len(cases))
	for _, c := range cases {
		scenario := filepath.Join(scenariosDir, c.scenario, "scenario.json")
		work := filepath.Join(goldDir, c.scenario, c.variant)
		reps := 1
		if !c.expectConformant {
			reps = 3
		}
		for r := 0; r < reps; r++ {
			rep, code, err := judge.Evaluate(judge.Options{
				ScenarioPath:   scenario,
				IntegrationDir: work,
				Expect:         true,
			}, io.Discard)
			if code == 2 {
				errs++
				fmt.Printf("ERROR  %-22s/%-4s rep%d (exit 2: %v)\n", c.scenario, c.variant, r, err)
				continue
			}

			actualConformant := code == 0
			expectedIdx, actualIdx := 0, 0
			if !c.expectConformant {
				expectedIdx = 1
			}
			if !actualConformant {
				actualIdx = 1
			}
			matrix[expectedIdx][actualIdx]++

			label, mark := "conformant", "ok"
			if !actualConformant {
				label = "NON-COMPLIANT"
			}
			if c.variant == "bad" {
				switch {
				case actualConformant:
					falsePass++
					mark = "FALSE-PASS"
				case unmetCount(rep) == 0:
					abstentionOnly++
					mark = "caught-without-UNMET(abstention)"
				default:
					mark = fmt.Sprintf("caught(%d UNMET)", unmetCount(rep))
				}
			} else if !actualConformant {
				falseFail++
				mark = "FALSE-FAIL"
			}
			fmt.Printf("%-22s/%-4s rep%d -> %-13s [%s]\n", c.scenario, c.variant, r, label, mark)
		}
	}

	fmt.Printf("\nconfusion matrix (rows=expected, cols=actual):\n")
	fmt.Printf("                      actual:conformant   actual:NON-COMPLIANT\n")
	fmt.Printf("  expect:conformant         %3d                 %3d\n", matrix[0][0], matrix[0][1])
	fmt.Printf("  expect:NON-COMPLIANT      %3d                 %3d\n", matrix[1][0], matrix[1][1])
	fmt.Printf("\nfalse-PASS: %d   false-FAIL: %d   abstention-only catches: %d   errors: %d\n",
		falsePass, falseFail, abstentionOnly, errs)

	if falsePass > 0 || falseFail > 0 || abstentionOnly > 0 || errs > 0 {
		fmt.Println("\nMETA-EVAL FAILED")
		os.Exit(1)
	}
	fmt.Println("\nMETA-EVAL PASSED (good=conformant, bad=caught by active UNMET, zero false-PASS)")
}
```
Notes: this drops `encoding/json`, `os/exec`, and the temp-binary/`reportPath` plumbing; `goldDir` now derives directly from `backendDir` (no `judgeDir` build var). The `code == 2` branch replaces both the old "judge failed" and "exit 2" error cases.

- [ ] **Step 2: Build and vet**

```bash
cd /Users/stan/code/fsk/eval-harness/backend && go build ./... && go vet ./judge_eval
```
Expected: exits 0.

- [ ] **Step 3: Run the goldset meta-eval — the P2 correctness gate (needs `claude` CLI + token; makes ~12 model calls)**

```bash
cd /Users/stan/code/fsk/eval-harness/backend && go run ./judge_eval
```
Expected: the per-case lines (good → `conformant [ok]`, bad → `NON-COMPLIANT [caught(N UNMET)]`), a confusion matrix with all weight on the diagonal, `false-PASS: 0  false-FAIL: 0  abstention-only catches: 0  errors: 0`, and a final `META-EVAL PASSED` (exit 0). If it reports `META-EVAL FAILED`, the library lift changed judge behavior — stop and report; do not paper over it. If the `claude` CLI/token is genuinely unavailable, record SKIPPED with the reason — but this is the phase's gate, so flag it prominently as unverified.

- [ ] **Step 4: Confirm clean tree + committed build, then commit**

```bash
cd /Users/stan/code/fsk
git add -A && git status
git commit -m "refactor: judge_eval imports internal/judge instead of build+exec"
git push origin main
(cd eval-harness/backend && go build ./...) && echo "committed tree builds"
```
Expected: clean `git status` after commit; post-commit build exits 0.

---

## Self-Review

**Spec coverage (P2):** Spec P2 = "Export `internal/judge`; reduce `cmd/judge` to a thin wrapper; `judge_eval` imports the package. Verify the goldset meta-eval is green." Covered: the package + `Evaluate` (T1 S2–S4), the thin wrapper (T1 S5), `judge_eval` importing it (T2 S1), and the goldset gate (T2 S3). The single-`Evaluate`-export decision is a documented, narrower-than-prose realization; flagged in Design decisions.

**Placeholder scan:** `evaluate.go`, the wrapper `main.go`, and the `judge_eval` rewrite are given as complete files. `report.go` is a verbatim move of named declarations from the old `main.go` with one rename (`judgeReport`→`Report`) at three sites, plus its import list — concrete, with `grep`/`go build` as the catch. The two model-dependent steps (T1 S8 Docker e2e, T2 S3 meta-eval) explicitly handle unavailability as SKIPPED, and T2 S3 flags itself as the phase gate if skipped.

**Type/name consistency:** `Evaluate`/`Options`/`Report` (defined T1 S2–S3) are used identically in the wrapper (T1 S5: `judge.Evaluate`, `judge.Options`, `judge.Report`) and in `judge_eval` (T2 S1: same three). `Report.Expectations.Criteria[].Verdict` accessed in `judge_eval`'s `unmetCount` matches the `rubricReport`/`verdict` field names carried over in `rubric.go`. The `buildReport` signature (`checksReport`, `*rubricReport`) referenced in `evaluate.go` matches the declaration moved into `report.go`. Exit codes returned by `Evaluate` (0/1/2) match the wrapper's `os.Exit(code)` and `judge_eval`'s `code == 0`/`code == 2` checks.
