# Public Feedback & Real-World Signal on fiskaly

Standalone research compiling every public signal we could find about how real people experience fiskaly — developer
pain, reliability, and recurring confusion. Gathered 2026-06-13. This is research only; it is deliberately **not** wired
into the persona/scenario work.

## Methodology and honest caveats

fiskaly is low-level B2B compliance infrastructure. Its actual users are engineers at POS/ERP vendors integrating it
under NDA — a population that does not post public reviews. So **direct** sentiment (review sites, Reddit, Stack
Overflow) is thin by nature, and the strongest grounding is **indirect**:

- **GitHub SDK issues** — real developer pain, verbatim (but V1-SDK era, ~2020–22).
- **fiskaly's own Zendesk knowledge base** — 242 articles; each title is a question common enough that fiskaly chose to
  document it = institutionalized pain. Current (2026) and SIGN-IT-specific. _Richest source._
- **Status page / incident feed** — what actually breaks in production.

Access notes: the Zendesk KB HTML 403s but its public API is open; the status HTML is JS-rendered but the Atom/RSS feed
is open; **Stack Overflow is not accessible to our crawler and surfaced no fiskaly content via search** — and the
absence is itself a signal (developers don't self-serve fiskaly on SO; support flows through GitHub issues and private
Zendesk, which is why the KB is so large); **G2** has a fiskaly page but blocked the fetch (403).

---

## 1. fiskaly's Zendesk knowledge base — 242 articles (richest signal)

