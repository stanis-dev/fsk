#!/usr/bin/env python3
"""Generate report/research-report.html — a 3-tab review of the initial
research and the decision that produced the Zero-to-Receipt prototype.

Tab 1  Progress at a glance        (build status + evidence, with research links)
Tab 2  The decision               (prompt -> research -> options -> proposal)
Tab 3  Full context & research     (exercise brief + all 6 research areas + opportunity map)

Reproducible: reads research/fiskaly_research.json. Run from repo root:
    python3 report/build_report.py
"""
import html
import json
import os
import re

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
RESEARCH = json.load(open(os.path.join(ROOT, "research/fiskaly_research.json")))

# --- lightweight markdown -> HTML ------------------------------------------

def _inline(text):
    text = html.escape(text)
    text = re.sub(r"\[([^\]]+)\]\((https?://[^)]+)\)",
                  r'<a href="\2" target="_blank" rel="noopener">\1</a>', text)
    text = re.sub(r"\*\*([^*]+)\*\*", r"<strong>\1</strong>", text)
    text = re.sub(r"`([^`]+)`", r"<code>\1</code>", text)
    return text


def markdown(md):
    """Render the subset of markdown the research notes use."""
    lines = md.split("\n")
    out, i = [], 0
    list_open = None  # 'ul' | 'ol' | None

    def close_list():
        nonlocal list_open
        if list_open:
            out.append(f"</{list_open}>")
            list_open = None

    while i < len(lines):
        line = lines[i]
        stripped = line.strip()

        if stripped.startswith("```"):
            close_list()
            i += 1
            buf = []
            while i < len(lines) and not lines[i].strip().startswith("```"):
                buf.append(html.escape(lines[i]))
                i += 1
            out.append("<pre class='code'>" + "\n".join(buf) + "</pre>")
            i += 1
            continue

        # markdown table: header row followed by a |---| separator
        if "|" in line and i + 1 < len(lines) and re.match(r"^\s*\|?[\s:|-]+\|[\s:|-]*$", lines[i + 1]):
            close_list()
            header = [c.strip() for c in line.strip().strip("|").split("|")]
            out.append("<table class='facts'><thead><tr>" +
                       "".join(f"<th>{_inline(c)}</th>" for c in header) +
                       "</tr></thead><tbody>")
            i += 2
            while i < len(lines) and "|" in lines[i]:
                cells = [c.strip() for c in lines[i].strip().strip("|").split("|")]
                out.append("<tr>" + "".join(f"<td>{_inline(c)}</td>" for c in cells) + "</tr>")
                i += 1
            out.append("</tbody></table>")
            continue

        m = re.match(r"^(#{2,5})\s+(.*)", stripped)
        if m:
            close_list()
            level = min(len(m.group(1)) + 1, 6)
            out.append(f"<h{level}>{_inline(m.group(2))}</h{level}>")
            i += 1
            continue

        m = re.match(r"^[-*]\s+(.*)", stripped)
        if m:
            if list_open != "ul":
                close_list()
                out.append("<ul>")
                list_open = "ul"
            out.append(f"<li>{_inline(m.group(1))}</li>")
            i += 1
            continue

        m = re.match(r"^\d+\.\s+(.*)", stripped)
        if m:
            if list_open != "ol":
                close_list()
                out.append("<ol>")
                list_open = "ol"
            out.append(f"<li>{_inline(m.group(1))}</li>")
            i += 1
            continue

        if not stripped:
            close_list()
            i += 1
            continue

        close_list()
        out.append(f"<p>{_inline(stripped)}</p>")
        i += 1

    close_list()
    return "\n".join(out)


# --- static content (known from the working session) -----------------------

AREA_META = {
    "company":       ("fiskaly — company & strategy", "Who they are, the portfolio, the AI-First initiative this role belongs to."),
    "italy-context": ("Italian fiscalization landscape", "Why SIGN IT is timely: the 2027 software-fiscalization window and the sanctions that price non-compliance."),
    "sign-it-api":   ("SIGN IT API & docs audit", "The API mapped first-hand from the spec, and an honest read of its documentation."),
    "docs-platform": ("Documentation platforms (old + new)", "What fiskaly already shipped — and the action-layer gap we must not duplicate."),
    "github-sdks":   ("SDK & open-source footprint", "The integrator tooling gap: archived SDKs, specs only on the docs site."),
    "dx-benchmark":  ("API developer-experience benchmark (2026)", "The frontier we measured fiskaly against: action-MCP servers, Agent Skills, llms.txt."),
}
AREA_ORDER = ["company", "italy-context", "sign-it-api", "docs-platform", "github-sdks", "dx-benchmark"]

