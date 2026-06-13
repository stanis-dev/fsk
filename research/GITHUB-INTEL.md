# What fiskaly's Public GitHub Reveals

Inferred engineering profile of fiskaly, built entirely from public evidence at
github.com/fiskaly (30 repos), gathered 2026-06-13 via four parallel research
passes (PR/review culture, issue/support culture, current stack, org
conventions + hiring). Everything here is **inference from public scraps** — repo
metadata, READMEs, Dockerfiles, go.mod files, commit history, PR/issue threads —
not any internal source. Confidence levels are stated.

## The one caveat that frames everything

**The public org is a satellite, not the workshop.** fiskaly's actual
fiscalization backends are private (named in passing in public comments —
`mice`, `fiskaly-gateway`, `sign-de` — but not visible). The public surface is:
archived SDKs, vendored third-party forks, internal dev-tooling Docker images,
and the hiring challenge. So judge the org on a *small* first-party set
(`golog`, `grafana.surrealdb`, the `docker.*` family, the GOBL forks, the
coding challenge), not the 30-repo headline — and treat all of this as the
*periphery* of their engineering, which can still be revealing about habits.

## Top inferences at a glance

1. **Stack (high confidence):** Go backend on **PostgreSQL**, **OpenAPI-first**
   (codegen both Go server and TS client from one spec), running on **Google
   Cloud**, observed via **Grafana**, auth via **Keycloak**. Italy/e-invoicing
   built on the **GOBL** open-source document stack.
2. **Strategic arc:** Germany-only SDK company (2019–21) → Go/OpenAPI/Postgres
   platform (2022–23) → **multi-country EU e-invoicing, Italy first via GOBL**
   (2024–26). The German TSE business is mature/archived; **Italy + GOBL is
   where the new engineering energy is** — i.e. exactly the SIGN IT surface our
   prototype targets.
3. **How they build country compliance:** not from scratch — they **vendor GOBL
   at a pinned tag, carry the minimum surgical patch for local legal
   correctness, and intend to upstream and delete the fork.** Their moat is
   correctness + integration, not a reinvented invoice engine.
4. **Culture:** friendly, collegial, European-polite; JIRA-driven (`META-`,
   `MSB-`, `TSE-` ticket prefixes); pragmatic reviews (blast-radius and design,
   not style-policing); candid about their own bugs and docs gaps. Light-but-real
   process. Weak spot: external PRs/issues sometimes stall.
5. **Security maturity:** a coordinated, ticket-tracked sweep to **SHA-pin all
   GitHub Actions** across repos (May 2026) — supply-chain hygiene befitting a
   compliance company.
6. **Hiring (directly relevant): the take-home challenge is a miniature of their
   own product** (a tamper-evident signing service), it explicitly **permits and
   even "encourages" AI with a disclosure-and-defend requirement**, and it
   optimizes for **verifiable correctness** and the ability to **reason about
   your design** to two senior engineers.

---

## 1. The repo landscape

