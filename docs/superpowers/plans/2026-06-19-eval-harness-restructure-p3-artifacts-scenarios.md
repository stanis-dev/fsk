# Eval harness restructure — Phase 3: artifacts + scenarios packages Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port the dashboard's run-summary derivation (`lib/runs.ts` + its parsers) into a Go `internal/artifacts` package and scenario discovery/validation (`lib/scenarios.ts`) into `internal/scenarios`, making the artifact-file contract single-sourced — the read/derive surface the server (P4) will expose.

**Architecture:** `internal/artifacts` owns the artifact filename constants, the run-summary/detail derivation, and the transcript/diff/telemetry parsers; it reuses `internal/judge.Report` for the judge report. The orchestrator's writer side imports the filename constants from `internal/artifacts` so the contract has one definition. `internal/scenarios` owns the scenario `Config` type, discovery, validation, and id-assignment; the orchestrator uses it for discovery. Faithful ports of the TS logic, proven by Go table tests re-expressing the existing TS test cases. No behavior change to existing runs.

**Tech Stack:** Go 1.23 (stdlib). The TS modules under `eval-harness/dashboard/lib/` are the canonical reference for the ports.

**Spec:** `docs/superpowers/specs/2026-06-19-eval-harness-restructure-design.md` (Sequencing → P3). P0–P2 complete.

## Global Constraints

- No AI/Claude attribution in commits, code, or docs. No `Co-Authored-By` trailer, no generated-with footer.
- Commit and push directly to `main`. Commit-message style: lowercase prefix (`refactor:`, `feat:`, `docs:`).
- `mcp/`, `research/`, `memo/` stay at the repo root; Docker build context stays the repo root.
- System-under-test data is not modified: `scenarios/`, `pos/`, per-fixture `go.mod`.
- Backend module stays stdlib-only (no `require`). `internal/` packages stay disjoint: `artifacts`/`scenarios` must not import `orchestrator`; no HTTP/SSE deps.
- Behavior preserved: the derivation rules must match `lib/runs.ts`/`lib/scenarios.ts` exactly (the ported tests are the guard). The dashboard's TS modules are NOT changed this phase — they keep working until P7; temporary Go/TS duplication is expected and intentional.
- **Process discipline:** after edits, confirm `git status` clean AND the committed tree builds; the reviewer treats a diff missing an expected edit as a finding.

## Design decisions (resolved)

