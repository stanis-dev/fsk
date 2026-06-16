# Design: `search_fiskaly_docs` — grounded docs retrieval for the consumer agent

**Date:** 2026-06-16 · **Status:** Designed — not yet implemented · **Implements:** ADR-001

## Context

A consumer's AI agent integrating SIGN IT needs documentation context. ADR-001 decided that
context is supplied as **curated local docs + agentic search (keyword, no vector store)**, with
fiskaly's hosted Ask-AI RAG demoted to an optional remote fallback. This spec is the concrete
realization of that decision as MCP tools on the (currently empty) Go server in `mcp/`.

Two constraints shape the design:

1. **The hermetic eval mounts only the `pos` fixture.** The Docker arm (`evals/run-eval-docker.sh`)
   gives the consumer agent no repository and no docs on its filesystem — only the fixture and the
   one MCP it is allowed (`--strict-mcp-config`). So "local docs" cannot mean files the agent greps;
   the corpus must travel *inside the MCP binary*.
2. **The eval is a differential.** The MCP is empty today as the control arm. This tool is the
   treatment arm; its value is the measured gap between the two.

## Goal / non-goals

**Goal:** a deterministic, offline, keyword-ranked retrieval tool pair that lets a clean-room
consumer agent discover the real SIGN IT contract from curated fiskaly docs, and cite it.

**Non-goals:** no vector store / embeddings (ADR-001); no remote calls (the Ask-AI passthrough is a
separate future tool, never a hidden fallback inside this one); no answer synthesis — this returns
documents, not answers.

## Tool contracts

Both tools follow the search+fetch convention codified by OpenAI's ChatGPT connector / deep-research
spec and the MCP `2025-06-18` revision: results returned as both `structuredContent` and a
JSON-encoded string in `content`, with a declared `outputSchema`. Both are annotated
`readOnlyHint: true`, `openWorldHint: false` (closed corpus).

### `search_fiskaly_docs`

- **Input:** `{ "query": string (required), "limit": int (optional, default 8) }`
- **Output:**
  ```json
  { "results": [ { "id": "string", "title": "string", "url": "string", "snippet": "string" } ] }
  ```
  - `id` — stable document id, the argument to `fetch_fiskaly_doc`.
  - `title` — human-readable (e.g. `POST /records — create record`).
  - `url` — canonical citation URL (see Citation mapping).
  - `snippet` — best-matching passage, ~200 chars, so the agent can choose without a fetch.
- **No match:** returns `{ "results": [] }` (empty is a valid answer, not an error).
- **Empty/whitespace query:** tool error (`isError`), message `query must be non-empty`.

### `fetch_fiskaly_doc`

- **Input:** `{ "id": string (required) }`
- **Output:**
  ```json
  { "id": "string", "title": "string", "text": "string", "url": "string",
    "metadata": { "source": "string", "path": "string", "version": "string" } }
  ```
  - `text` — the complete section: e.g. a whole `POST /records` operation with resolved request
    schema (required-together fields explicit), responses, and error bodies.
  - `metadata.source` — one of `spec | probe | brief | kb`.
- **Unknown id:** tool error (`isError`), message `no document with id <id>` — never a silent empty doc.

## Corpus

Reference-grade, marketing-stripped. One `id` = one section, split along each source's natural seams.

| Source | Unit → document | `metadata.source` | `url` (citation) |
| --- | --- | --- | --- |
| SIGN IT OpenAPI `2026-02-03` (`research/specs/fiskaly_IT_2026-02-03.yaml`, unified spec) | one operation (method + path) | `spec` | `https://developer.fiskaly.com/api/sign-it/2026-02-03#<anchor>` |
| Probed contract (`research/api-probes/NOTES.md`) | one `##` section | `probe` | `fsk://probe/notes#<slug>` (internal; verified contract, not public docs) |
| SIGN IT integration quickstart (curated from the public getting-started guide; the working call order in `NOTES.md` is the spine) | one section | `brief` | the public getting-started URL |
| High-value Zendesk KB articles (titles inventoried in `research/PUBLIC-FEEDBACK.md`) | one article | `kb` | the public Zendesk article URL |