# tab 1 — build status
TASKS = [
    ("done", "Reconnaissance & fact-checked research synthesis",
     "Fetched the job posting and SIGN IT docs, extracted the client-rendered OpenAPI spec from the JS bundle, ran a 6-agent research workflow (gather → adversarial verify → 4 ideation lenses).",
     [("Synthesis", "../research/RESEARCH.md"), ("Raw research JSON", "../research/fiskaly_research.json")]),
    ("done", "Live SIGN IT TEST API probe — full happy path",
     "Walked token → UNIT org → scoped subject → taxpayer → location → system → commission → INTENTION → TRANSACTION → COMPLETED with a real AdE document reference. Captured a redacted transcript and the API behaviours the docs omit.",
     [("Probe notes", "../research/api-probes/NOTES.md"), ("Transcript", "../research/api-probes/transcript.json"), ("Probe script", "../research/api-probes/probe.py")]),
    ("done", "Go client layer",
     "Typed SIGN IT client (token lifecycle, X-Api-Version, per-attempt idempotency keys, typed error envelope, audit-trail hook), ProvisionStack, BuildReceipt (gross prices → full VAT breakdown), IssueReceipt. Unit tests for money maths and receipt totals.",
     [("client.go", "../internal/fiskaly/client.go"), ("flow.go", "../internal/fiskaly/flow.go")]),
    ("done", "Action-MCP server (6 tools) + deterministic judge",
     "provision_sandbox, issue_receipt, get_record, cancel_receipt, get_integration_context, audit_session. Credentials stay server-side (agents hold opaque sandbox ids); LIVE hosts rejected by construction. Judge = 6 compliance rules with regulation citations.",
     [("tools.go", "../internal/mcpserver/tools.go"), ("rules.go", "../internal/audit/rules.go")]),
    ("done", "Simulator with fault injection + bad-POS conviction",
     "Local SIGN IT stand-in (happy / ade-outage / slow-ade) faithful to probed behaviours; a deliberately sloppy POS the judge convicts with 3 violations + 1 warning and exit code 1 — a CI gate.",
     [("sim.go", "../internal/sim/sim.go"), ("z2r-badpos", "../cmd/z2r-badpos/main.go")]),
    ("in_progress", "Packaging: skill, README, opportunity memo, demo script",
     "Claude Code agent skill (with judge persona), README, the opportunity memo that answers the exercise, and an 8-minute demo script with Q&A prep.",
     [("Opportunity memo", "../memo/OPPORTUNITIES.md"), ("Demo script", "../memo/DEMO.md"), ("Agent skill", "../.claude/skills/zero-to-receipt/SKILL.md")]),
    ("pending", "Claimable-sandbox HTTP factory",
     "The provisioning logic exists; the web-facing 'claim into HUB' flow (Stripe's start-before-signup move) is not built yet.",
     []),
    ("pending", "Judge: externalize rules to rules.yaml + optional LLM audit layer",
     "Today the rules are deterministic Go. Externalising them to YAML and layering a citation-gated LLM judge on top is planned.",
     []),
    ("pending", "GitHub push",
     "Awaiting your call on repository name and visibility before anything goes public.",
     []),
    ("pending", "Live Claude Code demo rehearsal",
     "Walk the full storyline end-to-end inside Claude Code and time it.",
     []),
]

EVIDENCE = [
    ("z2r-smoke (real TEST API)", "Receipt COMPLETED with AdE reference in ~3s", "../cmd/z2r-smoke/main.go"),
    ("Scripted MCP session", "6 tools listed; receipt + cancellation COMPLETED; audit verdict PASS", "../research/api-probes/mcp_session.py"),
    ("z2r-badpos under ade-outage", "Judge verdict FAIL — 3 violations + 1 warning, exit code 1", "../cmd/z2r-badpos/main.go"),
    ("go test ./...", "Green — money maths, receipt totals, slug rules", "../internal/fiskaly/money_test.go"),
]

