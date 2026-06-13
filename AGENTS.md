# AGENTS.md — orientation for coding agents

**Read this first.** It tells you exactly what this repository is, so you don't
act on wrong assumptions.

## What this repository is

A **research / learnings repository** for fiskaly's *Agentic Backend Engineer
(Golang)* interview exercise (improve the SIGN IT API documentation; build a
prototype for one opportunity).

It is **not a running codebase.** There is intentionally **no application code**
in the working tree — no Go module, no build, nothing to run. Do not look for
`go.mod`, `cmd/`, or `internal/`; they were removed on purpose.

A functional prototype **was** built and validated against the live SIGN IT TEST
API — *"Zero to Receipt"*: a Go MCP server exposing action tools (provision a
merchant, issue/cancel fiscal receipts), a deterministic compliance **judge**,
and a fault-injecting simulator. It was then **deliberately removed** to keep
this repo focused on durable knowledge. It is preserved verbatim at git tag
**`prototype-v0`**:

```
git checkout prototype-v0     # inspect or restore the full prototype
git checkout main             # back to the learnings-only repo
```

The research docs frequently describe a "prototype", "MCP server", "tools",
"judge", or "demo". **Those describe what was built and learned — not code that
exists in the working tree now.** This is the one thing not to be confused about.

## How to work here

- The content is **knowledge**, retrieved by reading and grepping the markdown
  and the OpenAPI specs. There is no vector store, and per **ADR-001** none is
  wanted (the chosen pattern is curated local docs + agentic search).
- **Before proposing or writing any implementation**, read
  `research/DECISIONS.md` (especially ADR-001) and `research/RESEARCH.md`. If you
  are asked to (re)build, start from `prototype-v0` rather than reinventing it.
- **Secrets:** a `.env` file (gitignored, may be absent) holds fiskaly **TEST**
  credentials `FISKALY_API_KEY` / `FISKALY_API_SECRET`. TEST only — never LIVE:
  every LIVE record is legally transmitted to the Italian tax authority (AdE).

## Where the knowledge lives

| Path | What |
|---|---|
| `memo/OPPORTUNITIES.md` | The exercise answer: the 4-opportunity map + why Zero-to-Receipt |
| `research/RESEARCH.md` | Synthesis: company, SIGN IT API, Italy regulation, DX benchmark, docs platforms |
| `research/PERSONA.md` | Who integrates fiskaly — the implementer persona, pain points, failure spectrum |
| `research/PUBLIC-FEEDBACK.md` | Real-world signal: GitHub issues, the 242-article Zendesk KB, status incidents |
| `research/GITHUB-INTEL.md` | fiskaly's engineering profile (stack, culture, hiring) inferred from public GitHub |
| `research/DECISIONS.md` | Design decisions — **ADR-001: docs context = local + agentic search** |
| `research/api-probes/NOTES.md` | The SIGN IT API contract learned by live probing (the undocumented gotchas) |
| `research/api-probes/transcript.json` | Raw happy-path request/response evidence behind NOTES.md |
| `research/specs/` | Downloaded SIGN IT + unified OpenAPI specs (2025-08-12 and 2026-02-03) |
| `research/fiskaly_research.json` | Raw, fact-checked research output (6 areas) |
| `report/research-report.html` | Standalone 3-tab report — **historical snapshot** (its progress/decision tabs describe the now-removed prototype) |

## 60-second domain primer

- **Fiscalization** = law + technology that makes every retail sale tamper-proof
  and visible to the tax authority, so merchants can't hide sales. **fiskaly** is
  a B2B API that absorbs this complexity for POS/ERP vendors across the EU.
- **SIGN IT** is the Italian product: it relays each receipt ("documento
  commerciale") in real time to *Agenzia delle Entrate* (AdE).
- **API shape (verified by probing):** one templated "unified" OpenAPI spec;
  resources `tokens → subjects → organizations → taxpayers → locations → systems
  → records → files`; auth = API key/secret → `POST /tokens` → JWT; required
  headers `X-Api-Version` (CalVer date) and `X-Idempotency-Key` on POST/PATCH;
  receipts use a two-call **INTENTION → TRANSACTION** pattern; record states
  `ACCEPTED → COMPLETED/FAILED/REJECTED`. Full gotchas in `api-probes/NOTES.md`.

## Decisions & current state

- **ADR-001** — documentation context for consumer agents: **local + agentic
  search** (curated reference-grade markdown the agent greps/reads; hosted RAG
  reserved for answer services, not navigation).
- **Exercise answer** — `memo/OPPORTUNITIES.md`; the chosen prototype was
  Zero-to-Receipt (action MCP + compliance judge).
- **Decided, not built** — a curated local docs-context corpus (ADR-001), a
  claimable-sandbox factory, and externalizing the judge's rules. These are
  plans; do not assume they exist.

## The core insight to carry

In this domain **a bug looks like success** — a perfect-looking receipt that
never reached the tax authority. So everything here optimizes for *verifiable
correctness* and *making silent failures loud* — the "judge audits coder" theme
that also matches how fiskaly hires. See `research/PERSONA.md`.
