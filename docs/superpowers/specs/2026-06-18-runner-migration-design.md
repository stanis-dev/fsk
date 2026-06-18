# Runner migration: Bash eval harness to a single Go CLI

Status: Design, approved 2026-06-18.

## Context

The eval workbench runs a consumer agent against fiskaly integration scenarios,
captures artifacts, and grades them. The runner is core product code, not
incidental scripting. Execution today is split across four Bash entrypoints in
`sims/evals/`:

- `run-scenario.sh` — local, context-isolated run (clean `HOME`, only the fiskaly MCP).
- `run-eval.sh` — thin delegator to `run-scenario.sh` with a default scenario.
- `run-eval-docker.sh` — hermetic run; the coder runs in Docker with only the fixture mounted.
- `assert-grounded.sh` — checks the transcript searched the docs before mutating code.

A first pass already shipped `runner baselines` (Go, module `runner` at
`sims/runner`). This design completes the migration: one Go binary owns every
eval operation, and the Bash orchestration scripts are deleted.

## Decisions

These were settled during brainstorming and are fixed for this design:

1. **Docker is the only execution path.** The local clean-`HOME` mode is
   eliminated, not ported. Every scenario runs identically.
2. **One command, two phases.** `runner run [ids...]` runs a baseline preflight,
   then the Docker eval. There is no separate `baselines` or `ground` command;
   that logic becomes a phase and an observe step.
3. **Architecture A: phase functions with a single test seam.** The only mocked
   boundary is the Docker agent step. Everything else is concrete and tested
   directly.
4. **Dashboard artifact compatibility is a hard constraint,** preserved
   byte-for-byte and verified against `sims/dashboard/lib/runs.ts`.

## Goals and non-goals

Goals: a single binary; all scenarios through one identical path; artifacts that
the existing dashboard reads unchanged; orchestration testable without Docker
except one gated integration test; deletion of the Bash orchestration scripts.

Non-goals: changing scenarios, fixtures, judge rules, or the dashboard;
supporting a local (non-Docker) run; changing how the Docker image is built;
migrating `dashboard.sh` (it launches the Next.js dev server and is not eval
orchestration).

## Command surface

```
runner run [ids...]      # no ids = all scenarios under sims/scenarios/[0-9]*; ids = a subset
```

This is the only command. `baselines` and the standalone grounding check are
removed; both fold into `run`.

## The single path (identical for every scenario)

Each scenario produces one run directory and is processed through the same four
phases. The build/test/judge core is shared and runs in both the preflight and
the post-agent observe. Diff and grounding run only post-agent: the pristine
copy has no agent changes to diff and no transcript to ground.

| Phase | Where | Does |
| --- | --- | --- |
| setup | host | `mktemp ~/.cache/fiskaly-eval/run.XXXXXX`, copy `fixture` to `pos`, `git init` + baseline commit, write `meta.json` `{harness:"docker", coder, model, effort, scenario}`. |
| preflight | host | build/test/judge the pristine copy; assert build PASS, tests PASS, judge NON-COMPLIANT, cross-checked against the scenario's `scenario.json` baseline block. A violation is a harness error: the seed is unsound, fail loud. |
| agent | Docker | `docker build` (cached) + `docker run` with only the work dir mounted; capture `transcript.jsonl` and `claude.err`; move telemetry out of the work tree to `mcp-telemetry.jsonl`. |
| observe | host | build/test/judge the agent-modified copy, then diff and grounding, writing `build.txt`, `test.txt`, `judge.txt`, `changes.diff`, `grounded.txt`. |

The runner does not compute a turns/cost summary. The dashboard already derives
that from `transcript.jsonl`, so there is no `jq` step and no summary artifact.

## Artifact contract (preserved byte-for-byte)

Verified against `sims/dashboard/lib/runs.ts`:

