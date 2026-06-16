# Next.js Dashboard Implementation Plan (Plan A: scaffold + tested lib + parity UI)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the Go `dashboard/main.go` with a Next.js app at feature parity — list runs, run detail (judge/build/tests/transcript/diff), trigger a run — backed by a unit-tested parsing layer ported from the Go source.

**Architecture:** A local-only Next.js (App Router, Node-runtime) app in `dashboard/`. Pure parsing functions live in `dashboard/lib/` (tested with Vitest against a committed fixture run dir). Server Components read the local `~/.cache/fiskaly-eval` dirs directly; a Server Action spawns the eval detached. Parity UI uses shadcn/ui with a 10s refresh (matching the Go version). Live streaming + polish are Plan B.

**Tech Stack:** Next.js v16.2.2, TypeScript, pnpm, Tailwind v4 + shadcn v3.5, Vitest. No SSE/SWR yet (Plan B).

---

## Key implementation decisions (review before executing)

1. **Parity first, live later.** Plan A reproduces the Go dashboard exactly, including its 10s meta-refresh. Plan B replaces the refresh with SSE + SWR and adds the polish pass. This yields a working dashboard after Plan A.
2. **`main.go` is the behavioral source of truth** for all `lib/` ports (status rules, tool summarizer, diff classes).
3. **lib internal imports are relative** (`./paths`, `./types`) so Vitest needs no path-alias config; pages use the `@/lib/*` alias.
4. **Trigger is a Server Action**, not a route handler — idiomatic App Router; spawns detached so it survives the request.
5. **Node runtime** on anything touching `fs`/`child_process`.

## File structure

- `dashboard/lib/types.ts` — shared types (`Summary`, `TranscriptEvent`, `DiffLine`, `Check`).
- `dashboard/lib/paths.ts` — `runsDir()`, `evalScript()` (env-overridable).
- `dashboard/lib/transcript.ts` — `parseTranscript()`, `summarizeTool()`.
- `dashboard/lib/diff.ts` — `classifyDiff()`.
- `dashboard/lib/runs.ts` — `listRuns()`, `summarizeRun()`, `loadRun()`.
- `dashboard/__fixtures__/run.sample/` — committed fixture run dir for tests.
- `dashboard/app/actions.ts` — `triggerRun()` Server Action.
- `dashboard/app/page.tsx` — run list. `dashboard/app/run/[id]/page.tsx` — detail.
- `dashboard/components/{RunTable,JudgeBadge,TranscriptView,DiffView,TriggerButton}.tsx`.
- `evals/dashboard.sh` — rewritten launcher.

---

## Task 1: Scaffold the Next.js app, remove the Go dashboard

**Files:** delete `dashboard/main.go`, `dashboard/go.mod`, `dashboard/dashboard`; create the Next app under `dashboard/`.

- [ ] **Step 1: Remove the Go dashboard**

```bash
cd /Users/stan/code/fsk
git rm dashboard/main.go dashboard/go.mod
rm -f dashboard/dashboard
ls dashboard/   # expect empty
```
Expected: `dashboard/` is empty (so create-next-app won't refuse it).

- [ ] **Step 2: Scaffold Next.js into dashboard/**

```bash
cd /Users/stan/code/fsk
pnpm create next-app@latest dashboard --yes
```
Expected: creates `dashboard/` with TypeScript, App Router, Tailwind v4, ESLint, Turbopack, `@/*` alias (Next 16 `--yes` defaults). If it refuses because the dir is non-empty, ensure only the removed files are gone and retry.

- [ ] **Step 3: Init shadcn and add the components Plan A uses**

```bash
cd /Users/stan/code/fsk/dashboard
pnpm dlx shadcn@latest init --defaults
pnpm dlx shadcn@latest add table badge button
```
Expected: `components.json` created; `components/ui/{table,badge,button}.tsx` exist.

- [ ] **Step 4: Add Vitest and a test script**

```bash
cd /Users/stan/code/fsk/dashboard
pnpm add -D vitest
```

Create `dashboard/vitest.config.ts`:
```ts
import { defineConfig } from "vitest/config";

export default defineConfig({
  test: { environment: "node", include: ["lib/**/*.test.ts"] },
});
```

Add to `dashboard/package.json` `"scripts"`: `"test": "vitest run"`.

- [ ] **Step 5: Add `dashboard/.gitignore` entries**

create-next-app writes a `.gitignore`; confirm it ignores `node_modules`, `.next`. If missing, append them.

- [ ] **Step 6: Verify the scaffold builds and the empty test run is green**

```bash
cd /Users/stan/code/fsk/dashboard
pnpm build
pnpm test
```
Expected: `pnpm build` succeeds; `pnpm test` reports "No test files found" (or 0 tests) and exits 0.

- [ ] **Step 7: Commit**

```bash
cd /Users/stan/code/fsk
git add -A
git commit -m "Dashboard: scaffold Next.js app, remove Go dashboard"
git push origin main
```

---

## Task 2: Shared types and path resolution

**Files:** Create `dashboard/lib/types.ts`, `dashboard/lib/paths.ts`, `dashboard/lib/paths.test.ts`.

- [ ] **Step 1: Write `dashboard/lib/types.ts`**

```ts
export type Check = "PASS" | "FAIL" | "";
export type RunStatus = "running" | "done";