# tab 2 — decision path
DECISION_STEPS = [
    ("Your prompt",
     "You sent the job posting and the exercise: <em>“What opportunities do you see to drive fiskaly's mission to the next level? What improvements to the API documentation bring value to customers? Go crazy. For one opportunity, build a functional prototype.”</em> Plus a clear instruction: gather all context, surface questions, and reach alignment before building.",
     None),
    ("Reconnaissance",
     "Fetched the two URLs. The SIGN IT reference turned out to be client-rendered (the raw HTML just says “Loading…”), so I dug the real OpenAPI spec out of the JS bundle — <code>live.unified.fiskaly.com/&lt;hash&gt;/en/oas.yaml</code> plus per-country overlay files. That one discovery shaped everything: the docs are a build pipeline, not hand-written pages.",
     ("SIGN IT reference", "https://developer.fiskaly.com/api/sign-it/2026-02-03")),
    ("Deep research (6 agents, fact-checked)",
     "Ran a background workflow: six parallel researchers (the API, both docs platforms, the company, the Italian regulatory landscape, the DX benchmark, the GitHub/SDK footprint), each followed by an adversarial fact-checker that tried to <em>refute</em> the load-bearing claims. Then four ideation lenses turned findings into prototype candidates.",
     ("Full research", "research")),
    ("First-hand verification",
     "I didn't trust the docs — I probed. Downloaded the actual specs, diffed the 2025-08-12 → 2026-02-03 versions (a breaking resource rename), and ran a complete receipt against the live TEST API. The probe surfaced four contracts the docs never state (subject-name regex, mandatory trade name, composite record types, PATCH idempotency).",
     ("Probe notes", "research")),
    ("The strategic insight",
     "Everything converged on one read: <strong>fiskaly already built the AI <em>read</em> layer</strong> (llms.txt, CLAUDE.md, a working RAG docs chat) <strong>but the <em>action</em> layer is missing</strong> — their MCP is unpublished, the try-it console is a dead link, every SDK is archived, the spec has zero examples. And their own job ad asks for judge-agents-auditing-coder-agents. The gap and the role pointed the same direction.",
     None),
    ("Four candidates",
     "The lenses produced four buildable options, all in Go, all grounded in a real gap, all echoing the judge-audits-coder theme. Rather than pick for you, I laid them out with honest trade-offs.",
     None),
    ("Options put to you",
     "Four questions: which prototype, how much time, which stack, and sandbox access. I recommended <strong>Sandbox MCP + Judge</strong> as the safe-but-impressive default.",
     None),
    ("Your decision",
     "You chose the most ambitious option — <strong>Zero-to-Receipt</strong> — with a week+ of runway, Go, and a real TEST account. Zero-to-Receipt is a superset of my recommendation: the action-MCP + judge, plus a claimable sandbox factory, a published Agent Skill, and time-to-first-receipt as the framing metric.",
     None),
    ("The proposal we aligned on",
     "A four-part build: (1) a claimable TEST-sandbox factory, (2) a SIGN IT action-MCP server, (3) a compliance Judge over the session trail, (4) an Agent Skill + fault-injecting simulator. Demo: an agent onboards a Milan trattoria and issues a receipt live; then the judge convicts a sloppy POS during an AdE outage.",
     ("Opportunity memo", "memo")),
]

CANDIDATES = [
    ("Sandbox MCP + Judge", "agent-era-dx", "Recommended",
     "Action-taking MCP server over the TEST API (provision, issue receipt, audit), with a judge that replays the session against compliance rules.",
     "Speaks the role's language exactly, fills the unshipped-MCP gap, demos live in Claude Code. The safe-but-impressive pick.",
     "Chosen as the core — Zero-to-Receipt is this plus more."),
    ("Zero-to-Receipt", "time-to-first-receipt", "Chosen",
     "Sandbox MCP + Judge, plus a claimable TEST-sandbox factory and a published Agent Skill — framed around time-to-first-signed-receipt under 5 minutes.",
     "Moves the mission metric (onboarding speed) and demonstrates the whole agentic stack: coder agent, judge agent, human-in-the-loop.",
     "Selected — the most ambitious, and a superset of the recommendation."),
    ("Docs Judge CI", "multi-country-scale", "Considered",
     "A conformance pipeline over the docs supply chain that catches the 168 blank descriptions and drafts PR-ready fixes.",
     "Zero credentials needed; safest possible demo; opens with a finding their team likely doesn't know.",
     "Internal-tooling flavoured — kept as opportunity #3 in the memo."),
    ("CERTIFY harness", "compliance-confidence", "Considered",
     "Planner/Executor/Chaos/Judge agents run compliance scenarios against an integration and emit an audit-style report with sanction exposure.",
     "Strongest compliance-confidence story; the judge persona made executable.",
     "Most moving parts for the timebox — folded into the memo as opportunity #2."),
]

