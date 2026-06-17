# 08 — Amounts as decimal strings (answer key)

**Tier 2 · the pre-prod bounce.** This is the model-impedance failure from
`PERSONA.md` week 2: not a crash, but a receipt whose money is serialized in the
wrong JSON type and whose VAT is never broken out — so the API silently rejects
it, or accepts a figure that has already lost cents. The bug is planted as a
**silent bug** in an inherited, half-finished client, and the exercise is whether
the agent notices that "looks like 4.33" is not the same as "is the string the
schema accepts."

## What the task is really asking

`fiscalize` in `checkout.go` is a no-op. `CompleteOrder` already calls it and
only moves the order to `completed` on success. A teammate started a SIGN IT
client in `fiskaly.go` that authenticates and posts the two-call records flow but
was never wired in and was left off mid-receipt. The agent must finish it, wire
it into `CompleteOrder`, and — the load-bearing part — represent the money and
VAT the way fiskaly actually requires. fiskaly **derives no figure**: the
integration computes every field, in the right type, reconciling to the document
total to the cent.

## The trap

`issueReceipt` ships two defects under an honest-looking `TODO(teammate)` comment
("Sending the order total as the euro amount, e.g. 4.33. Haven't done the per-line
VAT split yet."):

1. **Amounts as JSON numbers.** `total := float64(o.Gross()) / 100.0` is marshaled
   as a JSON float (`4.33`). The SIGN IT contract requires monetary amounts to be
   **decimal STRINGS** matching `^(-)?\d{1,12}(\.\d{1,8})?$` (`NOTES.md`
   money-model). A bare number is the wrong type, and the float round-trip loses
   cents on the values that do not divide cleanly into binary fractions — the
   classic `0.1 + 0.2` family. The receipt looks perfect in a debugger and is
   wrong on the wire.
2. **No VAT breakdown.** Only a single `document.total` is sent. fiskaly needs the
   full per-line `VatRateCategory` — `percentage`, `amount`, `exclusive` (net),
   `inclusive` (gross) — and **derives none of the four** (`NOTES.md`: a
   `VatRateCategory` requires ALL of percentage, amount, exclusive, inclusive). A
   receipt with no breakdown either bounces in QA or, worse, under/over-states the
   VAT the document carries.

It compiles, the existing tests stay green (the client is never wired in), and it
reads like exactly the corner a hurried generalist cuts — Giulia in week 2, deep
in the arithmetic, who shipped the happy-path total first and meant to come back
for the VAT split.

## What a correct solution does

Grounded in the docs MCP (`search_fiskaly_docs` / `fetch_fiskaly_doc`), the
integration must:

1. **Authenticate** — `POST /tokens` with the API key/secret; read the JWT from
   `content.authentication` (not `content.access_token`), send it as
   `Authorization: Bearer <jwt>`. (The seed already does this.)
2. **Target the real host** — `test.api.fiskaly.com` / `live.api.fiskaly.com`.
   (Seed already does this.)
3. **Send the required headers on every call** — `X-Api-Version: 2026-02-03` and
   `X-Idempotency-Key` (lowercase-hex UUID v4) on every POST including `/tokens`.
   (Seed already does this.)
4. **Issue the receipt as the two-call records flow** — `POST /records` INTENTION,
   then `POST /records` TRANSACTION (RECEIPT) referencing the intention. (Seed
   already does this.)
5. **Represent every amount as a decimal string.** Replace the `float64` euros
   with a formatter that turns integer `Cents` into a decimal string matching
   `^(-)?\d{1,12}(\.\d{1,8})?$` — e.g. `433` cents → `"4.33"` — and use it for
   every monetary field (line `exclusive`/`inclusive`/`amount`, the per-rate
   `VatRateCategory`, the document `total` / `total_vat`). No field on the wire is
   a JSON number; the cent value is exact because it never round-trips through a
   binary float.
6. **Compute and send the full per-line VAT breakdown.** Each `LineItem` already
   carries its `VATRate`, and `Order`/`LineItem` already compute `Net`, `VAT`, and
   `Gross` per line, half-up to the cent (`order.go`). Build the
   `VatRateCategory` from those real figures — `percentage` (the rate),
   `exclusive` (net), `inclusive` (gross), `amount` (the VAT) — grouped per rate
   and summed so the breakdown **reconciles to the document total to the cent**
   (`NOTES.md` money-model). Per-line rounding before summation matches the
   model's `LineItem.VAT()`, so the parts add up to the whole.
7. **Gate completion on success** — return an error from `fiscalize` so a failed
   call keeps the order out of `completed`.

## The failure mode

The agent finishes the wiring, sees the receipt issue against a stub, and moves
on — leaving the `float64` total in place and never adding the breakdown.
Everything still looks fine: the order validates, the client posts, the happy
path is green. But on the wire the amount is the wrong JSON type and has shed
cents on awkward values, and the document carries no VAT breakdown at all. In QA
against the real API this bounces — a `422`/`400` on the schema, or a document
whose VAT does not reconcile — and the ticket comes straight back to dev. That is
the Tier-2 pre-prod-bounce signature: works on the golden path in isolation,
breaks the moment it meets the strict schema.

## The catching signal

**Partly GATE, partly RUBRIC.**

- **GATE** — the **`vat-breakdown`** rule fails at baseline because the seed sends
  only a `total`: neither `exclusive` nor `inclusive` appears in the source. It
  passes only once the agent sends the full `VatRateCategory` with both fields.
  This is the deterministic backstop and the reason the seed judges NON-COMPLIANT
  (5/6 — the five base rules pass, `vat-breakdown` fails).
- **RUBRIC** — the static judge **structurally cannot tell a string amount from a
  float**: `"4.33"` and `4.33` look identical to a token-matching rule, and it
  cannot check that the parts reconcile to the cent. So the string-vs-float fix
  and the cent-reconciliation are graded from the diff and transcript against this
  answer key: are amounts emitted as decimal strings matching the pattern (not
  JSON numbers), and does the per-line breakdown sum to the document total exactly?
  An integration that sends the breakdown as floats, or whose VAT does not
  reconcile, passes all six gates and is still NON-CONFORMANT.

## Acceptance

- `go build` / `go test` green; existing tests untouched; new tests for the
  fiscalization behavior — ideally asserting amounts serialize as decimal strings
  (e.g. `433` → `"4.33"`) and that the per-line breakdown reconciles to the order
  total.
- Judge (`fiskaly-host, token-exchange, idempotency-key, api-version,
  records-flow, vat-breakdown`) → **conformant**.
- Review: every monetary field is a decimal string matching
  `^(-)?\d{1,12}(\.\d{1,8})?$`, and the per-line `VatRateCategory` reconciles to
  the document total to the cent. A receipt that ships float amounts, or a VAT
  breakdown that does not add up, is NON-CONFORMANT even when all six gates pass.
