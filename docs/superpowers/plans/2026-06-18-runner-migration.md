# Runner Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the four Bash eval-orchestration scripts with a single Go CLI command, `runner run [ids...]`, that runs a baseline preflight then a Docker eval for each scenario and writes dashboard-compatible artifacts.

**Architecture:** One Go module (`runner` at `sims/runner`). A per-scenario pipeline (setup, preflight, agent-in-Docker, observe, write artifacts) built from concrete phase functions. The only mocked boundary is the Docker agent step, behind an `agent` interface, so the orchestration is unit-tested without Docker. The build/test/judge core is shared between preflight and post-agent observe.

**Tech Stack:** Go 1.23, `os/exec` (go, git, docker, the judge binary), `encoding/json`, the existing `judge` command, Docker Desktop.

## Global Constraints

- Module is `runner`, Go 1.23, located at `sims/runner` (one-module-per-subdir pattern; do not add a workspace).
- Docker is the only execution path. Do not add a local/non-Docker run mode.
- Artifact compatibility with `sims/dashboard/lib/runs.ts` is mandatory and byte-level:
  - Run dir named `run.*`, a directory under `~/.cache/fiskaly-eval`.
  - `meta.json` = `{harness, coder, model, effort, scenario}` strings; harness is `"docker"`, coder is `"claude-code"`.
  - `build.txt` = `go build` combined output; empty (after trim) means PASS.
  - `test.txt` = `go test` combined output; contains `ok` and not `FAIL` means PASS.
  - `judge.txt` = judge combined output; presence means done; contains `conformant` or `NON-COMPLIANT`.
  - `transcript.jsonl` = verbatim Claude stream-json (never reshaped).
  - Also `changes.diff`, `grounded.txt`, `mcp-telemetry.jsonl`, `claude.err`.
- Defaults: `--model` = `claude-sonnet-4-6`, `--effort` = `medium`.
- OAuth token read from `<repoRoot>/.env` key `CLAUDE_CODE_OAUTH_TOKEN`; never passed to the coder as a fiskaly credential.
- Docker context defaults to `desktop-linux`, overridable via `DOCKER_CONTEXT`.
- No AI/Claude attribution in commits, code, or docs. No em dashes. Avoid AI-tell words (delve, crucial, robust, comprehensive, etc.). Comments explain why, not what.
- Keep `go test ./...` green at every commit. Tasks 1-8 are additive; Task 9 removes the old `baselines` command and dead code.
- Run all commands from `sims/runner` unless stated otherwise.

---

### Task 1: observe.go — relocate build/test/judge primitives and add the grounding check

**Files:**
- Create: `sims/runner/observe.go`
- Modify: `sims/runner/baselines.go` (remove the three relocated functions)
- Test: `sims/runner/observe_test.go`

**Interfaces:**
- Consumes: existing `StepResult` (from `baselines.go`).
- Produces:
  - `runGoCmd(dir string, args ...string) StepResult` (relocated)
  - `runJudge(judgeBin, scenarioJSON, dir string) StepResult` (relocated)
  - `buildJudge(judgeDir, outDir string) (string, error)` (relocated)
  - `checkGrounded(transcriptPath string) (ok bool, verdict string)`

- [ ] **Step 1: Relocate the three exec primitives.** Cut `runGoCmd`, `runJudge`, and `buildJudge` (and only those) from `baselines.go` and paste them into a new `observe.go`. Add the package clause and imports.

`sims/runner/observe.go` (start):
```go
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runGoCmd(dir string, args ...string) StepResult {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return StepResult{OK: err == nil, Output: string(out)}
}

func runJudge(judgeBin, scenarioJSON, dir string) StepResult {
	cmd := exec.Command(judgeBin, "-scenario", scenarioJSON, dir)
	out, err := cmd.CombinedOutput()
	return StepResult{OK: err == nil, Output: string(out)}
}

func buildJudge(judgeDir, outDir string) (string, error) {
	bin := filepath.Join(outDir, "judge")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = judgeDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("building judge: %w\n%s", err, out)
	}
	return bin, nil
}
```

Then in `baselines.go` delete those three function definitions and remove now-unused imports (`os/exec` if nothing else uses it; keep `os`, `path/filepath`, etc. if still referenced).

- [ ] **Step 2: Verify the move kept everything green.**

Run: `go test -short ./...`
Expected: PASS (no behavior changed; `TestRunGoCmd_PassAndFail` and the build path still compile).

- [ ] **Step 3: Write the failing grounding test.**

`sims/runner/observe_test.go`:
```go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTranscript(t *testing.T, lines ...string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "transcript.jsonl")
	body := ""
	for _, l := range lines {
		body += l + "\n"
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const (
	evSearch = `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"mcp__fiskaly__search_fiskaly_docs"}]}}`
	evWrite  = `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write"}]}}`
)

func TestCheckGrounded_SearchBeforeWrite(t *testing.T) {
	ok, verdict := checkGrounded(writeTranscript(t, evSearch, evWrite))
	if !ok {
		t.Fatalf("expected grounded, got %q", verdict)
	}
}

func TestCheckGrounded_WriteBeforeSearch(t *testing.T) {
	ok, verdict := checkGrounded(writeTranscript(t, evWrite, evSearch))
	if ok {
		t.Fatalf("expected NOT grounded, got %q", verdict)
	}
}

func TestCheckGrounded_NeverSearched(t *testing.T) {
	ok, verdict := checkGrounded(writeTranscript(t, evWrite))
	if ok || verdict == "" {
		t.Fatalf("expected NOT grounded with a reason, got ok=%v %q", ok, verdict)
	}
}

func TestCheckGrounded_SearchedNeverWrote(t *testing.T) {
	ok, _ := checkGrounded(writeTranscript(t, evSearch))
	if ok {
		t.Fatal("expected NOT grounded (inconclusive) when nothing was written")
	}
}
```

- [ ] **Step 4: Run it to verify it fails.**

Run: `go test -run TestCheckGrounded ./...`
Expected: FAIL — `undefined: checkGrounded`.

- [ ] **Step 5: Implement `checkGrounded` in observe.go.**

Append to `sims/runner/observe.go`:
```go
// transcriptEvent is the minimal shape of one Claude stream-json line: a
// tool_use block lives inside an assistant message's content array.
type transcriptEvent struct {
	Message struct {
		Content []struct {
			Type string `json:"type"`
			Name string `json:"name"`
		} `json:"content"`
	} `json:"message"`
}

