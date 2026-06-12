#!/usr/bin/env python3
"""End-to-end probe of the fiskaly SIGN IT TEST API (2026-02-03).

Walks: token -> UNIT org -> taxpayer (IT fiscalization, dummy Fisconline)
-> location -> system -> commission all -> Record INTENTION -> Record
TRANSACTION (RECEIPT) -> poll record state.

Saves a redacted transcript of every request/response to
research/api-probes/transcript.json for use as ground truth when building
the Go client. Reads credentials from .env; never prints or stores secrets.
"""
import json
import time
import urllib.request
import urllib.error
import uuid
import sys
import os

BASE = "https://test.api.fiskaly.com"
API_VERSION = "2026-02-03"
HERE = os.path.dirname(os.path.abspath(__file__))

# --- load .env -------------------------------------------------------------
env = {}
with open(os.path.join(HERE, "../../.env")) as f:
    for line in f:
        line = line.strip()
        if line and not line.startswith("#") and "=" in line:
            k, v = line.split("=", 1)
            env[k.strip()] = v.strip().strip('"').strip("'")

KEY, SECRET = env["FISKALY_API_KEY"], env["FISKALY_API_SECRET"]

transcript = []
state = {}
bearer = None


def redact(obj):
    if isinstance(obj, dict):
        out = {}
        for k, v in obj.items():
            if k in ("bearer", "secret", "password", "pin", "key") and isinstance(v, str):
                out[k] = f"<redacted:{len(v)} chars>"
            else:
                out[k] = redact(v)
        return out
    if isinstance(obj, list):
        return [redact(x) for x in obj]
    return obj


def call(step, method, path, body=None, scope=None, expect=(200, 201)):
    global bearer
    url = BASE + path
    headers = {
        "Content-Type": "application/json",
        "X-Api-Version": API_VERSION,
    }
    if method in ("POST", "PATCH"):
        headers["X-Idempotency-Key"] = str(uuid.uuid4())
    if bearer and path != "/tokens":
        headers["Authorization"] = f"Bearer {bearer}"
    if scope:
        headers["X-Scope-Identifier"] = scope

    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    started = time.time()
    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            status = resp.status
            payload = json.loads(resp.read().decode() or "{}")
            resp_headers = dict(resp.headers)
    except urllib.error.HTTPError as e:
        status = e.code
        try:
            payload = json.loads(e.read().decode() or "{}")
        except Exception:
            payload = {"raw": "<unparseable>"}
        resp_headers = dict(e.headers)
    ms = int((time.time() - started) * 1000)

    entry = {
        "step": step,
        "request": {"method": method, "path": path, "scope_header": scope, "body": redact(body)},
        "response": {
            "status": status,
            "ms": ms,
            "headers": {k: v for k, v in resp_headers.items() if k.lower().startswith("x-") or k.lower() == "retry-after"},
            "body": redact(payload),
        },
    }
    transcript.append(entry)
    ok = status in expect
    print(f"{'OK ' if ok else 'ERR'} [{status}] {method} {path} ({ms}ms) — {step}")
    if not ok:
        print(json.dumps(redact(payload), indent=2)[:1500])
    return status, payload


def save():
    with open(os.path.join(HERE, "transcript.json"), "w") as f:
        json.dump(transcript, f, indent=2)
    with open(os.path.join(HERE, "state.json"), "w") as f:
        json.dump(state, f, indent=2)


def fail(msg):
    print(f"\nFAILED: {msg}")
    save()
    sys.exit(1)


# --- 1. token ---------------------------------------------------------------
status, tok = call("auth", "POST", "/tokens", {
    "content": {"type": "API_KEY", "key": KEY, "secret": SECRET},
})
if status != 200:
    fail("auth")
bearer = tok["content"]["authentication"]["bearer"]
state["group_organization_id"] = tok["content"]["organization"]["id"]
state["root_subject_id"] = tok["content"]["subject"]["id"]

# --- 2. UNIT organization ----------------------------------------------------
status, org = call("create UNIT organization", "POST", "/organizations", {
    "content": {"type": "UNIT", "name": "Trattoria Da Mario (Z2R probe)"},
})
if status not in (200, 201):
    fail("organization create")
unit_id = org["content"]["id"]
state["unit_organization_id"] = unit_id

# --- 3. taxpayer (try X-Scope-Identifier shortcut with GROUP token) ----------
taxpayer_body = {
    "content": {
        "type": "COMPANY",
        "name": {"legal": "Trattoria Da Mario S.r.l.", "trade": "Trattoria Da Mario"},
        "address": {
            "line": {"type": "STREET_NUMBER", "street": "Via Roma", "number": "42"},
            "code": "20121",
            "city": "Milano",
            "country": "IT",
        },
        "fiscalization": {
            "type": "IT",
            "tax_id_number": "12345678903",
            "vat_id_number": "12345678903",
            "credentials": {
                "type": "FISCONLINE",
                "pin": "1234567890",
                "password": "ProbePassword1!",
                "tax_id_number": "RSSMRA85M01H501Z",
            },
        },
    },
}
status, tp = call("create taxpayer (scoped to UNIT)", "POST", "/taxpayers", taxpayer_body, scope=unit_id)