KB note: only article **titles** are inventoried today. The generator must source the bodies for the
high-value subset from the public help center at build time and strip them to reference content.
This is a build-time curation step, captured as a task in the implementation plan.

## Indexing & packaging

- A build-time generator (`corpus/gen`) reads the sources, splits them into sections, resolves
  OpenAPI `$ref`s and any mustache partials (`{{>...}}`) so `fetch` returns clean operations, and
  emits `corpus/index.json`: an array of
  `{ id, title, url, source, path, version, text, tokens }`.
  - Resolving partials here is also where the generator surfaces the same template defects the
    opportunity memo flags (the 168 blank descriptions / `{{>...}}` leaks) — a placeholder in the
    rendered text is a generation-time warning.
- The server does `//go:embed corpus/index.json` and loads it into memory at startup. No runtime
  filesystem or network access. This is what carries the curated corpus into the hermetic eval.

## Ranking

Field-weighted BM25-lite, pure Go stdlib, deterministic:

- **Tokenize:** lowercase; split on non-alphanumeric; preserve path tokens (`/records`,
  `x-idempotency-key`). No stemming in v1.
- **Score:** BM25/TF-IDF over the tokenized index.
- **Field weights:** matches in `title` / `operationId` / `path` boosted over `text`.
- `tokens` is precomputed in `index.json` so the server does no parse work per query.

## Module layout

- `mcp/corpus/gen/` — build-time generator (`main`); reads sources → writes `index.json`.
- `mcp/corpus/index.json` — generated, embedded artifact.
- `mcp/corpus/corpus.go` — load embedded index, the section type, id lookup (`fetch`).
- `mcp/corpus/search.go` — tokenizer + BM25-lite ranker (`search`).
- `mcp/main.go` — register the two tools (annotations, `outputSchema`, `structuredContent`) on the
  existing server.

Each unit is independently testable: the ranker against a fixed index, the loader against a known
`index.json`, the tools against a stub corpus. The exact Go SDK call shapes
(`modelcontextprotocol/go-sdk/mcp`) for tool registration, annotations, and structured output are
verified against the SDK source during planning — not asserted from memory here.

## Eval scenario (AGENTS.md mandate — authored before implementation)

A differential over the existing harness; same business-framed, vendor-blind task in both arms.

- **Arm A (control):** today's empty MCP. Expected: the agent invents a host/endpoints → the
  deterministic judge FAILs on the contract rules.
- **Arm B (treatment):** MCP with `search_fiskaly_docs` + `fetch_fiskaly_doc`. Expected: the agent
  searches, fetches the real operations, and the integration satisfies all five judge rules
  (`test.api.fiskaly.com`, `POST /tokens`, `X-Idempotency-Key`, `X-Api-Version`, two-call
  `/records`), with `go build` and `go test` green.
- **Grounding assertion (not just hoped):** Arm B's transcript must contain a `search_fiskaly_docs`
  call *before* any integration code is written — the "agent always grounded" + telemetry goals.
- **Metrics:** judge verdict per arm; turns and cost delta.

**Success criterion:** Arm B passes the judge where Arm A fails. That gap is the tool's measured value.

## Risks / open items

- **KB body sourcing.** Bodies are not yet vendored; the generator's KB step depends on the public
  help center being fetchable and the high-value subset being curated. If a body can't be sourced,
  the article is omitted (logged), not stubbed.
- **`url` anchors for spec operations.** The exact `#<anchor>` scheme on developer.fiskaly.com must
  be confirmed against the live docs site; if anchors aren't stable, fall back to the page URL plus
  `metadata.path`.

## References

- `research/DECISIONS.md` (ADR-001) · `research/api-probes/NOTES.md` · `memo/OPPORTUNITIES.md`
- OpenAI — Building MCP servers for ChatGPT (search/fetch schema): https://developers.openai.com/api/docs/mcp
- MCP `2025-06-18`: `structuredContent` / `outputSchema` (modelcontextprotocol issue #1624); tool
  annotations (`readOnlyHint`, `openWorldHint`).