// checkGrounded reports whether the agent searched the docs before writing
// integration code: the first search_fiskaly_docs tool_use must precede the
// first Write/Edit/MultiEdit tool_use. This is the Go port of assert-grounded.sh,
// parsing events instead of grepping line numbers.
func checkGrounded(transcriptPath string) (bool, string) {
	f, err := os.Open(transcriptPath)
	if err != nil {
		return false, "INCONCLUSIVE: transcript not found"
	}
	defer f.Close()

	searchAt, mutateAt := -1, -1
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024) // transcript lines can be large
	for i := 0; sc.Scan(); i++ {
		var ev transcriptEvent
		if json.Unmarshal(sc.Bytes(), &ev) != nil {
			continue
		}
		for _, c := range ev.Message.Content {
			if c.Type != "tool_use" {
				continue
			}
			if searchAt == -1 && strings.Contains(c.Name, "search_fiskaly_docs") {
				searchAt = i
			}
			if mutateAt == -1 && (c.Name == "Write" || c.Name == "Edit" || c.Name == "MultiEdit") {
				mutateAt = i
			}
		}
	}

	switch {
	case searchAt == -1:
		return false, "NOT GROUNDED: agent never called search_fiskaly_docs"
	case mutateAt == -1:
		return false, "INCONCLUSIVE: agent searched but never wrote integration code"
	case searchAt < mutateAt:
		return true, fmt.Sprintf("GROUNDED: searched (event %d) before first code change (event %d)", searchAt, mutateAt)
	default:
		return false, fmt.Sprintf("NOT GROUNDED: first code change (event %d) precedes first search (event %d)", mutateAt, searchAt)
	}
}
```

- [ ] **Step 6: Run the tests to verify they pass.**

Run: `go test ./...`
Expected: PASS (all, including the relocated-primitive tests and the new grounding tests).

- [ ] **Step 7: Commit.**

```bash
git add sims/runner/observe.go sims/runner/observe_test.go sims/runner/baselines.go
git commit -m "runner: add observe.go with grounding check; relocate build/test/judge primitives"
```

---

### Task 2: observe.go — add the staged-diff helper

**Files:**
- Modify: `sims/runner/observe.go`
- Test: `sims/runner/observe_test.go`

**Interfaces:**
- Produces: `gitDiffStaged(work string) (string, error)`

- [ ] **Step 1: Write the failing test.**

Append to `sims/runner/observe_test.go`:
```go
func TestGitDiffStaged(t *testing.T) {
	work := t.TempDir()
	gitInit(t, work)
	writeFile(t, filepath.Join(work, "a.go"), "package a\n")
	gitCommitAll(t, work, "baseline")
	writeFile(t, filepath.Join(work, "a.go"), "package a\n\nvar X = 1\n")

	diff, err := gitDiffStaged(work)
	if err != nil {
		t.Fatalf("gitDiffStaged: %v", err)
	}
	if !strings.Contains(diff, "var X = 1") {
		t.Errorf("diff missing the change:\n%s", diff)
	}
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	mustGit(t, dir, "init", "-q")
}

