---
name: zero-to-receipt
description: Issue compliant Italian fiscal receipts via fiskaly SIGN IT using the zero-to-receipt MCP tools, then act as the compliance judge over the session. Use when asked to integrate Italian fiscalization, provision a merchant, issue/cancel receipts, run the zero-to-receipt demo, or audit a SIGN IT integration.
---

# Zero to Receipt — fiskaly SIGN IT via MCP

You drive a real fiscalization flow against fiskaly's TEST environment using
the `zero-to-receipt` MCP server, then judge the session's compliance.

## Workflow (always in this order)

1. **Context first**: call `get_integration_context` once before anything
   else. It contains operational facts that are not in the official docs.
2. **Provision**: `provision_sandbox` with the merchant's name. Keep the
   returned `sandbox_id`; it is your only handle (credentials never leave
   the server).
3. **Issue receipts**: `issue_receipt` with gross (VAT-inclusive) prices.
   Italian VAT codes: STANDARD 22% (default), REDUCED_1 10% (e.g. most food
   service), REDUCED_2 5%, REDUCED_3 4% (e.g. bread, milk, books). Do not
   compute net/VAT amounts yourself — the tool derives them.
4. **Verify**: a receipt is only done when `state=COMPLETED` and an
   `ade_reference` is present. If state is anything else, use `get_record`
   to poll, and report honestly.
5. **Judge before claiming success**: call `audit_session` and read the
   report. Never tell the user the integration is compliant unless the
   verdict is PASS. Present any findings with their citation and fix.

## Acting as the judge

After `audit_session`, add your own reasoning pass over the report —
the deterministic rules are ground truth for facts; you add judgment:

- If a record is `FAILED`: the merchant's legal fallback is a paper
  document plus an electronic invoice within 12 days (fiskaly "Connection
  Loss" guide; D.Lgs. 127/2015). Spell this out with the deadline date.
- Weigh severity in euros where you can: omitted transmission risks 70% of
  the VAT (min €300); late transmission €100/transmission capped at
  €1,000/quarter (D.Lgs. 87/2024). State which applies and why.
- Be adversarial with yourself: if anything in the session was retried,
  reordered or left unconfirmed, say so before the user finds out.

## Hard rules

- TEST only. Never attempt LIVE hosts; the server refuses them by design,
  and every LIVE record legally reaches Agenzia delle Entrate.
- Never invent an `ade_reference`. Only report what the tools returned.
- Cancellations (`cancel_receipt`) are themselves fiscal transactions —
  audit after cancelling too.

## Demo choreography (when asked to "run the demo")

1. Provision "Trattoria Da Mario", issue a 3-line lunch receipt
   (e.g. spaghetti alle vongole 14.50 STANDARD, tiramisù 6.00 STANDARD,
   caffè 1.20 REDUCED_1), show the AdE reference.
2. Cancel the receipt to show the annullamento flow.
3. Run `audit_session` and present the judge's report.
4. For the failure scene, the human runs `go run ./cmd/z2r-sim --scenario
   ade-outage` and `go run ./cmd/z2r-badpos` in a terminal — a sloppy POS
   the judge convicts with citations.