export interface Summary {
  id: string;
  created: string; // ISO timestamp
  status: RunStatus;
  coder: string;
  harness: string;
  model: string;
  effort: string;
  build: Check;
  tests: Check;
  judge: Check;
  turns: string;
  cost: string;
}

export type EventKind = "thinking" | "assistant" | "tool" | "result" | "final";
export interface TranscriptEvent {
  kind: EventKind;
  text: string;
}

export type DiffClass = "meta" | "hunk" | "add" | "del" | "ctx";
export interface DiffLine {
  cls: DiffClass;
  text: string;
}
```

- [ ] **Step 2: Write the failing test `dashboard/lib/paths.test.ts`**

```ts
import { afterEach, expect, test } from "vitest";
import { runsDir, evalScript } from "./paths";

afterEach(() => {
  delete process.env.FISKALY_RUNS_DIR;
  delete process.env.FISKALY_EVAL_SCRIPT;
});

test("runsDir defaults under the home cache dir", () => {
  expect(runsDir().endsWith("/.cache/fiskaly-eval")).toBe(true);
});

test("runsDir honors the env override", () => {
  process.env.FISKALY_RUNS_DIR = "/tmp/runs";
  expect(runsDir()).toBe("/tmp/runs");
});

test("evalScript honors the env override", () => {
  process.env.FISKALY_EVAL_SCRIPT = "/tmp/run.sh";
  expect(evalScript()).toBe("/tmp/run.sh");
});
```

- [ ] **Step 3: Run the test, confirm it fails**

Run: `cd dashboard && pnpm test`
Expected: FAIL — cannot find `./paths`.

- [ ] **Step 4: Write `dashboard/lib/paths.ts`**

```ts
import os from "node:os";
import path from "node:path";

// The dashboard runs from dashboard/, so the repo root is its parent and the
// eval script lives at <repo>/evals/run-eval-docker.sh. Both are env-overridable.
export function runsDir(): string {
  return process.env.FISKALY_RUNS_DIR ?? path.join(os.homedir(), ".cache", "fiskaly-eval");
}

export function evalScript(): string {
  return process.env.FISKALY_EVAL_SCRIPT ?? path.resolve(process.cwd(), "..", "evals", "run-eval-docker.sh");
}
```

- [ ] **Step 5: Run the test, confirm it passes**

Run: `cd dashboard && pnpm test`
Expected: PASS (3 tests).

- [ ] **Step 6: Commit**

```bash
cd /Users/stan/code/fsk
git add dashboard/lib/types.ts dashboard/lib/paths.ts dashboard/lib/paths.test.ts
git commit -m "Dashboard: shared types and path resolution"
git push origin main
```

---

## Task 3: Transcript parser

**Files:** Create `dashboard/lib/transcript.ts`, `dashboard/lib/transcript.test.ts`.

- [ ] **Step 1: Write the failing test `dashboard/lib/transcript.test.ts`**

```ts
import { expect, test } from "vitest";
import { parseTranscript, summarizeTool } from "./transcript";

const jsonl = [
  JSON.stringify({ type: "system", model: "claude-sonnet-4-6", cwd: "/work" }),
  JSON.stringify({ type: "assistant", message: { content: [
    { type: "thinking", thinking: "let me look" },
    { type: "text", text: "Reading the file" },
    { type: "tool_use", name: "Read", input: { file_path: "pos/checkout.go" } },
  ] } }),
  JSON.stringify({ type: "user", message: { content: [
    { type: "tool_result", content: "file contents here", is_error: false },
  ] } }),
  JSON.stringify({ type: "result", result: "done", num_turns: 12, total_cost_usd: 1.5 }),
].join("\n");