Of 30 public repos: ~9 are **vendored forks** (`keycloak`, the three
`vendor.invopop-*`, `go-crypto`, `cmac`, `gojsonschema`, `TTC`, `airbyte`) whose
recent "activity" is often upstream commits (overstating fiskaly's cadence); the
`fiskaly-kassensichv-*` (2019) and `fiskaly-sdk-*` (2020, node/php/dotnet/java/
swift) repos are **all archived**; the genuinely first-party, active set is small
— `golog`, `grafana.surrealdb`, and the `docker.*` build-image family. Licensing
shifted deliberately: old SDKs were **MIT** (open-distribution era); current
first-party tooling carries a proprietary **"fiskaly GmbH Software License v1"**
("use… permitted only under the condition of a valid contract… Stress testing,
penetration testing… is strictly prohibited").

## 2. Current stack & strategic direction

| Layer | Inference | Evidence | Confidence |
|---|---|---|---|
| Language | **Go** | every active infra repo is Go or wraps a Go tool; backend challenge is Go | High |
| Database | **PostgreSQL** | `docker.migrate-pgsql` (golang-migrate) + `docker.sqlc` (SQL→type-safe Go) | High |
| API tooling | **OpenAPI-first**, contract-driven | `docker.oapi-codegen` (OpenAPI→Go, pinned v2.6.0 Apr 2026) + `docker.sta` (`swagger-typescript-api`, OpenAPI→TS) → spec-as-source-of-truth, generating both server and client | High |
| Fiscal-format codegen | **XSD→Go** | `docker.xgen` (`xuri/xgen`) — FatturaPA & TSE formats are XSD-defined | Med-High |
| Hosting + logging | **Google Cloud**, structured logs | `golog` = "structured logger for GCP logs" | High |
| Observability | **Grafana** | first-party `grafana.surrealdb` datasource plugin, active 2026 | High |
| Auth | **Keycloak** (self-hosted, perf-patched) | fork tracking 20.0–22.0 + custom `improve-group-update-performance` branch | Med-High (live as of mid-2024) |
| Secondary store | **SurrealDB** (experimental) | the Grafana plugin depends on `surrealdb.go`, but no migration/schema tooling like Postgres gets | Medium |
| CI / supply chain | GHA, **SHA-pinned**, explicit-version containers | org-wide `META-3063: Pin to SHAs all GitHub actions` sweep, May 2026 | High |

Speculative (don't rely on): NATS + Echo + templ + protobuf appear in the
`invopop-client.go` go.mod they *vendor* — that's Invopop's dependency set, not
provably fiskaly's own services. The `airbyte` fork (one-day push, Jan 2025)
signals data-pipeline *interest*, not proven production use.

**The GOBL story (a gem).** GOBL = *Go Business Language* (open Go + JSON-Schema
library modelling/validating/signing tax documents, by Invopop); FatturaPA =
Italy's mandatory e-invoice XML cleared through the tax authority's SDI. fiskaly's
three forks (created Dec 2025–May 2026, pushed as recently as 11 Jun 2026) show
**SIGN IT / Italian e-invoicing is built on GOBL, not from scratch.** Their own
`FORK_NOTES.md` documents the discipline: a patch on upstream tag `v0.306.0`,
re-tagged `v0.306.1` ("This is what consumers pin"), with the rationale spelled
out — without their two added Italian fields (`StatoLiquidazione`/`SocioUnico` →
the FatturaPA `IscrizioneREA` block) "companies in liquidation get structurally
incorrect XML." And the kicker: *"If/when the upstream PR is accepted, drop the
fork."* This is a thin-fork, fast-forward-port, upstream-first strategy with
every change tied to a ticket — a culture that values staying close to upstream
and surgical correctness.

**Timeline.** 2019 KassenSichV clients (archived) → 2020 multi-language SDKs
(archived) → 2022–23 platform/CI tooling (`docker.*`, `keycloak`, `golog`,
`TTC`) industrializing a Go/OpenAPI/Postgres backend → 2024–26 Grafana
observability, `airbyte`, and the **GOBL Italy push** plus still-active toolchain
curation. German→platform→Italy.

## 3. PR structure & code-review culture

- **JIRA-driven, light template discipline.** Branches/commits carry ticket
  prefixes (`MSB-2608`, `META-3063`, `TSE-783`) and `feature/`,`bugfix/`,`docs/`
  naming; titles often *are* the ticket. Only `fiskaly-sdk-swift` has a real PR
  template (Motivation / Test Plan) + CONTRIBUTING ("Make sure the PR does only
  one thing, otherwise please split it") — not propagated elsewhere.
- **Merge commits, not squash** (feature-branch history preserved). PR size is
  bimodal (tiny internal bumps vs large scaffolding/external/candidate PRs). CI
  exists where the repo is real (swiftlint/sonarcloud; ci/release on the active
  repos).
- **Tone: friendly, low-ceremony, design-focused.** Reviews chase "who else
  depends on this / what breaks downstream," not style nitpicks; authors discuss
  and push back rather than just comply. A second approval is sometimes required.
- **Weak spot: external contributions stall** — multiple outside PRs sit
  unanswered or get superseded ("any news about it?"); candidate PRs are never
  publicly reviewed.

Representative voice:
> *"Nobody is using this apart from us, but recheck mice, it can be we are using this package there"* — reviewer on blast-radius (golog#9)
> *"would you mind taking a look at what I changed and giving feedback before I merge this into master?"* — collaborative, not gatekeeping (sdk-swift#17)
> *"Well, I think I just overcomplicated this whole process… I am sorry."* — candid about churn (sdk-swift#16)

*Thin/unknowable:* the core product is private, so we don't see review of their
actual fiscalization backend; most public PRs show `REVIEW_REQUIRED` even when
merged (branch protection wasn't enforced on these peripheral repos, so absence
of an approval ≠ absence of review). Recent 2026 public activity is dominated by
Dependabot and one engineer's SHA-pinning sweep.

## 4. Issue & developer-support culture

From ~82 human-filed issues (five archived SDKs + the active grafana plugin):

- **Responsive when engaged:** ~65–70% get a maintainer reply, frequently
  **same/next day**; only ~10–15% go truly stale. The failure mode is *silent
  open* on late-life archived SDKs (2021 PHP/dotnet "How to connect to V2?"), not
  stale-closing.
- **Warm, candid, bilingual.** Standard "Hello @user / Best, …" register; German
  issues get German replies; maintainers openly admit bugs and docs drift and
  even fault their own backend (a rate-limiter postmortem).
- **They debug to root cause in-thread and ship fixes** (a 12-comment Mac-dylib
  thread; "I just released v1.2.001… armv7 enabled again").
- **GitHub is explicitly *not* their main channel** — they said so: *"As we are
  receiving reports on Github as well as other platforms, the Issue count you are
  seeing on Github doesn't represent the actual number."* The real SLA lives on
  support.fiskaly.com / email (invisible here). Deflection-to-private-support is
  otherwise essentially absent — issues are treated as a real support channel.

Representative voice:
> *"Yes, I am able to reproduce, and I will work on a fix."* (sdk-java#17)
> *"You are right, the `created_by_user` field is indeed not documented. We will add this to the specification soon."* (sdk-java#25)
> *"Currently the .NET SDK is not intended for use in a multi-threaded environment."* (sdk-dotnet#25)

*Thin/unknowable:* small, mostly 2020–21 sample reflecting a team under
KassenSichV launch pressure; latency figures are impressionistic; the grafana
repo's excellent recent support is essentially one engineer on a side-project.

## 5. Org conventions & process maturity

- **No org-wide community-health scaffolding.** The special `.github` repo holds
  *only* a profile README — no default issue/PR templates, CONTRIBUTING,
  CODE_OF_CONDUCT, SECURITY, or reusable workflows. Conventions live in private
  repos and an internal JIRA.
- **Tidy where it matters:** consistent semver `vX.Y.Z` tags + GitHub Releases
  (one repo even auto-tags via GHA); PR-with-merge-commit flow as the norm;
  consistent proprietary licensing on first-party repos.
- **Commit style:** two coexisting disciplined patterns — ticket-prefixed
  (`META-3063: …`) and area-prefixed (`Configuration: …`, `CI: …`, `docs: …`) —
  not strict Conventional Commits, but traceable.
- **Security maturity:** the org-wide SHA-pinning of GitHub Actions (`META-3063`)
  is the standout positive signal.

Overall: moderate-to-good for a deliberately minimal public surface; the
impressive engineering is private.

## 6. Hiring signal — the coding challenge (directly relevant)

`coding-challenges` (59 forks) is the take-home, mapped to roles:
`signing-service-challenge-go`/`-ts` = **"Mid / Senior Backend developer"**;
plus fullstack, React, and SRE variants.

**The flagship is a miniature of fiskaly's own product**, explicitly: *"This
challenge is heavily influenced by the regulations for KassenSichV (Germany) as
well as the RKSV (Austria) and our solutions for them."* Candidates build a REST
API that creates signature devices (RSA/ECC keypair, label, `signature_counter`)
and signs transactions, **chaining each signature to the last** — a
tamper-evident hash chain, the heart of fiscal signing.

What it tests, in their words:
- **Verifiable correctness (the compliance mindset):** *"we need to make sure
  that our implementation is verifiably correct. Think of an automatable way to
  assure the correctness… of the system."*
- **Concurrency / integrity:** *"used by many concurrent clients… The
  `signature_counter` has to be strictly monotonically increasing and ideally
  without any gaps."*
- **Extensible design & persistence abstraction:** extend to new algorithms
  without touching core domain logic; in-memory now but "keep in mind that we may
  later want to switch to a relational database."
- **Pragmatic scoping:** "Efficiency is not a priority for this." "The quality of
  your code is more important to us than the quantity."

Process & values:
- The take-home is the **second and final stage**, feeding a **skill-fit
  interview with two developers** where you **defend your design** — they
  optimize for *defensible reasoning*, not a green test suite. (No time budget or
  written rubric is given — thin spot.)
- **AI policy is the tell for the role you're applying to:** the Go challenge
  permits AI but requires you to *"reason about the design and implementation
  choices"* and disclose tools used; the newer fullstack challenge goes further —
  AI *"permitted and encouraged… At fiskaly, we embrace AI as a productivity
  tool"* — with a structured `## AI Tools Used` disclosure template. The newest
  variant adds production realism: PostgreSQL/Drizzle, Docker Compose, explicit
  thread-safety (mutexes / DB transactions), and a graded `## Caching Strategy`
  writeup ("correctness of cache invalidation… especially for critical data like
  signature counters").

## 7. What this implies for the interview

- **The themes we built around are fiskaly's own:** tamper-evident signing,
  *verifiable correctness*, concurrency/counter integrity, and "prove the system
  is correct in an automatable way" — that last line is, almost verbatim, the
  judge-audits-coder idea our prototype embodies. The Zero-to-Receipt judge is a
  direct answer to a value they put in their own hiring rubric.
- **They reward AI-with-accountability** — used to move fast, but you must
  disclose it and defend every choice. Mirrors how this project was built (agentic
  research + a human-defensible design), which is itself the story to tell.
- **Speak their stack:** Go, OpenAPI-first codegen, Postgres, GCP/Grafana,
  GOBL-based country compliance, thin-fork upstream discipline. Our prototype's
  spec-driven Go + "generate from the OpenAPI spec, regenerate per CalVer"
  argument lands directly.
- **Their culture is collegial, candid, design-debate-oriented, JIRA-tracked, and
  honest about gaps** — match that register: pragmatic, blast-radius-aware, happy
  to be corrected.

## Methodology & limitations

Authenticated `gh` over public endpoints; four parallel agents on PR culture,
issue/support culture, stack, and conventions+hiring. Limits: the core product is
private; the SDK evidence is 2020–21 and archived; "maintainer" attribution from
GitHub is fuzzy; latency/coverage figures are directional, not measured; forks
inflate apparent activity. Individuals' names/handles are deliberately omitted —
this is about engineering practice, not people.
