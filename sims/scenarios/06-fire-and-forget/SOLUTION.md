# 06 — Fire-and-forget (no polling) (answer key)

**Tier 3 · the silent catastrophe.** This is the canonical fire-and-forget
failure from `PERSONA.md`: a sale that never reaches the tax authority, where the
customer is happy, the receipt looks normal, and it surfaces only at an audit
with retroactive fines. The bug is planted as a **silent bug** in an inherited,
half-finished client — and the exercise is whether the agent notices that
"accepted" is not "filed."

## What the task is really asking

`fiscalize` in `checkout.go` is a no-op, and `CompleteOrder` already calls it and
only moves the order to `completed` on success. A teammate left a `fiskalyClient`
in `fiskaly.go` that authenticates and runs the two-call records flow, but it is
**not wired into `fiscalize`** and it stops one step short. The agent must finish
the client and wire it in so a paid order is fiscalized — *and actually filed* —
before it can reach `completed`. The task says it in one line: "correct in
production, not just in the test sandbox."

## The trap

`issueReceipt` posts the INTENTION and then the TRANSACTION (RECEIPT), and the
moment the TRANSACTION POST is accepted it returns `nil`:

```go
// Accepted. We're done. (TEST is synchronous.)
return nil
```

A `TODO(teammate)` comment rationalizes it: "In TEST the transaction comes back
state=COMPLETED synchronously, so once the TRANSACTION POST is accepted we just
treat the receipt as issued and return — nothing to wait on."

That comment is **true in TEST and dangerously wrong in LIVE.** Per `NOTES.md`
step 11: in TEST the TRANSACTION returns `state=COMPLETED mode=FINISHED`
synchronously, so a fire-and-forget client looks perfect. But **LIVE is async** —
the TRANSACTION POST is merely *accepted* (the INTENTION is `state=ACCEPTED
mode=PROCESSING`), and the record only reaches the `FINISHED` terminal state
later. A client that returns on the accepted POST treats `PROCESSING` as done: on
LIVE the record may sit in `PROCESSING`, or transition to `FAILED`, and never be
filed with the Agenzia delle Entrate. The order is marked `completed`, the
customer walks out with a receipt, and nothing reached the authority.

This is the exact shape `PERSONA.md` flags as the genuinely dangerous one: **a
normal bug throws an error; a fiscalization bug looks like success.** Loud
failures get caught in TEST; this one is silent precisely *because* TEST is
synchronous and hides it.

## What a correct solution does

Grounded in the docs MCP (`search_fiskaly_docs` / `fetch_fiskaly_doc`), the
integration must:

1. **Authenticate** — `POST /tokens` with the API key/secret; read the JWT from
   `content.authentication` (not `content.access_token`), send it as
   `Authorization: Bearer <jwt>`. (The seed already does this.)
2. **Target the real host** — `test.api.fiskaly.com` (TEST) / `live.api.fiskaly.com`.
3. **Send the required headers on every call** — `X-Api-Version: 2026-02-03` and
   `X-Idempotency-Key` (lowercase-hex UUID v4) on every POST. (The seed already
   does this.)
4. **Issue the receipt as the two-call records flow** — `POST /records` INTENTION,
   then `POST /records` TRANSACTION (RECEIPT) referencing the intention. (The seed
   already does this much.)
5. **Poll the record to the `FINISHED` terminal state — the load-bearing fix.**
   After the TRANSACTION is accepted, `GET /records/{id}` and wait until the
   record's `state`/`mode` reaches `FINISHED` (`NOTES.md` step 11: "LIVE will be
   async → still poll"). Only a `FINISHED` record is filed. A bounded poll
   (sensible interval + cap/`ctx` deadline) that ends in `FINISHED` is the bar;
   if it does not reach `FINISHED` — it stays `PROCESSING` past the deadline, or
   goes `FAILED` — the call must **return an error**, not success.
6. **Gate completion on it** — wire `issueReceipt` into `fiscalize` (replace the
   no-op) so that a receipt that never reaches `FINISHED` returns an error from
   `fiscalize`, and `CompleteOrder` therefore keeps the order out of `completed`.

## The failure mode

The agent reads the inherited client, sees a complete-looking two-call flow with
a confident `TODO` explaining why there is nothing to wait on, and **wires it
into `fiscalize` exactly as-is** — keeping the fire-and-forget. Everything passes:
the build is green, the seed tests stay green, and in TEST the receipt comes back
`FINISHED` synchronously so even a new happy-path test looks correct. The
scenario is "done." But on LIVE the same code returns success the instant the
TRANSACTION is accepted, so a sale stuck in `PROCESSING` (or one that goes
`FAILED`) is silently marked `completed` and never filed — the audit-time
catastrophe. The agent shipped a green build that does not fiscalize in
production. That is the Tier-3 signature: the test sandbox is the very thing that
hides the bug.

## The catching signal

**The GATE.** Unlike the wrong-VAT scenario, this trap is visible to the
deterministic judge. The `polling` rule requires the `FINISHED` terminal-state
token (`(?i)\bFINISHED\b`) somewhere in the non-test source. The stricter
polling gates also require `FAILED` handling, context/timer bounds, checked
record IDs, checked response decoding, no no-op fiscalization path, and no
network call under the store lock. The five base rules (`fiskaly-host`,
`token-exchange`, `idempotency-key`, `api-version`, `records-flow`) all PASS at
baseline because the inherited client already targets the right host, exchanges
the token, sets both headers, and uses `/records`. The seed now judges **5/12,
NON-COMPLIANT**: the base flow is present, but the production-safety gates fail.

Review backs the gate: a faithful poll must also *gate completion* on the
`FINISHED` result (return an error when the record does not reach `FINISHED`) and
be wired into `fiscalize`. An agent that prints "`FINISHED`" in a comment to
satisfy the token but still returns on `PROCESSING`, or that polls but ignores a
non-`FINISHED` outcome, passes the gate yet is NON-CONFORMANT on review.

## Acceptance

- `go build` / `go test` green; existing tests untouched; new tests for the
  fiscalization behavior (ideally asserting that a record left in `PROCESSING`,
  or returned `FAILED`, makes `fiscalize`/`CompleteOrder` fail and the order does
  **not** reach `completed`, while a `FINISHED` record completes).
- Judge (`fiskaly-host, token-exchange, idempotency-key, api-version,
  records-flow, polling, terminal-failure, bounded-polling, terminal-record-id,
  no-swallowed-response-errors, no-fiscalization-noop,
  no-lock-during-fiscalization`) → **conformant**.
- Review: `issueReceipt` polls `GET /records/{id}` to the `FINISHED` terminal
  state and returns an error otherwise; it is wired into `fiscalize` so a
  non-`FINISHED` record keeps the order out of `completed`. A client that keeps
  the fire-and-forget is NON-CONFORMANT even if every other gate passes.
