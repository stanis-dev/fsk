# Eval harness restructure — Phase 0: Rename `sims/` → `eval-harness/` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rename the `sims/` directory to `eval-harness/` and fix the path references that break, with zero behavior change, verified by the existing test suite plus a real Docker run.

**Architecture:** A pure rename + restructure-free move. The flat layout is preserved (`eval-harness/{dashboard,runner,judge,pos,scenarios,evals}`), so the runner's structural root discovery (`isSimsDir` looks for `scenarios/` + `judge/` siblings) keeps working and `dashboard/lib/paths.ts` needs no change. Only three functional references break: a fast-path literal in the runner, the Dockerfile COPY path, and stale ignore-file entries.

**Tech Stack:** Go (stdlib), Next.js (pnpm) — unchanged in this phase. Docker for the end-to-end check.

**Spec:** `docs/superpowers/specs/2026-06-19-eval-harness-restructure-design.md` (Sequencing → P0).

## Global Constraints

These apply to every task in every phase of this restructure (copied from the spec):

- No AI/Claude attribution in commits, code, or docs. No `Co-Authored-By` trailer, no generated-with footer.
- Commit and push directly to `main` (solo exercise, no branches/PRs). Match the existing commit-message style (lowercase prefix like `refactor:`, `docs:`, `cleanup:`).
- `mcp/`, `research/`, `memo/` stay at the repo root. The Docker build context stays the repo root (`fsk/`).
- System-under-test data is not modified by the restructure: `scenarios/` cases, the `pos/` reference seed, and every per-fixture `go.mod`.
- Touch only what the rename requires. Identifiers and comments that are scheduled for wholesale removal in a later phase are left as-is in this phase (called out where relevant), not renamed-then-deleted.

## Roadmap (later phases, each its own plan when reached)

P0 (this plan) Rename → P1 Consolidate into the `backend` module + explicit-config discovery → P2 Judge as a library → P3 Artifacts + scenarios packages → P4 Read endpoints → P5 Jobs (worker pool, cancel, reattach) → P6 SSE → P7 Frontend rewrite. See the spec's Sequencing section for each phase's deliverable and gate.

---

### Task 1: Rename the directory and fix functional path references

Renames `sims/` → `eval-harness/` and updates the three references that break at runtime, plus the two ignore files. Deliverable: every Go module builds, all unit tests pass, and a real Docker run produces artifacts.

**Files:**
- Move: `sims/` → `eval-harness/` (whole tree, including untracked `node_modules/` and `.next/`)
- Modify: `eval-harness/evals/Dockerfile` (line 24)
- Modify: `eval-harness/runner/main.go` (lines 106, 111)
- Modify: `eval-harness/runner/baselines_test.go` (line 41)
- Modify: `.dockerignore` (lines 7-9)
- Modify: `.gitignore` (lines 18-19)

**Interfaces:**
- Consumes: nothing (first task).
- Produces: the renamed tree at `eval-harness/` with the flat layout intact (`dashboard/`, `runner/`, `judge/`, `pos/`, `scenarios/`, `evals/` as direct children). Later phases assume this path.

- [ ] **Step 1: Establish a green baseline before any change**

Confirm the suite is green now, so any post-rename failure is attributable to this task.

Run:
```bash
cd /Users/stan/code/fsk/sims/runner && go build ./... && go test ./... -short
cd /Users/stan/code/fsk/sims/judge  && go build ./... && go test ./...
cd /Users/stan/code/fsk/sims/pos    && go build ./... && go test ./...
```
Expected: each ends with `ok` / `PASS`, exit 0. If anything is already red, stop and report — do not proceed.

- [ ] **Step 2: Move the directory**

Use a plain filesystem move (not `git mv`) so untracked `node_modules/` and `.next/` move with the tree; git detects the rename at commit time.

Run:
```bash
cd /Users/stan/code/fsk && mv sims eval-harness
```
Expected: `sims/` no longer exists; `eval-harness/` contains `dashboard runner judge pos scenarios evals`.

Verify:
```bash
ls /Users/stan/code/fsk/eval-harness
```
Expected: `dashboard  evals  judge  pos  runner  scenarios`

- [ ] **Step 3: Fix the Dockerfile COPY path**

The COPY path is relative to the build context (the repo root), so it must name the new directory. The `COPY mcp/` line above it is unchanged (`mcp/` stays at the repo root).

In `eval-harness/evals/Dockerfile` line 24, replace:
```dockerfile
COPY sims/evals/docker-entrypoint.sh /usr/local/bin/entrypoint.sh
```
with:
```dockerfile
COPY eval-harness/evals/docker-entrypoint.sh /usr/local/bin/entrypoint.sh
```

- [ ] **Step 4: Fix the runner's fast-path literal and error string**

