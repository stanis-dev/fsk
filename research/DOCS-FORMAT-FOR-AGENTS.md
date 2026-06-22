# API Docs Format for AI Agents

Date: 2026-06-22. Scope: the documentation **text and how it is shaped**, not the delivery layer. MCP, llms.txt,
SDKs, and tool definitions are out of scope here (covered in `RESEARCH.md`). Question: now that AI coding agents are a
primary reader of docs, has the best practice for *what to write* and *how to structure it* changed? Sources were fanned
out across five angles and each load-bearing claim adversarially verified (3-vote, 2 of 3 to kill). Confidence levels and
refutations are preserved below rather than smoothed over.

## Verdict

Yes, and it is measured, not speculative. Docs written for humans underperform for agents for one root reason: they
leave constraints implicit and assume background knowledge an agent cannot acquire from the page. A human fills those
gaps from experience; an agent retrieving a chunk cannot. Making the implicit explicit produces large, repeatable
reliability gains.

The sharpest single result: encoding one hidden field contract (an `order_id` must carry a `#` prefix) into a benchmark
task's docs raised agent success from **10% to 49%** [Emergence AI + UMD, arXiv 2505.24197]. The effect generalizes:
rewriting endpoint/tool descriptions for explicitness lifted multi-step success on StableToolBench 33.5 to 44.6 and on
RestBench TMDB 49.5% to 74.9% [Intuit AI Research, arXiv 2602.20426], independently corroborated by EASYTOOL [NAACL
2025, arXiv 2401.06201, +11.6% / +8.8%, p<0.001]. Three independent groups converge.

The reframe in one line: **for an agent the docs are not a reference to consult, they are the contract it executes
against, and a constraint that is not written does not exist.**

Two bounds keep this honest, stated up front:

1. The win is from *explicit, dense, concise* constraints, not bulk. Dumping more text can actively hurt through
   context-window competition [arXiv 2605.14312; Constraint Decay 2605.06445]. The axis is information density, not word
   count.
2. Descriptions *steer*, they do not *enforce*. A binding rule still needs code-level enforcement; good docs reduce wrong
   calls, they do not make them impossible (guardrails literature).

## CONTENT: what to include, what to cut

**Encode the five things a terse description omits** [arXiv 2602.20426; Anthropic, Writing tools for agents]:

1. Selection scope: when to use this endpoint, when not to, how it differs from a similar one.
2. Cross-endpoint dependencies: which input values must come from a specific upstream call.
3. Output shape: which fields are returned, their types, what is omitted.
4. Parameter constraints: formats, ranges, enums, defaults, validation.
5. Cross-parameter dependencies: required-together and mutually-exclusive fields.

Original human-written descriptions cover a small fraction of these; rewritten ones cover most. (The source's exact
"covers almost none / vast majority" figure did **not** survive verification, so treat the direction as solid and the
precise percentage as unverified.)

**State preconditions, ordering, and state transitions literally** [LogRocket; Anthropic; arXiv 2602.20426]. Valid
transitions (`pending -> paid -> shipped -> delivered`) and ordering requirements belong in the text, not in the
reader's head. This is the highest-leverage category: it is exactly the class the 10%-to-49% result lives in.

**Strict, explicit parameter definitions** [ReadMe; Anthropic; OpenAI structured outputs]. Data type, required vs
optional, accepted values as inline enums, defaults, validation rules. Caveat: very large or volatile enum sets are
better referenced than inlined.

**Error responses with remediation** [ReadMe; Stripe error-codes; Stytch; Fern]. Document the full response shape for
429 / auth / pagination errors plus the retry-or-fix pattern, so agent-generated code handles them correctly.
Distinguish failure modes (token missing vs invalid vs insufficient scope), each with at least one fix.

**Describe intent, not function, and write decision logic into the field** [LogRocket; Anthropic]. "Use only after
confirming the order is eligible and the refund does not exceed the captured payment" beats "Refunds an order." Include
the negative case: "Do not use this endpoint to suspend, deactivate, or hide a user."

**Cut human-only prose.** Marketing copy and narrative filler add tokens without adding constraints and compete for
context. Concise-but-explicit is the target, not short-for-its-own-sake.

**A caution on examples.** Vendors uniformly recommend complete runnable request/response examples [ReadMe; Mintlify],
and that remains reasonable. But the one strong *empirical* claim that examples-plus-critiques significantly improve
agent success did **not** survive adversarial verification in this pass [refuted 0-3, arXiv 2505.24197]. Honest read:
examples help humans and are cheap insurance, but the measured agent-reliability gains in the literature come from
explicit *constraints*, not from sample payloads. If forced to choose, fix descriptions before adding examples.

## STRUCTURE: shaping text to survive retrieval

Agents do not read top to bottom. RAG retrieves discrete chunks, document order is not preserved, and the model has not
read the prior page [State of Docs 2026, 1,131 respondents; kapa.ai; ReadMe]. The structural rules all follow from that.

**Self-contained chunks.** Every endpoint, schema, example, and section must make sense in isolation. Anthropic's
Contextual Retrieval quantifies the cost of the opposite: restoring per-chunk context cut top-20 retrieval failure by
35% / 49% / 67% [anthropic.com/news/contextual-retrieval]. Tension worth naming: full self-containment means duplicating
context, which fights token efficiency. The resolution is contextual augmentation of chunks, not bloat.

**Co-locate related information.** The closer two facts sit in the source, the likelier they land in the same chunk
[kapa.ai; Unstructured]. Put the constraint next to the field it governs, not in a distant "Notes" section.