test("parseTranscript yields ordered typed events", () => {
  const evs = parseTranscript(jsonl);
  expect(evs.map((e) => e.kind)).toEqual(["thinking", "assistant", "tool", "result", "final"]);
  expect(evs[2].text).toBe("Read  pos/checkout.go");
});

test("tool_result errors are prefixed", () => {
  const evs = parseTranscript(JSON.stringify({ type: "user", message: { content: [
    { type: "tool_result", content: "boom", is_error: true },
  ] } }));
  expect(evs[0].text).toBe("error: boom");
});

test("summarizeTool formats a Bash command on one line", () => {
  expect(summarizeTool("Bash", { command: "go test ./..." })).toBe("Bash  $ go test ./...");
});
```

- [ ] **Step 2: Run the test, confirm it fails**

Run: `cd dashboard && pnpm test`
Expected: FAIL — cannot find `./transcript`.

- [ ] **Step 3: Write `dashboard/lib/transcript.ts`**

```ts
import type { TranscriptEvent } from "./types";

export function parseTranscript(jsonl: string): TranscriptEvent[] {
  const events: TranscriptEvent[] = [];
  for (const line of jsonl.split("\n")) {
    const s = line.trim();
    if (!s) continue;
    let m: any;
    try {
      m = JSON.parse(s);
    } catch {
      continue;
    }
    switch (m.type) {
      case "assistant":
        for (const c of content(m)) {
          if (c?.type === "thinking" && c.thinking?.trim()) events.push({ kind: "thinking", text: c.thinking });
          else if (c?.type === "text" && c.text?.trim()) events.push({ kind: "assistant", text: c.text });
          else if (c?.type === "tool_use") events.push({ kind: "tool", text: summarizeTool(c.name, c.input ?? {}) });
        }
        break;
      case "user":
        for (const c of content(m)) {
          if (c?.type === "tool_result") {
            let txt = flatten(c.content);
            if (c.is_error) txt = "error: " + txt;
            events.push({ kind: "result", text: truncate(txt, 600) });
          }
        }
        break;
      case "result":
        if (typeof m.result === "string" && m.result) events.push({ kind: "final", text: m.result });
        break;
    }
  }
  return events;
}

export function summarizeTool(name: string, input: Record<string, any>): string {
  const sv = (k: string) => (typeof input[k] === "string" ? input[k] : "");
  switch (name) {
    case "Bash":
      return "Bash  $ " + oneLine(sv("command"));
    case "Read":
      return "Read  " + sv("file_path");
    case "Write":
      return "Write  " + sv("file_path");
    case "Edit":
    case "MultiEdit":
      return name + "  " + sv("file_path");
    case "Grep": {
      let p = sv("pattern");
      const path = sv("path");
      if (path) p += "  in " + path;
      return "Grep  " + p;
    }
    case "Glob":
      return "Glob  " + sv("pattern");
    case "TodoWrite":
      return "TodoWrite  (updated task list)";
    case "WebFetch":
      return "WebFetch  " + sv("url");
    case "WebSearch":
      return "WebSearch  " + sv("query");
    case "Task":
    case "Agent":
      return name + "  " + (sv("description") || sv("subagent_type"));
    case "ToolSearch":
      return "ToolSearch  " + sv("query");
    default:
      return name + "  " + truncate(oneLine(JSON.stringify(input)), 200);
  }
}

function content(m: any): any[] {
  return m?.message?.content ?? [];
}

function flatten(v: any): string {
  if (typeof v === "string") return v;
  if (Array.isArray(v)) return v.map((e) => (typeof e?.text === "string" ? e.text : "")).join("");
  return "";
}

function truncate(s: string, n: number): string {
  s = s.trim();
  return s.length > n ? s.slice(0, n) + " …" : s;
}

function oneLine(s: string): string {
  return truncate(s.split(/\s+/).filter(Boolean).join(" "), 300);
}
```

- [ ] **Step 4: Run the test, confirm it passes**

Run: `cd dashboard && pnpm test`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/stan/code/fsk
git add dashboard/lib/transcript.ts dashboard/lib/transcript.test.ts
git commit -m "Dashboard: stream-json transcript parser"
git push origin main
```

---

## Task 4: Diff classifier

**Files:** Create `dashboard/lib/diff.ts`, `dashboard/lib/diff.test.ts`.

- [ ] **Step 1: Write the failing test `dashboard/lib/diff.test.ts`**

