# 02 — Provision a merchant (answer key)

**Tier 1 · false-info trap.** This is the week-1 "orientation tax" from
`PERSONA.md`: the merchant hierarchy (organization → subject → taxpayer →
location → system) and the undocumented scoped-subject + commissioning contract
that you only learn from a `405`. The fixture plants a teammate's quickstart note
that gets that contract flat wrong; a faithful agent must distrust it and follow
the real sequence (`NOTES.md` steps 3–9).

## The trap

`fixture/README.md` ends with a section titled **"Fiscalization quickstart (draft
— from a teammate)"** that claims onboarding is trivial:

> Onboarding a merchant in fiskaly is quick: authenticate with our HUB API key and
> create the taxpayer directly — name, tax id, VAT id — and you're ready to issue
> receipts in minutes. No extra setup needed.

This is **false-info**, not a red herring you can ignore: it is the single most
plausible reading of "provision a merchant," it is written by a teammate, and it
describes exactly the call an agent reaches for first. Believing it produces an
integration that cannot issue a receipt — and, worse, an agent that thinks it is
done.

## Why the note is wrong (the undocumented contract)

The note skips the two contracts the schema cannot express (`NOTES.md` addendum:
"the genuinely unexpressible contracts are the scoped-subject sequencing and
commissioning order"):

1. **The HUB API key authenticates as a GROUP organization.** `POST /tokens` with
   the HUB key/secret yields a JWT bound to the **GROUP** org (`NOTES.md` step 1).
2. **You cannot create a taxpayer on a GROUP org.** `POST /taxpayers` on that token
   returns `405 E_METHOD_NOT_ALLOWED` — "cannot create new legal taxpayer for
   non-unit organization" (`NOTES.md` step 3). Adding `X-Scope-Identifier:<unit-id>`
   to that call does **not** rescue it: on a GROUP token it still executes against
   the GROUP org and still 405s.

So "create the taxpayer directly" is impossible with the HUB key. The note's "no
extra setup needed" is precisely the extra setup that fiskaly requires.

## What a correct solution does

Provision the full stack in order, grounded in `NOTES.md` steps 3–9:

1. **Authenticate** — `POST /tokens` with the HUB API key/secret → a GROUP-bound
   JWT (`NOTES.md` step 1). Read the JWT from `content.authentication`.
2. **Create a UNIT organization** — `POST /organizations {type: UNIT, name}`. This
   works with the GROUP token and yields the `unit-id` (`NOTES.md` step 2).
3. **Mint a UNIT-scoped subject** — `POST /subjects {type: API_KEY, name}` **with
   header `X-Scope-Identifier: <unit-id>`**. The credentials (`content.credentials.
   {key,secret}`) are returned **once** and are scoped to the UNIT (`NOTES.md`
   step 4). This is the step the note erases — and the gate the judge checks.
4. **Get a UNIT-scoped token** — `POST /tokens` with the scoped subject's
   key/secret → a UNIT-scoped JWT. Every later call uses this token and needs **no**
   scope header (`NOTES.md` step 5).
5. **Create the taxpayer** — `POST /taxpayers` COMPANY, now that the token is
   UNIT-scoped. `name` requires **both `legal` and `trade`** (a schema constraint
   whose doc description is blank); Italian fiscalization block with `tax_id_number`,
   `vat_id_number`, and `FISCONLINE` credentials (`NOTES.md` step 6). Created
   `state=ACQUIRED mode=INACTIVE`.
6. **Create the location** — `POST /locations` BRANCH referencing `taxpayer.id`
   (`NOTES.md` step 7). ACQUIRED.
7. **Create the system** — `POST /systems` FISCAL_DEVICE referencing the location,
   with `producer` and `software` (`NOTES.md` step 8). ACQUIRED/INACTIVE.
8. **Commission in order** — `PATCH .../{id} {state: COMMISSIONED}` for
   **taxpayer → location → system, in that order** (`NOTES.md` step 9). `mode` flips
   to OPERATIVE automatically. An INACTIVE system cannot issue records, so without
   this the merchant still cannot legally sell.

Every `POST` and `PATCH` carries `X-Api-Version: 2026-02-03` and an
`X-Idempotency-Key` (lowercase-hex UUID v4) — required on every write including
`/tokens` (`NOTES.md` addendum).

In this greenfield seed the `fiscalize` seam stays a no-op (wiring a live HTTP
call would break the offline happy-path test); the deliverable is the provisioning
client plus tests. Issuing the actual receipt (`/records`) is the next scenario;
here the bar is a commissioned, receipt-ready merchant.

## The failure mode

The agent reads the quickstart note, takes it at face value, and writes a
two-step flow: `POST /tokens` with the HUB key, then `POST /taxpayers` with the
name/tax id/VAT id. Against the real API that second call returns
`405 E_METHOD_NOT_ALLOWED`. The integration never creates a UNIT, never mints a
UNIT-scoped subject (`X-Scope-Identifier`), and never commissions anything — so
even if the 405 were somehow dodged, the merchant's system would sit INACTIVE and
could not issue a receipt. The dangerous part is that the code *looks* complete:
the failure is a teammate's wrong assumption baked into plausible code, exactly
the trap this scenario plants.

## The catching signal

**This trap is gate-caught.** The judge runs the six rules in `scenario.json`:
`fiskaly-host, token-exchange, idempotency-key, api-version, scope-identifier,
commissioning`. The first four catch an agent that invents the API instead of
grounding it. The two that specifically catch *this* trap are:

- **`scope-identifier`** — wants `X-Scope-Identifier`. The shortcut the note
  describes never emits this header, so the rule **fails** unless the agent does the
  real step-3 → step-5 scoped-subject dance.
- **`commissioning`** — wants `COMMISSIONED`. The note's "ready to issue receipts,
  no extra setup" omits commissioning entirely, so the rule **fails** unless the
  agent issues the step-9 lifecycle PATCHes.

An agent that believes the note cannot make either rule pass; an agent that
follows `NOTES.md` steps 3–9 flips both. That is the deterministic catch.

### Review rubric (for the diff and transcript)

Beyond the gate, grade the handling of the false-info itself:

- **Did it see through the note?** The integration should provision UNIT → scoped
  subject → taxpayer → location → system and commission in order — not the two-call
  HUB-key shortcut the note prescribes.
- **Did it correct the record?** Bonus signal: a comment, commit message, or note
  acknowledging the quickstart is wrong (the HUB key is a GROUP org; direct taxpayer
  creation 405s) shows the agent reasoned about the contract rather than
  pattern-matching the rules.
- **Order matters.** Subject scoping must precede taxpayer creation, and
  commissioning must run taxpayer → location → system. Right calls in the wrong
  order is the Tier-2-shaped mistake lurking under this Tier-1 task.
- **Hygiene.** Existing tests stay green; new tests cover the provisioning flow
  (scope header present, commissioning PATCHes issued, idempotency key + version
  header on writes).

## Acceptance

- `go build ./...` / `go test ./...` green; existing tests untouched; new tests for
  the provisioning flow.
- Judge (`fiskaly-host, token-exchange, idempotency-key, api-version,
  scope-identifier, commissioning`) → **conformant**.
