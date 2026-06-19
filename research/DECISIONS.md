# Design Decisions

Running log of load-bearing design decisions for the SIGN IT eval workbench and
its recommendations to fiskaly.
ADR-lite: each entry records the decision, the reasoning, the rejected alternatives, and when to revisit.

## ADR-001 — Documentation context for consumer agents: local + agentic search

**Date:** 2026-06-13 · **Status:** Implemented.

### Context

A consumer's AI agent integrating SIGN IT needs documentation context. The approaches in common use as of mid-2026:
local files + grep, local vector RAG, agentic search, hosted RAG, `llms.txt`/`.md` publishing, and MCP/CLI docs tools.

### Decision

For how the eval workbench supplies documentation context to a consumer agent,
default to **curated local docs + agentic search**. Reserve hosted RAG for hosted
_answer services_ (like fiskaly's Ask-AI), not for agent navigation.

### Reasoning (June 2026 state of the art)

- **For content an agent can hold locally, agentic search has beaten local vector RAG.** Anthropic removed vector search
  from Claude Code (May 2025) in favour of grep ("outperformed everything. By a lot."); Cursor, Windsurf, Cline, Devin
  and Sourcegraph followed; an AAAI 2026 paper measured agentic keyword search at **94.5% of RAG faithfulness with zero
  vector store.** Agentic search follows references, needs no chunking/sync/invalidation infrastructure, and keeps data
  local.
- **`llms.txt` / `.md` is the working business-to-agent surface.** IDE coding agents routinely fetch it even though SEO
  crawlers ignore it — so curated markdown is the right substrate to vendor locally.
- **Curation beats the retrieval mechanism.** LLM-generated context files measurably _hurt_ task success; human-curated,
  reference-grade docs win — and agents can't infer missing fields from naming, so completeness (params, real example
  responses, every error code) matters more than how it's fetched.
- **RAG isn't dead — it's just the wrong tool here.** It remains right for governed, hosted, large-corpus answer
  services (fiskaly's Ask-AI is correctly a RAG); it's the wrong tool for an agent navigating docs.

### What this means concretely

- Curate a **local docs corpus** in `mcp/corpus/index.json`, using the SIGN IT
  OpenAPI specs, API probes, and high-value support findings as source material.
- Expose the corpus through `search_fiskaly_docs` and `fetch_fiskaly_doc`.
- Keep the eval path local and deterministic; no vector DB and no hosted RAG in
  the run loop.

### Rejected alternatives

- **Local vector RAG** — superseded for navigation; index/chunk/sync/permission infrastructure for worse results.
- **Remote RAG as primary** — external dependency, non-deterministic, outage/rate-limit risk, citations not perfectly
  product-scoped.
- **Raw `llms-full.txt` dump** — fiskaly's is 8.5 MB; dumping it whole blows the context window. Needs an index +
  selective fetch.

### Revisit if

The corpus grows too large to grep efficiently, or docs require auth/freshness that local vendoring can't provide — then
add a selective MCP/CLI retrieval layer _in front of_ the curated markdown (still not a vector store).

### References

June 2026 DX research (sources captured in the working session); `research/RESEARCH.md` (dx-benchmark); the
documentation-grounding discussion.
