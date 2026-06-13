# CLAUDE.md

Agent guidance for this repository lives in **[AGENTS.md](AGENTS.md)** — read it
first. The essentials:

- This is a **research / learnings repository**, not a running codebase. There is
  **no application code** in the working tree by design (no `go.mod`, `cmd/`,
  `internal/`). Don't try to build or run anything.
- A full working prototype ("Zero to Receipt": Go MCP server + compliance judge +
  simulator, validated against the live SIGN IT TEST API) was built and then
  removed to keep this repo focused on learnings. It's preserved at git tag
  **`prototype-v0`** — `git checkout prototype-v0` to inspect or restore it.
- Research docs describe that prototype and a "demo"; that's **history and
  learnings, not current code**.
- Before any implementation, read `research/DECISIONS.md` (ADR-001) and
  `research/RESEARCH.md`. Per ADR-001, docs context = curated local docs +
  agentic search (no vector store).
- `.env` (gitignored, may be absent) holds fiskaly **TEST** credentials only —
  never use LIVE.
