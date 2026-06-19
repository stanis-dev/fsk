# Project Rules

- Every iteration on the project must have eval scenario that will exercise it before implementation starts.
- A feature/fix is only considered done when evals exercise it and pass eval criteria, with no regressions.

- Domain is extremely sensitive

## Where the knowledge lives

| Path                                  | What                                                                               |
| ------------------------------------- | ---------------------------------------------------------------------------------- |
| `memo/OPPORTUNITIES.md`               | Opportunity map + why Zero-to-Receipt                                             |
| `research/RESEARCH.md`                | Synthesis: company, SIGN IT API, Italy regulation, DX benchmark, docs platforms    |
| `research/PERSONA.md`                 | Who integrates fiskaly — the implementer persona, pain points, failure spectrum    |
| `research/PUBLIC-FEEDBACK.md`         | Real-world signal: GitHub issues, the 242-article Zendesk KB, status incidents     |
| `research/GITHUB-INTEL.md`            | fiskaly's engineering profile (stack, culture, hiring) inferred from public GitHub |
| `research/DECISIONS.md`               | Design decisions — **ADR-001: docs context = local + agentic search**              |
| `research/EVAL-CHECKS.md`             | Eval-check taxonomy (standard + fiskaly), application architecture, gap analysis    |
| `research/api-probes/NOTES.md`        | The SIGN IT API contract learned by live probing (the undocumented gotchas)        |
| `research/api-probes/transcript.json` | Raw happy-path request/response evidence behind NOTES.md                           |
| `research/specs/`                     | Downloaded SIGN IT + unified OpenAPI specs (2025-08-12 and 2026-02-03)             |
| `research/fiskaly_research.json`      | Raw, fact-checked research output (6 areas)                                        |