# --- HTML assembly ----------------------------------------------------------

def area_section(area):
    title, sub = AREA_META[area["key"]]
    parts = [f"<section class='area' id='area-{area['key']}'>"]
    parts.append(f"<h3>{html.escape(title)}</h3>")
    parts.append(f"<p class='area-sub'>{html.escape(sub)}</p>")
    parts.append(f"<p class='summary'>{_inline(area['summary'])}</p>")

    parts.append("<h4>Key facts</h4><ul class='facts-list'>")
    for f in area["key_facts"]:
        src = f.get("source", "")
        link = f' <a class="src" href="{html.escape(src)}" target="_blank" rel="noopener">source ↗</a>' if src.startswith("http") else ""
        parts.append(f"<li>{_inline(f['fact'])}{link}</li>")
    parts.append("</ul>")

    checked = (area.get("verification") or {}).get("checked") or []
    corrections = [c for c in checked if c["verdict"] != "confirmed"]
    if corrections:
        parts.append("<div class='callout warn'><strong>Fact-check corrections</strong><ul>")
        for c in corrections:
            parts.append(f"<li><span class='verdict {c['verdict']}'>{c['verdict']}</span> {_inline(c['claim'])}<br><span class='note'>{_inline(c['note'])}</span></li>")
        parts.append("</ul></div>")

    oq = area.get("open_questions") or []
    if oq:
        parts.append("<details class='oq'><summary>Open questions (" + str(len(oq)) + ")</summary><ul>")
        for q in oq:
            parts.append(f"<li>{_inline(q)}</li>")
        parts.append("</ul></details>")

    parts.append("<details class='detail'><summary>Full detail notes</summary><div class='detail-body'>")
    parts.append(markdown(area["details"]))
    parts.append("</div></details>")
    parts.append("</section>")
    return "\n".join(parts)


def opportunity_block(lens):
    res = lens["result"]
    top = res["opportunities"][0]
    parts = [f"<details class='opp'><summary><span class='lens'>{html.escape(lens['lens'])}</span> {html.escape(top['title'])}</summary><div class='opp-body'>"]
    for label, key in [("Problem", "problem"), ("Solution", "solution"), ("Value", "value"),
                       ("Prototype feasibility", "prototype_feasibility"), ("Why it impresses", "wow_factor")]:
        if top.get(key):
            parts.append(f"<p><strong>{label}.</strong> {_inline(top[key])}</p>")
    others = res["opportunities"][1:]
    if others:
        parts.append("<p class='also'><strong>Also surfaced by this lens:</strong> " +
                     "; ".join(html.escape(o["title"]) for o in others) + ".</p>")
    parts.append("</div></details>")
    return "\n".join(parts)


def task_row(status, title, desc, links):
    badge = {"done": "Done", "in_progress": "In progress", "pending": "Pending"}[status]
    link_html = " ".join(f'<a href="{html.escape(u)}" target="_blank" rel="noopener">{html.escape(t)}</a>' for t, u in links)
    return f"""<div class='task {status}'>
      <div class='task-head'><span class='badge {status}'>{badge}</span><span class='task-title'>{html.escape(title)}</span></div>
      <p class='task-desc'>{html.escape(desc)}</p>
      {f"<p class='task-links'>{link_html}</p>" if link_html else ""}
    </div>"""


def decision_step(idx, title, body, link):
    link_html = ""
    if link:
        label, target = link
        href = target if target.startswith("http") else ({"research": "#tab3", "memo": "../memo/OPPORTUNITIES.md"}.get(target, target))
        attr = ' target="_blank" rel="noopener"' if href.startswith("http") or href.endswith(".md") else ' onclick="showTab(\'tab3\');return false;"' if href == "#tab3" else ""
        link_html = f'<p class="step-link"><a href="{html.escape(href)}"{attr}>{html.escape(label)} →</a></p>'
    return f"""<div class='step'>
      <div class='step-num'>{idx}</div>
      <div class='step-content'><h3>{html.escape(title)}</h3><p>{body}</p>{link_html}</div>
    </div>"""