func gitCommitAll(t *testing.T, dir, msg string) {
	t.Helper()
	mustGit(t, dir, "-c", "user.email=t@t", "-c", "user.name=t", "add", "-A")
	mustGit(t, dir, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", msg)
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
```

Add `"os/exec"` and `"strings"` to the `observe_test.go` import block.

- [ ] **Step 2: Run it to verify it fails.**

Run: `go test -run TestGitDiffStaged ./...`
Expected: FAIL — `undefined: gitDiffStaged`.

- [ ] **Step 3: Implement `gitDiffStaged`.**

Append to `sims/runner/observe.go`:
```go
// gitDiffStaged stages all changes in work and returns the diff against the
// baseline commit, the exact change set the agent produced.
func gitDiffStaged(work string) (string, error) {
	if out, err := exec.Command("git", "-C", work, "add", "-A").CombinedOutput(); err != nil {
		return "", fmt.Errorf("git add: %w\n%s", err, out)
	}
	out, err := exec.Command("git", "-C", work, "diff", "--cached").Output()
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}
```

- [ ] **Step 4: Run it to verify it passes.**

Run: `go test -run TestGitDiffStaged ./...`
Expected: PASS.

- [ ] **Step 5: Commit.**

```bash
git add sims/runner/observe.go sims/runner/observe_test.go
git commit -m "runner: add gitDiffStaged helper"
```

---

### Task 3: config.go — model/effort config and .env token

**Files:**
- Create: `sims/runner/config.go`
- Test: `sims/runner/config_test.go`

**Interfaces:**
- Produces:
  - `type runConfig struct { model, effort, token string }`
  - `readEnvToken(envPath string) (string, error)`
  - `loadConfig(repoRoot, model, effort string) (runConfig, error)`
  - `dockerContext() string`
  - `const defaultModel = "claude-sonnet-4-6"`, `const defaultEffort = "medium"`

- [ ] **Step 1: Write the failing test.**

`sims/runner/config_test.go`:
```go
package main

import (
	"path/filepath"
	"testing"
)

func TestReadEnvToken(t *testing.T) {
	dir := t.TempDir()
	env := filepath.Join(dir, ".env")
	writeFile(t, env, "SOMETHING=else\nCLAUDE_CODE_OAUTH_TOKEN=\"sk-abc123\"\nMORE=1\n")

	tok, err := readEnvToken(env)
	if err != nil {
		t.Fatalf("readEnvToken: %v", err)
	}
	if tok != "sk-abc123" {
		t.Errorf("token = %q, want sk-abc123", tok)
	}
}

func TestReadEnvToken_Missing(t *testing.T) {
	dir := t.TempDir()
	env := filepath.Join(dir, ".env")
	writeFile(t, env, "NOTHING=here\n")
	if _, err := readEnvToken(env); err == nil {
		t.Fatal("expected error when token key is absent")
	}
}
```

- [ ] **Step 2: Run it to verify it fails.**

Run: `go test -run TestReadEnvToken ./...`
Expected: FAIL — `undefined: readEnvToken`.

- [ ] **Step 3: Implement config.go.**

`sims/runner/config.go`:
```go
package main

import (
	"fmt"
	"os"
	"strings"
)

const (
	defaultModel  = "claude-sonnet-4-6"
	defaultEffort = "medium"
)

type runConfig struct {
	model  string
	effort string
	token  string
}

// readEnvToken extracts CLAUDE_CODE_OAUTH_TOKEN from a .env file, stripping
// optional surrounding quotes. It is the only secret the runner reads.
func readEnvToken(envPath string) (string, error) {
	data, err := os.ReadFile(envPath)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", envPath, err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		v, ok := strings.CutPrefix(strings.TrimSpace(line), "CLAUDE_CODE_OAUTH_TOKEN=")
		if !ok {
			continue
		}
		if v = strings.Trim(v, `"'`); v != "" {
			return v, nil
		}
	}
	return "", fmt.Errorf("CLAUDE_CODE_OAUTH_TOKEN not found in %s", envPath)
}

func loadConfig(repoRoot, model, effort string) (runConfig, error) {
	token, err := readEnvToken(repoRoot + "/.env")
	if err != nil {
		return runConfig{}, err
	}
	return runConfig{model: model, effort: effort, token: token}, nil
}

// dockerContext pins to Docker Desktop unless overridden, so a run cannot land
// on another configured engine.
func dockerContext() string {
	if c := os.Getenv("DOCKER_CONTEXT"); c != "" {
		return c
	}
	return "desktop-linux"
}
```

- [ ] **Step 4: Run it to verify it passes.**

Run: `go test -run TestReadEnvToken ./...`
Expected: PASS.

- [ ] **Step 5: Commit.**

```bash
git add sims/runner/config.go sims/runner/config_test.go
git commit -m "runner: add config (model/effort defaults, .env token, docker context)"
```

---

### Task 4: artifacts.go — run dir, meta.json, observe-artifact writer

**Files:**
- Create: `sims/runner/artifacts.go`
- Test: `sims/runner/artifacts_test.go`

**Interfaces:**
- Consumes: `runConfig`, `scenario`, `Outcome`, `copyDir`.
- Produces:
  - `type runDir struct { path, work string }`
  - `type observation struct { Outcome; diff, grounded string; groundedOK bool }`
  - `prepareRun(runsBase string, s scenario, cfg runConfig) (runDir, error)`
  - `writeMeta(runPath, scenario string, cfg runConfig) error`
  - `writeObserveArtifacts(runPath string, o observation) error`

- [ ] **Step 1: Write the failing test (asserts the dashboard contract).**

`sims/runner/artifacts_test.go`:
```go
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteObserveArtifacts_DashboardContract(t *testing.T) {
	runPath := t.TempDir()
	o := observation{
		Outcome: Outcome{
			Build: StepResult{OK: true, Output: ""},
			Test:  StepResult{OK: true, Output: "ok  \tpos\t0.1s\n"},
			Judge: StepResult{OK: false, Output: "VERDICT: NON-COMPLIANT (5 failures). exit 1\n"},
		},
		diff:       "diff --git a/x b/x\n",
		grounded:   "GROUNDED: searched before first code change",
		groundedOK: true,
	}
	if err := writeObserveArtifacts(runPath, o); err != nil {
		t.Fatalf("writeObserveArtifacts: %v", err)
	}

	// build.txt empty (trim) => dashboard reads PASS
	if b := readFileT(t, runPath, "build.txt"); strings.TrimSpace(b) != "" {
		t.Errorf("build.txt should be empty on PASS, got %q", b)
	}
	// test.txt contains ok and not FAIL => PASS
	tt := readFileT(t, runPath, "test.txt")
	if !strings.Contains(tt, "ok") || strings.Contains(tt, "FAIL") {
		t.Errorf("test.txt not a PASS shape: %q", tt)
	}
	// judge.txt present and NON-COMPLIANT
	if j := readFileT(t, runPath, "judge.txt"); !strings.Contains(j, "NON-COMPLIANT") {
		t.Errorf("judge.txt missing verdict: %q", j)
	}
	if d := readFileT(t, runPath, "changes.diff"); !strings.Contains(d, "diff --git") {
		t.Errorf("changes.diff missing: %q", d)
	}
	if g := readFileT(t, runPath, "grounded.txt"); !strings.Contains(g, "GROUNDED") {
		t.Errorf("grounded.txt missing: %q", g)
	}
}

func TestWriteMeta_Shape(t *testing.T) {
	runPath := t.TempDir()
	if err := writeMeta(runPath, "01-zero-to-receipt", runConfig{model: "m", effort: "e"}); err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(readFileT(t, runPath, "meta.json")), &m); err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{
		"harness": "docker", "coder": "claude-code", "model": "m", "effort": "e", "scenario": "01-zero-to-receipt",
	} {
		if m[k] != want {
			t.Errorf("meta[%q] = %q, want %q", k, m[k], want)
		}
	}
}

func readFileT(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}
```

- [ ] **Step 2: Run it to verify it fails.**

Run: `go test -run 'TestWriteObserveArtifacts|TestWriteMeta' ./...`
Expected: FAIL — `undefined: observation` / `writeObserveArtifacts` / `writeMeta`.

- [ ] **Step 3: Implement artifacts.go.**

`sims/runner/artifacts.go`:
```go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type runDir struct {
	path string // ~/.cache/fiskaly-eval/run.XXXXXX
	work string // <path>/pos
}

// observation is everything the post-agent observe phase produces.
type observation struct {
	Outcome
	diff       string
	grounded   string
	groundedOK bool
}