if status in (401, 403, 405):
    # Fallback: create a subject (API key) scoped to the UNIT, mint a new token.
    print(">> scope shortcut rejected; falling back to scoped subject flow")
    status, sub = call("create scoped subject", "POST", "/subjects", {
        "content": {"type": "API_KEY", "name": "z2r-probe"},
    }, scope=unit_id)
    if status not in (200, 201):
        fail("subject create")
    creds = sub["content"].get("credentials") or {}
    sub_key = creds.get("key")
    sub_secret = creds.get("secret")
    if not (sub_key and sub_secret):
        fail(f"subject response had no key/secret: fields={list(sub['content'].keys())}")
    status, tok2 = call("auth as scoped subject", "POST", "/tokens", {
        "content": {"type": "API_KEY", "key": sub_key, "secret": sub_secret},
    })
    if status != 200:
        fail("scoped auth")
    bearer = tok2["content"]["authentication"]["bearer"]
    status, tp = call("create taxpayer (as scoped subject)", "POST", "/taxpayers", taxpayer_body)

if status not in (200, 201):
    fail("taxpayer create")
taxpayer_id = tp["content"]["id"]
state["taxpayer_id"] = taxpayer_id
print(f"   taxpayer state={tp['content'].get('state')} mode={tp['content'].get('mode')} locations={tp['content'].get('locations')}")

USE_SCOPE = transcript[-1]["request"]["scope_header"]  # carries forward whichever path worked

# --- 4. location --------------------------------------------------------------
status, loc = call("create BRANCH location", "POST", "/locations", {
    "content": {
        "type": "BRANCH",
        "taxpayer": {"id": taxpayer_id},
        "name": "Milano Centro",
        "address": {
            "line": {"type": "STREET_NUMBER", "street": "Via Roma", "number": "42"},
            "code": "20121",
            "city": "Milano",
            "country": "IT",
        },
    },
}, scope=USE_SCOPE)
if status not in (200, 201):
    fail("location create")
location_id = loc["content"]["id"]
state["location_id"] = location_id

# --- 5. system -----------------------------------------------------------------
status, sysr = call("create FISCAL_DEVICE system", "POST", "/systems", {
    "content": {
        "type": "FISCAL_DEVICE",
        "location": {"id": location_id},
        "producer": {"type": "MPN", "number": "Z2R-POS-001", "details": {"name": "Zero-to-Receipt Probe POS"}},
        "software": {"name": "z2r-probe", "version": "0.1.0"},
    },
}, scope=USE_SCOPE)
if status not in (200, 201):
    fail("system create")
system_id = sysr["content"]["id"]
state["system_id"] = system_id
print(f"   system state={sysr['content'].get('state')} mode={sysr['content'].get('mode')}")

# --- 6. commission taxpayer -> location -> system --------------------------------
for name, path, rid in [
    ("taxpayer", "/taxpayers", taxpayer_id),
    ("location", "/locations", location_id),
    ("system", "/systems", system_id),
]:
    status, r = call(f"commission {name}", "PATCH", f"{path}/{rid}", {
        "content": {"state": "COMMISSIONED"},
    }, scope=USE_SCOPE)
    if status not in (200, 201):
        fail(f"commission {name}")
    print(f"   {name}: state={r['content'].get('state')} mode={r['content'].get('mode')}")

# --- 7. record INTENTION ----------------------------------------------------------
status, intent = call("record INTENTION", "POST", "/records", {
    "content": {
        "type": "INTENTION",
        "system": {"id": system_id},
        "operation": {"type": "TRANSACTION"},
    },
}, scope=USE_SCOPE)
if status not in (200, 201):
    fail("intention")
intention_id = intent["content"]["id"]
state["intention_id"] = intention_id
print(f"   intention state={intent['content'].get('state')} mode={intent['content'].get('mode')}")

# --- 8. record TRANSACTION (RECEIPT) -----------------------------------------------
# Clean VAT math: one item, gross 12.20, net 10.00, 22% standard Italian VAT.
status, txn = call("record TRANSACTION (RECEIPT)", "POST", "/records", {
    "content": {
        "type": "TRANSACTION",
        "record": {"id": intention_id},
        "operation": {
            "type": "RECEIPT",
            "document": {
                "number": "1",
                "total_vat": {"amount": "2.20", "exclusive": "10.00", "inclusive": "12.20"},
            },
            "entries": [
                {
                    "type": "SALE",
                    "data": {
                        "type": "ITEM",
                        "text": "Menu del giorno",
                        "unit": {"quantity": "1", "price": "12.20"},
                        "value": {"base": "12.20"},
                        "vat": {
                            "type": "VAT_RATE",
                            "code": "STANDARD",
                            "percentage": "22.00",
                            "amount": "2.20",
                            "exclusive": "10.00",
                            "inclusive": "12.20",
                        },
                    },
                    "details": {"concept": "GOOD"},
                }
            ],
            "payments": [
                {"type": "CASH", "details": {"amount": "12.20", "currency": "EUR"}}
            ],
        },
    },
}, scope=USE_SCOPE)
if status not in (200, 201):
    fail("transaction")
txn_id = txn["content"]["id"]
state["transaction_id"] = txn_id
print(f"   transaction state={txn['content'].get('state')} mode={txn['content'].get('mode')}")

# --- 9. poll record until terminal --------------------------------------------------
for i in range(20):
    status, rec = call(f"poll record ({i})", "GET", f"/records/{txn_id}", scope=USE_SCOPE)
    st, mode = rec["content"].get("state"), rec["content"].get("mode")
    print(f"   poll: state={st} mode={mode}")
    if st in ("COMPLETED", "FAILED", "REJECTED") and mode == "FINISHED":
        compliance = rec["content"].get("compliance")
        print(f"\n=== TERMINAL: {st} ===")
        print("compliance:", json.dumps(redact(compliance), indent=2)[:2000] if compliance else None)
        break
    time.sleep(3)

save()
print(f"\nTranscript: {len(transcript)} calls saved to research/api-probes/transcript.json")
