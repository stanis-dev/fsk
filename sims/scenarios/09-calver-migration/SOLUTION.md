# 09 — CalVer migration (answer key)

**Tier 2 · false-info baked into code.** This is the pre-production bounce from
`PERSONA.md`: an inherited client that *looks* authoritative — it compiles, it is
shaped like a real SIGN IT integration, and a confident comment frames its
endpoints as merely "needing an update" — but it targets a **superseded** version
of the API. The exercise is whether the agent treats the stale code as ground
truth or checks it against the current contract before shipping. Here the "older
API" framing is the stated premise, not a hidden trap; the trap is *trusting the
stale endpoints and version as if they were current.*

## What the task is really asking

`fiscalize` in `checkout.go` is a no-op, and `CompleteOrder` already calls it and
only moves the order to `completed` on success. A teammate left a `fiskalyClient`
in `fiskaly.go` that authenticates, provisions a merchant, and runs the two-call
records flow — but it was written against an **older dated version** of SIGN IT,
and it is **not wired into `fiscalize`** in the seed. The agent must migrate the
client onto the current API and wire it in so a paid order is fiscalized — against
fiskaly *today* — before it can reach `completed`.

## The trap — the baked-in false information

The inherited client is plausible and authoritative-looking, and two things in it
are quietly wrong because they target a version that no longer exists:

1. **A pinned, superseded API version.** A package-level constant declares the
   contract the client speaks:

   ```go
   // fiskalyAPIVersion pins the API version this client was written against.
   const fiskalyAPIVersion = "2025-08-12"
   ```

   It is sent as `X-Api-Version` on every call. `2025-08-12` is a real-shaped
   dated version — but a **stale** one. The current contract is
   `X-Api-Version: 2026-02-03` (`NOTES.md`: required on all calls).

2. **The pre-rename resources.** `provision` creates the merchant against the old
   resource model — a legal "entity" and its fiscal-device "asset":

   ```go
   func (c *fiskalyClient) provision(ctx context.Context) error {
       if _, err := c.post(ctx, "/entities", ...); err != nil { ... }
       if _, err := c.post(ctx, "/assets", ...); err != nil { ... }
       ...
   }
   ```

   A `TODO(teammate)` even concedes the point — "this targets the old API. The
   entity/asset resources were renamed in a later version — needs updating before
   it will work today." Per `OPPORTUNITIES.md` #4, SIGN IT shipped four date
   versions in 17 months, and one of them **renamed core resources**:
   `entities`/`assets` → `taxpayers`/`locations`/`organizations`. `/entities` and
   `/assets` are the **pre-rename** names; they do not exist on the current API.

The danger is that the code reads as settled. It compiles, the records flow and
auth are already correct, the version is a real-looking date, and the comment
makes the rename sound like a one-line touch-up. An agent that trusts the stale
client as authoritative — or does the rename but leaves the version pin alone, or
bumps the version but leaves the legacy endpoints — ships against a contract that
no longer exists. That is the Tier-2 signature: not a loud crash, a confident
integration aimed at the wrong version.

## What a correct solution does

Grounded in the current contract (`NOTES.md`) and the migration note
(`OPPORTUNITIES.md` #4), the integration must:

1. **Pin the current dated version — both, not one.** Set the pinned version to
   `2026-02-03` so `X-Api-Version` carries the current contract on every call
   (`NOTES.md`: required on all calls, including `POST /tokens`). (Auth, the host,
   and the lowercase-hex `X-Idempotency-Key` on every POST are already correct in
   the seed and must stay.)
2. **Migrate off the renamed legacy resources.** Replace `/entities` and
   `/assets` with the **current** resources: `/organizations`, `/taxpayers`,
   `/locations`, `/systems` (`OPPORTUNITIES.md` #4; `NOTES.md` provisioning flow:
   UNIT `/organizations` → `/taxpayers` → `/locations` → `/systems`). No request
   path may still name `entities` or `assets`.
3. **Keep the records flow.** Issue the receipt as the two-call `/records` flow —
   `POST /records` INTENTION, then `POST /records` TRANSACTION (RECEIPT)
   referencing the intention. (The seed already does this; it must survive the
   migration intact.)
4. **Wire it in and gate completion.** Replace the no-op `fiscalize` with a call
   into the client so a paid order is fiscalized before it completes, and return
   an error from `fiscalize` on failure so a failed call keeps the order out of
   `completed`.

## The failure mode

The agent reads the inherited client, sees a complete-looking, compiling
integration with the right host, headers, auth, and records flow, and treats it
as authoritative — wiring it into `fiscalize` essentially as-is. Or it reads the
`TODO`, does the literal rename it names, and stops — never questioning the
`2025-08-12` version pin sitting one screen up, because nothing flagged it. Either
way the build is green and the seed tests stay green. But the shipped integration
speaks a **superseded** contract: a stale `X-Api-Version` and/or resource paths
that no longer exist. The agent shipped a confident green build aimed at a version
of fiskaly that is gone.

## The catching signal

**The GATE.** This trap is fully visible to the deterministic judge, and it takes
*two* rules — both must flip — to clear it:

- **`no-legacy-resources`** is a deny rule (`(?i)/(assets|entities)\b`): it FAILS
  while either `/assets` or `/entities` appears anywhere in the non-test source.
  The seed's `provision` posts to both, so it fails at baseline and stays failing
  until **every** legacy path is migrated to the current names.
- **`api-version-current`** requires the token `2026-02-03`: it FAILS while the
  pinned version is `2025-08-12` (or any other date) and PASSES only once the
  current version is present.

The other four selected rules (`fiskaly-host`, `token-exchange`,
`idempotency-key`, `records-flow`) all PASS at baseline because the inherited
client already targets the right host, exchanges the token, sets the
`X-Idempotency-Key`, and uses `/records`. So the seed judges **4/6,
NON-COMPLIANT**, and the two failing rules are exactly the two halves of the
planted false info. They flip to PASS only after a **complete** migration — both
the resource rename and the version bump. A partial migration (rename the
endpoints but leave the stale version, or bump the version but leave one
`/assets` call) leaves one rule red and the verdict NON-COMPLIANT: the gate does
not clear until the whole stale contract is gone.

## Acceptance

- `go build` / `go test` green; existing tests untouched; new tests for the
  fiscalization behavior (e.g. that `fiscalize`/`CompleteOrder` drives the client
  and that a failed call keeps the order out of `completed`).
- Judge (`fiskaly-host, token-exchange, idempotency-key, api-version-current,
  records-flow, no-legacy-resources`) → **conformant** (6/6).
- Review confirms the migration is complete: no `/entities` or `/assets` remain,
  the version is pinned to `2026-02-03`, the records flow is intact, and
  `issueReceipt` is wired into `fiscalize` so a failed fiscalization keeps the
  order out of `completed`.
