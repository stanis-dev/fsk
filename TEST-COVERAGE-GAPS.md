# Test coverage gaps

AGENTS.md lists "code not covered by tests" as never-tolerate. This is the
backlog of production functions currently at 0.0% coverage (or with materially
untested branches), grouped by what each one needs to become testable. Line
numbers are from the 2026-06-22 audit, verified against `go tool cover`.

Regenerate the source of truth with:

```sh
cd eval-harness/backend && go test ./... -short -coverprofile=/tmp/be.cov && go tool cover -func=/tmp/be.cov | awk '$3=="0.0%"'
cd mcp && go test ./... -coverprofile=/tmp/mcp.cov && go tool cover -func=/tmp/mcp.cov | awk '$3=="0.0%"'
```

## 1. Testable now — no seam needed

- [ ] `internal/config/config.go:49` `Load` — drive with `t.Setenv`/`t.TempDir`; assert the resolved `Config` and the error when the root can't be found. (`config_test.go` covers only `ReadEnvToken` and `ResolveRunsDir`.)
- [ ] `internal/config/config.go:84` `resolveRoot` (+ `isDir` at `:101`) — `t.TempDir` with `backend/` and `dashboard/` subdirs, `t.Chdir` into a nested dir, assert it walks up; plus a not-found negative case.
- [ ] `internal/config/config.go:138` `resolveDockerContext` — `t.Setenv("DOCKER_CONTEXT", ...)` for the override branch and the unset → `desktop-linux` default.
- [ ] `internal/judge/checks.go:142` `parseScenarioChecks` — write a temp `scenario.json` with a `judge.checks` block, assert the parsed `judgeChecks`; plus malformed-JSON and missing-file cases. (Mirror `TestParseScenarioExpectations`.)
- [ ] `internal/judge/report.go:28` `WriteReport` — `WriteReport` to a `t.TempDir()` path, re-read, assert indented + newline-terminated + round-trips.
- [ ] `internal/orchestrator/runner.go:74` `Resolve` — point `scenariosDir` at the fixtures; assert exact-id and numeric-prefix (`"06"`) match and `ok=false` for unknown.
- [ ] `internal/orchestrator/runner.go:88` `DockerContext` — assert the accessor returns the configured context (free once a `Runner` is constructed in a test).
- [ ] `internal/orchestrator/docker.go:27` `ContainerName` — exported wrapper over the 100%-covered `containerName`; one assertion that `ContainerName(p) == containerName(p)`.

## 2. Needs a test seam (external dependency)

- [ ] `internal/judge/rubric.go:16` `claudeModel` — shells out to the `claude` CLI and parses `--output-format json`. Make the binary overridable (package var or PATH shim) and assert it returns `Result` for a known envelope and errors on empty/invalid output.
- [ ] `internal/judge/evaluate.go:21` `Evaluate` and `:96` `infra` — the package's primary orchestrator (gate vs. expectation branching, infra-error paths, the neither-checks-nor-expectations guard) and its NON-COMPLIANT report builder. `Evaluate` hardcodes `claudeModel` at `evaluate.go:70`; inject the model function so the expectation path is reachable, then table-test: source-only (`RunDir=""`), gate-fail short-circuit, `Expect=true` with a fake model, and the infra-error report `Note` prefix.

## 3. Integration-only (covered solely by `TestRunScenario_RealDocker`)

These execute only under the docker-gated integration test, which is skipped by
`-short` and when docker/the `.env` token are absent — so they read as 0% in
ordinary runs. Decide per function: add a docker-gated unit test, or accept that
the real-docker integration test is their coverage.

- [ ] `internal/orchestrator/docker.go:48` `build`, `:59` `run`, `:96` `dockerEnv`, `:102` `dockerReachable`, `:30` `KillContainer`
- [ ] `internal/orchestrator/runner.go:33` `NewRunner`, `:62` `RunScenario`
- [ ] `internal/orchestrator/observe.go:21` `runJudge`

## 4. Entry points / thin adapters (conventionally untested)

Accept-with-rationale unless the policy is to cover these too.

- `cmd/eval-harness/main.go:18` `main`; `:63/:67/:71/:75` the `runnerAdapter` delegations (`RunScenario`/`Resolve`/`ContainerName`/`KillContainer`).
- `mcp/main.go:14` `main`.

## 5. Partial coverage (branches untested)

- [ ] `internal/artifacts/transcript.go` `summarizeTool` — ~13 named tool branches, roughly one exercised. Add table cases for at least Read/Write/Edit, the Grep path-append branch, the Task/Agent description fallback, and the default JSON-marshal branch.

## 6. Dashboard — no test tooling at all

`eval-harness/dashboard` ships zero unit tests; its gate is browser-e2e (AGENTS.md:
"a feature is not done until it has been run e2e in browser"). The one piece with
real branching logic:

- [ ] `lib/api.ts:5` `req` — URL build, non-2xx throw (`${method} ${path}: ${status}`), and the 204 → `undefined` branch. Either add a `vitest` unit test with a mocked `fetch`, or confirm these paths are exercised by an e2e pass.