done = sum(1 for t in TASKS if t[0] == "done")
inprog = sum(1 for t in TASKS if t[0] == "in_progress")
pct = round(100 * (done + 0.5 * inprog) / len(TASKS))

areas_by_key = {a["key"]: a for a in RESEARCH["research"]}
lenses_by_key = {l["lens"]: l for l in RESEARCH["ideas"]}

tab1 = f"""
<p class='lead'>A snapshot of the Zero-to-Receipt prototype: what is built and verified, what is in flight, and what remains. Every row links to the relevant research or source.</p>
<div class='progress-wrap'>
  <div class='progress-stat'><span class='big'>{pct}%</span><span class='small'>complete</span></div>
  <div class='progress-bar'><div class='progress-fill' style='width:{pct}%'></div></div>
  <div class='progress-legend'>{done} done · {inprog} in progress · {len(TASKS)-done-inprog} pending</div>
</div>
<h2>Build status</h2>
{"".join(task_row(*t) for t in TASKS)}
<h2>Verification evidence</h2>
<p class='muted'>What actually ran, not what should run.</p>
<table class='evidence'><thead><tr><th>Check</th><th>Result</th><th></th></tr></thead><tbody>
{"".join(f"<tr><td>{html.escape(c)}</td><td>{html.escape(r)}</td><td><a href='{html.escape(u)}' target='_blank' rel='noopener'>file ↗</a></td></tr>" for c,r,u in EVIDENCE)}
</tbody></table>
<div class='callout'><strong>Jump to the research:</strong>
  <a href='../research/RESEARCH.md' target='_blank' rel='noopener'>Synthesis</a> ·
  <a href='../research/fiskaly_research.json' target='_blank' rel='noopener'>Raw JSON</a> ·
  <a href='../research/api-probes/NOTES.md' target='_blank' rel='noopener'>Probe notes</a> ·
  <a href='#tab3' onclick="showTab('tab3');return false;">Full context (tab 3)</a>
</div>
"""

tab2 = f"""
<p class='lead'>How your one prompt became a concrete, aligned build plan — every link in the chain, in order.</p>
<div class='timeline'>
{"".join(decision_step(i+1, *s) for i, s in enumerate(DECISION_STEPS))}
</div>
<h2>The four candidates, and why</h2>
<p class='muted'>All four came out of the research, all in Go, all echoing the judge-audits-coder theme in the job ad. Here is the honest comparison I put in front of you.</p>
<table class='candidates'><thead><tr><th>Candidate</th><th>What it is</th><th>Why</th><th>Outcome</th></tr></thead><tbody>
{"".join(f"<tr class='{'chosen' if c[2]=='Chosen' else ''}'><td><strong>{html.escape(c[0])}</strong><br><span class='tag'>{html.escape(c[2])}</span></td><td>{html.escape(c[3])}</td><td>{html.escape(c[4])}</td><td>{html.escape(c[5])}</td></tr>" for c in CANDIDATES)}
</tbody></table>
<div class='callout key'>
  <strong>The crux.</strong> I recommended <em>Sandbox MCP + Judge</em> as the safe pick; you chose <em>Zero-to-Receipt</em>, which contains it and adds the claimable sandbox, the published skill, and the onboarding-speed framing. The three options not chosen weren't discarded — they became opportunities #2–#4 in the
  <a href='../memo/OPPORTUNITIES.md' target='_blank' rel='noopener'>opportunity memo</a>, so the exercise answer stays broad while the prototype stays focused.
</div>
"""

prompt_box = """
<div class='callout brief'>
  <strong>The exercise, verbatim.</strong>
  <p>fiskaly offers APIs for fiscalization, e-invoicing and digital receipts, built for POS systems and omni-channel operators. The mission: enable customers to implement compliant solutions on the fiskaly platform.</p>
  <p><em>“What opportunities do you see to drive the mission of fiskaly to the next level? What improvements could be made to the API documentation that bring value to customers? We appreciate it if you want to go crazy. Fixing typos in the API documentation will not empower the mission. For one of the opportunities, build a functional prototype.”</em></p>
  <p><strong>Role:</strong> Agentic Backend Engineer (Golang) — 90% designing multi-agent workflows that automate the SDLC; “Judge” agents auditing “Coder” agents so fiscal signatures stay legally compliant; human-in-the-loop design.
  <a href='https://www.fiskaly.com/jobs/4797666101' target='_blank' rel='noopener'>job posting ↗</a> ·
  <a href='https://developer.fiskaly.com/api/sign-it/2026-02-03' target='_blank' rel='noopener'>SIGN IT docs ↗</a></p>
</div>
"""

