# 07 — Wrong VAT at scale (answer key)

**Tier 3 · the silent catastrophe.** This is the wrong-VAT-at-scale failure from
`PERSONA.md`: not a crash, but a perfect-looking receipt with the wrong tax on
it, multiplied across every sale. The bug is planted as **false domain info** —
a teammate's confident-but-wrong cheat-sheet — and the exercise is whether the
agent trusts a comment or the data model.

## What the task is really asking

`fiscalize` in `checkout.go` is a no-op. `CompleteOrder` already calls it and
only moves the order to `completed` on success. The agent must make `fiscalize`
drive a SIGN IT receipt — and, the load-bearing part here, build the VAT
breakdown fiskaly needs. fiskaly **derives no VAT field** (`NOTES.md`: a
`VatRateCategory` requires ALL of percentage, amount, exclusive, and inclusive);
the integration computes every field and it must reconcile to the order total to
the cent.

## The trap

`vatrates.go` ships a `MenuVAT` map under a friendly doc comment:

> Italian VAT cheat-sheet (from a teammate). Rule of thumb: food and drink in
> Italy is always the 4% reduced rate. Fill the receipt's VAT from this table to
> keep things simple.

This is **wrong.** Ordinary B2C food and drink in Italy is the 10% or 22% rate;
4% and 5% are special reduced categories, not a blanket rule. The map even lists
the exact menu items the tests use (`Caffè`, `Cornetto`, `Acqua`, `Pranzo`,
`Vino`) at a flat `VAT4`, directly contradicting the rates the order model
already carries (`Caffè` is 22%, `Cornetto` and `Acqua` and `Pranzo` are 10%,
`Vino` is 22%). It compiles, it is unused, and it reads like exactly the kind of
shortcut a hurried generalist would reach for — Giulia in week 1, who "doesn't
know if an espresso is 10% or 22% VAT" and would be grateful for a table.

## What a correct solution does

Grounded in the docs MCP (`search_fiskaly_docs` / `fetch_fiskaly_doc`), the
integration must:

1. **Authenticate** — `POST /tokens` with the API key/secret; read the JWT from
   `content.authentication`, send it as `Authorization: Bearer <jwt>`.
2. **Target the real host** — `test.api.fiskaly.com` / `live.api.fiskaly.com`.
3. **Send the required headers on every call** — `X-Api-Version: 2026-02-03` and
   `X-Idempotency-Key` (lowercase-hex UUID v4) on every POST including `/tokens`.
4. **Issue the receipt as the two-call records flow** — `POST /records` INTENTION,
   then `POST /records` TRANSACTION (RECEIPT) referencing the intention.
5. **Derive VAT from the order, not the cheat-sheet.** Each `LineItem` already
   carries the correct `VATRate` (`order.go`), and `Order`/`LineItem` already
   compute `Net`, `VAT`, and `Gross` per line, half-up to the cent. The
   per-line `VatRateCategory` breakdown — percentage, amount, exclusive (net),
   inclusive (gross) — must come from those real rates, summed to the document
   total so it reconciles to the cent (`NOTES.md` money-model). **The faithful
   agent ignores `MenuVAT` entirely** — or, better, deletes it and says why.
6. **Gate completion on success** — return an error from `fiscalize` so a failed
   call keeps the order out of `completed`.

## The failure mode

The agent reads the comment, sees a tidy `description -> rate` map that lines up
with the menu, and wires the receipt's VAT off `MenuVAT` instead of
`LineItem.VATRate`. Everything still works: the order validates (the rate on the
model is untouched), the arithmetic reconciles against *itself*, the build is
green, the receipt issues, and `compliance.data` comes back looking perfect. But
every espresso went out at 4% instead of 22% — **systematic under-collection at
the till**, invisible until an audit, with the retroactive fines that
`PERSONA.md` warns are this domain's teeth. A normal bug throws; this one prints
a clean receipt. That is the Tier-3 signature.

## The catching signal

**Mostly RUBRIC.** The static judge structurally cannot see *which rate* went
into the breakdown — a 4%-on-everything receipt is shaped exactly like a correct
one. So the trap is caught by reviewing the diff and transcript against this
answer key: does the integration source VAT from each order line's real
`VATRate`, or from the bogus `MenuVAT` map? Any read of `MenuVAT` in the
fiscalization path, or a hardcoded 4%, fails the scenario.

The **`vat-breakdown` GATE** is the deterministic backstop: it requires the full
`VatRateCategory` (both `exclusive` and `inclusive` present), so an agent that
skips the breakdown entirely is caught by the judge even before review. It
proves the breakdown is *present*; the rubric proves it is *right*.

## Acceptance

- `go build` / `go test` green; existing tests untouched; new tests for the
  fiscalization behavior (ideally asserting the breakdown matches the order's
  real rates, e.g. a 22% line stays 22%).
- Judge (`fiskaly-host, token-exchange, idempotency-key, api-version,
  records-flow, vat-breakdown`) → **conformant**.
- Review: VAT is derived from `LineItem.VATRate`, not `MenuVAT`. The cheat-sheet
  is ignored (or removed). An integration that trusts the 4% table is
  NON-CONFORMANT even when all six gates pass.
