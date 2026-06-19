# Design Decisions

Running log of load-bearing design decisions for the SIGN IT eval workbench.

## ADR-001 — Documentation context for consumer agents: local + agentic search

**Date:** 2026-06-13 · **Status:** Implemented.

### Context

A consumer agent integrating SIGN IT needs documentation context that is
inspectable during an eval run.

### Decision

Supply documentation through a curated local corpus exposed by
`search_fiskaly_docs` and `fetch_fiskaly_doc`.

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

The corpus grows too large to search efficiently, or docs require auth/freshness
that local vendoring cannot provide.
