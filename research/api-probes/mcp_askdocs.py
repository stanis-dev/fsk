#!/usr/bin/env python3
"""Verify ask_fiskaly_docs through the MCP stdio interface end to end."""
import json, subprocess, sys, threading

proc = subprocess.Popen(["go", "run", "./cmd/z2r-mcp"],
    stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
threading.Thread(target=lambda: [print(f"  [srv] {l.rstrip()}", file=sys.stderr) for l in proc.stderr], daemon=True).start()

seq = 0
def rpc(method, params=None, notify=False):
    global seq
    msg = {"jsonrpc": "2.0", "method": method}
    if params is not None: msg["params"] = params
    if not notify:
        seq += 1; msg["id"] = seq
    proc.stdin.write(json.dumps(msg) + "\n"); proc.stdin.flush()
    if notify: return
    while True:
        resp = json.loads(proc.stdout.readline())
        if resp.get("id") == seq:
            if "error" in resp: raise RuntimeError(resp["error"])
            return resp["result"]

rpc("initialize", {"protocolVersion": "2025-06-18", "capabilities": {}, "clientInfo": {"name": "askdocs-test", "version": "0"}})
rpc("notifications/initialized", notify=True)
tools = [t["name"] for t in rpc("tools/list")["tools"]]
print("tools:", ", ".join(tools))
assert "ask_fiskaly_docs" in tools, "tool not registered!"

res = rpc("tools/call", {"name": "ask_fiskaly_docs", "arguments": {
    "question": "What does the X-Idempotency-Key header do and is it required on PATCH?",
    "product": "SIGN_IT"}})
sc = res["structuredContent"]
print("\ngrounded:", sc.get("grounded"))
print("answer:", (sc.get("answer") or "")[:400])
print("#citations:", len(sc.get("citations") or []))
for c in (sc.get("citations") or [])[:4]:
    print("   -", c.get("title"), "→", c.get("url"))
print("follow_ups:", sc.get("follow_ups"))
print("note:", sc.get("note"))

proc.stdin.close(); proc.wait(timeout=10)
print("\nOK")
