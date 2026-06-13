# fiskaly Interview Exercise — Research & Learnings

This repository is the **research foundation** for fiskaly's *Agentic Backend
Engineer (Golang)* exercise: identify opportunities to improve the SIGN IT API
documentation, and build a functional prototype for one of them.

A working prototype **was** built — **"Zero to Receipt"**: a Go MCP server
exposing action tools over the live SIGN IT TEST API (provision a merchant,
issue and cancel fiscal receipts), a deterministic compliance **judge**, and a
fault-injecting simulator. It was validated end-to-end against
`test.api.fiskaly.com`, then **deliberately removed** to keep this repository
focused on durable learnings rather than code.

> 🔖 The full prototype is preserved at git tag **`prototype-v0`**.
> Recover it with `git checkout prototype-v0`.

**🤖 If you're an AI coding agent, read [`AGENTS.md`](AGENTS.md) first** — it
explains the repo's nature so you don't act on stale assumptions.

## What's here (learnings only)

| Path | What |
|---|---|
| [`memo/OPPORTUNITIES.md`](memo/OPPORTUNITIES.md) | The exercise answer: the opportunity map + why Zero-to-Receipt |
| [`research/RESEARCH.md`](research/RESEARCH.md) | Synthesis: company, SIGN IT API, Italy regs, DX benchmark, docs platforms |
| [`research/PERSONA.md`](research/PERSONA.md) | Who integrates fiskaly — the implementer persona + failure spectrum |
| [`research/PUBLIC-FEEDBACK.md`](research/PUBLIC-FEEDBACK.md) | Real-world signal (GitHub issues, the Zendesk KB, status incidents) |
| [`research/GITHUB-INTEL.md`](research/GITHUB-INTEL.md) | fiskaly's engineering profile inferred from its public GitHub |
| [`research/DECISIONS.md`](research/DECISIONS.md) | Design decisions (ADR-001: docs context = local + agentic search) |
| [`research/api-probes/NOTES.md`](research/api-probes/NOTES.md) | The SIGN IT API contract learned by live probing — the gotchas |
| `research/api-probes/transcript.json` | Raw happy-path request/response evidence |
| `research/specs/` | Downloaded SIGN IT / unified OpenAPI specs (two versions) |
| `research/fiskaly_research.json` | Raw fact-checked research output (6 areas) |
| `report/research-report.html` | Standalone 3-tab report (historical snapshot — see its banner) |

## The headline learnings

- **fiskaly built the AI *read* layer (RAG chat, llms.txt, machine-readable
  manifests) but not the *action* layer** — the docs-MCP is unpublished, the
  try-it console is a dead link, the SDKs are archived, the spec has zero
  examples. That gap is the opportunity.
- **The docs pipeline has no QA gate** — the live SIGN IT reference renders
  **168 silently blank descriptions** from unresolved template placeholders.
- **In this domain a bug looks like success** — a perfect-looking receipt that
  never reached the tax authority — so verifiable correctness and a "judge that
  audits the integration" are the winning themes (and match how fiskaly hires).
- **Italy's 2027 software-fiscalization window** (≈80% of hardware registers
  end-of-life, sanctions priced in law) makes fast, *provable* integration the
  thing that wins the market.

## Recovering / inspecting the prototype

```bash
git checkout prototype-v0   # the full Go prototype: MCP server, judge, simulator
git checkout main           # return to this learnings-only repo
```

The prototype's design rationale survives here in `memo/OPPORTUNITIES.md` and
`research/DECISIONS.md`; the API contract it relied on is in
`research/api-probes/NOTES.md`.
