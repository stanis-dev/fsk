package mcpserver

// integrationBrief is the docs-layer payload of get_integration_context: a
// compact, agent-oriented map of the SIGN IT integration. It compounds
// fiskaly's machine-readable docs (workspace.fiskaly.com/CLAUDE.md and
// llms.txt) with the operational facts this server learned from live probing.
const integrationBrief = `# fiskaly SIGN IT — agent integration brief (API 2026-02-03)

## What this is
SIGN IT issues legally compliant Italian retail receipts ("documento
commerciale") by transmitting each transaction to Agenzia delle Entrate (AdE).
This MCP server wraps the TEST environment (test.api.fiskaly.com) and absorbs
all protocol mechanics: authentication, X-Api-Version, idempotency keys,
the two-call record pattern and state polling.

## Resource hierarchy
GROUP organization (your HUB account)
  └─ UNIT organization (one merchant/customer)
       └─ subject (API key scoped to the UNIT — only path into a UNIT org)
            └─ taxpayer (legal entity; Italian fiscalization + Fisconline credentials)
                 └─ location (BRANCH; a HEAD_OFFICE is auto-created)
                      └─ system (FISCAL_DEVICE = one POS)
                           └─ records (the receipts)

Lifecycle: taxpayer/location/system are created ACQUIRED/INACTIVE and must be
PATCHed to state=COMMISSIONED (mode flips to OPERATIVE automatically) before
records can be issued. DECOMMISSIONED is final. There is no DELETE anywhere.

## The receipt pattern (two calls, always)
1. POST /records {type: INTENTION, system, operation: {type: TRANSACTION}}
2. POST /records {type: TRANSACTION, record: <intention id>, operation:
   {type: RECEIPT, document: {number, total_vat}, entries: [...], payments: [...]}}
States: ACCEPTED → COMPLETED | FAILED (or REJECTED); modes PROCESSING → FINISHED.
TEST completes synchronously; LIVE transmits to AdE and must be polled.
On COMPLETED, compliance.data carries the AdE progressive document reference.

## Amounts (the API derives nothing)
All amounts are decimal strings. Each entry needs the full VAT breakdown
(percentage, amount, exclusive, inclusive) and document.total_vat must equal
the sums. Italian VAT codes: STANDARD 22%, REDUCED_1 10%, REDUCED_2 5%,
REDUCED_3 4%. The issue_receipt tool derives all of this from gross prices.

## Operational facts (learned from live probing, beyond the docs)
- X-Idempotency-Key (UUID) is required on PATCH too, not just POST; reuse
  with a different payload returns 422, concurrent reuse 409.
- Subject names must match ^[a-z0-9-]{3,30}$ (only documented in the error).
- Company taxpayer name requires BOTH legal and trade.
- Error bodies embed the violated JSON schema — read them, they are precise.
- Merchant Fisconline credentials expire every 90 days on LIVE (renew at 60);
  the first AdE portal login is manual; SPID is not supported.
- AdE outage on LIVE: issue a paper document, then an electronic invoice
  within 12 days (fiskaly "Connection Loss" guide).

## Tools on this server
provision_sandbox → full merchant stack, returns an opaque sandbox_id
issue_receipt     → gross-priced items in, AdE-referenced receipt out
get_record        → record state + compliance data
cancel_receipt    → CANCELLATION transaction for a completed receipt
audit_session     → the judge: replays your trail against compliance rules
ask_fiskaly_docs  → cited answers from fiskaly's own Ask-AI (advisory; verify)

Docs: https://developer.fiskaly.com/sign-it/2026-02-03/integration_guide
Machine-readable: https://workspace.fiskaly.com/llms.txt and /CLAUDE.md`
