# Authoring eval scenarios

A scenario is one **code exercise**: a fiskaly-integration task handed to a
headless consumer agent against an isolated copy of a Go fixture, then graded.
The suite exists to push a coding agent to the limit on the work the fiskaly
implementer persona actually does, and deliberately seeds some fixtures with
**red herrings, false information, and dormant silent bugs** that interfere with
getting the integration right.

Read first: `research/PERSONA.md` (the failure spectrum these scenarios target),
`research/api-probes/NOTES.md` (the verified SIGN IT contract - the source of
truth for every domain claim), and `memo/OPPORTUNITIES.md`.

## The core design principle

From `PERSONA.md`: **a normal bug throws an error; a fiscalization bug looks like
success.** The dangerous failures are silent and compliance-shaped - a
perfect-looking receipt that never reached the tax authority. So the suite mixes
two kinds of trap on purpose:

- **Loud traps** - an invented `/refunds` endpoint, a missing poll to `FINISHED`,
  a leftover legacy `/assets`: a wrong contract that a careful read of the code
  catches.
- **Silent traps** - idempotency-key reuse, a blocking checkout call, a wrong VAT
  rate at scale, conflating the 24h JWT with the 90-day credential: the build stays
  green and the receipt still looks right.

Both kinds are graded the same way (see [The judge](#the-judge)): a deterministic
`checks` gate over the agent's trajectory, then an LLM `expectations` layer over the
resulting source. Every scenario encodes both in its `scenario.json`.

## Layout

```
eval-harness/backend/scenarios/<NN-slug>/
  scenario.json   # metadata + the judge's checks and expectations
  task.md         # the business-framed prompt handed to the agent
  fixture/        # a self-contained Go module (module `pos`), the seed codebase
```

## Fixture invariants (hold for every scenario)

- Module `pos`, Go 1.23, **standard library only** (no third-party imports - runs
  offline, hermetically).
- `go build ./...` and `go test ./...` are **green at baseline.** A silent bug is
  silent: no seed test reveals it. Where a trap needs a coverage gap, the gap is
  the point - it is intentional, not an oversight.
- The fiscalization hook is `fiscalize(ctx, *Order) error` in `checkout.go`,
  called by `CompleteOrder`, a no-op in greenfield seeds.
- **Greenfield** seeds (seam only) stay vendor-blind in code; the trap lives in a
  comment, the README, or a plausible-looking domain helper.
- **Brownfield** seeds ship an unfinished, *flawed* fiskaly client (e.g.
  `fiskaly.go`) that the agent inherits and must finish + fix. It must compile and
  leave the existing tests green, so it is **not wired into `fiscalize`** in the
  seed (wiring a real HTTP call would break the offline happy-path test); the task
  asks the agent to finish wiring it.
- The seed must judge as **NON-COMPLIANT**: with the trap unaddressed, at least one
  expectation is UNMET at baseline, and a correct solution flips it.

## task.md

Business-framed: what the business needs, never *how* fiscalization works, and
**never names the trap.** Discovering the "how" and seeing through the trap is the
exercise. Keep it to the register a senior backend engineer would get in a ticket.

## scenario.json

```json
{
  "id": "NN-slug",
  "title": "Human Title",
  "traps": [
    { "kind": "red-herring | false-info | silent-bug",
      "where": "fixture/<file>",
      "detail": "what the trap is",
      "correct": "what a faithful agent does instead" }
  ],
  "judge": {
    "checks": {
      "groundedBeforeWrite": true,
      "toolsCalled": [{ "name": "search_fiskaly_docs", "min": 1 }],
      "docsFetched": ["probe:records-flow"],
      "maxMcpErrors": 0
    },
    "expectations": [
      { "id": "records-flow", "expectation": "Issues the receipt as the two-call records flow (INTENTION then TRANSACTION), not a single POST." }
    ]
  }
}
```

`traps` is documentation for the author; the judge reads only `judge`. Expectation
`id`s are assigned automatically when absent.

## The judge

`eval-harness/backend/cmd/judge` runs two layers; conformance requires both.

1. **Deterministic checks** (`judge.checks`) - trajectory signals from the agent's
   run, evaluated programmatically. A failing check is a hard gate: the run is
   NON-COMPLIANT and the LLM layer is skipped. Fields (all optional):
   - `groundedBeforeWrite` - the agent called `search_fiskaly_docs` before its first
     code write (`Write`/`Edit`/`MultiEdit`).
   - `toolsCalled` - `[{ name, min }]`; each tool must be called at least `min` times.
   - `docsFetched` - corpus doc ids (`mcp/corpus/index.json`) the agent must fetch
     via `fetch_fiskaly_doc`.
   - `maxMcpErrors` - caps the MCP error count.
2. **LLM expectations** (`judge.expectations`) - natural-language conformance
   criteria graded by a stronger model over the source **and** trajectory, run only
   after the gate passes (`-expect`). Each MET must cite a verbatim `evidence_quote`
   that actually appears in the source or trajectory; an uncited or absent quote is
   downgraded to UNMET. Conformance requires every expectation to be a cited MET -
   the judge is conservative to a false PASS.

The judge reads non-test Go source only (a mock in `_test.go` cannot satisfy a
criterion), and the citation surface is the comment-stripped source, so a claim
that lives only in a comment is not evidence. Across the suite the checks are the
same grounding gate (search the docs, fetch the relevant article, no MCP errors);
the trap-specific conformance lives in each scenario's `expectations`.

## Run and verify a scenario

```sh
# source-only expectation grading of the seed (no trajectory, so the checks gate is
# skipped; needs the claude CLI):
cd eval-harness/backend && go run ./cmd/judge -scenario ../scenarios/<id>/scenario.json -expect ../scenarios/<id>/fixture

# full run in Docker, including the trajectory checks gate (needs CLAUDE_CODE_OAUTH_TOKEN
# in repo .env and the claude CLI):
cd eval-harness/backend && go run ./cmd/eval-harness run <id>
```