**Kill cross-references and pronouns.** "See above", "it", "check the logs" break the instant a chunk is retrieved alone
[Redocly; Google tech-writing-for-LLMs; Anthropic]. Name the entity: "check the application server logs."

**Clean heading hierarchy, single-topic sections, tables over prose** [ReadMe; kapa.ai]. Structure-aligned chunking
measurably beats naive splitting (one 2025 study: 87% vs 13% retrieval accuracy [MDPI, Nov 2025]); header-based chunking
is the 2025 norm. Caveat: the splashier vendor figures (+150% citations, +35% accuracy) are low-rigor; the direction is
sound, the exact numbers are not.

## What did NOT survive verification (do not assert these)

- Adding citations / statistics / quotations to docs boosts LLM visibility [refuted 0-3]. Legacy SEO does not transfer
  either: keyword stuffing runs ~10% *worse* on generative engines [GEO, KDD '24], but the inverse "add stats and
  quotes" boost was also refuted in this pass.
- Agents need the *smallest possible atomic* unit, distinct from good human structure [refuted 0-3]. Good structure for
  humans and for agents converges; agents do not need a separate atomization regime.
- A single canonical term per concept is *required* [refuted 1-2]. Replacing vague pronouns with named entities is
  confirmed; mandating one synonym everywhere is not supported.
- The precise figures "covers almost none / vast majority" and "39% absolute drop from a wrong schema" [both refuted].
  Use the directional claim only.
- Examples-plus-critiques significantly improve success [refuted, see above].

Open question the sources do not resolve: exactly where the self-containment / DRY line sits, and whether 1M-token
context windows shift the balance away from chunk-survivability rules. 2026 sources report agents still consume docs
opportunistically by relevance, so the rules hold for now.

## Applied to SIGN IT

The general findings reprioritize the existing fiskaly map (`OPPORTUNITIES.md`, `PERSONA.md`, `api-probes/NOTES.md`):

1. **The undocumented contracts are the 10%-to-49% case, exactly.** Giulia learns scoped-subject sequencing and
   commissioning order only from `405`s. That is the single highest-leverage agent-doc fix the literature identifies: an
   implicit ordering contract written down literally. It outranks examples and prose polish.

2. **The 168 blank descriptions are the worst possible failure under this lens.** The description field is the exact slot
   where an agent reads enums, required-together rules, and preconditions. Rendering it blank does not merely lose prose,
   it deletes the one machine-consumable contract the agent has. Opportunity #2 (Docs CI) is therefore not cosmetic; it
   restores the agent's contract surface.

3. **Required-together VAT fields are a textbook cross-parameter dependency.** Six required fields with no derivation is
   category 5 above. Stating inline "type, code, percentage, amount, exclusive, inclusive are all required; the API
   derives none" beats discovering it from a `422`.

4. **Reprioritize "zero examples" downward.** The existing research lists missing operation-level examples as a top
   defect. Verification says examples are cheap insurance, not the measured lever. Lead with explicit constraints
   (points 1 to 3); add examples after.

5. **The mustache leak and the rename-without-migration-guide** are contract-integrity failures rather than format ones,
   but they compound: an agent retrieving a chunk containing `{{>...}}` gets a literal non-answer. Same root as the blank
   descriptions.

Through-line, consistent with the presentation thesis: the docs are the contract the agent executes. Every finding here
restates from the agent side what the persona research found from the human side, now with measured numbers behind it:
**make the implicit explicit, and make each chunk stand alone.**

## Sources

Primary / peer-reviewed:

- Intuit AI Research (Guo et al., 2026), arXiv 2602.20426 (preprint) — explicit-constraint rewrite, the five content
  categories, StableToolBench / RestBench gains.
- Emergence AI + UMD (2025), arXiv 2505.24197 — the 10%-to-49% hidden-contract result.
- EASYTOOL (NAACL 2025), arXiv 2401.06201.
- GEO (Aggarwal et al., KDD '24), openreview.net/pdf/d7aef3973... — legacy SEO does not transfer.
- Anthropic, Contextual Retrieval, anthropic.com/news/contextual-retrieval — empirical chunk self-sufficiency gains.
- State of Docs 2026 (1,131 respondents), stateofdocs.com/2026/ai-and-documentation-consumption.
- Stripe, Building with LLMs, docs.stripe.com/building-with-llms.

Vendor guidance (individually weak and self-interested, mutually corroborating, none contradicted):

- Anthropic, Writing tools for agents, anthropic.com/engineering/writing-tools-for-agents.
- ReadMe, readme.com/blog/llm-ready-api-documentation.
- LogRocket, blog.logrocket.com/how-write-agent-friendly-api-documentation.
- kapa.ai, docs.kapa.ai/improving/writing-best-practices.
- Redocly, redocly.com/blog/optimizations-to-make-to-your-docs-for-llms.
- Google, tech-writing for LLMs, developers.google.com/tech-writing/two/llms.
- Stripe error codes (docs.stripe.com/error-codes), Fern, Mintlify, Stytch.

Counter-evidence (the bounds):

- arXiv 2505.18135 (EMNLP 2025) — verbosity effects are model-dependent.
- Constraint Decay (arXiv 2605.06445); context-window competition (arXiv 2605.14312, 2602.18537).

Method note: 5 angles, 20 sources fetched, 96 claims extracted, 25 verified (17 confirmed, 8 killed), 102 agents.
