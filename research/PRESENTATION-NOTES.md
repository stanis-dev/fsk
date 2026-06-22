# Presentation speaking notes

## Goal & process

Goal: find opportunities to improve the SIGN IT API docs, maximizing value for the customer.

Process:

- Focused on SIGN IT only, to go deep in one area rather than wide.
- Unfamiliar with the domain, so I researched the problem space first: the average API user (who they are, where fiskaly
  sits in their workflow, their pain points), a gap review of the SIGN IT docs, and public feedback.

Assumption I'm working under: fiskaly has no telemetry on how agents or developers actually consume the docs (or an MCP).
You can't improve what you can't see, which is half of why the prototype is shaped the way it is.

## Thesis (two layers, say both)

**Problem.** The docs are the contract an AI agent integrates against. The funnel that wins Italy 2027 is the one that
gets a POS vendor from docs to provable compliance fastest. Every opportunity below turns a contract nobody checks into
one that's enforced, for the agent that now reads the docs.

**Prototype (the point).** The obvious answer is an MCP server. I did not build that. I built the **eval harness that
makes building and iterating one safe.** Shipping an MCP is the easy part. Knowing a change won't silently break existing
agent workflows is the hard part. That hard part is what I built.

## The recommendation (what I am NOT claiming to have built)

An easy conclusion is that an MCP server could greatly improve the developer experience for fiskaly API users:

- Merchant sandbox (provision against TEST).
- Action tools, flow-focused, not a 1:1 API map, returning structured feedback on the integration flow.
- Integration judge (subagent, skill, or both) that reviews flows and surfaces the silent failures.
- Telemetry per call, to measure the server and, more importantly, to see the real DX flows.

This is the easy conclusion, and it doubles as internal leverage: the same server powers agent-driven development inside
fiskaly. Stripe, Square, and PayPal already ship action MCPs. The only differentiator left is the compliance judge.
Proposing all this is cheap. Building it without regressing what already works is not, which is the gap the prototype
fills.

## What I built (the prototype): an eval harness for an MCP + a coding agent

I chose to focus on an **eval harness for an MCP server + coding agents (Claude Code only).** A minimal implementation
that gives feedback on any desired change right away: plug in a code example and a set of expected outcomes, and see it
in action immediately.

Why this and not the MCP:

- Shipping an MCP and iterating on it is the easy part. What is hard is knowing a change will not break existing
  workflows.
- A docs or tool tweak that helps one flow can silently break another. Nothing catches that today.
- The harness turns "did this change help or hurt?" into a measured, repeatable answer, before it ships.

What's in it (built and verified):

- 10 isolated scenarios across the failure spectrum, each a task prompt + fixture + expected outcomes.
- The docs MCP under test: `search_fiskaly_docs` + `fetch_fiskaly_doc`, with per-call telemetry. This is the example
  workload, not the point.
- Runner: Docker-isolated. Copies the fixture, commits a baseline, runs the agent, captures diff / build / test / judge
  artifacts, streams phase events over SSE, and cancels live runs.
- Judge: deterministic trajectory checks (grounded-before-write, tools-called, docs-fetched, max-MCP-errors) plus an
  optional citation-checked LLM expectation layer; uncited verdicts are downgraded.
- Next.js dashboard: run status, diff, transcript, judge JSON, telemetry summary; trigger and cancel runs live.

The demo line: change a doc, run the scenario, watch the agent ground itself or fail, see build / test / judge flip. A
docs change becomes measurable in one loop.

Why a local corpus and not the prod RAG (ADR-001): the eval path must be deterministic and inspectable. Rejected vector
RAG (infra for worse navigation), remote RAG (non-deterministic, outage risk), and the raw ~9MB dump (blows the context
window). This is a measurement substrate, not a customer feature.

---

## The research behind it

API user profile:

- Integrators know no tax law, by design.

API docs:

- 168 template keys render as blank descriptions.
- Live API error bodies leak unresolved mustache partials.
- Scope-header sequencing undocumented; learned only from 405s.
- No code samples in any integration guide; zero request/response examples across operations.
- Webhooks mentioned once, never documented.
- Resource hierarchy undocumented; a location needs a taxpayer, a system needs a location.
- Core resources renamed, noted only in the spec changelog, not the guide.
- AI adaptation: the read layer shipped (llms.txt, ~9MB llms-full.txt, a CLAUDE.md agent guide, a working RAG chat); the
  action and proof layer is missing or undocumented; the official npm MCP covers the design system, not the API; the docs
  pipeline has no QA gate.

