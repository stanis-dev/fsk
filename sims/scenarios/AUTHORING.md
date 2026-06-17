# Authoring eval scenarios

A scenario is one **code exercise**: a fiskaly-integration task handed to a
headless consumer agent against an isolated copy of a Go fixture, then graded.
The suite exists to push a coding agent to the limit on the work the fiskaly
implementer persona actually does — and, deliberately, to seed some fixtures with
**red herrings, false information, and dormant silent bugs** that interfere with
getting the integration right.

Read first: `research/PERSONA.md` (the failure spectrum these scenarios target),
`research/api-probes/NOTES.md` (the verified SIGN IT contract — the source of
truth for every domain claim), and `memo/OPPORTUNITIES.md`.

## The core design principle

From `PERSONA.md`: **a normal bug throws an error; a fiscalization bug looks like
success.** The dangerous failures are silent and compliance-shaped — a
perfect-looking receipt that never reached the tax authority. So the suite mixes
two kinds of trap on purpose:

- **Gate-caught traps** — the deterministic judge flips a rule (e.g. an invented
  `/refunds` endpoint, a missing poll to `FINISHED`, a leftover legacy `/assets`).
- **Review-caught traps** — silent bugs the static judge structurally cannot see
  (idempotency-key reuse, a blocking checkout call, a wrong VAT rate applied at
  scale, conflating the 24h JWT with the 90-day credential). These are graded by
  the scenario's `SOLUTION.md` rubric against the diff and transcript.

Every scenario states, in its `SOLUTION.md`, **which signal catches its trap.**

## Layout

```
sims/scenarios/<NN-slug>/
  scenario.json   # metadata + the judge rule set for this scenario
  task.md         # the business-framed prompt handed to the agent
  SOLUTION.md     # answer key: the trap, correct handling, fail modes, catching signal
  fixture/        # a self-contained Go module (module `pos`), the seed codebase
```

## Fixture invariants (hold for every scenario)

- Module `pos`, Go 1.23, **standard library only** (no third-party imports — runs
  offline, hermetically).
- `go build ./...` and `go test ./...` are **green at baseline.** A silent bug is
  silent: no seed test reveals it. Where a trap needs a coverage gap, the gap is
  the point — document it in `SOLUTION.md`.
- The fiscalization seam is `fiscalize(ctx, *Order) error` in `checkout.go`,
  called by `CompleteOrder`, a no-op in greenfield seeds.
- **Greenfield** seeds (seam only) stay vendor-blind in code; the trap lives in a
  comment, the README, or a plausible-looking domain helper.
- **Brownfield** seeds ship an unfinished, *flawed* fiskaly client (e.g.
  `fiskaly.go`) that the agent inherits and must finish + fix. It must compile and
  leave the existing tests green, so it is **not wired into `fiscalize`** in the
  seed (wiring a real HTTP call would break the offline happy-path test); the task
  asks the agent to finish wiring it.
- The seed must judge as **NON-COMPLIANT**: at least one selected rule fails at
  baseline and a correct solution flips it to pass.

## task.md

Business-framed: what the business needs, never *how* fiscalization works, and
**never names the trap.** Discovering the "how" and seeing through the trap is the
exercise. Keep it to the register a senior backend engineer would get in a ticket.

## scenario.json

```json
{
  "id": "NN-slug",
  "title": "Human Title",
  "tier": 1,
  "capability": "one line: what integration capability this exercises",
  "persona_ref": "where in PERSONA.md / OPPORTUNITIES.md this failure lives",
  "traps": [
    { "kind": "red-herring | false-info | silent-bug",
      "where": "fixture/<file>",
      "detail": "what the trap is",
      "correct": "what a faithful agent does instead" }
  ],
  "judge": { "rules": ["<rule-id>", "..."] },
  "baseline": { "build": "PASS", "tests": "PASS", "judge": "NON-COMPLIANT" },
  "target":   { "build": "PASS", "tests": "PASS", "judge": "conformant" }
}
```

`tier` is the `PERSONA.md` failure tier (1 friction · 2 pre-prod bounce · 3 silent
catastrophe). The runner reads only `judge.rules`; the rest is the answer key for
humans and the dashboard.

## The judge rule catalog

`cd sims/judge && go run . -list` prints every rule. Positive rules require a
distinctive token the correct contract must contain; negative (`deny`) rules fire
when a red-herring token appears. Today's catalog:

| rule | kind | what it asserts |
| --- | --- | --- |
| `fiskaly-host` | want | targets `test/live.api.fiskaly.com` |
| `token-exchange` | want | `POST /tokens` for the JWT |
| `idempotency-key` | want | `X-Idempotency-Key` on writes |
| `api-version` | want | the `X-Api-Version` header |
| `api-version-current` | want | the **current** `2026-02-03` version |
| `records-flow` | want | issues via `/records` |
| `scope-identifier` | want | `X-Scope-Identifier` (UNIT-scoped subject) |
| `commissioning` | want | `COMMISSIONED` lifecycle PATCH |
| `cancellation-ref` | want | a `CANCELLATION` record (voiding) |
| `no-invented-refunds` | deny | fails if a `/refunds` endpoint appears |
| `polling` | want | polls to the `FINISHED` terminal state |
| `vat-breakdown` | want | sends `exclusive` **and** `inclusive` VAT fields |
| `no-legacy-resources` | deny | fails if `/assets` or `/entities` appears |

## Run and verify a scenario

```sh
# baseline judge verdict on the seed (no agent):
cd sims/judge && go run . -scenario ../scenarios/<id>/scenario.json ../scenarios/<id>/fixture

# full run (needs a CLAUDE_CODE_OAUTH_TOKEN in repo .env and the claude CLI):
sims/evals/run-scenario.sh <id>
```