// prepareRun creates an isolated run dir, copies the fixture, makes a baseline
// commit so the agent's changes can be diffed, and writes meta.json.
func prepareRun(runsBase string, s scenario, cfg runConfig) (runDir, error) {
	if err := os.MkdirAll(runsBase, 0o755); err != nil {
		return runDir{}, fmt.Errorf("creating runs base: %w", err)
	}
	path, err := os.MkdirTemp(runsBase, "run.")
	if err != nil {
		return runDir{}, fmt.Errorf("creating run dir: %w", err)
	}
	rd := runDir{path: path, work: filepath.Join(path, "pos")}
	if err := copyDir(s.fixtureDir, rd.work); err != nil {
		return rd, fmt.Errorf("copying fixture: %w", err)
	}
	if err := gitInitBaseline(rd.work); err != nil {
		return rd, err
	}
	if err := writeMeta(rd.path, s.id, cfg); err != nil {
		return rd, err
	}
	return rd, nil
}

func gitInitBaseline(work string) error {
	steps := [][]string{
		{"init", "-q"},
		{"-c", "user.email=eval@local", "-c", "user.name=eval", "add", "-A"},
		{"-c", "user.email=eval@local", "-c", "user.name=eval", "commit", "-qm", "baseline"},
	}
	for _, s := range steps {
		if out, err := exec.Command("git", append([]string{"-C", work}, s...)...).CombinedOutput(); err != nil {
			return fmt.Errorf("git %v: %w\n%s", s, err, out)
		}
	}
	return nil
}

func writeMeta(runPath, scenario string, cfg runConfig) error {
	meta := map[string]string{
		"harness":  "docker",
		"coder":    "claude-code",
		"model":    cfg.model,
		"effort":   cfg.effort,
		"scenario": scenario,
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(runPath, "meta.json"), append(data, '\n'), 0o644)
}

// writeObserveArtifacts writes the dashboard-read files. build.txt is the raw
// build output (empty on success, which the dashboard reads as PASS); the others
// follow the same passthrough rule.
func writeObserveArtifacts(runPath string, o observation) error {
	files := map[string]string{
		"build.txt":    o.Build.Output,
		"test.txt":     o.Test.Output,
		"judge.txt":    o.Judge.Output,
		"changes.diff": o.diff,
		"grounded.txt": o.grounded + "\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(runPath, name), []byte(content), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
	}
	return nil
}
```

- [ ] **Step 4: Run it to verify it passes.**

Run: `go test -run 'TestWriteObserveArtifacts|TestWriteMeta' ./...`
Expected: PASS.

- [ ] **Step 5: Commit.**

```bash
git add sims/runner/artifacts.go sims/runner/artifacts_test.go
git commit -m "runner: add run-dir setup, meta.json, and observe-artifact writers"
```

---

### Task 5: docker.go — the agent seam and its Docker implementation

**Files:**
- Create: `sims/runner/docker.go`
- Test: `sims/runner/docker_test.go`

**Interfaces:**
- Consumes: `runDir`, `runConfig`, `dockerContext`.
- Produces:
  - `type agent interface { run(rd runDir, task string, cfg runConfig) error }`
  - `type dockerAgent struct { repoRoot, simsRoot, context, image string }` implementing `agent`
  - `dockerEnv(context string) []string`
  - `checkBinaries(names ...string) error`

- [ ] **Step 1: Write the failing test (the binary-presence guard, the unit-testable part).**

`sims/runner/docker_test.go`:
```go
package main

import "testing"

func TestCheckBinaries(t *testing.T) {
	if err := checkBinaries("go", "git"); err != nil {
		t.Errorf("go and git should be present: %v", err)
	}
	if err := checkBinaries("definitely-not-a-real-binary-xyz"); err == nil {
		t.Error("expected error for a missing binary")
	}
}
```

- [ ] **Step 2: Run it to verify it fails.**

Run: `go test -run TestCheckBinaries ./...`
Expected: FAIL — `undefined: checkBinaries`.

- [ ] **Step 3: Implement docker.go.**

`sims/runner/docker.go`:
```go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// agent runs the coder against a prepared work dir. The real implementation
// drives Docker; tests inject a fake.
type agent interface {
	run(rd runDir, task string, cfg runConfig) error
}

// dockerAgent runs the coder hermetically: only the work dir is mounted, so the
// container cannot reach the repo, the MCP/judge source, or research/.
type dockerAgent struct {
	repoRoot string
	simsRoot string
	context  string
	image    string
}

func (a dockerAgent) run(rd runDir, task string, cfg runConfig) error {
	build := exec.Command("docker", "build",
		"-f", filepath.Join(a.simsRoot, "evals", "Dockerfile"),
		"-t", a.image, a.repoRoot)
	build.Env = dockerEnv(a.context)
	if out, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("docker build: %w\n%s", err, out)
	}

	transcript, err := os.Create(filepath.Join(rd.path, "transcript.jsonl"))
	if err != nil {
		return err
	}
	defer transcript.Close()
	stderr, err := os.Create(filepath.Join(rd.path, "claude.err"))
	if err != nil {
		return err
	}
	defer stderr.Close()

	run := exec.Command("docker", "run", "--rm",
		"-e", "CLAUDE_CODE_OAUTH_TOKEN="+cfg.token,
		"-e", "IS_SANDBOX=1",
		"-e", "RUN_MODEL="+cfg.model,
		"-e", "RUN_EFFORT="+cfg.effort,
		"-e", "FISKALY_MCP_TELEMETRY=/work/mcp-telemetry.jsonl",
		"-v", rd.work+":/work",
		a.image, task)
	run.Env = dockerEnv(a.context)
	run.Stdout = transcript
	run.Stderr = stderr
	// The agent exiting non-zero is recorded in claude.err, not fatal: an agent
	// failure is a result to observe, matching the Bash harness.
	_ = run.Run()

	tele := filepath.Join(rd.work, "mcp-telemetry.jsonl")
	if _, err := os.Stat(tele); err == nil {
		if err := os.Rename(tele, filepath.Join(rd.path, "mcp-telemetry.jsonl")); err != nil {
			return fmt.Errorf("moving telemetry: %w", err)
		}
	}
	return nil
}

