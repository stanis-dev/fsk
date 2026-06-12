# Demo script (~8 minutes)

## Setup (before the call)

- `.env` with TEST credentials; `go run ./cmd/z2r-smoke` once to warm caches
  and confirm the TEST API is up.
- Claude Code open in the repo (MCP server auto-wired via `.mcp.json`).
- Two spare terminals for the failure scene.
- Fallback if TEST is down: add `--base-url http://127.0.0.1:8484` to the
  server args in `.mcp.json` and run `go run ./cmd/z2r-sim` — the demo is
  identical.

## Scene 1 — the claim (30s)

> "fiskaly's docs are already excellent for AI agents to *read* —
> llms.txt, CLAUDE.md, a RAG chat. My claim: the next step for the mission
> is letting agents *act*. I built the missing action layer for SIGN IT,
> plus the thing none of Stripe/Square/PayPal's MCP servers have:
> a compliance judge."

## Scene 2 — zero to receipt (3 min)

In Claude Code:

> Onboard "Trattoria da Mario" in Milan and issue a receipt for spaghetti
> alle vongole (14.50), tiramisù (6.00) and a caffè (1.20). Then audit the
> session.

Narrate while it runs:
- provision_sandbox: UNIT org → scoped subject → taxpayer with Italian
  fiscalization → location → fiscal system, commissioned — the 11-step
  guide, one tool call, ~5 seconds, real TEST API.
- issue_receipt: agent passed gross prices; the tool derived the full VAT
  breakdown (22% + 10% lines) — the API derives nothing, the middleware
  does. INTENTION → TRANSACTION, the documented two-call pattern.
- Point at the **AdE reference** (compliance.data) — that's the documento
  commerciale number from Agenzia delle Entrate's flow.
- audit_session: verdict PASS, six rules, citations.

## Scene 3 — the judge convicts (3 min)

```bash
go run ./cmd/z2r-sim --scenario ade-outage    # terminal 1
go run ./cmd/z2r-badpos                       # terminal 2
```

- "This is a sloppy POS integration during an AdE outage — reused
  idempotency keys, no commissioning, a phantom intention, fire-and-forget
  records."
- Walk one finding end-to-end: the FAILED record → the 12-day e-invoice
  fallback → €100/transmission capped €1,000/quarter, D.Lgs. 87/2024.
- "Exit code 1. This is a CI gate. Productized, it's 'fiskaly CERTIFY' —
  and architecturally it's your job ad: a judge auditing a coder."

## Scene 4 — how it's built, what's next (90s)

- Go; types mirror the unified spec; spec-driven regeneration per CalVer
  release means SIGN FR/PT/BE come nearly free.
- Tool granularity: workflow tools, not tool-per-endpoint (token economics,
  safety). Credentials never enter the model context.
- TEST-only by construction; the LIVE path needs OAuth, restricted tool
  visibility, human confirmation — happy to go deep on that design.
- The other opportunities from the memo: docs CI that makes the 168 blank
  descriptions impossible, CalVer migration codemods, claimable sandboxes.

## Likely deep-dive questions

- *Why workflow tools instead of generated tool-per-endpoint?* Token cost
  per schema, error-handling ownership, safety surface. Twilio chose
  search+retrieve (2 tools) for the same reason at the docs layer.
- *How do judge rules scale?* Deterministic core per country pack, authored
  from the provvedimento; LLM judgment layered on top, citation-gated so it
  can't hallucinate law. Human review queue for rule changes.
- *Prompt injection / blast radius?* Read-only docs tool + TEST-only action
  tools + opaque sandbox handles; LIVE would add per-tool OAuth scopes and
  confirmation. The judge itself is an output filter on the session.
- *Why not contribute to workspace.fiskaly.com's docs-mcp instead?* That MCP
  is docs-only by design and unpublished; this compounds it — their
  llms.txt/CLAUDE.md become get_integration_context's source of truth.
