# Zero to Receipt

**An AI agent lands on fiskaly's docs and has a signed Italian fiscal receipt
in under five minutes — with a judge proving it was done compliantly.**

This is a functional prototype built for fiskaly's interview exercise:
*"What improvements to the API documentation would bring value to customers?
Go crazy — fixing typos will not empower the mission."*

The thesis: in 2026 the consumer of API documentation is increasingly an AI
coding agent. fiskaly already ships the **read layer** for that world
(llms.txt, CLAUDE.md, a RAG docs chat on workspace.fiskaly.com). What's
missing is the **action layer** — and the proof of compliance. This
prototype builds both:

- an **action-taking MCP server** that lets any AI agent run a complete
  SIGN IT integration against fiskaly's real TEST API, and
- a **judge** that audits everything the agent did against machine-readable
  compliance rules, with regulation citations and euro-denominated stakes —
  the same *judge-audits-coder* architecture the Agentic Backend Engineer
  role describes for fiskaly's own SDLC.

The full opportunity analysis is in [`memo/OPPORTUNITIES.md`](memo/OPPORTUNITIES.md);
the demo storyline in [`memo/DEMO.md`](memo/DEMO.md).

## What it does

```
"Onboard 'Trattoria da Mario' in Milan and issue a lunch receipt."
        │
        ▼  Claude Code, via the zero-to-receipt MCP server
provision_sandbox ─▶ UNIT org → scoped subject → taxpayer (IT fiscalization)
                     → location → fiscal system, all COMMISSIONED   (~5s)
issue_receipt     ─▶ INTENTION → TRANSACTION (full VAT breakdown derived
                     from gross prices) → COMPLETED
                     compliance.data = AdE documento commerciale reference
audit_session     ─▶ the judge replays the trail against compliance rules
                     → PASS, or findings with citation + remediation
```

Everything runs against `test.api.fiskaly.com` (API version 2026-02-03).
LIVE hosts are **rejected by construction** — every LIVE record legally
reaches Agenzia delle Entrate.

## Quickstart

```bash
cp .env.example .env          # add your fiskaly TEST API key + secret
go test ./...                 # money math, receipt totals
go run ./cmd/z2r-smoke        # full happy path: stack + receipt in ~3s
```

Then open Claude Code in this repo (the MCP server is wired via `.mcp.json`,
the agent skill via `.claude/skills/`) and say:

> Onboard "Trattoria da Mario" in Milan and issue a receipt for spaghetti
> alle vongole (14.50), tiramisù (6.00) and a caffè (1.20). Then audit the
> session.

### The failure scene (chaos engineering for fiscal compliance)

```bash
go run ./cmd/z2r-sim --scenario ade-outage    # terminal 1: AdE goes down
go run ./cmd/z2r-badpos                       # terminal 2: a sloppy POS
```

`z2r-badpos` is a deliberately bad integration — reused idempotency keys,
no commissioning, a transaction referencing a phantom intention, fire-and-
forget records. The judge convicts it: three violations and one warning,
each with the rule, the citation (down to D.Lgs. 87/2024 sanction amounts)
and the fix. Exit code 1 — it works as a CI gate.

## Layout

| Path | What |
|---|---|
| `internal/fiskaly` | Typed SIGN IT client: token lifecycle, idempotency keys, receipt builder (gross → full VAT breakdown), provisioning flow |
| `internal/mcpserver` | The 7 MCP tools (incl. `ask_fiskaly_docs`, a grounded passthrough to fiskaly's own Ask-AI RAG); credentials stay server-side, agents hold opaque sandbox ids |
| `internal/audit` | Session trail + deterministic compliance rules + report renderer |
| `internal/sim` | Local SIGN IT simulator, faithful to live-probed behaviors, with fault scenarios |
| `cmd/z2r-mcp` | MCP server (stdio) |
| `cmd/z2r-smoke` | End-to-end happy path |
| `cmd/z2r-sim`, `cmd/z2r-badpos` | The chaos demo |
| `research/` | Fact-checked research, OpenAPI specs, live API probe transcripts |
| `memo/` | The opportunity analysis and demo script |

## Design decisions worth discussing

- **Workflow tools, not tool-per-endpoint.** `issue_receipt` is one tool,
  not eight API calls — agents pay tokens for every schema, and protocol
  mechanics (idempotency, version headers, two-call pattern, polling)
  belong in middleware, not in the model's context.
- **Credentials never enter the model.** Agents get `sbx-001`, not API
  secrets. Blast radius is one TEST sandbox.
- **Deterministic facts, model judgment.** The judge's rules are plain Go
  over the trail — reproducible, cheap, CI-able. The LLM layer (the skill's
  judge persona) reasons *on top of* those facts, citation-gated.
- **TEST-only by design**, and what a LIVE path would need: OAuth per the
  MCP spec, human confirmation, restricted tool visibility (PayPal's
  pattern), per-tool scopes.
- **Hand-written thin client over generated unions** for a prototype the
  interviewer reads; production would regenerate via oapi-codegen per
  CalVer release — and the same spec diff that drives codegen drives the
  migration-guide generator (see the memo's opportunity #4).