Conclusion: the gap is the action and verification layers, and the
[AI and Agents](https://workspace.fiskaly.com/ai-agents/overview/) guidance should be available as skills.

Integration pain points:

- Idempotency key required on every POST, including tokens.
- The API derives no VAT; you compute everything.
- The outage compliance rule lives outside the API reference.
- Fisconline credentials silently expire every ninety days.
- 429 defined, but no rate-limit numbers or Retry-After.
- Fiscalization bugs look like success, not errors.

Conclusion: integration is prone to "you have to know" bugs.

Domain opportunity:

- Software fiscalization targets a 2027 start (a 4th option, supplementing hardware; provv. AdE 111204, 7 Mar 2025;
  adoption optional).
- ~1.7M registers, ~80% of the installed base, reach end-of-life by 2027 (fiskaly/Format Research).

Conclusion: the funnel that wins that window is the one that gets a vendor from docs to provable compliance fastest.
Time-to-first-receipt as the metric.

---

## The opportunities (what the MCP would deliver, funnel order)

These are the recommendation, not the prototype. None of them is the thing I built. The harness is what lets you build
and change any of them without silent regressions.

### #1 Zero to Receipt (the flagship recommendation)

- **Claim:** make the metric time-to-first-signed-receipt. An agentic docs layer, then an action layer, so a vendor's own
  AI assistant integrates SIGN IT against TEST without a human reading the reference cover to cover.
- **Lever:** that metric is the 2027 funnel. Shorter docs-to-first-receipt is shorter docs-to-provable-compliance.
- **Evidence:** the read layer exists; no official API/action MCP, no try-it console (`/api/console` 404), archived SDKs.
- **Wedge:** Stripe, Square, PayPal all ship action MCPs. None ships a compliance judge. That's fiskaly's, and it's #4.

### #2 Docs CI, the 168 blank descriptions (what I'd ship first)

- **Claim:** a conformance gate on the docs supply chain. Resolve the templated spec against every country overlay, fail
  the build on any unresolved key, diff documented behavior against live TEST.
- **Lever:** the docs are the contract; if fields render blank in production, the front of the funnel is broken before
  anyone reaches compliance. The fix is deterministic, no LLM.
- **Note:** "isn't this just a lint?" Yes, deliberately. It's embarrassingly cheap and currently absent. The same gate
  makes "launch the Sweden / Poland overlay" a checklist that writes itself.

### #3 CalVer migration tooling

- **Claim:** generate migration guides and codemods from the spec diff, per version bump, per country.
- **Lever:** "wire it once and forget it" is the integrator's own priority; re-integration tax is friction between a
  vendor and multi-market compliance.
- **Note:** "why not just write release notes?" Manual notes don't scale to six markets times N versions under
  regulatory deadlines, and they're prose an agent can't apply. Spec-diff output is both human- and agent-consumable.

### #4 The compliance judge (crescendo, and the bridge back to docs)

- **Claim:** a rule engine that replays what an integration did against machine-readable compliance rules, with
  regulation citations and euro stakes. Productized: "fiskaly CERTIFY."
- **Bridge:** this is where docs stop being prose and become executable. The judge runs the same contracts the docs
  describe. A blank field description (#2) and an unenforced compliance rule are the same failure (a contract nobody
  checks) at two layers. The judge is the docs made executable.
- **Lever:** the literal job-ad mandate and the mission's end-state, docs to provable compliance, and the one thing the
  payment-MCP frontier lacks.
- **Honest limit (say it, don't oversell):** the deterministic judge is in the harness, gating each run with
  scenario-selected rules, but it checks source shape, not live SIGN IT behavior; the VAT check proves the fields are
  constructed, not that the rate is correct. That's the roadmap, not a finished claim.

---

## Close (30s)

- **Through-line:** shipping an MCP is the easy part; keeping it from silently breaking is the hard part, and that is what
  I built. Every opportunity above turns a contract nobody checks into one that's enforced, for the agent that now reads
  the docs.
- **What I'd ship first if hired: Docs CI (#2).** Deterministic, embarrassingly cheap, shippable in days. Then the
  harness gates every change after it, so the ambitious work (action MCP, CERTIFY) ships without regressing what already
  works.

---

**Regulatory / Italy**

- Software fiscalization targets a **2027** start (separate roadmap; software a 4th option supplementing hardware). Provv.
  AdE **111204** (7 Mar 2025) defines the specs; adoption is optional, not operational.
- ~**1.7M** registers, **~80% of the installed base**, reach end-of-life by 2027 (fiskaly/Format Research).
- POS-RT linkage mandatory **1 Jan 2026** (provv. 424470).
- Fines: omitted memorization/transmission **70% of VAT, min €300**; late transmission **€100 each, capped
  €1,000/quarter**; missing POS-RT linkage **€1,000–4,000 + license suspension**.
- Outage fallback: **paper document + e-invoice within 12 days**.

**Docs defects (verified)**

- **168** unresolved `((<...))` template keys (all description fields) in the 2026-02-03 IT reference.
- **Zero** operation-level request/response examples and **zero** x-codeSamples (67 field-level `example:` values exist).
- **3** versions in ~15 months (2024-10-31, 2025-08-12, 2026-02-03); rename `/assets`→`/organizations`,
  `/entities`→`/taxpayers`+`/locations`.
- Server-side template leak: a `400` from `POST /tokens` with an uppercase idempotency key returns a body whose schema
  `description` is the literal `{{>oas_..._description}}`. The spec template uses `((<...))`; the live API serializes the
  unresolved partial as mustache `{{>...}}`.

**Undocumented contracts (the genuinely unexpressible ones, learned from errors)**

- Scoped-subject sequencing: need a UNIT-scoped subject before `POST /taxpayers`, else `405 E_METHOD_NOT_ALLOWED`.
- Resource creation is hierarchical: a location references its taxpayer, a system references its location. Each resource
  is commissioned independently with `PATCH state=COMMISSIONED`, which flips its mode to OPERATIVE; no commissioning order
  is documented.
- `X-Idempotency-Key` required on **every** POST including `POST /tokens`; the value must be a **lowercase-hex UUID
  v3/v4**. On `POST /tokens` the header is required but its idempotency-caching effect is a documented no-op.
- VatRateCategory has **six** required fields (type, code, percentage, amount, exclusive, inclusive); the API **derives
  nothing**; amounts are decimal strings.

**What I built (for the "show, don't tell" moment)**

- An eval harness for an MCP + a coding agent (Claude Code): 10 scenarios across the failure spectrum, Docker runner,
  deterministic + citation-checked LLM judge, Next.js dashboard, per-call telemetry.
- The docs MCP (`search_fiskaly_docs`, `fetch_fiskaly_doc`) is the workload under test, the example you plug in, not the
  deliverable. The deliverable is the loop that makes any docs or MCP change measurable.