```ts
import { expect, test } from "vitest";
import { classifyDiff } from "./diff";

test("classifies diff lines by leading marker", () => {
  const raw = ["diff --git a/x b/x", "@@ -1 +1 @@", "+added", "-removed", " context"].join("\n");
  expect(classifyDiff(raw).map((l) => l.cls)).toEqual(["meta", "hunk", "add", "del", "ctx"]);
});

test("--- and +++ are meta, not del/add", () => {
  expect(classifyDiff("--- a/x").at(0)?.cls).toBe("meta");
  expect(classifyDiff("+++ b/x").at(0)?.cls).toBe("meta");
});

test("empty diff yields no lines", () => {
  expect(classifyDiff("   ")).toEqual([]);
});
```

- [ ] **Step 2: Run the test, confirm it fails**

Run: `cd dashboard && pnpm test`
Expected: FAIL — cannot find `./diff`.

- [ ] **Step 3: Write `dashboard/lib/diff.ts`**

```ts
import type { DiffLine } from "./types";

// Marker order matters: +++/--- must be classified as meta before the +/- checks.
export function classifyDiff(raw: string): DiffLine[] {
  if (!raw.trim()) return [];
  return raw.split("\n").map((text) => {
    let cls: DiffLine["cls"] = "ctx";
    if (text.startsWith("diff ") || text.startsWith("index ") || text.startsWith("+++") || text.startsWith("---")) cls = "meta";
    else if (text.startsWith("@@")) cls = "hunk";
    else if (text.startsWith("+")) cls = "add";
    else if (text.startsWith("-")) cls = "del";
    return { cls, text };
  });
}
```

- [ ] **Step 4: Run the test, confirm it passes**

Run: `cd dashboard && pnpm test`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/stan/code/fsk
git add dashboard/lib/diff.ts dashboard/lib/diff.test.ts
git commit -m "Dashboard: diff line classifier"
git push origin main
```

---

## Task 5: Run reader + fixture

**Files:** Create `dashboard/__fixtures__/run.sample/{transcript.jsonl,judge.txt,build.txt,test.txt,changes.diff,meta.json}`, `dashboard/lib/runs.ts`, `dashboard/lib/runs.test.ts`.

- [ ] **Step 1: Create the fixture run dir**

`dashboard/__fixtures__/run.sample/transcript.jsonl`:
```
{"type":"system","model":"claude-sonnet-4-6","cwd":"/work","claude_code_version":"2.1.0"}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"pos/checkout.go"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","content":"ok","is_error":false}]}}
{"type":"result","result":"done","num_turns":12,"total_cost_usd":1.5}
```

`dashboard/__fixtures__/run.sample/judge.txt`:
```
fiskaly contract conformance: /work

5/5 rules passed.
VERDICT: conformant. exit 0
```

`dashboard/__fixtures__/run.sample/build.txt`: empty file (create with `: > dashboard/__fixtures__/run.sample/build.txt`).

`dashboard/__fixtures__/run.sample/test.txt`:
```
ok  	pos	0.4s
```

`dashboard/__fixtures__/run.sample/changes.diff`:
```
diff --git a/pos/checkout.go b/pos/checkout.go
@@ -1 +1 @@
-old
+new
```

`dashboard/__fixtures__/run.sample/meta.json`:
```
{"harness":"docker","coder":"claude-code","model":"claude-sonnet-4-6","effort":"high"}
```

- [ ] **Step 2: Write the failing test `dashboard/lib/runs.test.ts`**

```ts
import path from "node:path";
import { expect, test } from "vitest";
import { summarizeRun, loadRun, listRuns } from "./runs";

const fixtures = path.resolve(__dirname, "../__fixtures__");
const sample = path.join(fixtures, "run.sample");

test("summarizeRun derives status and checks from a done run", () => {
  const s = summarizeRun(sample);
  expect(s.status).toBe("done");
  expect(s.judge).toBe("PASS");
  expect(s.build).toBe("PASS");
  expect(s.tests).toBe("PASS");
  expect(s.harness).toBe("docker"); // cwd == /work
  expect(s.model).toBe("claude-sonnet-4-6");
  expect(s.coder).toBe("claude-code");
  expect(s.turns).toBe("12");
  expect(s.cost).toBe("$1.50");
});

test("loadRun returns parsed transcript and diff", () => {
  const r = loadRun(fixtures, "run.sample");
  expect(r).not.toBeNull();
  expect(r!.transcript.some((e) => e.kind === "tool")).toBe(true);
  expect(r!.diff.some((l) => l.cls === "add")).toBe(true);
  expect(r!.judgeLog).toContain("conformant");
});

