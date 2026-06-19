# Eval harness restructure — Phase 1: Consolidate into the `backend` module Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Collapse the separate `runner` and `judge` Go modules into one `backend` module (`cmd/eval-harness`, `cmd/judge`, `internal/orchestrator`, `judge_eval`), and replace the runner's structural directory discovery with explicit configuration — with no change to what is evaluated.

**Architecture:** The runner becomes `package orchestrator` (a library) under `backend/internal/orchestrator/`, driven by a thin `backend/cmd/eval-harness` CLI that resolves paths and calls an exported `orchestrator.Run(Config)`. The judge moves verbatim to `backend/cmd/judge/` (still `package main`, still consumed by `go build` + exec) with `judge_eval` alongside. The dashboard keeps its filesystem+spawn model but points at the new CLI location.

**Tech Stack:** Go 1.23 (stdlib only — no external deps in any module). Docker for the end-to-end gate. Next.js/pnpm for the one dashboard path edit.

**Spec:** `docs/superpowers/specs/2026-06-19-eval-harness-restructure-design.md` (Sequencing → P1). P0 (rename) is complete.

## Global Constraints

(Same as the whole restructure; copied from the spec.)

- No AI/Claude attribution in commits, code, or docs. No `Co-Authored-By` trailer, no generated-with footer.
- Commit and push directly to `main` (solo exercise, no branches/PRs). Match the existing commit-message style (lowercase prefix like `refactor:`, `docs:`, `cleanup:`).
- `mcp/`, `research/`, `memo/` stay at the repo root. The Docker build context stays the repo root (`fsk/`).
- System-under-test data is not modified: `scenarios/` cases, the `pos/` reference seed, and every per-fixture `go.mod`.
- The backend module stays stdlib-only; `internal/` packages stay disjoint so no HTTP/SSE dependency ever enters the orchestrator's import graph (relevant from P4 onward).
- The judge's exit-code contract is load-bearing and unchanged: `0` conformant, `1` NON-COMPLIANT, `2` infra error. `cmd/judge` stays `package main` this phase (it becomes a library only in P2).

## Design decisions for this phase (resolved, not open)

