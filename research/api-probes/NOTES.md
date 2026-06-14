# SIGN IT TEST API — probe learnings (2026-06-12)

Ground truth from a complete happy-path run (14 calls, see `transcript.json`). All against `test.api.fiskaly.com`,
`X-Api-Version: 2026-02-03`.

## Flow that actually works

1. `POST /tokens` with HUB API key/secret → JWT (24h), bound to **GROUP** org.
2. `POST /organizations` `{type: UNIT, name}` → UNIT org (works with GROUP token).
3. **Scope-header shortcut does NOT work for taxpayers**: `POST /taxpayers` with `X-Scope-Identifier: <unit-id>` on a
   GROUP token still executes against the GROUP org → `405 E_METHOD_NOT_ALLOWED` "cannot create new legal taxpayer for
   non-unit organization". (Semantically an authz/scoping error surfaced as 405 — error-catalog material.)
4. Documented flow required: `POST /subjects` `{type: API_KEY, name}` **with** `X-Scope-Identifier: <unit-id>` → subject
   **scoped to the UNIT**; key+secret returned ONCE under `content.credentials.{key,secret}`.
5. `POST /tokens` with the scoped subject credentials → UNIT-scoped JWT. All subsequent calls need NO scope header.
6. `POST /taxpayers` COMPANY: `name` requires BOTH `legal` and `trade`. Italian fiscalization
   `{type: IT, tax_id_number(11), vat_id_number(11), credentials: {type: FISCONLINE, pin, password, tax_id_number(16)}}`
   — dummy values accepted in TEST. Created `state=ACQUIRED mode=INACTIVE`.
7. `POST /locations` BRANCH (taxpayer.id, name ≤32 chars, address) → ACQUIRED.
8. `POST /systems` FISCAL_DEVICE requires `location`, `producer {type: MPN, number, details.name}`,
   `software {name, version}` → ACQUIRED/INACTIVE.
9. Commission each via `PATCH .../{id}` `{content: {state: "COMMISSIONED"}}` in order taxpayer → location → system;
   **mode flips to OPERATIVE automatically** on commissioning.
10. `POST /records` `{type: INTENTION, system.id, operation: {type: TRANSACTION}}` → `state=ACCEPTED mode=PROCESSING`.
11. `POST /records`
    `{type: TRANSACTION, record.id=<intention>, operation: {type: RECEIPT, document: {number, total_vat{amount,exclusive,inclusive}}, entries: [SALE/ITEM with full VAT breakdown], payments: [CASH ≥1]}}`
    → in TEST returns `state=COMPLETED mode=FINISHED` **synchronously**. `compliance.data = "DCW0000/0000-0000"` (AdE
    documento commerciale web progressive number, zeroed in TEST), `compliance.url` = AdE print endpoint
    (ivaservizi.agenziaentrate.gov.it). LIVE will be async → still poll.

## API behaviors worth documenting (gaps in official docs)

- Error bodies embed the violated JSON schema **with unresolved mustache partials**
  (`{{>oas_components_schemas_company_name_description}}`) — the backend's spec has the same placeholder-resolution
  defect as the docs site.
- `X-Idempotency-Key` required on PATCH too, not just POST.
- All resource IDs are UUIDv7 (time-ordered).
- VatRateCategory requires ALL of percentage/amount/exclusive/inclusive — the API does not derive any of them.
- Amounts are decimal STRINGS (pattern `^(-)?\d{1,12}(\.\d{1,8})?$`).
- Subject credentials are non-recoverable after creation (shown once).

## IDs from last run (TEST, reusable)

See `state.json`. Each probe run creates a fresh UNIT org stack — cleanup is via DECOMMISSIONED/DISABLED states only (no
DELETE in the API).

## Addendum (2026-06-14 re-probe)

Confirmed live against `test.api.fiskaly.com`, `X-Api-Version: 2026-02-03`:

- `X-Idempotency-Key` is required on **every** POST, including `POST /tokens` (omitting it returns `400 E_BAD_REQUEST`
  "Header parameter X-Idempotency-Key is required, but not found"). Not only POST/PATCH on resources.
- The key must be a **lowercase-hex** UUID v3/v4. `uuidgen` output (uppercase) is rejected with a regex mismatch against
  `^[0-9a-f]{8}-?...$`; the validator tries both the v4 and v3 patterns.
- Request/header validation runs **before** auth: the idempotency-key `400` fires even with an empty/invalid bearer.
- Token response top-level `content` keys are `authentication, id, organization, subject`; the JWT lives under
  `content.authentication`, not `content.access_token`.
- Mustache leak confirmed firsthand: a request-validation `400` embeds the violated schema, whose `description` is the
  literal `{{>oas_components_schemas_universally_unique_identifier_v4_description}}`. Same template-resolution defect as
  the 168 blank docs descriptions, surfacing in live API error bodies. The terse `405` authz errors do NOT embed schema.
- Earlier "four undocumented contracts" framing corrected: the schema-level constraints (subject-name regex, `legal`+`trade`
  required, the record-type taxonomy, PATCH idempotency) ARE in the spec. The genuinely unexpressible contracts are the
  scoped-subject sequencing and commissioning order.