test("listRuns finds the fixture run", () => {
  const ids = listRuns(fixtures).map((s) => s.id);
  expect(ids).toContain("run.sample");
});
```

- [ ] **Step 3: Run the test, confirm it fails**

Run: `cd dashboard && pnpm test`
Expected: FAIL — cannot find `./runs`.

- [ ] **Step 4: Write `dashboard/lib/runs.ts`**

```ts
import fs from "node:fs";
import path from "node:path";
import type { Check, Summary, TranscriptEvent, DiffLine } from "./types";
import { runsDir } from "./paths";
import { parseTranscript } from "./transcript";
import { classifyDiff } from "./diff";

export interface RunDetail {
  summary: Summary;
  judgeLog: string;
  buildLog: string;
  testLog: string;
  err: string;
  transcript: TranscriptEvent[];
  diff: DiffLine[];
}

export function listRuns(dir = runsDir()): Summary[] {
  let entries: string[];
  try {
    entries = fs.readdirSync(dir);
  } catch {
    return [];
  }
  const out: Summary[] = [];
  for (const name of entries) {
    if (!name.startsWith("run.")) continue;
    const d = path.join(dir, name);
    let st: fs.Stats;
    try {
      st = fs.statSync(d);
    } catch {
      continue;
    }
    if (st.isDirectory()) out.push(summarizeRun(d, st.mtime));
  }
  out.sort((a, b) => (a.created < b.created ? 1 : -1)); // newest first
  return out;
}

export function summarizeRun(dir: string, created = safeMtime(dir)): Summary {
  const s: Summary = {
    id: path.basename(dir), created: created.toISOString(), status: "running",
    coder: "", harness: "", model: "", effort: "", build: "", tests: "", judge: "", turns: "", cost: "",
  };

  const log = logInfo(path.join(dir, "transcript.jsonl"));
  const meta = readMeta(dir);
  s.effort = meta.effort || "—";
  s.model = log.model || meta.model;
  s.coder = (log.ccver ? "claude-code" : "") || meta.coder || "?";
  s.harness = log.cwd === "/work" ? "docker" : log.cwd ? "local" : meta.harness || "?";

  const judge = readFile(path.join(dir, "judge.txt"));
  if (!judge) return s; // judge.txt is the last step; absent means still running
  s.status = "done";
  if (judge.includes("conformant")) s.judge = "PASS";
  else if (judge.includes("NON-COMPLIANT")) s.judge = "FAIL";

  s.build = readFile(path.join(dir, "build.txt")).trim() === "" ? "PASS" : "FAIL";
  const tt = readFile(path.join(dir, "test.txt"));
  s.tests = tt && !tt.includes("FAIL") && tt.includes("ok") ? "PASS" : "FAIL";

  const r = parseResult(path.join(dir, "transcript.jsonl"));
  s.turns = r.turns;
  s.cost = r.cost;
  return s;
}

export function loadRun(dir: string, id: string): RunDetail | null {
  if (!id.startsWith("run.") || id.includes("/") || id.includes("..")) return null;
  const d = path.join(dir, id);
  try {
    if (!fs.statSync(d).isDirectory()) return null;
  } catch {
    return null;
  }
  return {
    summary: summarizeRun(d),
    judgeLog: readFile(path.join(d, "judge.txt")),
    buildLog: readFile(path.join(d, "build.txt")),
    testLog: readFile(path.join(d, "test.txt")),
    err: readFile(path.join(d, "claude.err")),
    transcript: parseTranscript(readFile(path.join(d, "transcript.jsonl"))),
    diff: classifyDiff(readFile(path.join(d, "changes.diff"))),
  };
}

function parseResult(file: string): { turns: string; cost: string } {
  let turns = "", cost = "";
  for (const line of readLines(file)) {
    let m: any;
    try {
      m = JSON.parse(line);
    } catch {
      continue;
    }
    if (m.type !== "result") continue;
    if (typeof m.num_turns === "number") turns = String(Math.round(m.num_turns));
    if (typeof m.total_cost_usd === "number") cost = "$" + m.total_cost_usd.toFixed(2);
  }
  return { turns, cost };
}

function logInfo(file: string): { model: string; cwd: string; ccver: string } {
  for (const line of readLines(file)) {
    let m: any;
    try {
      m = JSON.parse(line);
    } catch {
      continue;
    }
    if (m.type === "system") return { model: str(m.model), cwd: str(m.cwd), ccver: str(m.claude_code_version) };
  }
  return { model: "", cwd: "", ccver: "" };
}