`findSimsRoot` walks up looking for a dir with `scenarios/` + `judge/` siblings; its one-level fast-path checks a child literally named `sims`. Point it at the new name so invocation from the repo root still resolves. (The function names `findSimsRoot`/`isSimsDir` and the `simsRoot` variables are removed wholesale in P1; leave them here.)

In `eval-harness/runner/main.go` line 106, replace:
```go
		if nested := filepath.Join(dir, "sims"); isSimsDir(nested) {
```
with:
```go
		if nested := filepath.Join(dir, "eval-harness"); isSimsDir(nested) {
```

In `eval-harness/runner/main.go` line 111, replace:
```go
			return "", fmt.Errorf("could not locate sims/ (with scenarios/ and judge/) from %s", start)
```
with:
```go
			return "", fmt.Errorf("could not locate eval-harness/ (with scenarios/ and judge/) from %s", start)
```

- [ ] **Step 5: Update the test that exercises the fast-path literal**

`TestFindSimsRoot` builds a temp dir named `sims` to exercise the fast-path; it must match the new literal. Only the literal on line 41 changes — the local variable name `sims` and the comments stay, since this test and the discovery code it covers are removed in P1.

In `eval-harness/runner/baselines_test.go` line 41, replace:
```go
	sims := filepath.Join(root, "sims")
```
with:
```go
	sims := filepath.Join(root, "eval-harness")
```

- [ ] **Step 6: Fix the ignore files**

In `.dockerignore`, replace lines 7-9:
```
sims/judge/
sims/pos/
sims/dashboard/
```
with:
```
eval-harness/judge/
eval-harness/pos/
eval-harness/dashboard/
```

In `.gitignore`, replace lines 18-19:
```
/sims/judge/judge
/sims/runner/runner
```
with:
```
/eval-harness/judge/judge
/eval-harness/runner/runner
```

- [ ] **Step 7: Verify all modules build and unit tests pass**

Run:
```bash
cd /Users/stan/code/fsk/eval-harness/runner && go build ./... && go test ./... -short
cd /Users/stan/code/fsk/eval-harness/judge  && go build ./... && go test ./...
cd /Users/stan/code/fsk/eval-harness/pos    && go build ./... && go test ./...
cd /Users/stan/code/fsk/eval-harness/judge  && go build ./judge_eval
```
Expected: each `go test` ends with `ok` / `PASS`; the `judge_eval` build produces no output and exits 0. (Running the `judge_eval` meta-eval is deferred to P2; it needs the `claude` CLI + auth + LLM calls and P0 does not touch judge logic.)

- [ ] **Step 8: Verify root discovery specifically**

Run:
```bash
cd /Users/stan/code/fsk/eval-harness/runner && go test ./... -run 'TestFindSimsRoot|TestIsSimsDir|TestDiscoverScenarios' -v
```
Expected: `--- PASS: TestFindSimsRoot`, `--- PASS: TestIsSimsDir_RequiresBoth`, `--- PASS: TestDiscoverScenarios_RealCount` (the last asserts the real scenario count under `eval-harness/scenarios`).

- [ ] **Step 9: Verify the dashboard still builds (no code change expected)**

`lib/paths.ts` resolves `../runner` and `../scenarios` from `process.cwd()` and is unaffected by the flat-layout move; this step confirms the move did not break the build.

Run:
```bash
cd /Users/stan/code/fsk/eval-harness/dashboard && pnpm install --frozen-lockfile && pnpm test run && pnpm build
```
Expected: vitest reports all `lib/*.test.ts` passing; `pnpm build` completes without error.

- [ ] **Step 10: End-to-end Docker check (requires Docker running + a valid token in `.env`)**

This is the real gate for Steps 3-4: it exercises the Dockerfile COPY fix and the repo-root build context. It is slow (image build + a coder run).

Run:
```bash
cd /Users/stan/code/fsk/eval-harness/runner && go run . run 01
```
Expected: a line like `01-zero-to-receipt        run=run.XXXXXX judge=...` and a new run dir under `~/.cache/fiskaly-eval/run.*` containing `meta.json`, `transcript.jsonl`, `build.txt`, `judge.txt`, `judge.json`. If Docker or the token is unavailable, record this step as skipped and note it explicitly — do not claim it passed.

- [ ] **Step 11: Commit**

Git detects the directory rename across the staged add/delete.

Run:
```bash
cd /Users/stan/code/fsk && git add -A && git commit -m "refactor: rename sims/ to eval-harness/" && git push origin main
```
Expected: `git show --stat HEAD` lists the files as renames (`sims/... => eval-harness/...`) plus the edits to `.dockerignore`, `.gitignore`, `Dockerfile`, `main.go`, `baselines_test.go`.

---

