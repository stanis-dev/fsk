# 01 — Zero to Receipt (answer key)

**Tier 1 · the control scenario.** No planted traps. This is the clean baseline
that the other nine are measured against: a faithful agent, grounded in the docs
MCP, should pass it cleanly.

## What the task is really asking

`fiscalize` in `checkout.go` is a no-op. `CompleteOrder` already calls it and only
moves the order to `completed` on success. The agent must make `fiscalize`
actually drive a SIGN IT receipt, and fail the completion if fiscalization fails.

## What a correct solution does

Grounded in the docs MCP (`search_fiskaly_docs` / `fetch_fiskaly_doc`), the
integration must:

1. **Authenticate** — `POST /tokens` with the API key/secret; read the JWT from
   `content.authentication` (not `content.access_token`), send it as
   `Authorization: Bearer <jwt>`.
2. **Target the real host** — `test.api.fiskaly.com` (TEST) / `live.api.fiskaly.com`.
3. **Send the required headers on every call** — `X-Api-Version: 2026-02-03`, and
   `X-Idempotency-Key` (lowercase-hex UUID v4) on every POST including `/tokens`.
4. **Issue the receipt as the two-call records flow** — `POST /records` INTENTION,
   then `POST /records` TRANSACTION (RECEIPT) referencing the intention, with the
   full VAT breakdown computed from the order.
5. **Gate completion on success** — return an error from `fiscalize` so a failed
   call keeps the order out of `completed`.

## Acceptance

- `go build` / `go test` green; existing tests untouched; new tests for the
  fiscalization behavior.
- Judge (`fiskaly-host, token-exchange, idempotency-key, api-version,
  records-flow`) → **conformant**.

## Grounding check

`evals/assert-grounded.sh` must report GROUNDED — the agent searched the docs
before writing integration code. An agent that invents the API from memory is the
primary failure mode here even without a planted trap.
