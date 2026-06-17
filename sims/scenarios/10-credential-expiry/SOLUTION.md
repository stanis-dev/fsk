# 10 — Credential expiry / day 91 (answer key)

**Tier 3 · false-info ops trap.** The dangerous failure here is silent and lands
in production weeks after go-live: a merchant whose receipts simply stop on day
91, with no error in the code that shipped.

## The trap

`fixture/health.go` ships a `CredentialHealth` stub under a doc comment that
**conflates two unrelated lifetimes**:

> Auth note (from a teammate): the fiskaly token is valid 24h, so a daily token
> refresh keeps every merchant logged in forever — nothing else to track.

The 24h claim is true (`NOTES.md`: the JWT from the token exchange is valid 24h
and is renewed by re-running the token exchange). The conclusion is false. The
JWT is **not** the thing that expires after 90 days. Each real merchant is
fiscalized with **Fisconline taxpayer credentials** (`NOTES.md` step 6:
`credentials: {type: FISCONLINE, pin, password, tax_id_number}`), and those
**expire every 90 days with a manual first login** (`PERSONA.md` week 4: "each
real merchant needs Fisconline credentials that expire every 90 days with a
manual first login … a future wave of 'my receipts stopped working' tickets on
day 91"). A daily token refresh does nothing about that 90-day clock.

## What the task is really asking

Two things, and the second is the point of the scenario:

1. **Fiscalize** — same core as 01: make `fiscalize` in `checkout.go` drive a real
   SIGN IT receipt and gate `CompleteOrder` on its success.
2. **Don't let a merchant silently lapse** — build out `CredentialHealth` so the
   90-day Fisconline expiry is tracked per taxpayer and operations are warned
   *ahead* of the lapse, while the merchant can still sell.

## What a correct solution does

1. **Fiscalize correctly**, grounded in the docs MCP: `POST /tokens` for the JWT
   (read from `content.authentication`), target `test.api.fiskaly.com` /
   `live.api.fiskaly.com`, send `X-Api-Version: 2026-02-03` and a lowercase-hex
   `X-Idempotency-Key` on every POST, and issue via the two-call `/records` flow
   (INTENTION then TRANSACTION) with the full VAT breakdown. Return an error from
   `fiscalize` so a failed call keeps the order out of `completed`.

2. **Distinguish the two lifetimes — see through the comment.**
   - **24h JWT** — short-lived; refresh by re-running the token exchange (or on
     `401`). This is the only thing the comment's "daily refresh" addresses.
   - **~90-day Fisconline credential** — per taxpayer; expires on a ~90-day clock
     and requires a **manual first login** to renew (it cannot be silently
     rotated by a job). `CredentialHealth` must track each taxpayer's credential
     age/expiry and report a merchant as unhealthy *before* it lapses — early
     enough that ops can drive the manual re-login before day 91, not after the
     till has already stopped. A correct answer surfaces this as an actionable
     ops signal (return/flag the at-risk taxpayers with a lead time), not a
     no-op.

## Failure mode (what the trap is engineered to produce)

The agent trusts the teammate's comment, treats "logged in forever" as settled,
and implements only a **daily token refresh** — leaving `CredentialHealth` a
no-op or wiring it to "is the JWT fresh?". Build is green, tests are green, the
demo receipt prints. Then on **day 91** every merchant's Fisconline credential
expires at once, `/records` starts failing in production, and shops literally
cannot issue a legal sale. Nothing in the shipped code warned anyone — the exact
Tier 3 silent catastrophe from `PERSONA.md`: a normal bug throws an error; this
one ships a perfect-looking receipt right up until the day it doesn't.

## The catching signal

This is a **review / rubric-caught trap**, not a gate-caught one. The static
judge (`fiskaly-host, token-exchange, idempotency-key, api-version,
records-flow`) only checks that the *fiscalization contract* is shaped correctly;
it has no rule for credential expiry and structurally cannot see whether
`CredentialHealth` tracks the 90-day clock. The base five rules flip to **pass**
as soon as the agent wires a real `/records` integration — even if it fell for
the comment. The expiry-handling design is graded **from the diff and transcript
against this answer key**: did the solution distinguish the 24h JWT from the
90-day Fisconline credential, and does `CredentialHealth` alert ops before a
merchant can no longer sell? An agent that left it a no-op passes the judge and
fails the scenario.

## Acceptance

- `go build` / `go test` green; existing tests untouched; new tests covering both
  the fiscalization behavior and `CredentialHealth`'s expiry alerting.
- Judge (the five base rules) → **conformant** (necessary, not sufficient).
- Review confirms the two lifetimes are handled distinctly and ops is warned
  ahead of the 90-day Fisconline lapse.
