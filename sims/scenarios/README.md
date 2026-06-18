# Eval scenarios

Ten **code exercises** that grade a coding agent on the fiskaly-integration work
the implementer persona actually does. Each scenario hands a headless consumer
agent a business-framed task against an isolated copy of a Go fixture (the
`pos` checkout backend), with the fiskaly docs MCP for grounding, then scores the
result: does it still build and test green, what changed (diff), was it grounded
in the docs, and does it satisfy the deterministic conformance judge.

The single original exercise (`run-eval.sh`'s hardcoded "Zero to Receipt") is now
scenario `01`; the other nine extend coverage across the persona's failure
spectrum and **deliberately seed red herrings, false information, and dormant
silent bugs** that interfere with getting the integration right.

## The design principle

From `research/PERSONA.md`: **a normal bug throws an error; a fiscalization bug
looks like success.** So the traps split into two kinds, on purpose:

- **Gate-caught** — the deterministic judge flips a rule (an invented `/refunds`,
  a missing poll to `FINISHED`, a leftover legacy `/assets`, an absent VAT
  breakdown).
- **Review-caught** — silent bugs the static judge structurally cannot see
  (idempotency-key reuse, a blocking checkout call, a wrong VAT rate at scale,
  conflating the 24h JWT with the 90-day credential). These are graded by the
  judge's `expectations` (an LLM rubric) over the source and transcript.

A capable agent that grounds itself in the docs and reasons about the domain
should pass the gate scenarios and avoid the review traps. An agent that pattern-
matches or trusts the planted text will fail in ways that mirror real production
incidents.

## The ten scenarios

| # | Scenario | Tier | Trap | Planted in | Caught by |
| --- | --- | --- | --- | --- | --- |
| 01 | Zero to Receipt | 1 | — (control) | — | gate |
| 02 | Provision a merchant | 1 | false-info: a "quickstart" says POST the taxpayer directly (the 405 trap) | README | gate (`scope-identifier`, `commissioning`) |
| 03 | Cancellation / void | 2 | red-herring: a comment claims a fiskaly "refunds endpoint" | `refund.go` | gate (`cancellation-ref`, `no-invented-refunds`) |
| 04 | Idempotency under retry | 2 | silent-bug: one idempotency key reused across all requests | `fiskaly.go` | review |
| 05 | Outage / don't block the till | 3 | false-info: "calls are fast — call inline, no timeout, under the lock" | `checkout.go` | review |
| 06 | Fire-and-forget (no polling) | 3 | silent-bug: returns on PROCESSING, never polls to FINISHED | `fiskaly.go` | gate (`polling`) |
| 07 | Wrong VAT at scale | 3 | false-info: a cheat-sheet claims all food is 4% VAT | `vatrates.go` | review (+ `vat-breakdown` gate) |
| 08 | Amounts as decimal strings | 2 | silent-bug: money serialized as JSON floats, no VAT breakdown | `fiskaly.go` | gate (`vat-breakdown`) + review |
| 09 | CalVer migration | 2 | false-info: stale `/entities`/`/assets` + old `X-Api-Version` | `fiskaly.go` | gate (`no-legacy-resources`, `api-version-current`) |
| 10 | Credential expiry (day 91) | 3 | false-info: a daily 24h-token refresh "keeps you logged in forever" | `health.go` | review |

`Tier` is the `PERSONA.md` failure tier: 1 friction · 2 pre-prod bounce · 3 silent
catastrophe. Scenarios 04/06/08/09 are **brownfield** — they ship an unfinished,
flawed fiskaly client the agent inherits; the rest are **greenfield** (the
fiscalization seam plus a planted comment, README note, or domain helper).

## Baseline verification (the seed, before any agent)

Every fixture builds and tests green, and every seed judges **NON-COMPLIANT** with
at least one selected rule failing — so a correct solution has something to flip:

| # | build | tests | judge | rules pass/total at baseline |
| --- | --- | --- | --- | --- |
| 01 | PASS | PASS | NON-COMPLIANT | 0/5 |
| 02 | PASS | PASS | NON-COMPLIANT | 0/6 |
| 03 | PASS | PASS | NON-COMPLIANT | 1/7 (`no-invented-refunds` passes until a `/refunds` appears) |
| 04 | PASS | PASS | NON-COMPLIANT | 4/5 (`records-flow` is the task) |
| 05 | PASS | PASS | NON-COMPLIANT | 0/5 |
| 06 | PASS | PASS | NON-COMPLIANT | 5/6 (`polling` fails) |
| 07 | PASS | PASS | NON-COMPLIANT | 0/6 |
| 08 | PASS | PASS | NON-COMPLIANT | 5/6 (`vat-breakdown` fails) |
| 09 | PASS | PASS | NON-COMPLIANT | 4/6 (`api-version-current`, `no-legacy-resources` fail) |
| 10 | PASS | PASS | NON-COMPLIANT | 0/5 |

## Running

```sh
# One scenario, end to end (needs CLAUDE_CODE_OAUTH_TOKEN in repo .env + the claude CLI):
sims/evals/run-scenario.sh 06-fire-and-forget

# The original entrypoint still works; it is scenario 01 by default:
sims/evals/run-eval.sh                      # 01-zero-to-receipt
SCENARIO=03-cancellation sims/evals/run-eval.sh

# Hermetic Docker variant takes a scenario id too:
sims/evals/run-eval-docker.sh 09-calver-migration

# Just the deterministic judge against a seed (no agent):
cd sims/judge && go run . -scenario ../scenarios/06-fire-and-forget/scenario.json \
                                    ../scenarios/06-fire-and-forget/fixture
cd sims/judge && go run . -list             # the rule catalog
```

Each scenario directory holds `task.md` (the prompt), `fixture/` (the seed), and
`scenario.json` (metadata + the judge's `checks` and `expectations`). To author a
new scenario, follow [AUTHORING.md](AUTHORING.md).
