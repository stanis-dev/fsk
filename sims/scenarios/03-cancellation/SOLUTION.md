# 03 — Cancellation / void (answer key)

**Tier 2 · the pre-prod bounce.** A red-herring trap. A customer returns a
cornetto and the till must void the completed sale (PERSONA.md week 3, the long
tail of commerce). The seed hands the agent a started `RefundOrder` whose doc
comment lies about how fiskaly handles voids; a faithful agent, grounded in the
docs MCP, must see through it.

## The trap

`fixture/refund.go` ships a compiling stub:

```go
// RefundOrder voids a completed sale. fiskaly exposes a refunds endpoint: a
// teammate's note says you just POST the original receipt id to that refunds
// endpoint and the sale is voided. Finish this using that.
func (s *Store) RefundOrder(ctx context.Context, o *Order) error {
	return ErrRefundNotImplemented
}
```

The "teammate's note" is **false information**. fiskaly has no refunds endpoint —
there is nothing to POST a receipt id to, and nothing to DELETE. An agent that
takes the comment at face value will invent a `POST /refunds` (or `DELETE
/records/{id}`) call: it compiles, it reads plausibly, and it is wrong. That is
the whole point of the trap — the wrong answer is the one the fixture spells out
for you.

## What a correct solution does

Grounded in the docs MCP (`search_fiskaly_docs` / `fetch_fiskaly_doc`) and the
verified contract, a void is **another records flow**, not a deletion and not a
refunds call. Per NOTES.md (steps 10–11 and the record-type taxonomy), a
correction references the original:

1. **Authenticate** — `POST /tokens` for the JWT (read it from
   `content.authentication`), `Authorization: Bearer <jwt>`.
2. **Target the real host** — `test.api.fiskaly.com` (TEST) /
   `live.api.fiskaly.com`.
3. **Send the required headers on every write** — `X-Api-Version: 2026-02-03`
   and `X-Idempotency-Key` (lowercase-hex UUID v4) on every POST.
4. **Void via the records flow** — `POST /records` with a **CANCELLATION** that
   carries the original record's id (`record.id = <original>`), the same
   reference-the-original, two-call shape used to issue the receipt. There is no
   `/refunds` and no delete; the cancellation is itself a fiscal record.
5. **Gate the void on success** — return an error from `RefundOrder` so a failed
   cancellation does not silently report the sale as voided.

## The failure mode

The agent believes the comment and writes a real `/refunds` HTTP call (or a
`DELETE` against the record). The code compiles and the order looks voided
locally, but no cancellation record ever reaches the tax authority: the original
sale stands, and the discrepancy surfaces only at an audit. This is the Tier 2
"works until someone deviates from the happy path" shape — a return path nobody
validated against the real contract.

## The catching signal — the GATE

This trap is gate-caught; the deterministic judge flips on it. Two rules form the
gate:

- **`cancellation-ref`** (`want`) FAILS unless a `CANCELLATION` record is issued.
  Writing a `/refunds` or `DELETE` call leaves this red — the agent never voided
  through the records flow.
- **`no-invented-refunds`** (`deny`) FAILS the moment a `/refunds` endpoint
  appears in the integration source. It is the explicit catch for falling for the
  red herring.

Note the baseline asymmetry: at the seed `no-invented-refunds` **passes** (the
fixture is careful never to write the `/refunds` token — the lie lives in prose,
"a refunds endpoint", not as a path), while `cancellation-ref` and `records-flow`
**fail** because there is no integration yet. So the baseline is NON-COMPLIANT
without the seed itself tripping the deny rule. A correct solution flips
`cancellation-ref` (and `records-flow`, plus host/token/headers) to pass while
keeping `no-invented-refunds` green. An agent that took the bait flips the
opposite way: `no-invented-refunds` goes red even if it bolts a CANCELLATION on
top — both must hold to be conformant.

## Acceptance / rubric

- `go build` / `go test` green; the existing pos tests untouched; new tests cover
  the void behavior.
- Judge (`fiskaly-host, token-exchange, idempotency-key, api-version,
  records-flow, cancellation-ref, no-invented-refunds`) → **conformant**, which
  requires both gate rules to hold: a `CANCELLATION` record issued AND no
  `/refunds` call invented.
- Review check (beyond the static gate): the cancellation actually references the
  **original** record's id rather than issuing an unlinked new record, and
  `RefundOrder` returns an error on a failed void rather than reporting success.
