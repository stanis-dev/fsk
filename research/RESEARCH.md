# fiskaly Research Synthesis

Date: 2026-06-12. Full raw research (6 areas, fact-checked) in `fiskaly_research.json`; downloaded OpenAPI artifacts in
`specs/`.

## Company (verified mid-2026)

- fiskaly GmbH, Vienna, founded 2019 (Ferner/Tragatschnig/Gaubatz). ~110–120 employees, 1,600+ B2B customers, 1M+ active
  POS, 10B+ signatures. Verdane minority stake (2024). Acquired DF Deutsche Fiskal (Mar 2025) and InfraSec Sweden (Dec
  2025).
- Products: SIGN DE (BSI-certified to 2033)/AT/ES/IT/FR/PT, SUBMIT DE/IT, DSFINVK, E-INVOICE (BE live — Jan 2026 Peppol
  mandate), SAFE, RECEIPT, HUB. Sweden 6th market; Poland KSeF on radar.
- **"AI-First initiative" since 2025** — this job posting is part of it.
- Partners/customers: orderbird, Lightspeed, SumUp, ready2order, Oracle, Mastercard. Competitors: efsta, fiskaltrust,
  RetailForce (some resell fiskaly).

## SIGN IT API (first-hand from spec)

- One templated "Unified API" OpenAPI 3.0.3 spec (`live.unified.fiskaly.com/<hash>/en/oas.yaml`) shared by SIGN IT/FR +
  E-INVOICE BE/DE; per-country text overlays at `developer.fiskaly.com/static/unified/<version>/<CC>_en.yaml`, merged
  client-side.
- 27 operations, 8 resources: tokens → subjects → organizations → taxpayers → locations → systems → records → files. No
  DELETEs (lifecycle via PATCH).
- Auth: API key+secret → POST /tokens → JWT bearer. CalVer via required `X-Api-Version` header. `X-Idempotency-Key`
  (UUID) required on POST/PATCH, 24h replay cache, `X-Idempotency-Replayed` header.
- Receipt flow: `Record::INTENTION` → `Record::TRANSACTION` (RECEIPT/CORRECTION/CANCELLATION); states
  ACCEPTED→COMPLETED/FAILED/REJECTED, transitions internal to fiskaly. AdE progressive number in `compliance.data`; raw
  AdE payloads base64 in `transmission.*`.
- SIGN IT _lite_ = relay to AdE "documento commerciale online" portal using merchant Fisconline credentials (90-day
  expiry, manual first login, no SPID). _Full_ = upcoming certified software fiscalization (art. 24 D.Lgs. 1/2024).
- Hosts: test.api.fiskaly.com / live.api.fiskaly.com. Testing in LIVE forbidden (real AdE transmission).

### Documented well

CalVer/idempotency, record + taxpayer/location/system state machines, AdE outage page (paper receipt + e-invoice within
12 days), weekly changelog, Postman collection, step-by-step guide.

### Verified defects/gaps