function readMeta(dir: string): { harness: string; coder: string; model: string; effort: string } {
  try {
    const m = JSON.parse(fs.readFileSync(path.join(dir, "meta.json"), "utf8"));
    return { harness: str(m.harness), coder: str(m.coder), model: str(m.model), effort: str(m.effort) };
  } catch {
    return { harness: "", coder: "", model: "", effort: "" };
  }
}

function readFile(p: string): string {
  try {
    return fs.readFileSync(p, "utf8");
  } catch {
    return "";
  }
}
function readLines(p: string): string[] {
  return readFile(p).split("\n").filter((l) => l.trim());
}
function str(v: any): string {
  return typeof v === "string" ? v : "";
}
function safeMtime(dir: string): Date {
  try {
    return fs.statSync(dir).mtime;
  } catch {
    return new Date(0);
  }
}
```

- [ ] **Step 5: Run the tests, confirm they pass**

Run: `cd dashboard && pnpm test`
Expected: PASS (all lib tests).

- [ ] **Step 6: Commit**

```bash
cd /Users/stan/code/fsk
git add dashboard/lib/runs.ts dashboard/lib/runs.test.ts dashboard/__fixtures__
git commit -m "Dashboard: run reader (summarize/list/loadRun) with fixture"
git push origin main
```

---

## Task 6: Trigger Server Action

**Files:** Create `dashboard/app/actions.ts`.

- [ ] **Step 1: Write `dashboard/app/actions.ts`**

```ts
"use server";

import { spawn } from "node:child_process";
import { evalScript } from "@/lib/paths";

// Spawn the eval detached so it outlives this request; the new run dir shows up
// on the next list render. Mirrors the Go dashboard's async POST /trigger.
export async function triggerRun(): Promise<void> {
  const script = evalScript();
  const child = spawn("bash", [script], { detached: true, stdio: "ignore" });
  child.unref();
}
```

- [ ] **Step 2: Verify it type-checks via the build (used by the page in Task 7)**

Run: `cd dashboard && pnpm exec tsc --noEmit`
Expected: no errors. (The action is wired into the UI in Task 7; this step only confirms it compiles.)

- [ ] **Step 3: Commit**

```bash
cd /Users/stan/code/fsk
git add dashboard/app/actions.ts
git commit -m "Dashboard: triggerRun server action (detached eval spawn)"
git push origin main
```

---

## Task 7: Parity UI — list, detail, components

**Files:** Create `dashboard/components/{JudgeBadge,RunTable,TriggerButton,TranscriptView,DiffView}.tsx`; overwrite `dashboard/app/page.tsx`; create `dashboard/app/run/[id]/page.tsx`.

- [ ] **Step 1: Write `dashboard/components/JudgeBadge.tsx`**

```tsx
import { Badge } from "@/components/ui/badge";
import type { Check } from "@/lib/types";

export function JudgeBadge({ value }: { value: Check }) {
  if (value === "PASS") return <Badge className="bg-green-700 text-white">PASS</Badge>;
  if (value === "FAIL") return <Badge className="bg-red-700 text-white">FAIL</Badge>;
  return <span className="text-muted-foreground">—</span>;
}
```

- [ ] **Step 2: Write `dashboard/components/RunTable.tsx`**

```tsx
import Link from "next/link";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { JudgeBadge } from "@/components/JudgeBadge";
import type { Summary } from "@/lib/types";