The article catalogue is fiskaly admitting, in its own words, where integrators get stuck. The recurring clusters (SIGN
IT emphasised, since that's our target):

**Protocol mechanics integrators can't infer from the reference**

- "SIGN IT — What does X-Idempotency-Key refer to?"
- "SIGN IT — What does X-Scope-Identifier refer to?"
- "SIGN IT — What to enter in the 'producer' field if the POS is software-only"

    _(These are exactly the contracts we had to discover by probing — they're common enough to be standalone FAQs.)_

**Credentials & auth lifecycle**

- "SIGN IT — How to check if FISCONLINE credentials are valid"
- "SIGN IT — How to update the FISCONLINE credentials"
- "SIGN IT — What fiscalization data should be provided when creating a Taxpayer"
- a whole **Authentication** section

**Version migration / terminology churn**

- "SIGN IT — New Resource Terminology"
- "SIGN IT — Key changes in SIGN IT 2026-02-03"
- "SIGN IT — Key changes in SIGN IT 2025-08-12"

**Money model: VAT, rounding, discounts (the reconciliation pain)**

- "SIGN IT — Value calculations and decimal rounding"
- "SIGN IT — How to represent mandatory cash rounding"
- "SIGN IT — How to represent discounts"
- "SIGN IT — Which VAT rates and treatments can be used in Italy"

**The long tail of commerce (edge cases, one FAQ each)**

- returns ("How to handle returns (CORRECTION)"), "When should CANCELLATION be used?"
- single- and multi-purpose vouchers; meal-voucher payments; tips; complimentary item (GIFT); outstanding payments
- "How to integrate the Lottery Code in SIGN IT"; "Do I need to display a QR code?"

**Errors & resilience**

- "SIGN IT — Error Handling Guide"
- "SIGN IT — What to do in case of connectivity issues?"
- "SIGN IT — What is DEGRADED mode and how to recover"
- multiple **"Errors and Status Codes"** sections

**Output / what to print, and reading the response**

- "SIGN IT — What data is provided in the compliant document response?"
- "SIGN IT — Difference between document.number and AdE progressive number"
- "SIGN IT — Can I obtain the generated record PDF via API?"
- "SIGN IT — How to locate a commercial document in the AdE portal"

**A telling cross-country admission about examples**

- "SIGN ES — Where can I find examples for the request bodies?" — confirms "where are the examples?" is a known,
  recurring ask across products.

Section taxonomy seen in the KB: Authentication, API Keys, Records, Transactions, Taxpayers & Locations, Systems,
Exports, **Errors and Status Codes** (multiple), Countries and requirements, Receipts, Cash Point Closing.

## 2. GitHub SDK issues — developer pain, verbatim (V1-era, archived)

Dozens of issues across the five archived SDK repos (node/java/php/dotnet/swift). Grouped by theme, with representative
titles:

- **Docs drift / version confusion:** "Seems to be API documentation is outdated" (java #25); "How to connect to V2?"
  (dotnet #38); "This is an sdk for the version 1 api, isn't it?" (node #14); "Explain why the SDK requires a
  service/library to access the API" (node #10).
- **Auth / token / opaque errors:** "token's 'exp' has passed or could not [be] parsed" (dotnet #21); "How to obtain new
  access token? No api endpoint available" (java #26); "503 failed to fetch remote JWK" / "401 Invalid credentials" /
  "500 SlowDown" (php #15/#14/#13); "Rate Limit exceeding — different kinds of errors and no clear status code in
  exception" (dotnet #22).
- **Enterprise environment (HTTP proxy):** "Client ignores HTTP proxy during check certificate" (java #30); "Client does
  not ignore HTTP proxy for some requests" (java #23); "Question regarding configuration of HTTP proxy" (java #19).
- **Native-binary platform hell** (why the SDKs were ultimately abandoned for "use an HTTP client"): "Mac dylib not
  found" (java #17, 12 comments); "ffi-napi… M1 Mac" (node #15); "UnsatisfiedLinkError: Native library" (java #24);
  "Random segmentation faults" (java #2); "Building for iOS Simulator on ARM64 Mac doesn't work" (swift #36).
- **Distribution:** "Latest release is not available in maven" (java #32); "NPM packages broken" (node #7).

Caveat: these are several years old and the repos are archived. They are pattern-relevant (the same classes of pain —
docs drift, auth opacity, version migration, enterprise proxies — recur in the current KB), not proof of current state.

## 3. Status page & incident history (what breaks in production)

Current status at time of research: **all systems operational.** Components tracked: Management API, Dashboard,
Authentication Server, SAFE (global); SIGN DE, DSFINVK DE, SUBMIT DE, fiskalcheck; SIGN AT; SIGN ES; SIGN IT; SIGN FR;
RECEIPT. Incident feed (last ~3 months) shows clear patterns:

- **Uptime is hostage to external tax authorities** — the single most recurring theme. "External maintenance:
  FinanzOnline (FON) service temporarily unavailable" appears ~6× (Apr–Jun 2026); "SIGN IT – Italian AdE Web Portal
  temporarily unavailable" (Apr 1) and "…unavailable due to maintenance" (Mar 22); "SUBMIT DE — degraded transmitting to
  ELSTER portal" (May 24). This is the structural reality behind the outage-handling design pressure.
- **Timeouts recur:** "SIGN IT — Increased Timeouts" (Jun 9, ~28 min); "SIGN DE — Increased number of timeouts" (Apr
  14); "SUBMIT DE — increased timeouts and errors" (Apr 19).
- **Auth performance:** "SIGN IT — Rare Authentication Performance Issues" (Mar 29).
- **Heavy maintenance cadence**, especially SIGN DE v2 (multiple scheduled + unplanned/short-notice windows across
  Mar–Jun 2026).

No public uptime percentages are published.

## 4. Review & employer sites (thin, as expected)

- **OMR Reviews** (DACH B2B): a fiskaly page exists — 4 reviews, 5.0/5, **no review text**. Aggregate only.
- **G2**: a fiskaly reviews page exists but blocked the fetch (403); contents unverified.
- **Glassdoor**: ~4–8 employee reviews (not users, but adjacent). Praise for office, work-life balance, hardware, and
  team; a recent review flags a cultural shift — _"unreal to think this is the same company as last year"_, citing
  top-down decisions hurting satisfaction — consistent with the 2024 Verdane PE stake, 2025 chairman change, two 2025
  acquisitions, and the AI-First push.
- A general OMR/web mention surfaced a customer concern about _cloud dependency_: inability to process transactions
  during connectivity loss, plus subscription cost — i.e. the same outage anxiety the status feed substantiates.

## 5. Adjacent (not fiskaly-attributed) — illustrative only

Shopify community (German), multi-year thread: TSE info prints only on the **second** receipt, not the first — _"feels
like it's still in beta… a problem that's been there at least 2 years."_ fiskaly is **not named** as the provider, so
this is not attributed to them. Included only because the _failure shape_ — a first receipt silently missing its fiscal
signature — is the canonical "looks-like-success, isn't-compliant" failure of this domain.

---

## Recurring themes across all sources

1. **The reference under-specifies the contract.** Idempotency/scope headers, the "producer" field, response semantics —
   common enough to be FAQs, absent enough from the reference to need them. ("Where are the request examples?" is an
   explicit, recurring ask.)
2. **Auth/credential lifecycle is a top pain** — token expiry, FISCONLINE validity/rotation, opaque 401/JWK errors — in
   both the old issues and the current KB.
3. **Version migration is a standing tax** — "New Resource Terminology," dated "Key changes" articles, "How to connect
   to V2?".
4. **The money model is hard** — decimal rounding, discounts, VAT treatments, document-total reconciliation each warrant
   their own KB article.
5. **Reliability is partly outside fiskaly's control** — external tax-authority outages (FON, AdE, ELSTER) dominate the
   incident feed, making graceful degradation a first-class integration requirement, not an edge case.
6. **Public sentiment is structurally thin** — no telemetry, NDA'd users, near- zero Stack Overflow/Reddit footprint;
   the KB's 242 articles exist precisely because the questions can't be self-served elsewhere.

## Source list

- fiskaly GitHub org & SDK issue trackers: https://github.com/fiskaly
- Zendesk KB (public API): https://support.fiskaly.com/api/v2/help_center/en-001/articles.json
- Status page: https://status.fiskaly.com · feed: https://status.fiskaly.com/history.atom
- OMR Reviews: https://omr.com/en/reviews/product/fiskaly
- G2 (blocked): https://www.g2.com/products/fiskaly/reviews
- Glassdoor: https://www.glassdoor.com/Reviews/fiskaly-Reviews-E3059515.htm
- Shopify community (adjacent): https://community.shopify.com/t/pos-druck-tse-infos-erst-nach-dem-2-beleg/287777
- Stack Overflow: not accessible to our crawler; no fiskaly content surfaced via search.