tab3 = f"""
<p class='lead'>The complete context and research behind the decision: the exercise brief, the strategic read, all six research areas (each with summary, key facts, fact-check corrections, open questions and full notes), and the opportunity map.</p>
{prompt_box}
<div class='callout strong'>
  <strong>The strategic read.</strong> fiskaly built the AI <em>read</em> layer (RAG chat, llms.txt, CLAUDE.md, machine-readable manifests) but the <em>action</em> layer is conspicuously missing: an unpublished MCP, a dormant try-it console, a dead <code>/api/console</code> link, archived SDKs, zero request examples. Their own job ad asks for judge-agents-auditing-coder-agents. And the docs pipeline (one templated spec + per-country overlays) has no QA gate — which is why the live SIGN IT reference renders <strong>168 silently blank descriptions</strong>.
</div>
<h2>Research areas</h2>
{"".join(area_section(areas_by_key[k]) for k in AREA_ORDER if k in areas_by_key)}
<h2>The opportunity map</h2>
<p class='muted'>Four ideation lenses over the research. Each produced a lead candidate (expanded) and runners-up.</p>
{"".join(opportunity_block(lenses_by_key[k]) for k in ['agent-era-dx','time-to-first-receipt','compliance-confidence','multi-country-scale'] if k in lenses_by_key)}
<div class='callout'><strong>Sources on disk:</strong>
  <a href='../research/RESEARCH.md' target='_blank' rel='noopener'>RESEARCH.md</a> ·
  <a href='../research/fiskaly_research.json' target='_blank' rel='noopener'>fiskaly_research.json</a> ·
  <a href='../research/specs/' target='_blank' rel='noopener'>OpenAPI specs</a> ·
  <a href='../research/api-probes/' target='_blank' rel='noopener'>API probes</a>
</div>
"""