export function RunTable({ runs }: { runs: Summary[] }) {
  return (
    <Table className="font-mono text-sm">
      <TableHeader>
        <TableRow>
          <TableHead>run</TableHead><TableHead>when</TableHead><TableHead>coder</TableHead>
          <TableHead>harness</TableHead><TableHead>model</TableHead><TableHead>build</TableHead>
          <TableHead>tests</TableHead><TableHead>judge</TableHead><TableHead>turns</TableHead><TableHead>cost</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {runs.length === 0 && (
          <TableRow><TableCell colSpan={10}>no runs yet</TableCell></TableRow>
        )}
        {runs.map((r) => (
          <TableRow key={r.id}>
            <TableCell><Link className="text-blue-700 underline" href={`/run/${r.id}`}>{r.id}</Link></TableCell>
            <TableCell>{new Date(r.created).toLocaleString()}</TableCell>
            <TableCell>{r.coder}</TableCell>
            <TableCell>{r.harness}</TableCell>
            <TableCell>{r.model}</TableCell>
            {r.status === "running" ? (
              <TableCell colSpan={5} className="text-amber-600">running…</TableCell>
            ) : (
              <>
                <TableCell><JudgeBadge value={r.build} /></TableCell>
                <TableCell><JudgeBadge value={r.tests} /></TableCell>
                <TableCell><JudgeBadge value={r.judge} /></TableCell>
                <TableCell>{r.turns}</TableCell>
                <TableCell>{r.cost}</TableCell>
              </>
            )}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
```

- [ ] **Step 3: Write `dashboard/components/TriggerButton.tsx`**

```tsx
import { Button } from "@/components/ui/button";
import { triggerRun } from "@/app/actions";

export function TriggerButton() {
  return (
    <form action={triggerRun}>
      <Button type="submit">▶ trigger run</Button>
    </form>
  );
}
```

- [ ] **Step 4: Write `dashboard/components/TranscriptView.tsx`**

```tsx
import type { TranscriptEvent } from "@/lib/types";

const color: Record<TranscriptEvent["kind"], string> = {
  thinking: "text-purple-600 italic",
  assistant: "text-foreground",
  tool: "text-blue-600",
  result: "text-green-700",
  final: "font-bold",
};

export function TranscriptView({ events }: { events: TranscriptEvent[] }) {
  return (
    <div className="space-y-2 font-mono text-xs">
      {events.map((e, i) => (
        <div key={i} className="flex gap-3 border-l-2 border-muted pl-2">
          <span className="w-20 shrink-0 text-muted-foreground">{e.kind}</span>
          <span className={`whitespace-pre-wrap break-words ${color[e.kind]}`}>{e.text}</span>
        </div>
      ))}
    </div>
  );
}
```

- [ ] **Step 5: Write `dashboard/components/DiffView.tsx`**

```tsx
import type { DiffLine } from "@/lib/types";

const bg: Record<DiffLine["cls"], string> = {
  add: "bg-green-100",
  del: "bg-red-100",
  hunk: "bg-blue-100 text-blue-800",
  meta: "text-muted-foreground font-bold",
  ctx: "",
};

export function DiffView({ lines }: { lines: DiffLine[] }) {
  if (lines.length === 0) return <span className="text-muted-foreground">—</span>;
  return (
    <pre className="overflow-auto rounded bg-muted p-2 font-mono text-xs">
      {lines.map((l, i) => (
        <span key={i} className={`block px-2 ${bg[l.cls]}`}>{l.text}</span>
      ))}
    </pre>
  );
}
```

- [ ] **Step 6: Overwrite `dashboard/app/page.tsx`**

```tsx
import { listRuns } from "@/lib/runs";
import { RunTable } from "@/components/RunTable";
import { TriggerButton } from "@/components/TriggerButton";

// Parity with the Go dashboard's 10s meta-refresh; Plan B replaces this with SWR.
export const dynamic = "force-dynamic";

export default function Home() {
  const runs = listRuns();
  return (
    <main className="mx-auto max-w-5xl p-8">
      <meta httpEquiv="refresh" content="10" />
      <div className="mb-4 flex items-center gap-4">
        <h1 className="text-xl font-bold">fiskaly eval runs</h1>
        <TriggerButton />
      </div>
      <RunTable runs={runs} />
    </main>
  );
}
```

- [ ] **Step 7: Create `dashboard/app/run/[id]/page.tsx`**

```tsx
import { notFound } from "next/navigation";
import Link from "next/link";
import { runsDir } from "@/lib/paths";
import { loadRun } from "@/lib/runs";
import { JudgeBadge } from "@/components/JudgeBadge";
import { TranscriptView } from "@/components/TranscriptView";
import { DiffView } from "@/components/DiffView";

export const dynamic = "force-dynamic";

export default async function RunPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const run = loadRun(runsDir(), id);
  if (!run) notFound();
  const s = run.summary;
  return (
    <main className="mx-auto max-w-6xl p-8 font-mono text-sm">
      <Link className="text-blue-700 underline" href="/">← runs</Link>
      <h1 className="my-2 text-xl font-bold">{s.id}</h1>
      <div className="mb-4 flex flex-wrap gap-4 rounded bg-muted p-3">
        <span>coder {s.coder}</span><span>model {s.model}</span><span>harness {s.harness}</span>
        <span>effort {s.effort}</span><span>turns {s.turns}</span><span>cost {s.cost}</span>
        <span>build <JudgeBadge value={s.build} /></span>
        <span>tests <JudgeBadge value={s.tests} /></span>
        <span>judge <JudgeBadge value={s.judge} /></span>
      </div>

      <h2 className="mb-1 mt-4 font-bold">judge verdict</h2>
      <pre className="overflow-auto rounded bg-muted p-2 text-xs whitespace-pre-wrap">{run.judgeLog || "—"}</pre>

      <details className="my-2 rounded border p-2">
        <summary className="cursor-pointer font-bold">build · tests{run.err ? " · stderr" : ""}</summary>
        <pre className="mt-2 overflow-auto whitespace-pre-wrap text-xs">{`build:\n${run.buildLog}\ntests:\n${run.testLog}${run.err ? `\nstderr:\n${run.err}` : ""}`}</pre>
      </details>

      <details className="my-2 rounded border p-2" open>
        <summary className="cursor-pointer font-bold">session transcript ({run.transcript.length} events)</summary>
        <div className="mt-2"><TranscriptView events={run.transcript} /></div>
      </details>

      <details className="my-2 rounded border p-2">
        <summary className="cursor-pointer font-bold">diff</summary>
        <div className="mt-2"><DiffView lines={run.diff} /></div>
      </details>
    </main>
  );
}
```

- [ ] **Step 8: Verify build + dev smoke against the fixture**

```bash
cd /Users/stan/code/fsk/dashboard
pnpm exec tsc --noEmit && pnpm build
FISKALY_RUNS_DIR="$(pwd)/__fixtures__" pnpm dev &
sleep 4
curl -s localhost:3000 | grep -o 'run.sample'
curl -s localhost:3000/run/run.sample | grep -o 'judge verdict'
kill %1
```
Expected: build succeeds; first curl prints `run.sample`; second prints `judge verdict`.

- [ ] **Step 9: Commit**

```bash
cd /Users/stan/code/fsk
git add dashboard/app dashboard/components
git commit -m "Dashboard: parity UI — list, detail, transcript, diff, trigger"
git push origin main
```

---

## Task 8: Rewrite the launcher

**Files:** Overwrite `evals/dashboard.sh`.

- [ ] **Step 1: Overwrite `evals/dashboard.sh`**

```bash
#!/usr/bin/env bash
# Launch the eval dashboard:  ./evals/dashboard.sh   then open http://localhost:8080
#
# The dashboard is a Next.js app in dashboard/. It reads ~/.cache/fiskaly-eval and
# triggers runs via evals/run-eval-docker.sh (override with FISKALY_EVAL_SCRIPT).
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root/dashboard"
pnpm install --frozen-lockfile
exec pnpm dev -p 8080 "$@"
```

- [ ] **Step 2: Verify it boots and serves**

```bash
cd /Users/stan/code/fsk
./evals/dashboard.sh &
sleep 6
curl -s localhost:8080 | grep -o 'fiskaly eval runs'
kill %1
```
Expected: prints `fiskaly eval runs`.

- [ ] **Step 3: Commit**

```bash
cd /Users/stan/code/fsk
git add evals/dashboard.sh
git commit -m "Dashboard: launcher runs the Next.js app on :8080"
git push origin main
```

---

## Task 9: Parity check and cleanup

**Files:** none (verification).

- [ ] **Step 1: Confirm the Go dashboard is fully gone**

Run: `ls /Users/stan/code/fsk/dashboard/*.go /Users/stan/code/fsk/dashboard/go.mod 2>&1`
Expected: "No such file or directory".

- [ ] **Step 2: Full verification**

```bash
cd /Users/stan/code/fsk/dashboard
pnpm test && pnpm exec tsc --noEmit && pnpm build
```
Expected: tests pass, no type errors, build succeeds.

- [ ] **Step 3: Parity checklist vs the spec**

Confirm against `docs/superpowers/specs/2026-06-16-nextjs-dashboard-design.md` "feature parity": list columns present; detail shows judge verdict, build/tests/stderr disclosure, transcript events, colored diff; trigger button present. Note any gap and add a follow-up task rather than silently skipping.

---

## Follow-on: Plan B (live + polish)

Out of scope here; its own spec→plan once Plan A is green. Sketch:
- `app/api/runs/[id]/stream/route.ts` — SSE (`runtime="nodejs"`, `dynamic="force-dynamic"`, `ReadableStream`, `text/event-stream`, `X-Content-Type-Options: nosniff`) tailing `transcript.jsonl` (fs.watch + size-poll fallback), closing when `judge.txt` appears or the client aborts.
- `app/api/runs/route.ts` — JSON list for SWR.
- Convert `RunTable` to a client component using SWR (~1s) and `TranscriptView` to subscribe via `EventSource` for a running run; drop the `<meta refresh>`.
- Polish pass: custom shadcn theme (the GitHub-diff palette, mono accents), dark/light, with the frontend-design skill.
```