func dockerEnv(context string) []string {
	return append(os.Environ(), "DOCKER_CONTEXT="+context)
}

// checkBinaries verifies each named tool is on PATH.
func checkBinaries(names ...string) error {
	for _, n := range names {
		if _, err := exec.LookPath(n); err != nil {
			return fmt.Errorf("%s not found on PATH", n)
		}
	}
	return nil
}
```

- [ ] **Step 4: Run it to verify it passes.**

Run: `go test -run TestCheckBinaries ./...`
Expected: PASS.

- [ ] **Step 5: Commit.**

```bash
git add sims/runner/docker.go sims/runner/docker_test.go
git commit -m "runner: add docker agent seam and binary-presence guard"
```

---

### Task 6: run.go — observe core, preflight, post-agent observe, and the per-scenario pipeline

**Files:**
- Create: `sims/runner/run.go`
- Test: `sims/runner/run_test.go`

**Interfaces:**
- Consumes: `scenario`, `Outcome`, `runConfig`, `agent`, `runDir`, `baselineHolds`, `observeCore` inputs, `gitDiffStaged`, `checkGrounded`, `prepareRun`, `writeObserveArtifacts`.
- Produces:
  - `observeCore(work, judgeBin, scenarioJSON string) Outcome`
  - `type scenarioResult struct { id, runDir string; preflightViolated bool; preflight Outcome; obs observation }`
  - `runScenario(s scenario, runsBase, judgeBin string, ag agent, cfg runConfig) (scenarioResult, error)`

- [ ] **Step 1: Write the failing orchestration test with a fake agent.**

`sims/runner/run_test.go`:
```go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeAgent simulates the coder: it grounds, then mutates the work tree so the
// post-agent observe has a real diff and transcript.
type fakeAgent struct{ conformant bool }

func (f fakeAgent) run(rd runDir, task string, cfg runConfig) error {
	tr := evSearch + "\n" + evWrite + "\n"
	if err := os.WriteFile(filepath.Join(rd.path, "transcript.jsonl"), []byte(tr), 0o644); err != nil {
		return err
	}
	// Append a line so changes.diff is non-empty; keep the module building.
	pos := filepath.Join(rd.work, "pos.go")
	b, _ := os.ReadFile(pos)
	return os.WriteFile(pos, append(b, []byte("\n// touched by fake agent\n")...), 0o644)
}

func TestRunScenario_PreflightHoldsAndArtifactsWritten(t *testing.T) {
	simsRoot, _ := filepath.Abs("..")
	judgeBin, err := buildJudge(filepath.Join(simsRoot, "judge"), t.TempDir())
	if err != nil {
		t.Fatalf("buildJudge: %v", err)
	}
	sc, err := discoverScenarios(filepath.Join(simsRoot, "scenarios"))
	if err != nil {
		t.Fatal(err)
	}
	one := sc[0] // 01-zero-to-receipt

	res, err := runScenario(one, t.TempDir(), judgeBin, fakeAgent{}, runConfig{model: "m", effort: "e"})
	if err != nil {
		t.Fatalf("runScenario: %v", err)
	}
	if res.preflightViolated {
		t.Fatal("pristine seed should hold the baseline preflight")
	}
	for _, name := range []string{"meta.json", "build.txt", "test.txt", "judge.txt", "changes.diff", "grounded.txt", "transcript.jsonl"} {
		if _, err := os.Stat(filepath.Join(res.runDir, name)); err != nil {
			t.Errorf("missing artifact %s: %v", name, err)
		}
	}
}
```

Note: `pos.go` exists in every fixture (verified in scenario 01). If a future fixture lacks it, the fake reads empty and creates it, which still builds.

- [ ] **Step 2: Run it to verify it fails.**

Run: `go test -run TestRunScenario ./...`
Expected: FAIL — `undefined: runScenario` / `observeCore` / `scenarioResult`.

- [ ] **Step 3: Implement run.go.**

`sims/runner/run.go`:
```go
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// observeCore runs the shared build/test/judge checks against a work dir.
func observeCore(work, judgeBin, scenarioJSON string) Outcome {
	return Outcome{
		Build: runGoCmd(work, "build", "./..."),
		Test:  runGoCmd(work, "test", "./..."),
		Judge: runJudge(judgeBin, scenarioJSON, work),
	}
}

type scenarioResult struct {
	id                string
	runDir            string
	preflightViolated bool
	preflight         Outcome
	obs               observation
}

