# Original Opportunity Map

**1. The docs pipeline has no QA gate.** The SIGN IT reference is built from one templated OpenAPI spec shared by four
products, with per-country text overlays merged at render time. The IT overlay fails to define **168 template keys**, so
the live 2026-02-03 reference silently renders 168 blank descriptions. The same defect lives server-side: API _error
responses_ embed schema descriptions with unresolved `{{>...}}` partials. Nobody noticed, because nothing checks.

**2. AI enablement - read layer already built** - workspace.fiskaly.com ships llms.txt, an 8.5MB llms-full.txt, a
CLAUDE.md agent guide, machine-readable product/regulatory manifests, and a working RAG chat. The documented
`@fiskaly/docs-mcp` is not on npm, the try-it component ships in the bundle but never renders, `/api/console` is a 404,
every SDK repo is archived, and the spec has no authored request examples: only field-level values a renderer stitches
into samples that don't encode the required-together fields or call ordering the compositional flows (e.g. `POST /records`)
need. An AI agent can _read about_ fiskaly perfectly and still can't _do_ anything.

**3. The customer buys certainty, not endpoints.** Italy makes this concrete: software fiscalization goes fully
operational in 2027 (provv. 111204/2025), ~80% of Italy's 1.7M hardware registers reach end-of-life by then, and
non-compliance is priced in law — 70% of VAT (min €300) for omitted transmission, €100/transmission late, €1,000–4,000
for missing POS-RT linkage. The integration funnel that wins that window is the one that gets a POS vendor from docs to
_provable compliance_ fastest.

## The opportunity map

**#1 — Zero to Receipt: agent-executable onboarding** Time-to-first-signed-receipt as the metric. An action-taking MCP
server (provision a merchant sandbox, issue receipts with derived VAT, cancel, audit) plus an agent skill, so a POS
vendor's AI assistant integrates SIGN IT autonomously against TEST. Completes the journey their docs-MCP plans started,
and one-ups the 2026 benchmark (Stripe/Square/PayPal action MCPs) by adding what none of them have: a compliance judge.

**#2 — The judge: provable compliance as a product** A rule engine that replays everything an integration did against
machine-readable compliance rules with regulation citations and euro-denominated stakes. Productized, it's "fiskaly
CERTIFY" — scenario packs per country (AdE outage replay, idempotency discipline, correction flows), audit-style reports,
a CI gate POS vendors run on every release. The same architecture is the job ad's mandate: judge agents auditing coder
agents so fiscal code stays legally compliant — built first for customers, then turned inward.

**#3 — Docs CI: the 168 blank descriptions** A conformance pipeline over the docs supply chain: resolve the template
against every country overlay (catches all 168 unresolved keys — deterministically, no LLM needed), diff documented
behavior against live TEST API behavior (the schema-level contracts the probe hit are in fact documented: subject-name
regex, `legal`+`trade` required, the record-type taxonomy; what the diff catches is the same template defect leaking
server-side, with `400` error bodies embedding unresolved `{{>...}}` partials, confirmed live; see
`research/api-probes/NOTES.md`), and gate releases on zero placeholders. The launch checklist for Sweden and Poland overlays writes itself.

**#4 — CalVer migration tooling** SIGN IT shipped four date versions in 17 months, one of which renamed core resources
(`entities`/`assets` → `taxpayers`/`locations`/`organizations`) with migration notes living in a Zendesk article. The
spec diff is machine-readable; agent-generated migration guides and codemods per version bump are the multi-country
scale story ("you integrated SIGN ES — here's your exact delta to SIGN IT").

The delivered workbench now exercises this direction through local fixtures,
the docs MCP, the runner, the judge, and the dashboard. Current system details
live in the root `README.md`.
