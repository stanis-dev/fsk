# 04 — Idempotency under retry (answer key)

**Tier 2 · silent-bug over an unfinished records flow.** The seed ships an
unfinished `fiskaly.go`: a teammate wired up auth through a `postWithRetry`
helper, but `issueReceipt` is a stub (`not implemented`) and the client is not
wired into `CompleteOrder`. The task is to finish the records flow, wire it into
`fiscalize`, and make checkout retries safe — without double-issuing a receipt.
A silent bug is buried in the inherited retry helper.

## The trap (silent-bug)

`newFiskalyClient` generates **one** idempotency key at client creation and
stashes it on `c.idemKey`, and `postWithRetry` sends that same key on every
attempt of every call:

```go
idemKey: newIdempotencyKey(), // generated once
...
req.Header.Set("X-Idempotency-Key", c.idemKey) // reused for every request
```

A planted `TODO(teammate)` comment endorses exactly this — "generate the
idempotency key once and reuse it so retries don't double-write" — which sounds
right and is half right.

The subtlety is what an idempotency key is *for* (NOTES.md addendum, 2026-06-14):
`X-Idempotency-Key` is required on **every** POST and must be a **lowercase-hex**
UUID v3/v4. Its job is to make a **retry of the same request** safe: replay the
identical POST with the same key and the server returns the original result
instead of acting twice.

- **Reusing one key across retries of the *same* request — correct.** That is the
  whole point; it is how a retried `/tokens` or a retried TRANSACTION POST avoids
  double-writing.
- **Reusing one key across *different* requests — wrong.** The INTENTION POST and
  the TRANSACTION POST are two distinct logical writes. With a single shared key,
  the second distinct POST looks to fiskaly like a **replay of the first** — the
  key is already bound to the INTENTION — so it collides (duplicate / `422`)
  instead of creating the TRANSACTION. The shared key turns every call after the
  first into an accidental replay.

So the seed's helper is safe for retries and broken for the multi-call flow the
task requires — and the flow it's broken for is the one the agent is about to
build.

## What a correct solution does

Grounded in the docs MCP (`search_fiskaly_docs` / `fetch_fiskaly_doc`) and the
probed contract (NOTES.md), the integration must finish the flow **and** get the
key lifecycle right:

1. **Fix the key to be per logical request.** Generate a fresh lowercase-hex key
   (`newIdempotencyKey`) for each distinct POST — `/tokens`, the INTENTION
   record, the TRANSACTION record are three different requests and get three
   different keys. Reuse the **same** key only when retrying that exact request
   (so the existing retry loop replays one POST with one stable key). Concretely:
   mint the key once per logical call (e.g. compute it before the retry loop, or
   pass it into `postWithRetry`), not once per client.
2. **Finish the records flow** — implement `issueReceipt` as the two-call flow:
   `POST /records` `INTENTION` (`operation.type: TRANSACTION`), then `POST
   /records` `TRANSACTION` (`operation.type: RECEIPT`) referencing the intention's
   `record.id`, carrying the document total and VAT computed from the `Order`.
3. **Keep the rest of the contract intact** — `authenticate` already does `POST
   /tokens` and reads the JWT from `content.authentication`; keep the
   `test.api.fiskaly.com` host, `X-Api-Version: 2026-02-03`, and an
   `X-Idempotency-Key` on every POST including `/tokens`.
4. **Wire it into `fiscalize`** — `fiscalize` in `checkout.go` is a no-op; have it
   construct/use the client and call `issueReceipt`, returning the error so a
   failed fiscalization keeps the order out of `completed` (`CompleteOrder`
   already gates the `paid -> completed` transition on `fiscalize` succeeding).
5. **Never double-issue** — a transient retry of the *same* TRANSACTION must reuse
   that request's key so it can never create a second receipt; distinct requests
   must each carry their own key so they don't collide.

## The failure mode

The agent does the visible work — implements the INTENTION/TRANSACTION calls and
wires `issueReceipt` into `fiscalize` — but **keeps the inherited shared key**
(trusting the `TODO` comment). On the happy path nothing in TEST necessarily
trips, but the second distinct POST is treated as a replay of the first: the
TRANSACTION collides with the INTENTION's key and fiskaly returns a duplicate /
`422`, so the receipt is never issued — or, in the mirror case, a key scheme that
is too aggressive about minting fresh keys defeats the retry safety and a retried
TRANSACTION **double-issues** the receipt (Tier 3, never acceptable). Either way
the flow is shaped correctly and looks done while being silently wrong about the
one thing this scenario is about.

## Acceptance

- `go build` / `go test` green; existing tests untouched; new tests for the new
  behavior (e.g. a stub that records the `X-Idempotency-Key` of each request:
  retries of one request share a key; the INTENTION and TRANSACTION posts use
  different keys; a retried TRANSACTION issues exactly one receipt).
- Judge (`fiskaly-host, token-exchange, idempotency-key, api-version,
  records-flow`) → **conformant**. Baseline is **4/5**: the seed already targets
  the real host, exchanges the token, sets `X-Idempotency-Key` and
  `X-Api-Version` — `records-flow` is the only base failure, because the seed
  deliberately ships no `/records` call.

## The catching signal

This is a **rubric-caught trap.** The static judge's `idempotency-key` rule only
checks that the literal `X-Idempotency-Key` header **appears** in the source — it
cannot see *how many* keys there are or *when* they are minted. The seed already
passes that rule (the header is present), and an agent that ships the shared key
still passes it. The key-lifecycle fix — one fresh key per logical request,
reused only across retries of that request, and never double-issuing — is graded
from the **diff and transcript** against this rubric:

- Is a **fresh** lowercase-hex key minted **per distinct POST** (so INTENTION and
  TRANSACTION carry different keys), not once at client creation?
- Is the **same** key reused **only** across retries of that one request (so a
  transient retry can't double-write)?
- Does a retried TRANSACTION issue **exactly one** receipt (proven by a test)?

The deterministic part is the **gate**: `records-flow` flips from FAIL to PASS
only when the agent actually implements the `/records` two-call flow, which
confirms the integration is shaped right and the client is wired in. With the
shape gated deterministically, the reviewer is left to judge just the
key-lifecycle behavior the static rule structurally can't — exactly the split
this scenario is built around: a deterministic gate proves the flow exists, the
rubric proves the keys are per-request.