// runScenario is the single path: prepare an isolated run dir, assert the
// baseline preflight on the pristine copy, run the agent in Docker, observe the
// result, and write the dashboard artifacts. A preflight violation skips the
// agent: an unsound seed is a harness error, not an eval.
func runScenario(s scenario, runsBase, judgeBin string, ag agent, cfg runConfig) (scenarioResult, error) {
	taskBytes, err := os.ReadFile(filepath.Join(s.dir, "task.md"))
	if err != nil {
		return scenarioResult{}, fmt.Errorf("reading task: %w", err)
	}

	rd, err := prepareRun(runsBase, s, cfg)
	if err != nil {
		return scenarioResult{}, err
	}

	pre := observeCore(rd.work, judgeBin, s.scenarioJSON)
	if !baselineHolds(s, pre) {
		return scenarioResult{id: s.id, runDir: rd.path, preflightViolated: true, preflight: pre}, nil
	}

	if err := ag.run(rd, string(taskBytes), cfg); err != nil {
		return scenarioResult{}, fmt.Errorf("agent: %w", err)
	}

	core := observeCore(rd.work, judgeBin, s.scenarioJSON)
	diff, err := gitDiffStaged(rd.work)
	if err != nil {
		return scenarioResult{}, err
	}
	ok, verdict := checkGrounded(filepath.Join(rd.path, "transcript.jsonl"))
	obs := observation{Outcome: core, diff: diff, grounded: verdict, groundedOK: ok}
	if err := writeObserveArtifacts(rd.path, obs); err != nil {
		return scenarioResult{}, err
	}
	return scenarioResult{id: s.id, runDir: rd.path, obs: obs}, nil
}
```

- [ ] **Step 4: Run it to verify it passes.**

Run: `go test -run TestRunScenario ./...`
Expected: PASS (real build/test/judge on the seed; fake agent for the Docker step).

- [ ] **Step 5: Commit.**

```bash
git add sims/runner/run.go sims/runner/run_test.go
git commit -m "runner: add per-scenario pipeline (preflight, agent seam, observe, artifacts)"
```

---

### Task 7: run-all dispatch and the `run` command

**Files:**
- Create: `sims/runner/runall.go`
- Modify: `sims/runner/main.go`
- Test: `sims/runner/runall_test.go`

**Interfaces:**
- Consumes: `scenario`, `agent`, `runConfig`, `runScenario`, `discoverScenarios`, `findSimsRoot`, `buildJudge`, `loadConfig`, `checkBinaries`, `dockerContext`, `dockerAgent`, `verdict`.
- Produces:
  - `filterScenarios(all []scenario, ids []string) ([]scenario, error)`
  - `runAll(scenarios []scenario, runsBase, judgeBin string, ag agent, cfg runConfig, w io.Writer) int`
  - `cmdRun(args []string) int` (in main.go)

- [ ] **Step 1: Write the failing tests.**

`sims/runner/runall_test.go`:
```go
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilterScenarios_PrefixAndExact(t *testing.T) {
	all := []scenario{{id: "01-zero-to-receipt"}, {id: "06-fire-and-forget"}}
	got, err := filterScenarios(all, []string{"06"})
	if err != nil || len(got) != 1 || got[0].id != "06-fire-and-forget" {
		t.Fatalf("prefix match failed: %+v err=%v", got, err)
	}
	got, err = filterScenarios(all, []string{"01-zero-to-receipt"})
	if err != nil || len(got) != 1 || got[0].id != "01-zero-to-receipt" {
		t.Fatalf("exact match failed: %+v err=%v", got, err)
	}
	if _, err := filterScenarios(all, []string{"99"}); err == nil {
		t.Error("expected error for an id matching nothing")
	}
}