CSS = """
:root{
  --teal:#0d9488; --teal-d:#0f766e; --teal-l:#14b8a6; --ink:#0f2027; --body:#243b3b;
  --muted:#5f7575; --bg:#f5f7f6; --card:#ffffff; --line:#e2ebe9; --warn:#b45309; --warn-bg:#fff7ed;
  --done:#0d9488; --pending:#94a3b8; --inprog:#d97706; --shadow:0 1px 3px rgba(15,32,39,.08),0 8px 24px rgba(15,32,39,.04);
}
*{box-sizing:border-box}
body{margin:0;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Inter,Roboto,Helvetica,Arial,sans-serif;
  color:var(--body);background:var(--bg);line-height:1.6;font-size:15px}
code,pre{font-family:"SF Mono",ui-monospace,"Cascadia Code",Menlo,Consolas,monospace}
code{background:#eef4f3;color:var(--teal-d);padding:.1em .4em;border-radius:4px;font-size:.86em}
a{color:var(--teal-d);text-decoration:none}a:hover{text-decoration:underline}
header.top{background:linear-gradient(135deg,var(--ink),var(--teal-d));color:#fff;padding:34px 0 0}
.wrap{max-width:960px;margin:0 auto;padding:0 24px}
header.top h1{margin:0 0 6px;font-size:26px;letter-spacing:-.02em}
header.top .sub{opacity:.85;margin:0 0 4px;font-size:15px}
header.top .meta{opacity:.7;font-size:13px;margin:0 0 22px}
nav.tabs{display:flex;gap:4px;margin-top:8px}
nav.tabs button{background:rgba(255,255,255,.08);color:#fff;border:0;border-bottom:3px solid transparent;
  padding:12px 18px;font-size:14px;font-weight:600;cursor:pointer;border-radius:8px 8px 0 0;transition:.15s}
nav.tabs button:hover{background:rgba(255,255,255,.16)}
nav.tabs button.active{background:var(--bg);color:var(--teal-d);border-bottom-color:var(--teal-l)}
main{padding:32px 0 80px}
.tab{display:none}.tab.active{display:block}
.lead{font-size:17px;color:var(--ink);margin:0 0 24px}
.muted{color:var(--muted);font-size:14px}
h2{font-size:20px;letter-spacing:-.01em;color:var(--ink);margin:34px 0 14px;padding-bottom:8px;border-bottom:1px solid var(--line)}
h3{font-size:17px;color:var(--ink);margin:18px 0 6px}
h4{font-size:14px;text-transform:uppercase;letter-spacing:.05em;color:var(--teal-d);margin:18px 0 8px}
/* progress */
.progress-wrap{background:var(--card);border:1px solid var(--line);border-radius:14px;padding:22px 24px;box-shadow:var(--shadow);margin-bottom:8px;display:grid;grid-template-columns:auto 1fr;grid-template-rows:auto auto;gap:6px 20px;align-items:center}
.progress-stat{grid-row:1/3;text-align:center}
.progress-stat .big{display:block;font-size:38px;font-weight:700;color:var(--teal-d);line-height:1}
.progress-stat .small{font-size:12px;color:var(--muted);text-transform:uppercase;letter-spacing:.08em}
.progress-bar{height:12px;background:#e8efed;border-radius:99px;overflow:hidden}
.progress-fill{height:100%;background:linear-gradient(90deg,var(--teal),var(--teal-l));border-radius:99px}
.progress-legend{font-size:13px;color:var(--muted)}
/* tasks */
.task{background:var(--card);border:1px solid var(--line);border-left:4px solid var(--pending);border-radius:10px;padding:14px 18px;margin:10px 0;box-shadow:var(--shadow)}
.task.done{border-left-color:var(--done)}.task.in_progress{border-left-color:var(--inprog)}
.task-head{display:flex;align-items:center;gap:10px}
.task-title{font-weight:650;color:var(--ink)}
.task-desc{margin:6px 0 0;font-size:14px}
.task-links{margin:8px 0 0;display:flex;flex-wrap:wrap;gap:14px;font-size:13px}
.badge{font-size:11px;font-weight:700;text-transform:uppercase;letter-spacing:.05em;padding:3px 9px;border-radius:99px;color:#fff;white-space:nowrap}
.badge.done{background:var(--done)}.badge.in_progress{background:var(--inprog)}.badge.pending{background:var(--pending)}
/* tables */
table{width:100%;border-collapse:collapse;margin:12px 0;font-size:14px;background:var(--card);border:1px solid var(--line);border-radius:10px;overflow:hidden;box-shadow:var(--shadow)}
th,td{text-align:left;padding:10px 14px;border-bottom:1px solid var(--line);vertical-align:top}
th{background:#f0f5f4;font-size:12px;text-transform:uppercase;letter-spacing:.04em;color:var(--teal-d)}
tr:last-child td{border-bottom:0}
table.candidates tr.chosen{background:#f0fbf9}
.tag{display:inline-block;font-size:11px;font-weight:700;color:var(--teal-d);background:#d9f5f0;padding:2px 8px;border-radius:99px;margin-top:4px}
/* callouts */
.callout{background:#f0f7f6;border:1px solid var(--line);border-left:4px solid var(--teal-l);border-radius:10px;padding:14px 18px;margin:18px 0;font-size:14px}
.callout.warn{background:var(--warn-bg);border-left-color:var(--warn)}
.callout.strong,.callout.key{background:#ecfbf8;border-left-color:var(--teal)}
.callout.brief{background:#fbfdfc}
.callout p{margin:8px 0 0}
.verdict{font-size:11px;font-weight:700;text-transform:uppercase;padding:1px 7px;border-radius:99px;color:#fff}
.verdict.refuted{background:#dc2626}.verdict.unverifiable{background:#a16207}
.note{color:var(--muted);font-size:13px}
/* timeline */
.timeline{position:relative;margin:8px 0 0;padding-left:8px}
.step{display:grid;grid-template-columns:40px 1fr;gap:16px;position:relative;padding-bottom:8px}
.step:not(:last-child)::before{content:"";position:absolute;left:19px;top:38px;bottom:-6px;width:2px;background:var(--line)}
.step-num{width:38px;height:38px;border-radius:50%;background:var(--teal-d);color:#fff;display:flex;align-items:center;justify-content:center;font-weight:700;z-index:1}
.step-content{background:var(--card);border:1px solid var(--line);border-radius:10px;padding:14px 18px;margin-bottom:12px;box-shadow:var(--shadow)}
.step-content h3{margin:0 0 6px}
.step-content p{margin:0;font-size:14px}
.step-link{margin-top:8px!important;font-size:13px;font-weight:600}
/* research areas */
.area{background:var(--card);border:1px solid var(--line);border-radius:12px;padding:20px 24px;margin:16px 0;box-shadow:var(--shadow)}
.area-sub{color:var(--muted);font-size:14px;margin:0 0 12px}
.summary{font-size:14.5px}
.facts-list{margin:0;padding-left:20px}.facts-list li{margin:6px 0;font-size:14px}
.src{font-size:12px;color:var(--muted)}
details{margin:12px 0}
details summary{cursor:pointer;font-weight:600;color:var(--teal-d);padding:8px 0;font-size:14px}
details summary:hover{color:var(--teal)}
.detail-body,.opp-body{padding:8px 0 4px;font-size:14px;border-top:1px dashed var(--line);margin-top:4px}
.detail-body h3,.detail-body h4,.detail-body h5{margin:14px 0 6px;font-size:14px;color:var(--ink);text-transform:none;letter-spacing:0}
.detail-body ul{padding-left:20px}.detail-body li{margin:4px 0}
pre.code{background:#0f2027;color:#d7e8e6;padding:14px 16px;border-radius:8px;overflow-x:auto;font-size:12.5px;line-height:1.5}
pre.code code{background:none;color:inherit;padding:0}
.opp{background:var(--card);border:1px solid var(--line);border-radius:10px;padding:4px 18px;margin:10px 0;box-shadow:var(--shadow)}
.opp .lens{display:inline-block;font-size:11px;font-weight:700;color:#fff;background:var(--teal-d);padding:2px 8px;border-radius:99px;margin-right:8px;text-transform:uppercase;letter-spacing:.04em}
.also{color:var(--muted);font-size:13px;border-top:1px dashed var(--line);padding-top:8px;margin-top:10px}
.oq summary{color:var(--muted)}
footer{border-top:1px solid var(--line);padding:24px 0;color:var(--muted);font-size:13px;text-align:center}
"""