- **168 unresolved template placeholders** in the rendered SIGN IT 2026-02-03 reference → silently blank descriptions
  (IT_en.yaml doesn't define keys the template uses).
- Zero operation-level request/response examples; zero x-codeSamples; no code samples in guides.
- Spec intro links point to **E-INVOICE** pages (copy-paste drift); typos ("401.xf", "stabability"); leaked
  `/api/sign-it/local` dev page.
- No rate-limit numbers, no Retry-After; webhooks mentioned once in changelog, never documented.
- Breaking rename between versions: `/assets`→`/organizations`, `/entities`→`/taxpayers`+`/locations`
  (at 2026-02-03); 3 versions in ~15 months (2024-10-31, 2025-08-12, 2026-02-03); rename documented in the spec
  changelog, no migration guide.

## Docs platforms — what fiskaly ALREADY has (don't propose these)

Old site (developer.fiskaly.com): Docusaurus + Redocusaurus, Algolia, 5 locales, read-only reference, Postman-centric
quickstart, no llms.txt.

**New preview (workspace.fiskaly.com, Astro/Starlight)** — already shipped:

- Full GEO/AI stack: `/llms.txt`, `/llms-full.txt` (~9MB), `/CLAUDE.md` (agent integration guide incl. anti-patterns),
  `/products.json`, `/regulatory.json`, `/human-interventions.json`, `/.well-known/ai-plugin.json`, `/specs/` raw
  downloads.
- Working RAG "Ask AI" chat (Hono.js + Vertex AI, Gemini 2.5 Pro/2.0 Flash, citations, groundedness, personas, admin
  dashboard).
- Persona IA (Developer / PM Hub with effort estimator / Operator Center), country-first nav, version dropdowns,
  per-operation deep links, spec download buttons, page feedback, 5 locales.

**Built but NOT shipped (the open gap):**

- `@fiskaly/docs-mcp` (9 docs-only tools) is documented but **404 on npm**. The only official fiskaly MCP published is
  `@fiskaly/ui-mcp` for the design system; the only docs/API MCPs on npm are community packages
  (`fiskaly-docs-mcp-community`, `fiskaly-chat-mcp`).
- `ApiTryIt` try-it component ships in the JS bundle but **never renders** (mount condition never true); `/api/console/`
  is a **404**.
- No SDKs at all: all GitHub SDK repos archived (READMEs still point at SIGN DE v1), Go SDK gone; official stance "use
  an HTTP client". No generated code snippets.
- No webhook docs, no changelog product surface, status/naming inconsistencies across pages.

## Italy context (why this matters now)

- Provv. AdE n. 111204 (7 Mar 2025): certified **software solutions** for corrispettivi (PEM/PEL, MF1/MF2 modules);
  specs iterated v1.0→v1.3 (annexes Apr 2026); software transmission begins **from 2027** as a 4th option supplementing
  hardware (not a replacement; adoption optional); fiskaly publicly in certification pipeline.
- Budget Law 2025: **POS-RT linkage mandatory from 1 Jan 2026** (provv. 424470, portal live 5 Mar 2026; €1,000–4,000
  fines + license suspension) → SUBMIT IT.
- ~1.7M hardware RTs, **~80% of the installed base** reaching end-of-life by 2027 (fiskaly/Format Research) → the
  commercial window for SIGN IT.
- Sanctions: 70% of VAT (min €300) for omitted memorization/transmission; €100/transmission late (capped
  €1,000/quarter). 12-day transmission window; lottery codes; emergency procedures.

## DX benchmark (mid-2026)

- Action-taking remote MCP servers are the frontier: Stripe (mcp.stripe.com, ~25 tools incl. writes), Square (sandbox
  mode), PayPal (restricted tool visibility), Plaid (dashboard diagnostics). Twilio's is deliberately docs-only
  (search+retrieve).
- Agent Skills (SKILL.md, agentskills.io) cross-vendor standard since Dec 2025 (~40 clients). Stripe claimable sandboxes
  (`stripe sandbox create`, no account). llms.txt near-universal among API companies.
- Anthropic acquired Stainless (May 2026), winding down hosted SDK gen → Speakeasy/Fern are the managed OpenAPI→SDK+MCP
  pipelines.
- Table stakes: docs AI chat (kapa/Inkeep), error dictionaries with remediation (Twilio 500+), versioned upgrade guides.

## The strategic read

fiskaly built the AI **read** layer (RAG chat + GEO files) but the **action** layer is conspicuously missing: no official
API/action MCP (only `@fiskaly/ui-mcp` for the design system is published; docs MCPs are community-built), dormant
try-it, dead console link, archived SDKs. Their own job ad says they want
judge-agents-auditing-coder-agents. The docs pipeline itself (template + overlays) has no QA gate — hence 168 blank
descriptions in production.