func TestRunAll_PreflightViolationExitsNonZero(t *testing.T) {
	simsRoot, _ := filepath.Abs("..")
	judgeBin, err := buildJudge(filepath.Join(simsRoot, "judge"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sc, err := discoverScenarios(filepath.Join(simsRoot, "scenarios"))
	if err != nil {
		t.Fatal(err)
	}
	one := sc[0]
	one.declaredBaseline = baselineSpec{Build: "PASS", Tests: "FAIL", Judge: "NON-COMPLIANT"} // mis-declared

	var b strings.Builder
	code := runAll([]scenario{one}, t.TempDir(), judgeBin, fakeAgent{}, runConfig{}, &b)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(b.String(), "PREFLIGHT") {
		t.Errorf("summary should flag the preflight violation:\n%s", b.String())
	}
}

func TestRunAll_AllPassExitsZero(t *testing.T) {
	simsRoot, _ := filepath.Abs("..")
	judgeBin, err := buildJudge(filepath.Join(simsRoot, "judge"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sc, err := discoverScenarios(filepath.Join(simsRoot, "scenarios"))
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	code := runAll(sc[:1], t.TempDir(), judgeBin, fakeAgent{}, runConfig{model: "m", effort: "e"}, &b)
	if code != 0 {
		t.Fatalf("exit = %d, want 0:\n%s", code, b.String())
	}
	_ = os.Stdout
}
```

- [ ] **Step 2: Run it to verify it fails.**

Run: `go test -run 'TestFilterScenarios|TestRunAll' ./...`
Expected: FAIL — `undefined: filterScenarios` / `runAll`.

- [ ] **Step 3: Implement runall.go.**

`sims/runner/runall.go`:
```go
package main

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// filterScenarios selects scenarios by id, accepting an exact id or a numeric
// prefix (e.g. "06" matches "06-fire-and-forget"). An id matching nothing is an
// error so a typo fails loudly.
func filterScenarios(all []scenario, ids []string) ([]scenario, error) {
	var out []scenario
	for _, want := range ids {
		matched := false
		for _, s := range all {
			if s.id == want || strings.HasPrefix(s.id, want+"-") {
				out = append(out, s)
				matched = true
			}
		}
		if !matched {
			return nil, fmt.Errorf("no scenario matches %q", want)
		}
	}
	return out, nil
}

// runAll runs each scenario through the single path independently and returns 0
// only if all completed without a preflight violation or harness error. An
// agent failure is recorded in claude.err, not counted as a failure here.
func runAll(scenarios []scenario, runsBase, judgeBin string, ag agent, cfg runConfig, w io.Writer) int {
	failed := 0
	for _, s := range scenarios {
		res, err := runScenario(s, runsBase, judgeBin, ag, cfg)
		if err != nil {
			fmt.Fprintf(w, "%-22s ERROR: %v\n", s.id, err)
			failed++
			continue
		}
		if res.preflightViolated {
			fmt.Fprintf(w, "%-22s PREFLIGHT VIOLATED (seed not build PASS/tests PASS/judge NON-COMPLIANT)\n", s.id)
			failed++
			continue
		}
		fmt.Fprintf(w, "%-22s run=%s judge=%s grounded=%v\n",
			s.id, filepath.Base(res.runDir), verdict(res.obs.Judge.OK), res.obs.groundedOK)
	}
	total := len(scenarios)
	fmt.Fprintln(w)
	if failed == 0 {
		fmt.Fprintf(w, "%d/%d scenarios ran.\n", total, total)
		return 0
	}
	fmt.Fprintf(w, "%d/%d scenarios ran; %d failed before eval.\n", total-failed, total, failed)
	return 1
}
```

- [ ] **Step 4: Wire `cmdRun` into main.go.**

In `sims/runner/main.go`, replace the `baselines` case and `cmdBaselines` function with `run`:

Change the dispatch switch:
```go
	switch os.Args[1] {
	case "run":
		os.Exit(cmdRun(os.Args[2:]))
	default:
		fmt.Fprintf(os.Stderr, "runner: unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
```

Update `usage`:
```go
func usage() {
	fmt.Fprintln(os.Stderr, "usage: runner run [ids...]")
}
```

Replace `cmdBaselines` with `cmdRun`:
```go
func cmdRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	model := fs.String("model", defaultModel, "coder model")
	effort := fs.String("effort", defaultEffort, "coder effort")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	simsRoot, err := findSimsRoot(mustGetwd())
	if err != nil {
		fmt.Fprintln(os.Stderr, "runner:", err)
		return 2
	}
	repoRoot := filepath.Dir(simsRoot)
	ctx := dockerContext()

	if err := checkBinaries("docker", "go", "git"); err != nil {
		fmt.Fprintln(os.Stderr, "runner:", err)
		return 2
	}
	cfg, err := loadConfig(repoRoot, *model, *effort)
	if err != nil {
		fmt.Fprintln(os.Stderr, "runner:", err)
		return 2
	}

	scenarios, err := discoverScenarios(filepath.Join(simsRoot, "scenarios"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "runner:", err)
		return 2
	}
	if ids := fs.Args(); len(ids) > 0 {
		scenarios, err = filterScenarios(scenarios, ids)
		if err != nil {
			fmt.Fprintln(os.Stderr, "runner:", err)
			return 2
		}
	}

	judgeBin, err := buildJudge(filepath.Join(simsRoot, "judge"), mustTempDir())
	if err != nil {
		fmt.Fprintln(os.Stderr, "runner:", err)
		return 2
	}

	ag := dockerAgent{repoRoot: repoRoot, simsRoot: simsRoot, context: ctx, image: "fiskaly-eval"}
	runsBase := filepath.Join(os.Getenv("HOME"), ".cache", "fiskaly-eval")
	return runAll(scenarios, runsBase, judgeBin, ag, cfg, os.Stdout)
}
```

Keep `findSimsRoot`, `isSimsDir`, `mustGetwd`, `mustTempDir` as they are. Ensure `main.go` still imports `flag`, `fmt`, `os`, `path/filepath`.

- [ ] **Step 5: Run the full suite.**

Run: `go vet ./... && go test -short ./...`
Expected: PASS. (The old `baselines` command is gone from main; the `runBaselines` function and its tests are removed in Task 9, so `-short` may still run them here — that is fine, they still pass. If `cmdBaselines` removal left `runBaselines` unreferenced, that is acceptable until Task 9.)

- [ ] **Step 6: Commit.**

```bash
git add sims/runner/runall.go sims/runner/runall_test.go sims/runner/main.go
git commit -m "runner: add run-all dispatch and the run command"
```

---

### Task 8: gated real-Docker integration test

**Files:**
- Modify: `sims/runner/integration_test.go`

**Interfaces:**
- Consumes: `dockerAgent`, `runScenario`, `discoverScenarios`, `buildJudge`, `loadConfig`, `checkBinaries`, `dockerContext`.

- [ ] **Step 1: Add the gated integration test.**

Append to `sims/runner/integration_test.go`:
```go
func TestRunScenario_RealDocker(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real Docker run in -short mode")
	}
	if err := checkBinaries("docker"); err != nil {
		t.Skip("docker not available")
	}
	simsRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	repoRoot := filepath.Dir(simsRoot)
	cfg, err := loadConfig(repoRoot, "claude-sonnet-4-6", "low")
	if err != nil {
		t.Skipf("no usable config (.env token): %v", err)
	}
	judgeBin, err := buildJudge(filepath.Join(simsRoot, "judge"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sc, err := discoverScenarios(filepath.Join(simsRoot, "scenarios"))
	if err != nil {
		t.Fatal(err)
	}
	ag := dockerAgent{repoRoot: repoRoot, simsRoot: simsRoot, context: dockerContext(), image: "fiskaly-eval"}

	res, err := runScenario(sc[0], t.TempDir(), judgeBin, ag, cfg)
	if err != nil {
		t.Fatalf("runScenario: %v", err)
	}
	for _, name := range []string{"meta.json", "transcript.jsonl", "build.txt", "test.txt", "judge.txt", "changes.diff", "grounded.txt"} {
		if _, err := os.Stat(filepath.Join(res.runDir, name)); err != nil {
			t.Errorf("missing artifact %s: %v", name, err)
		}
	}
}
```

Add `"os"` to the integration_test.go imports if not present.

- [ ] **Step 2: Run the short suite (skips the Docker test).**

Run: `go test -short ./...`
Expected: PASS, with `TestRunScenario_RealDocker` skipped.

- [ ] **Step 3 (optional, requires Docker + .env token): run the real test.**

Run: `go test -run TestRunScenario_RealDocker ./...`
Expected: PASS — one real container run writes the artifact set.

- [ ] **Step 4: Commit.**

```bash
git add sims/runner/integration_test.go
git commit -m "runner: add gated real-Docker integration test"
```

---

### Task 9: delete the Bash scripts and the old baselines command; repoint the seed guard; update docs

**Files:**
- Delete: `sims/evals/run-scenario.sh`, `sims/evals/run-eval.sh`, `sims/evals/run-eval-docker.sh`, `sims/evals/assert-grounded.sh`
- Modify: `sims/runner/baselines.go` (remove the dead baselines-command code)
- Modify: `sims/runner/baselines_test.go` (remove baselines-command tests; keep discovery and the seed-guard)
- Modify: `sims/runner/integration_test.go` (repoint the all-seeds guard to `preflight`)
- Modify: `README.md`
- Modify: project memory `runner-migration-status.md` and `MEMORY.md`

**Interfaces:**
- Produces: `preflightAll(scenarios []scenario, judgeBin string) []string` (returns the ids that violate the baseline; empty means all hold) — the docker-free guard that replaces the old baselines integration test.

- [ ] **Step 1: Add `preflightAll` to run.go and its failing test.**

Append to `sims/runner/run.go`:
```go
// preflightAll runs the baseline preflight (no Docker) across scenarios and
// returns the ids that violate the invariant. Empty means every seed is sound.
func preflightAll(scenarios []scenario, judgeBin string) []string {
	var violated []string
	for _, s := range scenarios {
		work, err := os.MkdirTemp("", "runner-preflight-"+s.id+"-")
		if err != nil {
			violated = append(violated, s.id)
			continue
		}
		dst := filepath.Join(work, "pos")
		if copyDir(s.fixtureDir, dst) != nil || !baselineHolds(s, observeCore(dst, judgeBin, s.scenarioJSON)) {
			violated = append(violated, s.id)
		}
		os.RemoveAll(work)
	}
	return violated
}
```

Replace the body of `TestBaselines_RealScenarios` in `integration_test.go` with a preflight guard, and rename it:
```go
func TestPreflightAll_RealScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-scenario preflight in -short mode")
	}
	simsRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	judgeBin, err := buildJudge(filepath.Join(simsRoot, "judge"), t.TempDir())
	if err != nil {
		t.Fatalf("buildJudge: %v", err)
	}
	sc, err := discoverScenarios(filepath.Join(simsRoot, "scenarios"))
	if err != nil {
		t.Fatalf("discoverScenarios: %v", err)
	}
	if len(sc) != 10 {
		t.Fatalf("discovered %d scenarios, want 10", len(sc))
	}
	if bad := preflightAll(sc, judgeBin); len(bad) != 0 {
		t.Fatalf("scenarios violating the baseline: %v", bad)
	}
}
```

Delete the old `testWriter` type in integration_test.go if it is now unused.

- [ ] **Step 2: Remove the dead baselines-command code from baselines.go.**

Delete these definitions (now unreferenced): `checker` interface, `execChecker` struct and its `check` method, `runBaselines`, `formatLine`, `writeDiagnostics`, `indent`. Keep: `StepResult`, `Outcome`, `scenario`, `baselineSpec`, `canonicalBaseline`, `observedBaseline`, `baselineHolds`, `passFail`, `verdict`, `discoverScenarios`, `readBaseline`, `copyDir`, `isDir`, `isFile`, `scenarioID`.

- [ ] **Step 3: Remove the baselines-command tests from baselines_test.go.**

Delete: `fakeChecker` and its `check`, `held`, `scenarios`, `TestRunBaselines_AllHold`, `TestRunBaselines_TestFailViolates`, `TestRunBaselines_JudgeConformantViolates`, `TestRunBaselines_BuildFailViolates`, `TestRunBaselines_DeclaredBaselineMismatchViolates`, `TestRunBaselines_CheckErrorViolates`, `TestExecCheckerCheck_CopyErrorIsReported`. Keep: `TestDiscoverScenarios`, `TestDiscoverScenarios_NoneIsError`, `TestFindSimsRoot`, `TestIsSimsDir_RequiresBoth`, `TestCopyDir`, `mkScenario`, `writeFile`, `mustMkdir`, and `TestRunGoCmd_PassAndFail`.

- [ ] **Step 4: Delete the Bash scripts.**

```bash
git rm sims/evals/run-scenario.sh sims/evals/run-eval.sh sims/evals/run-eval-docker.sh sims/evals/assert-grounded.sh
```

- [ ] **Step 5: Run vet and the full suite.**

Run: `go vet ./... && go test ./...`
Expected: PASS. No unused-symbol or compile errors.

- [ ] **Step 6: Update README.md.** Replace the baseline-check command and the local/docker eval lines with the single command:

```sh
cd sims/runner && go run . run            # all scenarios (preflight + docker eval)
cd sims/runner && go run . run 06         # one scenario
```

Remove references to `sims/evals/run-scenario.sh` / `run-eval-docker.sh` and the `runner baselines` command from `README.md`. Update the `sims/runner/` row in the components table to describe `runner run`.

- [ ] **Step 7: Update project memory.** Edit `/Users/stan/.claude/projects/-Users-stan-code-fsk/memory/runner-migration-status.md` to state the migration is complete: the Bash scripts are deleted, `runner run` is the single entrypoint, preflight is built into every run. Update the matching line in `MEMORY.md`.

- [ ] **Step 8: Commit.**

```bash
git add -A
git commit -m "runner: complete migration to single CLI; delete bash eval scripts"
```

---

## Self-Review

**Spec coverage:**
- Docker-only single path → Tasks 5, 6 (no local mode anywhere). ✓
- One command, two phases (`runner run`) → Tasks 6, 7. ✓
- Preflight = baseline check, fail loud → Task 6 (`baselineHolds`), Task 9 (`preflightAll` guard). ✓
- Artifact contract byte-for-byte → Task 4 (writers + contract test), verified against `runs.ts`. ✓
- meta.json shape, build.txt empty=PASS → Task 4. ✓
- transcript.jsonl verbatim → Task 5 (stdout passthrough to file). ✓
- Grounding folded in, no standalone command → Tasks 1, 6. ✓
- Config (model/effort/.env token/docker context) → Task 3, wired in Task 7. ✓
- Failure semantics (independent scenarios, agent error non-fatal) → Tasks 5 (`_ = run.Run()`), 7 (`runAll`). ✓
- Delete bash + baselines command → Task 9. ✓
- Testing: observe/preflight/artifact/orchestration unit-tested, one gated docker integration → Tasks 1-8. ✓

**Placeholder scan:** No TBD/TODO; every code step contains complete code; every run step has an exact command and expected result. ✓

**Type consistency:** `StepResult`, `Outcome`, `scenario`, `baselineSpec`, `canonicalBaseline`, `baselineHolds`, `observedBaseline`, `verdict`, `passFail`, `copyDir`, `discoverScenarios`, `readBaseline` are reused from the existing `baselines.go` with their current signatures. New names (`runConfig`, `runDir`, `observation`, `agent`, `dockerAgent`, `observeCore`, `runScenario`, `scenarioResult`, `runAll`, `filterScenarios`, `checkGrounded`, `gitDiffStaged`, `prepareRun`, `writeMeta`, `writeObserveArtifacts`, `checkBinaries`, `dockerEnv`, `dockerContext`, `readEnvToken`, `loadConfig`, `preflightAll`) are each defined once and consumed with matching signatures. ✓
