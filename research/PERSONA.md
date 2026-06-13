# Who We're Building For — The fiskaly Implementer

The documentation we're improving is read by exactly one kind of person, and they
are **not** a fiscalization expert. Understanding them — their role, their
priorities, the shape of their work, and how they fail — is the foundation for
every documentation and tooling decision. This is that profile.

## Who actually reads the docs

The **buyer** is the POS/ERP vendor. The **reader of the docs** is a senior
backend / integrations engineer at that vendor (or at a system integrator, or on
a large retailer's in-house POS team).

Picture **Giulia**: a strong generalist backend developer — Go, Java, C#, PHP,
whatever the POS stack is — who has shipped plenty of API integrations. She owns
features end to end and is the one paged when something breaks in production.

The defining fact about Giulia: **she knows nothing about Italian tax law, and
she has no desire to.** Her job isn't "become a corrispettivi expert." It's "add
Italian receipt support to our POS by Q3." Fiscalization is one scary,
load-bearing part of a larger feature, and she has to learn just enough of an
entire regulatory domain to ship it correctly — without it becoming her whole
career.

## What she cares about (in priority order)

1. **Don't block the sale.** The checkout path is sacred. A slow fiscalization
   call, or an outage that freezes the till, is worse than almost any other bug —
   a shop that can't ring up coffee is losing money and calling support.
2. **Don't get us fined or de-certified.** Unlike a normal bug, a compliance bug
   has legal teeth: fines, register seizure, the product becoming illegal to
   sell.
3. **Let me map my world to yours without a PhD.** Her POS thinks in orders, line
   items, discounts, split payments. fiskaly thinks in taxpayers, systems,
   records, intentions, VAT breakdowns. The law thinks in *documento
   commerciale*. She is translating across three vocabularies at once.
4. **Let me prove it works before go-live.** She can't test in production — every
   live record is a real fiscal document sent to the tax authority. The sandbox
   has to behave like the real thing, and she must demonstrate correctness to her
   own compliance team.
5. **Don't make me relearn this every six months.** The law changes; she wants to
   wire it once and forget it.

## The end-to-end scenario

Sales closes a deal: the POS is expanding to Italy. Giulia gets the ticket
"Italian fiscal receipts." Her month:

**Week 1 — orientation tax.** She opens the SIGN IT reference and hits blank
field descriptions (the 168 unresolved template placeholders). She doesn't know
if an espresso is 10% or 22% VAT — a *domain* question the API docs don't answer.
She has to learn a resource hierarchy (organization → subject → taxpayer →
location → system) and it isn't obvious why "taxpayer," "location," and "system"
are three different things. She tries to create a taxpayer and gets a `405` — it
turns out you need a UNIT organization *and* a separately-scoped API subject
first, which the docs never state; she learns it from the error. Half a day gone
to something that should have been one sentence.

**Week 2 — the real work: model impedance.** She maps a real order — three items,
a 10%-off discount, paid half cash half card — into fiskaly's structure. The API
**derives nothing**: she must compute, per line, the VAT percentage, amount, net
and gross, and they must reconcile to the document total to the cent or the API
rejects the request. Rounding bugs everywhere. This is where the days actually go
— not the HTTP, the arithmetic and the schema.

**Week 3 — the long tail of commerce.** The first happy-path receipt works in
TEST; it feels done. Then: a customer returns a cornetto (CANCELLATION —
two-call pattern again, referencing the original record). A reprint. A €0
promotional line. A VAT-exempt item. The lottery code. Each is a new payload
variant she discovers one at a time.

**Week 4 — resilience and the things that aren't in the API reference.** She
wires it into checkout and asks the scary question: *what if the tax authority is
slow or down at the moment of sale?* The answer — issue a paper document and an
electronic invoice within 12 days — is a **legal** requirement living on a
separate "Connection Loss" page, not in the API docs. Miss it and there's a
compliance hole nobody sees until an audit. Then ops reality lands: each real
merchant needs Fisconline credentials that **expire every 90 days** with a manual
first login — so she has just inherited a credential-rotation problem and a
future wave of "my receipts stopped working" tickets on day 91.

## The pain points, grouped

- **Domain-translation pain** — the hard part isn't the API, it's knowing what
  the *law* requires (which rate, which document type, the 12-day rule). The docs
  explain endpoints, not obligations.
- **Model-impedance pain** — squeezing orders / discounts / split-payments into
  nested record/entry/VAT structures where nothing is derived and everything must
  reconcile.
- **Undocumented-contract pain** — the rules you only learn from `422`s and
  `405`s (subject-name regex, "legal *and* trade name required," scoped-subject
  requirement, commissioning order, composite record types). The docs say one
  thing; the error teaches you the truth.
- **Resilience pain** — outages, FAILED records, retries-with-idempotency,
  polling to a terminal state, all without freezing checkout.
- **Lifecycle / ops pain** — provisioning thousands of merchants, credential
  expiry, and migrating when fiskaly ships a new date-versioned API (the
  `/assets`→`/organizations` rename) under a regulatory deadline.
- **Testability pain** — can't test in prod, no scenario harness, proving
  correctness for certification by clicking through Postman.

## The failure spectrum

**Tier 1 — slows her down (no bug yet, just lost days).** Blank doc
descriptions, learning the hierarchy by trial and error, discovering constraints
via error responses, no copy-paste examples, and not knowing the domain well
enough to choose the right VAT rate or document type. Pure friction — but it is
most of the calendar.

**Tier 2 — the pre-prod bounce (caught in QA, ticket back to dev, blocks the
release).** VAT reconciliation off by a cent so the API rejects a real basket; a
refund/void/exempt path nobody implemented until QA tries it; an idempotency key
reused on a retry that surfaces as a duplicate or `422` under load; lifecycle
steps done out of order that work on the happy path but fail when QA deviates;
the outage fallback missing, flagged by a chaos test or a compliance reviewer.
The "works on the golden path, breaks the moment someone deviates" class —
expensive, but caught.

**Tier 3 — the silent production catastrophe (the genuinely dangerous one).**
What makes this domain different: **the worst failures don't crash — they print a
perfect-looking receipt.** Fire-and-forget without polling, so a sale never
reaches the tax authority — the customer is happy, the receipt looks normal, and
it surfaces only at an audit with retroactive fines. Or the wrong VAT applied at
scale — systematic under-collection nobody notices. Or credentials silently
expiring on day 91 so a merchant's receipts start failing in production and the
shop literally cannot issue a legal sale.

## The core insight

A normal bug throws an error. **A fiscalization bug looks like success.** Loud
failures get caught; the lethal ones are silent and compliance-shaped.

So *"make receipts easy"* for the implementer means four concrete things:

1. Shrink the domain knowledge she needs.
2. Make the undocumented contracts explicit.
3. Make the edge cases discoverable.
4. **Make the silent failures loud — before they reach production.**

The fourth is precisely what a judge that audits the integration is for: it turns
a quiet "you never confirmed this receipt reached the authority" into a visible,
cited finding while she is still at her desk — not at the audit. This persona is
the reason the prototype pairs an action layer (do the work) with a judge layer
(prove it was done compliantly).
