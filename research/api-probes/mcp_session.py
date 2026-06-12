#!/usr/bin/env python3
"""Scripted MCP stdio session against z2r-mcp: handshake, tools/list, then
the full agent flow (context -> provision -> receipt -> audit)."""
import json
import subprocess
import sys
import threading

proc = subprocess.Popen(
    ["go", "run", "./cmd/z2r-mcp"],
    stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True,
)

def pump_stderr():
    for line in proc.stderr:
        print(f"  [server] {line.rstrip()}", file=sys.stderr)

threading.Thread(target=pump_stderr, daemon=True).start()

seq = 0
def rpc(method, params=None, notify=False):
    global seq
    msg = {"jsonrpc": "2.0", "method": method}
    if params is not None:
        msg["params"] = params
    if not notify:
        seq += 1
        msg["id"] = seq
    proc.stdin.write(json.dumps(msg) + "\n")
    proc.stdin.flush()
    if notify:
        return None
    while True:
        line = proc.stdout.readline()
        if not line:
            raise RuntimeError("server closed stdout")
        resp = json.loads(line)
        if resp.get("id") == seq:
            if "error" in resp:
                raise RuntimeError(f"{method}: {resp['error']}")
            return resp["result"]

def call_tool(name, args):
    res = rpc("tools/call", {"name": name, "arguments": args})
    if res.get("isError"):
        raise RuntimeError(f"{name} failed: {json.dumps(res)[:800]}")
    return res

init = rpc("initialize", {
    "protocolVersion": "2025-06-18",
    "capabilities": {},
    "clientInfo": {"name": "z2r-session-test", "version": "0.0.1"},
})
print(f"server: {init['serverInfo']['name']} {init['serverInfo']['version']} (protocol {init['protocolVersion']})")
rpc("notifications/initialized", notify=True)

tools = rpc("tools/list")["tools"]
print(f"tools ({len(tools)}): {', '.join(t['name'] for t in tools)}")

brief = call_tool("get_integration_context", {})
print(f"\nintegration brief: {len(brief['content'][0]['text'])} chars, starts: {brief['content'][0]['text'][:60]!r}")

sandbox = call_tool("provision_sandbox", {"name": "Trattoria Da Mario", "city": "Milano"})
sc = sandbox["structuredContent"]
print(f"\nsandbox: {sc['sandbox_id']} system={sc['system_id']}")

receipt = call_tool("issue_receipt", {
    "sandbox_id": sc["sandbox_id"],
    "items": [
        {"text": "Spaghetti alle vongole", "gross": "14.50"},
        {"text": "Tiramisù", "gross": "6.00"},
        {"text": "Caffè", "gross": "1.20", "vat_code": "REDUCED_1"},
    ],
})
rc = receipt["structuredContent"]
print(f"receipt: doc#{rc['document_number']} state={rc['state']} total={rc['total_gross']} (net {rc['total_net']} + VAT {rc['total_vat']})")
print(f"AdE: {rc.get('ade_reference')}")

cancel = call_tool("cancel_receipt", {"sandbox_id": sc["sandbox_id"], "record_id": rc["record_id"]})
cc = cancel["structuredContent"]
print(f"cancellation: state={cc['state']} ade={cc.get('ade_reference')}")

auditres = call_tool("audit_session", {})
ac = auditres["structuredContent"]
print(f"\naudit verdict: {ac['verdict']}")
for a in ac["audits"]:
    r = a["report"]
    print(f"  {a['sandbox_id']} ({a['merchant']}): {r['calls_audited']} calls, "
          f"{len(r.get('findings') or [])} findings, passed: {', '.join(r.get('passed_rules') or [])}")
    for f in (r.get("findings") or []):
        print(f"    [{f['severity']}] {f['rule']}: {f['detail']}")

proc.stdin.close()
proc.wait(timeout=10)
print("\nMCP SESSION OK")