- **Spec-faithful split:** the orchestrator is extracted into `internal/orchestrator` now (not deferred), because P4/P5 import it and doing it during the move avoids re-touching these files later.
- **Explicit config at the orchestrator boundary:** `orchestrator.Run` takes a `Config` with every path explicit — it performs NO discovery. The thin `cmd/eval-harness` composition root resolves those paths: from a `-root` flag, else by walking up from cwd for a directory containing both `scenarios/` and `backend/` (a NEW anchor that survives the consolidation, replacing the removed `scenarios/`+`judge/` anchor). The dashboard relies on this walk-up from the fixed CLI directory, which resolves deterministically to `eval-harness/`.
- **Judge moves but stays `package main`** under `cmd/judge`; the orchestrator still builds+execs it (the in-process library lift is P2).
- **Module name:** `backend` (bare, matching the repo's existing bare module names `runner`/`judge`/`pos`/`fiskaly-mcp`). Imports are `backend/internal/orchestrator`.

## Target layout after this phase

```
eval-harness/
  dashboard/                         (unchanged except lib/paths.ts runnerDir default)
  backend/                           NEW: one module `backend`, go 1.23
    go.mod
    cmd/
      eval-harness/main.go           NEW thin CLI: resolves config, calls orchestrator.Run
      judge/                         MOVED from eval-harness/judge/ (package main, unchanged logic)
        checks.go rubric.go trajectory.go main.go + *_test.go
        testdata/goldset/...
    internal/
      orchestrator/                  MOVED from eval-harness/runner/ (package main -> package orchestrator)
        config.go docker.go baselines.go artifacts.go observe.go run.go runall.go
        orchestrator.go              NEW: Config + Run
        *_test.go                    (moved; package main -> package orchestrator; discovery tests deleted)
    judge_eval/                      MOVED from eval-harness/judge/judge_eval/ (path derivation fixed)
  scenarios/  pos/  evals/           UNCHANGED
# removed: eval-harness/runner/ , eval-harness/judge/
```

---

### Task 1: Consolidate runner + judge into the `backend` module

The full Go consolidation. This is one atomic task: the moment the judge moves, the runner's judge-build path and structural discovery must change in the same step, so there is no green intermediate to split on. Deliverable: the `backend` module builds, all unit tests pass, and a headless Docker run produces artifacts.

**Files:**
- Create: `eval-harness/backend/go.mod`
- Create: `eval-harness/backend/cmd/eval-harness/main.go`
- Create: `eval-harness/backend/internal/orchestrator/orchestrator.go`
- Move: `eval-harness/judge/{checks,rubric,trajectory,main}.go` + `*_test.go` → `eval-harness/backend/cmd/judge/`
- Move: `eval-harness/judge/testdata/` → `eval-harness/backend/cmd/judge/testdata/`
- Move: `eval-harness/judge/judge_eval/` → `eval-harness/backend/judge_eval/`
- Move: `eval-harness/runner/*.go` (non-`main.go`) + `*_test.go` → `eval-harness/backend/internal/orchestrator/`
- Modify: `eval-harness/backend/internal/orchestrator/config.go` (export consts)
- Modify: `eval-harness/backend/internal/orchestrator/docker.go` (`simsRoot` field → `dockerfilePath`)
- Modify: `eval-harness/backend/judge_eval/main.go` (path derivation)
- Modify: moved test files (`run_test.go`, `runall_test.go`, `integration_test.go`, `baselines_test.go`)
- Delete: `eval-harness/runner/` (incl. `go.mod`), `eval-harness/judge/` (incl. `go.mod`, `.gitignore`), and the old `runner/main.go`

**Interfaces:**
- Produces (consumed by `cmd/eval-harness` now, and by the server in P4/P5):
  - `package orchestrator`
  - `const DefaultModel = "claude-sonnet-4-6"`, `const DefaultEffort = "medium"`
  - `type Config struct { ScenariosDir, JudgeDir, RepoRoot, DockerfilePath, RunsBase, Image, Model, Effort string; IDs []string; Out io.Writer }`
  - `func Run(cfg Config) (int, error)` — returns the process exit code (0/1 from `runAll`, 2 on harness error) and a non-nil error on harness failure.

- [ ] **Step 1: Create the backend module file**

Create `eval-harness/backend/go.mod`:
```
module backend

go 1.23
```

- [ ] **Step 2: Move the judge into `cmd/judge` and `judge_eval`**

```bash
cd /Users/stan/code/fsk
mkdir -p eval-harness/backend/cmd/judge
git mv eval-harness/judge/checks.go eval-harness/judge/checks_test.go \
       eval-harness/judge/rubric.go eval-harness/judge/rubric_test.go \
       eval-harness/judge/trajectory.go eval-harness/judge/trajectory_test.go \
       eval-harness/judge/main.go eval-harness/judge/main_test.go \
       eval-harness/backend/cmd/judge/
git mv eval-harness/judge/testdata eval-harness/backend/cmd/judge/testdata
git mv eval-harness/judge/judge_eval eval-harness/backend/judge_eval
git rm eval-harness/judge/go.mod eval-harness/judge/.gitignore
```
The judge files stay `package main` — do not edit their package declarations.

- [ ] **Step 3: Fix `judge_eval`'s path derivation**

In `eval-harness/backend/judge_eval/main.go`, replace the `runtime.Caller` chain (the block starting `evalDir := filepath.Dir(thisFile)` through `goldDir := ...`):
```go
	evalDir := filepath.Dir(thisFile)
	judgeDir := filepath.Dir(evalDir)
	simsDir := filepath.Dir(judgeDir)
	scenariosDir := filepath.Join(simsDir, "scenarios")
	goldDir := filepath.Join(judgeDir, "testdata", "goldset")
```
with:
```go
	evalDir := filepath.Dir(thisFile)   // .../backend/judge_eval
	backendDir := filepath.Dir(evalDir) // .../backend
	ehRoot := filepath.Dir(backendDir)  // .../eval-harness
	judgeDir := filepath.Join(backendDir, "cmd", "judge")
	scenariosDir := filepath.Join(ehRoot, "scenarios")
	goldDir := filepath.Join(judgeDir, "testdata", "goldset")
```
Also update the usage comment near the top from `(from eval-harness/judge)` to `(from eval-harness/backend)`.

- [ ] **Step 4: Move the runner into `internal/orchestrator` (drop the old `main.go`)**

```bash
cd /Users/stan/code/fsk
mkdir -p eval-harness/backend/internal/orchestrator
git mv eval-harness/runner/config.go eval-harness/runner/docker.go \
       eval-harness/runner/baselines.go eval-harness/runner/artifacts.go \
       eval-harness/runner/observe.go eval-harness/runner/run.go eval-harness/runner/runall.go \
       eval-harness/runner/config_test.go eval-harness/runner/docker_test.go \
       eval-harness/runner/baselines_test.go eval-harness/runner/artifacts_test.go \
       eval-harness/runner/observe_test.go eval-harness/runner/run_test.go \
       eval-harness/runner/runall_test.go eval-harness/runner/integration_test.go \
       eval-harness/backend/internal/orchestrator/
git rm eval-harness/runner/main.go eval-harness/runner/go.mod
```
`main.go` is intentionally not moved — its `main`/`usage`/`cmdRun`/`findSimsRoot`/`isSimsDir` are replaced by `cmd/eval-harness` + `orchestrator.Run`.

- [ ] **Step 5: Rename the package in every moved orchestrator file**

In all 14 files now under `eval-harness/backend/internal/orchestrator/` (both non-test and `_test.go`), change the first line from:
```go
package main
```
to:
```go
package orchestrator
```

- [ ] **Step 6: Export the model/effort consts**

In `eval-harness/backend/internal/orchestrator/config.go`, rename the consts:
```go
const (
	defaultModel  = "claude-sonnet-4-6"
	defaultEffort = "medium"
)
```
to:
```go
const (
	DefaultModel  = "claude-sonnet-4-6"
	DefaultEffort = "medium"
)
```
(There are no other references to the unexported names once `main.go` is gone; `cmd/eval-harness` will use the exported names.)

- [ ] **Step 7: Replace `dockerAgent`'s `simsRoot` field with `dockerfilePath`**

In `eval-harness/backend/internal/orchestrator/docker.go`, change the struct:
```go
type dockerAgent struct {
	repoRoot string
	simsRoot string
	context  string
	image    string
}
```
to:
```go
type dockerAgent struct {
	repoRoot       string
	dockerfilePath string
	context        string
	image          string
}
```
and in `(dockerAgent).run`, change the build command's `-f` argument:
```go
		"-f", filepath.Join(a.simsRoot, "evals", "Dockerfile"),
```
to:
```go
		"-f", a.dockerfilePath,
```

- [ ] **Step 8: Add the orchestrator entry point**

Create `eval-harness/backend/internal/orchestrator/orchestrator.go`:
```go
package orchestrator

import (
	"io"
	"os"
)

// Config is the fully-resolved input to a run batch. The orchestrator performs
// no path discovery; every location is supplied explicitly by the caller.
type Config struct {
	ScenariosDir   string
	JudgeDir       string
	RepoRoot       string
	DockerfilePath string
	RunsBase       string
	Image          string
	Model          string
	Effort         string
	IDs            []string
	Out            io.Writer
}

// Run discovers scenarios under cfg.ScenariosDir (optionally filtered by
// cfg.IDs), builds the judge from cfg.JudgeDir, and runs each scenario through
// the Docker pipeline. It returns the batch exit code (0 all ran, 1 some
// failed) or 2 with a non-nil error on a harness-level failure before the batch.
func Run(cfg Config) (int, error) {
	ctx := dockerContext()
	if err := checkBinaries("docker", "go", "git"); err != nil {
		return 2, err
	}
	if err := dockerReachable(ctx); err != nil {
		return 2, err
	}
	rc, err := loadConfig(cfg.RepoRoot, cfg.Model, cfg.Effort)
	if err != nil {
		return 2, err
	}
	scenarios, err := discoverScenarios(cfg.ScenariosDir)
	if err != nil {
		return 2, err
	}
	if len(cfg.IDs) > 0 {
		scenarios, err = filterScenarios(scenarios, cfg.IDs)
		if err != nil {
			return 2, err
		}
	}
	tempDir, err := os.MkdirTemp("", "runner-judge-")
	if err != nil {
		return 2, err
	}
	judgeBin, err := buildJudge(cfg.JudgeDir, tempDir)
	if err != nil {
		return 2, err
	}
	ag := dockerAgent{repoRoot: cfg.RepoRoot, dockerfilePath: cfg.DockerfilePath, context: ctx, image: cfg.Image}
	return runAll(scenarios, cfg.RunsBase, judgeBin, ag, rc, cfg.Out), nil
}
```

- [ ] **Step 9: Add the thin CLI**

Create `eval-harness/backend/cmd/eval-harness/main.go`:
```go
// Command eval-harness runs scenarios through the eval pipeline.
//
// Usage: eval-harness run [-root dir] [-model m] [-effort e] [ids...]
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"backend/internal/orchestrator"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "run" {
		fmt.Fprintln(os.Stderr, "usage: eval-harness run [-root dir] [-model m] [-effort e] [ids...]")
		os.Exit(2)
	}
	os.Exit(cmdRun(os.Args[2:]))
}

func cmdRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	root := fs.String("root", "", "eval-harness root (dir with scenarios/ and backend/); default: discovered from cwd")
	model := fs.String("model", orchestrator.DefaultModel, "coder model")
	effort := fs.String("effort", orchestrator.DefaultEffort, "coder effort")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	ehRoot, err := resolveRoot(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "eval-harness:", err)
		return 2
	}

	code, err := orchestrator.Run(orchestrator.Config{
		ScenariosDir:   filepath.Join(ehRoot, "scenarios"),
		JudgeDir:       filepath.Join(ehRoot, "backend", "cmd", "judge"),
		RepoRoot:       filepath.Dir(ehRoot),
		DockerfilePath: filepath.Join(ehRoot, "evals", "Dockerfile"),
		RunsBase:       filepath.Join(os.Getenv("HOME"), ".cache", "fiskaly-eval"),
		Image:          "fiskaly-eval",
		Model:          *model,
		Effort:         *effort,
		IDs:            fs.Args(),
		Out:            os.Stdout,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "eval-harness:", err)
	}
	return code
}

// resolveRoot returns the eval-harness root: the -root flag if set, else the
// nearest ancestor of cwd containing both scenarios/ and backend/.
func resolveRoot(flagRoot string) (string, error) {
	if flagRoot != "" {
		return filepath.Abs(flagRoot)
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := wd; ; {
		if isDir(filepath.Join(dir, "scenarios")) && isDir(filepath.Join(dir, "backend")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not locate eval-harness root (a dir with scenarios/ and backend/) from %s; pass -root", wd)
		}
		dir = parent
	}
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}
```
Note: `isDir` is defined locally here (the orchestrator's `isDir` is unexported and in a different package). This is a deliberate small duplication of a 3-line stdlib wrapper across a package boundary, not shared logic worth exporting.

- [ ] **Step 10: Delete the two discovery tests**

In `eval-harness/backend/internal/orchestrator/baselines_test.go`, delete the entire `func TestFindSimsRoot(...)` and the entire `func TestIsSimsDir_RequiresBoth(...)` — the functions they test (`findSimsRoot`, `isSimsDir`) no longer exist. Leave the other tests in the file (`TestDiscoverScenarios`, `TestCopyDir`, and the `runGoCmd` tests) untouched.

- [ ] **Step 11: Fix the path setup in the three integration-style tests**

These tests previously computed `simsRoot := filepath.Abs("..")` (which resolved to `eval-harness/` from `runner/`). From the new location `backend/internal/orchestrator/`, the eval-harness root is `filepath.Abs("../../..")`. Apply, in each file:

`run_test.go` and `runall_test.go`: replace the `simsRoot := filepath.Abs("..")` setup and its uses so that:
```go
	ehRoot, err := filepath.Abs("../../..")
	// judge build dir:   filepath.Join(ehRoot, "backend", "cmd", "judge")
	// scenarios dir:     filepath.Join(ehRoot, "scenarios")
```
i.e. `buildJudge(filepath.Join(ehRoot, "backend", "cmd", "judge"), t.TempDir())` and `discoverScenarios(filepath.Join(ehRoot, "scenarios"))`.

`integration_test.go`: same `ehRoot := filepath.Abs("../../..")`, then:
```go
	// scenariosDir:    filepath.Join(ehRoot, "scenarios")
	// repoRoot:        filepath.Dir(ehRoot)
	// judge build dir: filepath.Join(ehRoot, "backend", "cmd", "judge")
	// dockerfile:      filepath.Join(ehRoot, "evals", "Dockerfile")
```
and change the `dockerAgent` struct literal from `dockerAgent{repoRoot: repoRoot, simsRoot: simsRoot, context: ..., image: ...}` to `dockerAgent{repoRoot: repoRoot, dockerfilePath: filepath.Join(ehRoot, "evals", "Dockerfile"), context: ..., image: ...}`.

- [ ] **Step 12: Build and run unit tests**

```bash
cd /Users/stan/code/fsk/eval-harness/backend
go build ./...
go test ./... -short
```
Expected: `go build` exits 0; `go test` reports `ok` for `backend/internal/orchestrator` and `backend/cmd/judge` (the judge's deterministic unit tests), with the Docker integration test skipped under `-short`. (The `judge_eval` goldset meta-eval is NOT run here — it needs the `claude` CLI + auth; it is exercised in P2.)

- [ ] **Step 13: Verify the judge_eval and eval-harness packages compile**

```bash
cd /Users/stan/code/fsk/eval-harness/backend
go vet ./cmd/eval-harness ./judge_eval
```
Expected: exits 0 (both `package main` entry points compile and their path code type-checks).

- [ ] **Step 14: Headless end-to-end run (requires Docker + token in `.env`)**

The real proof the consolidation works: explicit-config resolution, judge build at the new path, and the Docker pipeline.
```bash
cd /Users/stan/code/fsk/eval-harness/backend && go run ./cmd/eval-harness run 01
```
Expected: a line `01-zero-to-receipt        run=run.XXXXXX judge=...` and a new run dir under `~/.cache/fiskaly-eval/run.*` with `meta.json`, `transcript.jsonl`, `build.txt`, `judge.txt`, `judge.json`. If Docker or the token is unavailable, record this step as SKIPPED with the reason — do not claim it passed; do not treat it as a blocker.

- [ ] **Step 15: Commit**

```bash
cd /Users/stan/code/fsk && git add -A && git commit -m "refactor: consolidate runner and judge into the backend module" && git push origin main
```
Expected: `git show --stat HEAD` shows the moves into `backend/` plus the new `go.mod`, `orchestrator.go`, `cmd/eval-harness/main.go`, and the edits; `eval-harness/runner/` and `eval-harness/judge/` are gone.

---

### Task 2: Repoint the dashboard, Docker context, and docs at the new layout

With the runner now at `backend/cmd/eval-harness`, update the dashboard's spawn target, the Docker build-context ignore rules, the binary ignores, and the docs. Kept separate from Task 1 so the Go consolidation can be reviewed on its own. Deliverable: the dashboard can trigger a run against the new CLI, the Docker image builds with the consolidated source excluded from the context, and a full e2e run passes.

**Files:**
- Modify: `eval-harness/dashboard/lib/paths.ts` (`runnerDir` default)
- Modify: `.dockerignore`
- Modify: `.gitignore`
- Modify: docs that reference the old `runner/`/`judge/` paths (sweep — at least `README.md`, `eval-harness/scenarios/AUTHORING.md`, `eval-harness/scenarios/README.md`, `eval-harness/evals/dashboard.sh`)

**Interfaces:**
- Consumes: `backend/cmd/eval-harness` from Task 1 (the `go run .` target) and its walk-up root resolution.

- [ ] **Step 1: Repoint the dashboard at the new CLI**

The dashboard's `runScenario` action runs `go run . run <id>` with `cwd = runnerDir()`. The walk-up in `cmd/eval-harness` resolves the eval-harness root deterministically from that fixed directory, so only the directory needs to change.

In `eval-harness/dashboard/lib/paths.ts`, change `runnerDir`:
```ts
export function runnerDir(): string {
  return process.env.FISKALY_RUNNER_DIR ?? path.resolve(process.cwd(), "..", "runner");
}
```
to:
```ts
export function runnerDir(): string {
  return process.env.FISKALY_RUNNER_DIR ?? path.resolve(process.cwd(), "..", "backend", "cmd", "eval-harness");
}
```
`scenariosDir()` and `runsDir()` are unchanged (`scenarios/` did not move; the cache dir is absolute). `app/actions.ts` is unchanged — `spawn("go", ["run", ".", "run", scenarioId], { cwd: runnerDir() })` still works against the new `main.go`.

- [ ] **Step 2: Update the Docker build-context ignores**

The consolidated Go source now lives under `eval-harness/backend/`; keep it out of the build context (only `mcp/` and `eval-harness/evals/docker-entrypoint.sh` are needed).

In `.dockerignore`, replace the line:
```
eval-harness/judge/
```
with:
```
eval-harness/backend/
```
(Leave `eval-harness/pos/` and `eval-harness/dashboard/`.)

- [ ] **Step 3: Update the built-binary ignores**

In `.gitignore`, replace lines:
```
/eval-harness/judge/judge
/eval-harness/runner/runner
```
with:
```
/eval-harness/backend/cmd/judge/judge
/eval-harness/backend/cmd/eval-harness/eval-harness
/eval-harness/backend/judge_eval/judge_eval
```
(The third covers the known `go build ./judge_eval` name-collision binary; the meta-eval otherwise builds into `os.TempDir()`.)

- [ ] **Step 4: Sweep docs for the old runner/judge paths**

Run:
```bash
cd /Users/stan/code/fsk && grep -rn 'eval-harness/runner\|eval-harness/judge\|cd .*runner\|sims/runner\|sims/judge' \
  --include='*.md' --include='*.sh' --exclude-dir=node_modules --exclude-dir=.next --exclude-dir=.git --exclude-dir=docs .
```
For each hit that names the runner invocation or the runner/judge directories, update it to the new layout: `cd eval-harness/runner && go run . run <id>` becomes `cd eval-harness/backend && go run ./cmd/eval-harness run <id>`; references to `eval-harness/judge/` become `eval-harness/backend/cmd/judge/`. Review each match in context; do not rewrite prose using "judge"/"runner" as concepts. (Planning artifacts under `docs/` are excluded — they are historical.)

- [ ] **Step 5: Verify the dashboard builds**

```bash
cd /Users/stan/code/fsk/eval-harness/dashboard && pnpm test run && pnpm build
```
Expected: vitest passes (the `paths.test.ts` suite still passes — confirm it does not assert the old `runner` path; if it does, update that assertion to the new path) and `pnpm build` completes.

- [ ] **Step 6: Full Docker end-to-end via the dashboard path (requires Docker + token)**

Confirm the dashboard's exact spawn works against the new CLI and the rebuilt image excludes the backend source:
```bash
cd /Users/stan/code/fsk/eval-harness/backend/cmd/eval-harness && go run . run 02
```
Expected: `02-provision-merchant     run=run.XXXXXX judge=...` and a populated run dir. (This runs from the dashboard's `cwd`, proving the walk-up resolves the root from there.) If Docker/token unavailable, record SKIPPED with reason.

- [ ] **Step 7: Commit**

```bash
cd /Users/stan/code/fsk && git add -A && git commit -m "refactor: repoint dashboard, docker context, and docs at backend/" && git push origin main
```
Expected: commit on `main`, clean `git status`.

---

## Self-Review

**Spec coverage (P1):** Spec P1 = "Create the `backend` module; move runner into `internal/orchestrator` + `cmd/eval-harness`; replace structural discovery with explicit config; verify `go build ./...`, `go test ./...`, and a headless run." Covered: module (T1 S1), orchestrator move + package rename (T1 S4–S5), `cmd/eval-harness` (T1 S9), explicit `Config` with no in-orchestrator discovery + `findSimsRoot`/`isSimsDir` removed (T1 S8–S10), verification (T1 S12, S14). The judge move + `judge_eval`/test/docker path fixes (T1 S2–S3, S7, S11; T2 S2) are required side effects of the move that the spec's prose folds into "consolidate." Dashboard repoint (T2 S1) keeps the phase independently shippable (the spec's full FE rewrite is P7).

**Placeholder scan:** New files (`go.mod`, `orchestrator.go`, `cmd/eval-harness/main.go`) and the `docker.go`/`judge_eval`/`config.go` edits are given as complete code. The test path fixes (T1 S11) and doc sweep (T2 S4) are mechanical edits across files whose full bodies are not reproduced here; they are specified by exact target expressions and exact functions to delete, with `go test`/`go build` as the catch-all gate. The headless-run steps explicitly handle Docker-unavailable as SKIPPED, not a false pass.

**Type/name consistency:** `Config`/`Run`/`DefaultModel`/`DefaultEffort` (exported, T1 S6/S8) match their use in `cmd/eval-harness` (T1 S9). `dockerAgent.dockerfilePath` (T1 S7) matches the `integration_test.go` literal update (T1 S11) and the `DockerfilePath` config field threaded through `Run` → `dockerAgent`. The module name `backend` (T1 S1) matches the import path `backend/internal/orchestrator` (T1 S9). The dashboard `runnerDir` target `backend/cmd/eval-harness` (T2 S1) matches the CLI location (T1 S9) and the `.gitignore` binary path (T2 S3).