### Task 2: Sweep stale path references in documentation and comments

Updates documentation and code comments that name the old `sims/` path. These are accuracy-only and break nothing; kept separate from Task 1 so a reviewer can approve the functional rename independently of prose edits.

**Files:**
- Modify: `README.md` (repo root, ~11 hits)
- Modify: `eval-harness/scenarios/AUTHORING.md` (~4 hits)
- Modify: `eval-harness/scenarios/README.md` (~3 hits)
- Modify: `eval-harness/dashboard/README.md` (1 hit)
- Modify: `research/EVAL-CHECKS.md` (1 hit)
- Modify: `eval-harness/evals/dashboard.sh` (comments on lines 2, 4, 5)
- Modify: `eval-harness/judge/judge_eval/main.go` (comment on line 3)

**Interfaces:**
- Consumes: the renamed tree from Task 1.
- Produces: nothing consumed by later tasks (documentation only).

- [ ] **Step 1: List every remaining reference**

Run:
```bash
cd /Users/stan/code/fsk && grep -rn 'sims' --include='*.md' --include='*.sh' --include='*.go' \
  --exclude-dir=node_modules --exclude-dir=.next --exclude-dir=.git . \
  | grep -v 'simsRoot\|isSimsDir\|findSimsRoot\|TestFindSimsRoot\|TestIsSimsDir'
```
Expected: a list dominated by the docs files above plus the two comment locations. The excluded identifiers are the discovery symbols intentionally left for P1.

- [ ] **Step 2: Replace path-style references**

For each hit that is a path reference (`sims/`, `./sims`, `cd sims/...`, "the `sims` directory"), replace `sims` with `eval-harness`. Review each match in context first: do not rewrite prose that uses "sims" to mean the simulation concept rather than the directory.

Concretely, in `eval-harness/evals/dashboard.sh` replace the three comment lines:
```bash
# Launch the eval dashboard:  ./sims/evals/dashboard.sh   then open http://localhost:8080
#
# The dashboard is a Next.js app in sims/dashboard/. It reads ~/.cache/fiskaly-eval
# and triggers runs via the runner (cd sims/runner && go run . run <id>).
```
with:
```bash
# Launch the eval dashboard:  ./eval-harness/evals/dashboard.sh   then open http://localhost:8080
#
# The dashboard is a Next.js app in eval-harness/dashboard/. It reads ~/.cache/fiskaly-eval
# and triggers runs via the runner (cd eval-harness/runner && go run . run <id>).
```

In `eval-harness/judge/judge_eval/main.go` line 3, replace:
```go
// Usage: go run ./judge_eval   (from sims/judge)
```
with:
```go
// Usage: go run ./judge_eval   (from eval-harness/judge)
```

Apply the analogous path replacements in the four `.md` files.

- [ ] **Step 3: Verify no stale path references remain and code still builds**

Run:
```bash
cd /Users/stan/code/fsk && grep -rn 'sims/' --include='*.md' --include='*.sh' --include='*.go' \
  --exclude-dir=node_modules --exclude-dir=.next --exclude-dir=.git .
cd /Users/stan/code/fsk/eval-harness/judge && go build ./judge_eval
```
Expected: the grep returns nothing (no `sims/` path references left); the `judge_eval` build exits 0 (the comment edit did not break compilation).

- [ ] **Step 4: Commit**

Run:
```bash
cd /Users/stan/code/fsk && git add -A && git commit -m "docs: update sims/ path references to eval-harness/" && git push origin main
```
Expected: commit lands on `main`; `git status` is clean.

---

## Self-Review

**Spec coverage (P0 scope only):** The spec's P0 lists "rename `sims/` → `eval-harness/`; fix the three Docker breakers; runner and judge keep their current layout; verify `docker build` + one run." Task 1 covers the move (Step 2), the three functional breakers — Dockerfile (Step 3), runner fast-path/error (Step 4), ignore files (Step 6) — and the verification including the real Docker run (Steps 7-10). The spec also names `.gitignore` binary entries: covered in Step 6. The flat-layout-preserves-discovery claim is verified in Step 8. Documentation accuracy (spec: "~30 doc hits are cosmetic") is Task 2.

**Placeholder scan:** No TBD/TODO. Every code edit shows exact old/new text with line numbers; every verification step shows the exact command and expected output. Step 10 explicitly handles the Docker-unavailable case (record as skipped, do not claim pass) rather than hand-waving.

**Type consistency:** No new types or signatures introduced in P0. The only identifiers touched are the literal directory name `"eval-harness"` (consistent across `main.go:106`, `main.go:111` error text, and `baselines_test.go:41`) and the path `eval-harness/evals/docker-entrypoint.sh` (consistent between `Dockerfile:24` and the `.dockerignore`/build-context expectations).
