# Project Rules

- Every iteration on the MCP must have eval scenario that will exercise it before implementation starts.

## Code Rules - never tolerate

- comments that:
    - say what code does not do
    - explain information not specifically relevant to this particular implementation
    - context irrelevant to consumers
    - context of a specific implementation
    - information that can be inferred from the code itself
    - information that should be expressed through naming conventions
- backwards compatibility
- "just in case" logic of any kind
- dead code of any kind
- passthrough functions
- overengineering
- non-idiomatic code
- non-standard approaches unless justified (which must be confirmed with user)
- configurability without clear purpose
- manual implementation of standard libraries or what can be solved though a package
- naming conventions that express context aplicable only to the implementer
- functionality mocking. If feature doesn't exist - code must never lie about it
- code not covered by tests

## Code Rules - principles

- DRY
- KISS
- Pragmatism first
- A feature is not done until it has been run e2e in browser

## Where the knowledge lives

| Path                                  | What                                                                               |
| ------------------------------------- | ---------------------------------------------------------------------------------- |
| `research/OPPORTUNITIES.md`           | Opportunity map + why Zero-to-Receipt                                              |
| `research/RESEARCH.md`                | Synthesis: company, SIGN IT API, Italy regulation, DX benchmark, docs platforms    |
| `research/PERSONA.md`                 | Who integrates fiskaly — the implementer persona, pain points, failure spectrum    |
| `research/PUBLIC-FEEDBACK.md`         | Real-world signal: GitHub issues, the 242-article Zendesk KB, status incidents     |
| `research/GITHUB-INTEL.md`            | fiskaly's engineering profile (stack, culture, hiring) inferred from public GitHub |
| `research/DECISIONS.md`               | Design decisions — **ADR-001: docs context = local + agentic search**              |
| `research/EVAL-CHECKS.md`             | Eval-check taxonomy (standard + fiskaly), application architecture, gap analysis   |
| `research/api-probes/NOTES.md`        | The SIGN IT API contract learned by live probing (the undocumented gotchas)        |
| `research/api-probes/transcript.json` | Raw happy-path request/response evidence behind NOTES.md                           |
| `research/specs/`                     | Downloaded SIGN IT + unified OpenAPI specs (2025-08-12 and 2026-02-03)             |
| `research/fiskaly_research.json`      | Raw, fact-checked research output (6 areas)                                        |
