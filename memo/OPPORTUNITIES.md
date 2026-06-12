# Driving fiskaly's mission through the API documentation

*Prepared for the fiskaly interview exercise — June 2026*

## Where I started

I didn't start with opinions; I started with probes. Everything below is
grounded in fiskaly's live systems as of June 12, 2026 — the rendered docs,
the underlying OpenAPI pipeline, the new workspace.fiskaly.com preview, the
GitHub org, npm, and a complete integration run against the SIGN IT TEST API
(transcripts in `research/`).

Three findings frame everything:

**1. The docs pipeline has no QA gate.** The SIGN IT reference is built from
one templated OpenAPI spec shared by four products, with per-country text
overlays merged at render time. The IT overlay fails to define **168
template keys**, so the live 2026-02-03 reference silently renders 168 blank
descriptions. The same defect lives server-side: API *error responses* embed
schema descriptions with unresolved `{{>...}}` partials. Nobody noticed,
because nothing checks.

**2. fiskaly already built the AI read layer — and stopped there.**
workspace.fiskaly.com ships llms.txt, an 8.5MB llms-full.txt, a CLAUDE.md
agent guide, machine-readable product/regulatory manifests, and a working
RAG chat. That's ahead of most API companies. But the documented
`@fiskaly/docs-mcp` is not on npm, the try-it component ships in the bundle
but never renders, `/api/console` is a 404, every SDK repo is archived, and
the spec contains zero request examples. An AI agent can *read about*
fiskaly perfectly and still can't *do* anything.

**3. The customer buys certainty, not endpoints.** Italy makes this
concrete: software fiscalization goes fully operational in 2027 (provv.
111204/2025), ~80% of Italy's 1.7M hardware registers reach end-of-life by
then, and non-compliance is priced in law — 70% of VAT (min €300) for
omitted transmission, €100/transmission late, €1,000–4,000 for missing
POS-RT linkage. The integration funnel that wins that window is the one
that gets a POS vendor from docs to *provable compliance* fastest.

## The opportunity map

**#1 — Zero to Receipt: agent-executable onboarding** *(built — this repo)*
Time-to-first-signed-receipt as the metric. An action-taking MCP server
(provision a merchant sandbox, issue receipts with derived VAT, cancel,
audit) plus an agent skill, so a POS vendor's AI assistant integrates
SIGN IT autonomously against TEST. Completes the journey their docs-MCP
plans started, and one-ups the 2026 benchmark (Stripe/Square/PayPal action
MCPs) by adding what none of them have: a compliance judge.

**#2 — The judge: provable compliance as a product** *(built, in miniature)*
A rule engine that replays everything an integration did against
machine-readable compliance rules with regulation citations and
euro-denominated stakes. Today it's six deterministic rules in this repo;
productized, it's "fiskaly CERTIFY" — scenario packs per country (AdE
outage replay, idempotency discipline, correction flows), audit-style
reports, a CI gate POS vendors run on every release. The same architecture
is the job ad's mandate: judge agents auditing coder agents so fiscal code
stays legally compliant — built first for customers, then turned inward.

**#3 — Docs CI: the 168 blank descriptions never happen again**
A conformance pipeline over the docs supply chain: resolve the template
against every country overlay (catches all 168 unresolved keys —
deterministically, no LLM needed), diff documented behavior against live
TEST API behavior (the probe in this repo found four undocumented contracts
in one afternoon: subject-name regex, mandatory trade names, composite
record types, PATCH idempotency), and gate releases on zero placeholders.
The launch checklist for Sweden and Poland overlays writes itself.

**#4 — CalVer migration tooling**
SIGN IT shipped four date versions in 17 months, one of which renamed core
resources (`entities`/`assets` → `taxpayers`/`locations`/`organizations`)
with migration notes living in a Zendesk article. The spec diff is
machine-readable; agent-generated migration guides and codemods per version
bump are the multi-country scale story ("you integrated SIGN ES — here's
your exact delta to SIGN IT").

## Why I built #1 (+#2)

It's the only option that moves the mission metric *and* demonstrates the
role. "Make receipts easy" in 2026 means making them easy for the AI agents
POS vendors already use; the missing action layer is the gap between
fiskaly's excellent AI-readable docs and an agent actually shipping an
integration. And the judge is the interview talking point made executable:
deterministic facts, model judgment on top, human-in-the-loop where the law
requires it (Fisconline first login, credential rotation) — the exact
shape of the agentic SDLC fiskaly wants to build internally.

What five minutes of the demo shows: an agent provisions a merchant, issues
a multi-VAT receipt with a real AdE document reference from fiskaly's TEST
environment, cancels it, and is audited — then a sloppy POS integration
meets the judge during an AdE outage and leaves with three violations, the
relevant decreto legislativo citations, and an exit code CI would respect.

## From prototype to product

- **Claimable sandboxes**: the provisioning flow already creates isolated
  UNIT-org stacks from one master key; add a claim-into-HUB flow and
  fiskaly has Stripe's "start before you sign up" funnel.
- **LIVE path**: OAuth per the MCP spec, restricted tool visibility,
  human confirmation on writes, per-tool scopes.
- **Spec-driven regeneration**: tools and types regenerate per CalVer
  release; the unified spec means SIGN FR/PT/BE come nearly free.
- **Judge rule packs per country**, authored from each provvedimento —
  compounding fiskaly's regulatory.json into something executable.