- **Reuse `judge.Report`.** `internal/artifacts` imports `internal/judge`; `LoadRun`/`ParseJudgeReport` return `*judge.Report` (its JSON `{scenario,verdict,checks,expectations,note}` is exactly the dashboard's `JudgeReport`). No duplicate report type. (`artifacts → judge`, no cycle.)
- **Single-sourced filenames.** The artifact filename constants live in `internal/artifacts`; the orchestrator's writer imports them (replacing its string literals). `cancelled` and `run.json` are part of the contract even though the orchestrator writes `run.json` and the dashboard writes `cancelled`.
- **The TS source is canonical.** The implementer ports line-by-line from the named `lib/*.ts` files (read them directly); the rule checklists below are acceptance criteria, not a re-spec. Tests are ported from the named `*.test.ts` files.
- **Self-contained Go tests.** Go tests build their fixtures with `t.TempDir()` + written files (mirroring the TS tests); they do NOT read the dashboard's `__fixtures__`.

## Target layout after this phase

```
eval-harness/backend/internal/
  artifacts/              NEW package artifacts
    files.go              filename constants (single source of the file contract)
    types.go             Summary, RunDetail, TranscriptEvent, DiffLine, TelemetryEvent, TelemetryToolStat, TelemetrySummary
    runs.go              ListRuns, SummarizeRun, LoadRun, ParseJudgeReport (+ unexported parseResult/logInfo/readMeta)
    transcript.go        ParseTranscript (+ summarizeTool)
    diff.go              ClassifyDiff
    telemetry.go         ParseTelemetry, SummarizeTelemetry
    *_test.go            ported from dashboard/lib/{runs,transcript,diff,telemetry}.test.ts
  scenarios/             NEW package scenarios
    scenarios.go         Config type, Discover, List, Load, Validate, AssignExpectationIds, IsKnown
    scenarios_test.go    ported from dashboard/lib/scenarios.test.ts
  orchestrator/          MODIFIED: import artifacts filename constants; use scenarios.Discover
```

---

### Task 1: `internal/artifacts` — filename contract + run derivation + parsers

Create the artifacts package (filename constants, types, run reading, parsers) with tests ported from the dashboard, then point the orchestrator's writer at the shared constants. Deliverable: the package builds, its ported tests pass, and the orchestrator still builds/tests/e2e-runs unchanged.

**Files:**
- Create: `internal/artifacts/{files,types,runs,transcript,diff,telemetry}.go`
- Create: `internal/artifacts/{runs,transcript,diff,telemetry}_test.go`
- Modify: `internal/orchestrator/artifacts.go`, `internal/orchestrator/docker.go` (use `artifacts.*` filename constants)

**Interfaces produced (consumed by the server in P4, and the orchestrator now):**
- Filename constants (exact values — these ARE the contract):
  - `MetaFile = "meta.json"`, `RunHandleFile = "run.json"`, `BuildFile = "build.txt"`, `TestFile = "test.txt"`, `JudgeLogFile = "judge.txt"`, `DiffFile = "changes.diff"`, `TranscriptFile = "transcript.jsonl"`, `CoderErrFile = "claude.err"`, `TelemetryFile = "mcp-telemetry.jsonl"`, `JudgeJSONFile = "judge.json"`, `CancelledFile = "cancelled"`
- `func ListRuns(dir string) []Summary`
- `func SummarizeRun(dir string) Summary`
- `func LoadRun(baseDir, id string) (*RunDetail, bool)` (ok=false on bad id / not a dir)
- `func ParseJudgeReport(jsonText string) *judge.Report`
- `func ParseTranscript(jsonl string) []TranscriptEvent`
- `func ClassifyDiff(raw string) []DiffLine`
- `func ParseTelemetry(jsonl string) []TelemetryEvent`
- `func SummarizeTelemetry(events []TelemetryEvent) TelemetrySummary`
- The types in `types.go` (see Step 2).

- [ ] **Step 1: Filename constants**

Create `internal/artifacts/files.go` with `package artifacts` and the eleven constants listed above (exact string values). Add a one-line doc comment: these are the single source of the run artifact filenames, shared by writer (orchestrator) and reader.

- [ ] **Step 2: Types**

Create `internal/artifacts/types.go` (`package artifacts`) with Go structs mirroring `dashboard/lib/types.ts` exactly, with JSON tags matching the TS field names so the server's JSON equals what the dashboard expects. Required structs + fields + json tags (verbatim names from `types.ts`):
- `Summary`: `id`,`createdIso`,`status`,`scenario`,`coder`,`harness`,`model`,`effort`,`build`,`tests`,`judge`,`turns`,`cost` — all `string`.
- `TranscriptEvent`: `kind` string, `text` string.
- `DiffLine`: `cls` string, `text` string.
- `TelemetryEvent`: `ts`,`sessionId`,`tool` strings; `args map[string]any`; `resultCount int`; `isError bool`; `error` string; `latencyMs int` (json tags: `ts`,`sessionId`,`tool`,`args`,`resultCount`,`isError`,`error`,`latencyMs`).
- `TelemetryToolStat`: `tool` string, `calls` int, `errors` int.
- `TelemetrySummary`: `total` int, `errors` int, `byTool []TelemetryToolStat`, `p50LatencyMs` int, `p95LatencyMs` int, `queries []string`, `docsFetched []string`.
- `RunDetail`: `summary Summary`, `judgeLog string`, `judgeReport *judge.Report` (json `judgeReport`), `buildLog string`, `testLog string`, `err string`, `transcript []TranscriptEvent`, `diff []DiffLine`, `telemetry TelemetrySummary`.
(Import `backend/internal/judge` for `judge.Report`.)

- [ ] **Step 3: Parsers — port transcript.go, diff.go, telemetry.go**

Port from `dashboard/lib/transcript.ts`, `diff.ts`, `telemetry.ts` (read them as the canonical reference). Acceptance rules (must match the TS):
- `ParseTranscript`: split on `\n`, skip blanks; `assistant` → thinking/assistant/tool events (tool text via `summarizeTool`); `user` tool_result → `result` event, flatten content, prefix `"error: "` when `is_error`, truncate to 600; `result` with non-empty string → `final`. `summarizeTool` cases exactly per `transcript.ts:41-74` (Bash/Read/Write/Edit/MultiEdit/Grep/Glob/TodoWrite/WebFetch/WebSearch/Task/Agent/ToolSearch/default; `oneLine` collapses whitespace + truncates 300; default JSON-stringifies + truncates 200). Use a `bufio.Scanner` with a large buffer for big lines.
- `ClassifyDiff`: empty/whitespace → `[]`; classify each line: `meta` for `diff `/`index `/`+++`/`---` (checked BEFORE `+`/`-`), `hunk` for `@@`, `add` for `+`, `del` for `-`, else `ctx`.
- `ParseTelemetry`: split on `\n`, skip blank/malformed; map snake_case→fields (`ts`,`session_id`→sessionId,`tool`,`args` default `{}`,`result_count`→resultCount default 0,`is_error`→isError,`error`,`latency_ms`→latencyMs default 0). `SummarizeTelemetry`: per-tool calls/errors sorted desc by calls; p50/p95 via `idx = min(len-1, floor(p/100*len))` on sorted ascending latencies (0 if empty); `queries` from `search_fiskaly_docs` args.query (deduped, insertion order), `docsFetched` from `fetch_fiskaly_doc` args.id (deduped).

- [ ] **Step 4: Run derivation — port runs.go**

Port from `dashboard/lib/runs.ts`. Acceptance rules (must match exactly):
- `ParseJudgeReport(jsonText) *judge.Report`: empty/whitespace → nil; parse failure → nil; verdict not in {`conformant`,`NON-COMPLIANT`} → nil; if expectations non-null and (missing or criteria not an array) → nil; else the report. (Unmarshal into `judge.Report`.)
- `SummarizeRun(dir)`: init status `running`; `model = log.model || meta.model`; `coder = log.ccver!="" ? "claude-code" : meta.coder || "?"`; `harness = log.cwd=="/work" ? "docker" : log.cwd!="" ? "local" : meta.harness||"?"`; `scenario = meta.scenario||"-"`; `effort = meta.effort||"-"`; if `cancelled` file exists → status `cancelled`, return; if `judge.txt` empty → return (status running); else status `done`; `judge` = report? (conformant→PASS else FAIL) : ""; `build` = build.txt trimmed=="" ? PASS : FAIL; `tests` = test.txt non-empty && contains "ok" && !contains "FAIL" ? PASS : FAIL; turns/cost from `parseResult`. `createdIso` from the dir mtime (RFC3339/ISO; match the TS `toISOString()` shape — UTC, e.g. `2006-01-02T15:04:05.000Z`).
- `parseResult`: last JSONL line with `type=="result"`; `turns = round(num_turns)` as string; `cost = "$"+fmt("%.2f", total_cost_usd)`.
- `logInfo`: first line with `type=="system"` → model/cwd/claude_code_version (else empty).
- `readMeta`: parse `meta.json` → harness/coder/model/effort/scenario (empty on error).
- `ListRuns(dir)`: entries starting `run.` that are dirs → `SummarizeRun`; sort newest-first by `createdIso` desc.
- `LoadRun(baseDir,id)`: reject id not starting `run.` or containing `/`/`..`; stat must be a dir → false; else build `RunDetail` reading judge.txt/judge.json/build.txt/test.txt/claude.err and `ParseTranscript`/`ClassifyDiff`/`SummarizeTelemetry(ParseTelemetry(...))`.

- [ ] **Step 5: Port the tests**

Create the `_test.go` files porting every case from `dashboard/lib/{runs,transcript,diff,telemetry}.test.ts` (the inventory lists them; read the `.test.ts` files for exact inputs/expectations). Build run-dir fixtures with `t.TempDir()` + written artifact files. Must include: the done-run summary derivation (status/judge/build/tests/harness/model/coder/turns/cost), the cancelled-precedence case (cancelled file present alongside judge.txt → status cancelled), `ParseJudgeReport` null/garbage/expectations-shape cases, `LoadRun` transcript+diff+judgeReport, transcript event ordering + tool_result error prefix + Bash formatting, diff classification incl. `---`/`+++` as meta and empty→none, telemetry snake_case mapping + skip-malformed + aggregation (p50/p95/queries/docsFetched).

- [ ] **Step 6: Point the orchestrator writer at the shared constants**

In `internal/orchestrator/artifacts.go` and `docker.go`, replace the artifact filename string literals with the `artifacts.*` constants (import `backend/internal/artifacts`): `writeObserveArtifacts` map keys (`build.txt`/`test.txt`/`judge.txt`/`changes.diff`), `writeMeta` (`meta.json`), `writeRunHandle` (`run.json`), and in `docker.go` the `transcript.jsonl`, `claude.err`, and the `mcp-telemetry.jsonl` rename target. Do NOT change behavior — same filenames, now from constants. (The `/work/mcp-telemetry.jsonl` source path inside the container stays a literal; only the run-dir destination uses the constant.)

- [ ] **Step 7: Build, test, and confirm the orchestrator is unchanged in behavior**

```bash
cd /Users/stan/code/fsk/eval-harness/backend
go build ./...
go test ./internal/artifacts/... ./internal/orchestrator/... -short
```
Expected: build exits 0; `internal/artifacts` tests pass; orchestrator unit tests still pass (the filename-constant swap changed nothing observable).

- [ ] **Step 8: Docker e2e (requires Docker + token) — confirm artifacts unchanged**

```bash
cd /Users/stan/code/fsk/eval-harness/backend && go run ./cmd/eval-harness run 01
```
Expected: `01-zero-to-receipt ... judge=...` and a run dir with the same artifact filenames as before. If Docker/token unavailable, record SKIPPED.

- [ ] **Step 9: Clean tree + committed build, then commit**

```bash
cd /Users/stan/code/fsk
git add -A && git status
git commit -m "feat: add internal/artifacts (run derivation + file contract); orchestrator uses shared filename constants"
git push origin main
(cd eval-harness/backend && go build ./...) && echo "committed tree builds"
```

---

### Task 2: `internal/scenarios` — discovery + validation

Port scenario discovery, the `Config` type, validation, and id-assignment into `internal/scenarios`; have the orchestrator use it for discovery. Deliverable: the package builds, ported tests pass, orchestrator still discovers + runs.

**Files:**
- Create: `internal/scenarios/scenarios.go`, `internal/scenarios/scenarios_test.go`
- Modify: `internal/orchestrator/baselines.go` (delegate discovery to `scenarios.Discover`) and `run.go`/`runall.go`/`orchestrator.go` as needed to use the shared scenario type

**Interfaces produced:**
- `type Config struct { ID, Title string; Traps []any; Judge JudgeSpec }` with JSON tags `id`,`title`,`traps`,`judge`; `JudgeSpec{ Checks json.RawMessage or typed; Expectations []Expectation }`; `Expectation{ ID, Expectation string }` (tags `id`,`expectation`). (Mirror `types.ts` `ScenarioConfig`; `traps` stays opaque `[]any`.)
- `type Scenario struct { ID, Dir, FixtureDir, ScenarioJSON string }` (the discovery result; replaces orchestrator's unexported `scenario`).
- `func Discover(scenariosDir string) ([]Scenario, error)` (numeric-prefix dir with `fixture/` + `scenario.json`, sorted, errors if none — same as orchestrator's current `discoverScenarios`).
- `func List(scenariosDir string) ([]Config, error)` (discover + parse each scenario.json; used by the server in P4).
- `func Load(scenariosDir, id string) (*Config, string, bool)` (config + task.md; ok=false if unknown).
- `func IsKnown(scenariosDir, id string) bool`.
- `func Validate(raw []byte) string` (returns "" if valid, else the error message — port `validateConfig` rules exactly).
- `func AssignExpectationIds(c Config) Config` (port `assignExpectationIds`: keep non-empty ids, fill empties with `e1`,`e2`,... skipping used, no mutation).

- [ ] **Step 1: Port the package**

Create `internal/scenarios/scenarios.go` (`package scenarios`). Port `Discover` from the orchestrator's `discoverScenarios`/`scenario` (baselines.go) — same regex `^[0-9]`, same `fixture/`+`scenario.json` requirement, sorted ascending, error if none. Port `List`/`Load`/`IsKnown`/`Validate`/`AssignExpectationIds` from `dashboard/lib/scenarios.ts` (canonical reference). `Validate` rules (exact, from `scenarios.ts:65-78`): non-object→"config must be an object"; id/title not string; traps not array; judge not object; judge.checks not object; judge.expectations not array; each expectation needs string id+expectation; and "judge must have at least one non-empty checks field or a non-empty expectations array" where non-empty checks = `groundedBeforeWrite==true` OR non-empty `toolsCalled` OR non-empty `docsFetched` OR `maxMcpErrors` is a number.

- [ ] **Step 2: Port the tests**

Create `internal/scenarios/scenarios_test.go` porting every case from `dashboard/lib/scenarios.test.ts` (read it). Build scenario dirs with `t.TempDir()`. Must include: discovery (numeric-prefix + fixture + scenario.json only), `IsKnown` gating (known true, unknown false, `../etc` false), `Load` config+task and null-for-unknown, discovery/parse error on malformed scenario.json, the full `Validate` accept/reject matrix (good→""; bad title/judge/traps/null; non-array expectations; empty-checks+empty-expectations rejected; only-checks accepted; only-expectations accepted), and `AssignExpectationIds` (preserve existing, fill empties without collision, no mutation).

- [ ] **Step 3: Switch the orchestrator to `scenarios.Discover`**

In `internal/orchestrator`, replace the local `discoverScenarios`/`filterScenarios`/`scenario` usage so discovery comes from `scenarios.Discover` (import `backend/internal/scenarios`). Keep `filterScenarios` where it is (it operates on the discovered list) but retype it to `[]scenarios.Scenario`. Update `run.go`/`runall.go`/`orchestrator.go` signatures that referenced the local `scenario` type to use `scenarios.Scenario` (field access: `.ID`, `.Dir`, `.FixtureDir`, `.ScenarioJSON`). Remove the now-duplicated `discoverScenarios`/`scenario` from `baselines.go`. Update `baselines_test.go`'s discovery test accordingly (or move that coverage to the scenarios package and drop it here).

- [ ] **Step 4: Build, test, e2e**

```bash
cd /Users/stan/code/fsk/eval-harness/backend
go build ./... && go test ./... -short
go run ./cmd/eval-harness run 03   # Docker + token; confirms discovery+run still work end to end
```
Expected: build + all unit tests green; the e2e run produces a normal run dir. SKIPPED note if Docker/token unavailable.

- [ ] **Step 5: Clean tree + committed build, then commit**

```bash
cd /Users/stan/code/fsk
git add -A && git status
git commit -m "feat: add internal/scenarios (discovery + validation); orchestrator uses it"
git push origin main
(cd eval-harness/backend && go build ./...) && echo "committed tree builds"
```

---

## Self-Review

**Spec coverage (P3):** Spec P3 = "Centralize the file contract + summary derivation (port lib/runs.ts) and scenario validation (port lib/scenarios.ts). Verify against existing run dirs." Covered: filename contract single-sourced (T1 S1, S6), run derivation + parsers ported with tests (T1 S2–S5), scenario discovery+validation ported with tests (T2). "Verify against existing run dirs" → the ported tests + the Docker e2e (T1 S8, T2 S4).

**Placeholder scan:** Filename constants and type structs are fully specified. The logic functions are specified as faithful ports of named TS files with exact acceptance rules + the ported test suite as the gate — concrete because the TS source is the canonical reference and every test case is enumerated. Model-dependent steps don't apply here (no LLM); Docker e2e steps handle unavailability as SKIPPED.

**Type/name consistency:** `artifacts.RunDetail.JudgeReport` is `*judge.Report` (T1 S2), matching `ParseJudgeReport`'s return (T1 S4) and reusing P2's exported type. The filename constants (T1 S1) are the exact strings the orchestrator writer swaps to (T1 S6) and the reader uses (T1 S4). `scenarios.Scenario` fields (`ID`/`Dir`/`FixtureDir`/`ScenarioJSON`, T2 interfaces) replace the orchestrator's unexported `scenario` fields (`id`/`dir`/`fixtureDir`/`scenarioJSON`) — the orchestrator switch (T2 S3) updates every field access accordingly. `internal/` packages stay disjoint: `artifacts` and `scenarios` import only `judge`/stdlib, never `orchestrator`.
