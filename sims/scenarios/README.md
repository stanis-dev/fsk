# Eval scenarios

Ten **code exercises** that grade a coding agent on the fiskaly-integration work
the implementer persona actually does. Each scenario hands a headless consumer
agent a business-framed task against an isolated copy of a Go fixture (the
`pos` checkout backend), with the fiskaly docs MCP for grounding, then scores the
result: does it still build and test green, what changed (diff), was it grounded
in the docs, and does it satisfy the judge (deterministic checks + an LLM
expectation layer).

The single original exercise (`run-eval.sh`'s hardcoded "Zero to Receipt") is now
scenario `01`; the other nine extend coverage across the persona's failure
spectrum and **deliberately seed red herrings, false information, and dormant
silent bugs** that interfere with getting the integration right.

## The design principle

From `research/PERSONA.md`: **a normal bug throws an error; a fiscalization bug
looks like success.** So the traps split into two kinds, on purpose:

- **Loud traps** — an invented `/refunds`, a missing poll to `FINISHED`, a leftover
  legacy `/assets`, an absent VAT breakdown: a wrong contract a careful read catches.
- **Silent traps** — idempotency-key reuse, a blocking checkout call, a wrong VAT
  rate at scale, conflating the 24h JWT with the 90-day credential: the build stays
  green and the receipt still looks right.

Both are graded the same way: a deterministic `checks` gate over the agent's
trajectory (grounded in the docs, the relevant article fetched, no MCP errors), then
an LLM `expectations` layer over the resulting source. A capable agent that grounds
itself and reasons about the domain passes; one that pattern-matches or trusts the
planted text fails in ways that mirror real production incidents.

## The ten scenarios

| # | Scenario | Trap | Planted in |
| --- | --- | --- | --- |
| 01 | Zero to Receipt | — (control) | — |
| 02 | Provision a merchant | false-info: a "quickstart" says POST the taxpayer directly (the 405 trap) | README |
| 03 | Cancellation / void | red-herring: a comment claims a fiskaly "refunds endpoint" | `refund.go` |
| 04 | Idempotency under retry | silent-bug: one idempotency key reused across all requests | `fiskaly.go` |
| 05 | Outage / don't block the till | false-info: "calls are fast — call inline, no timeout, under the lock" | `checkout.go` |
| 06 | Fire-and-forget (no polling) | silent-bug: returns on PROCESSING, never polls to FINISHED | `fiskaly.go` |
| 07 | Wrong VAT at scale | false-info: a cheat-sheet claims all food is 4% VAT | `vatrates.go` |
| 08 | Amounts as decimal strings | silent-bug: money serialized as JSON floats, no VAT breakdown | `fiskaly.go` |
| 09 | CalVer migration | false-info: stale `/entities`/`/assets` + old `X-Api-Version` | `fiskaly.go` |
| 10 | Credential expiry (day 91) | false-info: a daily 24h-token refresh "keeps you logged in forever" | `health.go` |

Scenarios 04/06/08/09 are **brownfield** — they ship an unfinished, flawed
fiskaly client the agent inherits; the rest are **greenfield** (the fiscalization
seam plus a planted comment, README note, or domain helper).

## Baseline verification (the seed, before any agent)

Every fixture builds and tests green and judges **NON-COMPLIANT** at baseline: the
trap is unaddressed, so a correct solution has something to flip. The deterministic
`checks` are trajectory-derived, so they apply to an agent run; a bare seed is graded
by the source-only `expectations` layer (`go run . -scenario … -expect …/fixture`).

## Running

```sh
# One scenario, end to end (needs CLAUDE_CODE_OAUTH_TOKEN in repo .env + the claude CLI):
sims/evals/run-scenario.sh 06-fire-and-forget

# The original entrypoint still works; it is scenario 01 by default:
sims/evals/run-eval.sh                      # 01-zero-to-receipt
SCENARIO=03-cancellation sims/evals/run-eval.sh

# Hermetic Docker variant takes a scenario id too:
sims/evals/run-eval-docker.sh 09-calver-migration

# Source-only expectation grading of a seed (no trajectory; needs the claude CLI):
cd sims/judge && go run . -scenario ../scenarios/06-fire-and-forget/scenario.json -expect \
                                    ../scenarios/06-fire-and-forget/fixture
```

Each scenario directory holds `task.md` (the prompt), `fixture/` (the seed), and
`scenario.json` (metadata + the judge's `checks` and `expectations`). To author a
new scenario, follow [AUTHORING.md](AUTHORING.md).