JS = """
function showTab(id){
  document.querySelectorAll('.tab').forEach(t=>t.classList.toggle('active',t.id===id));
  document.querySelectorAll('nav.tabs button').forEach(b=>b.classList.toggle('active',b.dataset.tab===id));
  if(history.replaceState) history.replaceState(null,'','#'+id);
  window.scrollTo({top:0,behavior:'smooth'});
}
document.querySelectorAll('nav.tabs button').forEach(b=>b.addEventListener('click',()=>showTab(b.dataset.tab)));
const start=(location.hash||'#tab1').replace('#','');
showTab(['tab1','tab2','tab3'].includes(start)?start:'tab1');
"""

HTML = f"""<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>fiskaly exercise — research & decision report</title>
<style>{CSS}</style>
</head>
<body>
<header class="top">
  <div class="wrap">
    <h1>fiskaly Interview Exercise — Research &amp; Decision Report</h1>
    <p class="sub">From the prompt to the Zero-to-Receipt prototype: research, reasoning, and current state.</p>
    <p class="meta">Agentic Backend Engineer (Golang) · SIGN IT · prepared 2026-06-13 · /Users/stan/code/fsk</p>
    <nav class="tabs">
      <button data-tab="tab1">1 · Progress at a glance</button>
      <button data-tab="tab2">2 · The decision</button>
      <button data-tab="tab3">3 · Full context &amp; research</button>
    </nav>
  </div>
</header>
<main>
  <div class="wrap">
    <section id="tab1" class="tab">{tab1}</section>
    <section id="tab2" class="tab">{tab2}</section>
    <section id="tab3" class="tab">{tab3}</section>
  </div>
</main>
<footer><div class="wrap">Generated by <code>report/build_report.py</code> from <code>research/fiskaly_research.json</code>. Regenerate after research changes.</div></footer>
<script>{JS}</script>
</body>
</html>"""

out_path = os.path.join(ROOT, "report/research-report.html")
with open(out_path, "w") as f:
    f.write(HTML)
print(f"wrote {out_path} ({len(HTML):,} bytes)")