| File | Contract |
| --- | --- |
| run dir | named `run.*`, a directory under `~/.cache/fiskaly-eval`. |
| `meta.json` | `{harness, coder, model, effort, scenario}` strings. |
| `build.txt` | `go build` output; empty (trimmed) means PASS. |
| `test.txt` | `go test` output; contains `ok` and not `FAIL` means PASS. |
| `judge.txt` | presence means the run is done; contains `conformant` or `NON-COMPLIANT`. |
| `transcript.jsonl` | verbatim Claude stream-json; the dashboard reads its `system` event (`cwd == "/work"` implies docker) and `result` event. Passthrough, never reshaped. |
| `changes.diff` | `git diff --cached` of the agent's changes. |
| `grounded.txt` | grounding verdict. |
| `mcp-telemetry.jsonl` | per-call MCP telemetry. |
| `claude.err` | agent stderr. |

## Architecture and components

Module `runner` at `sims/runner` (unchanged location, matching the
one-module-per-subdir pattern of `judge`, `pos`, `mcp`). Files:

- `main.go` — argument dispatch and `findSimsRoot`.
- `run.go` — the per-scenario pipeline and the all-scenarios loop.
- `observe.go` — build, test, judge (the shared core, run in both phases) plus
  diff and grounding (post-agent only). The build/test/judge core is extracted
  from today's `baselines.go`.
- `docker.go` — the agent seam's real implementation (image build, `docker run`,
  context pinning, telemetry move).
- `artifacts.go` — run-dir creation, `meta.json`, and the artifact writers.
- `config.go` — flag parsing, `.env` token read, defaults.

The single interface:

```go
// agent runs the coder against the prepared work dir and returns where the
// transcript, stderr, and telemetry landed. The real implementation drives
// Docker; tests inject a fake.
type agent interface {
    run(work, task string, cfg runConfig) (agentResult, error)
}
```

`observe` reuses the existing `checker`-style functions so build/test/judge stay
the tested core.

## Config

- `--model` (default `claude-sonnet-4-6`), `--effort` (default `medium`).
- OAuth token read from `.env` (`CLAUDE_CODE_OAUTH_TOKEN`); never exported to the coder.
- Docker context `desktop-linux`, override with `DOCKER_CONTEXT`.
- Startup checks: `docker`, `go`, `git` present; daemon reachable; token found.

## Failure semantics (run-all)

Each scenario is independent. A preflight violation or a harness error for one
scenario is reported and forces a non-zero exit, but the remaining scenarios
still run. An agent error (Claude exits non-zero) is recorded in `claude.err`
and is not fatal, matching the Bash `|| true`. A final summary line reports how
many scenarios completed and the aggregate exit code.

## Testing strategy

- Unit: `observe` and preflight (mostly exist today), the artifact writer (read
  files back, assert the dashboard contract), `run` orchestration with a fake
  `agent` (no Docker), config parsing, and the ported grounding check.
- Integration: one `-short`-gated test that does a real Docker run of a single
  scenario and asserts the artifact set.

## Migration sequence (decomposition for the plan)

1. Extract `observe` from `baselines.go`; keep the existing tests green.
2. Add `config` and the artifact writer (`meta.json`, run dir).
3. Port `assert-grounded.sh` to Go as an observe step, using JSON parsing of the
   transcript instead of line-number grep.
4. Add the `agent` seam and its Docker implementation (port `run-eval-docker.sh`).
5. Wire `run` orchestration (setup, preflight, agent, observe, artifacts) with
   fake-agent unit tests.
6. Add all-scenarios and single-scenario handling and the failure semantics.
7. Add the gated real-Docker integration test.
8. Delete the four Bash scripts and the `baselines` subcommand; update `README`
   and project memory.

## Risks

- Artifact drift silently breaks the dashboard. Mitigated by writer tests that
  read files back and assert the contract above.
- Docker unavailable in CI. Mitigated by gating the integration test behind
  `-short`.
- `transcript.jsonl` must remain verbatim Claude stream-json; any reshaping
  breaks the dashboard's `system`/`result` parsing. The agent phase writes it as
  a passthrough.
