# Dashboard Scenario Management + Run Cancellation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let the dashboard pick which scenario to run, edit existing scenarios' config/task/solution, and cancel in-progress runs.

**Architecture:** New read/write libs + server actions in the Next.js dashboard, two new routes (`/scenarios`, `/scenarios/[id]`), and a small Go runner change so each run records a `run.json` (pgid + container name) that the dashboard reads to kill it. Cancellation = mark a `cancelled` marker, kill the runner's process group, `docker kill` its container.

**Tech Stack:** Next.js 16.2.9 (App Router, Turbopack, server actions), React, Tailwind v4, base-ui 1.5.0, lucide-react, vitest; Go 1.25 runner; Docker.

## Global Constraints

- Ground every external API (base-ui, Next.js, Tailwind) in Context7 or installed source — never memory. (`~/CLAUDE.md`)
- Before editing any Next.js framework API, read the relevant guide under `sims/dashboard/node_modules/next/dist/docs/`. (`sims/dashboard/AGENTS.md`)
- Design system on every new surface: Hanken Grotesk UI / IBM Plex Mono for ids+code; uppercase tracked muted labels (`text-[0.7rem] font-medium uppercase tracking-[0.08em] text-muted-foreground`); status as colored dots via `--success`/`--danger`/`--warning` tokens; hairline borders; works light + dark.
- No silent fallbacks; validate all external/user input; raise errors explicitly. No "just in case" code. (`~/CLAUDE.md`)
- Dashboard commands run from `sims/dashboard` with `pnpm`; runner from `sims/runner` with `go`.
- Tests: dashboard `pnpm test` (vitest), runner `go test ./...`.
- Git: this repo commits directly to `main` (solo exercise). Confirm with the user before the first commit of the run.

---

### Task 1: Scenarios read lib + types + validation

**Files:**
- Modify: `sims/dashboard/lib/paths.ts` (add `scenariosDir`)
- Modify: `sims/dashboard/lib/types.ts` (add scenario types; extend `RunStatus`)
- Create: `sims/dashboard/lib/scenarios.ts`
- Test: `sims/dashboard/lib/scenarios.test.ts`

**Interfaces:**
- Produces:
  - `scenariosDir(): string`
  - `interface Verdicts { build: string; tests: string; judge: string }`
  - `interface ScenarioConfig { id: string; title: string; tier: number; capability: string; persona_ref: string; traps: string[]; judge: { rules: string[] }; baseline: Verdicts; target: Verdicts }`
  - `interface ScenarioDetail { config: ScenarioConfig; task: string; solution: string }`
  - `listScenarios(dir?: string): ScenarioConfig[]`
  - `isKnownScenario(id: string, dir?: string): boolean`
  - `loadScenario(id: string, dir?: string): ScenarioDetail | null`
  - `validateConfig(obj: unknown): string | null` (null = valid; else first error message)
  - `RunStatus` becomes `"running" | "done" | "cancelled"`

- [ ] **Step 1: Write failing tests**

`sims/dashboard/lib/scenarios.test.ts`:
```ts
import { describe, it, expect } from "vitest";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { listScenarios, isKnownScenario, loadScenario, validateConfig } from "./scenarios";

function fixtureDir(): string {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "scenarios-"));
  const s = path.join(root, "01-demo");
  fs.mkdirSync(path.join(s, "fixture"), { recursive: true });
  fs.writeFileSync(
    path.join(s, "scenario.json"),
    JSON.stringify({
      id: "01-demo", title: "Demo", tier: 1, capability: "do x", persona_ref: "P",
      traps: [], judge: { rules: ["r1"] },
      baseline: { build: "PASS", tests: "PASS", judge: "NON-COMPLIANT" },
      target: { build: "PASS", tests: "PASS", judge: "conformant" },
    }),
  );
  fs.writeFileSync(path.join(s, "task.md"), "do the task");
  fs.writeFileSync(path.join(s, "SOLUTION.md"), "the solution");
  // a non-scenario dir that must be ignored (no numeric prefix)
  fs.mkdirSync(path.join(root, "notes"));
  return root;
}

describe("scenarios", () => {
  it("lists numeric-prefixed scenarios with fixture + scenario.json", () => {
    const dir = fixtureDir();
    const list = listScenarios(dir);
    expect(list.map((s) => s.id)).toEqual(["01-demo"]);
    expect(list[0].judge.rules).toEqual(["r1"]);
  });

  it("isKnownScenario gates ids", () => {
    const dir = fixtureDir();
    expect(isKnownScenario("01-demo", dir)).toBe(true);
    expect(isKnownScenario("99-nope", dir)).toBe(false);
    expect(isKnownScenario("../etc", dir)).toBe(false);
  });

  it("loadScenario returns config + task + solution, null for unknown", () => {
    const dir = fixtureDir();
    const d = loadScenario("01-demo", dir);
    expect(d?.task).toBe("do the task");
    expect(d?.solution).toBe("the solution");
    expect(d?.config.title).toBe("Demo");
    expect(loadScenario("99-nope", dir)).toBeNull();
  });

  it("validateConfig accepts a good config and rejects bad shapes", () => {
    const good = {
      id: "01-demo", title: "Demo", tier: 1, capability: "x", persona_ref: "P",
      traps: [], judge: { rules: ["r1"] },
      baseline: { build: "PASS", tests: "PASS", judge: "NON-COMPLIANT" },
      target: { build: "PASS", tests: "PASS", judge: "conformant" },
    };
    expect(validateConfig(good)).toBeNull();
    expect(validateConfig({ ...good, tier: "1" })).toMatch(/tier/);
    expect(validateConfig({ ...good, judge: {} })).toMatch(/judge.rules/);
    expect(validateConfig({ ...good, traps: "none" })).toMatch(/traps/);
    expect(validateConfig(null)).toMatch(/object/);
  });
});
```

- [ ] **Step 2: Run tests, verify they fail**

Run: `cd sims/dashboard && pnpm test scenarios`
Expected: FAIL ("Cannot find module './scenarios'").

- [ ] **Step 3: Add `scenariosDir` to `lib/paths.ts`**

Append to `sims/dashboard/lib/paths.ts`:
```ts
export function scenariosDir(): string {
  return process.env.FISKALY_SCENARIOS_DIR ?? path.resolve(process.cwd(), "..", "scenarios");
}
```

- [ ] **Step 4: Extend `lib/types.ts`**

Change `export type RunStatus = "running" | "done";` to:
```ts
export type RunStatus = "running" | "done" | "cancelled";
```
Append:
```ts
export interface Verdicts {
  build: string;
  tests: string;
  judge: string;
}

export interface ScenarioConfig {
  id: string;
  title: string;
  tier: number;
  capability: string;
  persona_ref: string;
  traps: string[];
  judge: { rules: string[] };
  baseline: Verdicts;
  target: Verdicts;
}

export interface ScenarioDetail {
  config: ScenarioConfig;
  task: string;
  solution: string;
}
```

- [ ] **Step 5: Create `lib/scenarios.ts`**

```ts
import fs from "node:fs";
import path from "node:path";
import { scenariosDir } from "@/lib/paths";
import type { ScenarioConfig, ScenarioDetail } from "@/lib/types";

const NUMERIC_PREFIX = /^[0-9]/;

function readConfig(dir: string): ScenarioConfig | null {
  try {
    return JSON.parse(fs.readFileSync(path.join(dir, "scenario.json"), "utf8")) as ScenarioConfig;
  } catch {
    return null;
  }
}

export function listScenarios(dir = scenariosDir()): ScenarioConfig[] {
  let entries: string[];
  try {
    entries = fs.readdirSync(dir);
  } catch {
    return [];
  }
  const out: ScenarioConfig[] = [];
  for (const name of entries) {
    if (!NUMERIC_PREFIX.test(name)) continue;
    const d = path.join(dir, name);
    try {
      if (!fs.statSync(d).isDirectory()) continue;
    } catch {
      continue;
    }
    if (!fs.existsSync(path.join(d, "fixture")) || !fs.existsSync(path.join(d, "scenario.json"))) continue;
    const cfg = readConfig(d);
    if (cfg) out.push(cfg);
  }
  out.sort((a, b) => (a.id < b.id ? -1 : 1));
  return out;
}

export function isKnownScenario(id: string, dir = scenariosDir()): boolean {
  return listScenarios(dir).some((s) => s.id === id);
}

export function loadScenario(id: string, dir = scenariosDir()): ScenarioDetail | null {
  if (!isKnownScenario(id, dir)) return null;
  const d = path.join(dir, id);
  const config = readConfig(d);
  if (!config) return null;
  const read = (f: string): string => {
    try {
      return fs.readFileSync(path.join(d, f), "utf8");
    } catch {
      return "";
    }
  };
  return { config, task: read("task.md"), solution: read("SOLUTION.md") };
}

function isStrArray(v: unknown): v is string[] {
  return Array.isArray(v) && v.every((x) => typeof x === "string");
}

function verdictsError(v: unknown, field: string): string | null {
  if (typeof v !== "object" || v === null) return `${field} must be an object`;
  const o = v as Record<string, unknown>;
  for (const k of ["build", "tests", "judge"]) {
    if (typeof o[k] !== "string") return `${field}.${k} must be a string`;
  }
  return null;
}

export function validateConfig(obj: unknown): string | null {
  if (typeof obj !== "object" || obj === null) return "config must be an object";
  const c = obj as Record<string, unknown>;
  if (typeof c.id !== "string") return "id must be a string";
  if (typeof c.title !== "string") return "title must be a string";
  if (typeof c.tier !== "number") return "tier must be a number";
  if (typeof c.capability !== "string") return "capability must be a string";
  if (typeof c.persona_ref !== "string") return "persona_ref must be a string";
  if (!isStrArray(c.traps)) return "traps must be an array of strings";
  const judge = c.judge as Record<string, unknown> | undefined;
  if (typeof judge !== "object" || judge === null || !isStrArray(judge.rules))
    return "judge.rules must be an array of strings";
  return verdictsError(c.baseline, "baseline") ?? verdictsError(c.target, "target");
}
```

- [ ] **Step 6: Run tests, verify pass**

Run: `cd sims/dashboard && pnpm test scenarios`
Expected: PASS (4 tests).

- [ ] **Step 7: Commit**

```bash
git add sims/dashboard/lib/paths.ts sims/dashboard/lib/types.ts sims/dashboard/lib/scenarios.ts sims/dashboard/lib/scenarios.test.ts
git commit -m "dashboard: scenarios read lib (list/load/validate) + types"
```

---

### Task 2: Global nav + scenarios list page

**Files:**
- Create: `sims/dashboard/components/Nav.tsx`
- Modify: `sims/dashboard/app/layout.tsx` (render `<Nav/>`)
- Create: `sims/dashboard/app/scenarios/page.tsx`

**Interfaces:**
- Consumes: `listScenarios()` (Task 1).
- Produces: `<Nav/>`; route `/scenarios`.

- [ ] **Step 1: Create `components/Nav.tsx`**

```tsx
"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/utils";

const LINKS = [
  { href: "/", label: "runs", match: (p: string) => p === "/" || p.startsWith("/run/") },
  { href: "/scenarios", label: "scenarios", match: (p: string) => p.startsWith("/scenarios") },
];

export function Nav() {
  const pathname = usePathname();
  return (
    <header className="border-b border-border">
      <div className="mx-auto flex h-12 w-full max-w-6xl items-center gap-6 px-8">
        <span className="text-sm font-semibold tracking-tight">fiskaly eval</span>
        <nav className="flex items-center gap-4 text-sm">
          {LINKS.map((l) => (
            <Link
              key={l.href}
              href={l.href}
              className={cn(
                "transition-colors",
                l.match(pathname) ? "text-foreground" : "text-muted-foreground hover:text-foreground",
              )}
            >
              {l.label}
            </Link>
          ))}
        </nav>
      </div>
    </header>
  );
}
```

- [ ] **Step 2: Render `<Nav/>` in `app/layout.tsx`**

Add `import { Nav } from "@/components/Nav";` and change the body to:
```tsx
<body className="min-h-full flex flex-col">
  <Nav />
  {children}
</body>
```

- [ ] **Step 3: Create `app/scenarios/page.tsx`**

```tsx
import Link from "next/link";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { cn } from "@/lib/utils";
import { listScenarios } from "@/lib/scenarios";

export const dynamic = "force-dynamic";

const HEAD = "h-9 px-3 text-[0.7rem] font-medium uppercase tracking-[0.08em] text-muted-foreground";
const CELL = "px-3 py-2.5";

export default function ScenariosPage() {
  const scenarios = listScenarios();
  return (
    <main className="mx-auto w-full max-w-6xl px-8 py-12">
      <header className="mb-8 border-b border-border pb-5">
        <h1 className="text-2xl font-semibold tracking-tight">scenarios</h1>
        <p className="mt-1 text-sm text-muted-foreground">{scenarios.length} scenarios</p>
      </header>
      <Table className="text-sm">
        <TableHeader>
          <TableRow className="border-border hover:bg-transparent">
            <TableHead className={HEAD}>id</TableHead>
            <TableHead className={HEAD}>title</TableHead>
            <TableHead className={cn(HEAD, "text-right")}>tier</TableHead>
            <TableHead className={cn(HEAD, "text-right")}>rules</TableHead>
            <TableHead className={cn(HEAD, "text-right")}>traps</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {scenarios.length === 0 && (
            <TableRow>
              <TableCell colSpan={5} className="h-24 text-center text-muted-foreground">
                no scenarios found
              </TableCell>
            </TableRow>
          )}
          {scenarios.map((s) => (
            <TableRow key={s.id} className="group border-border">
              <TableCell className={CELL}>
                <Link
                  href={`/scenarios/${s.id}`}
                  className="font-mono text-foreground underline-offset-4 decoration-muted-foreground/30 group-hover:underline"
                >
                  {s.id}
                </Link>
              </TableCell>
              <TableCell className={cn(CELL, "font-medium")}>{s.title}</TableCell>
              <TableCell className={cn(CELL, "text-right font-mono tabular-nums text-muted-foreground")}>{s.tier}</TableCell>
              <TableCell className={cn(CELL, "text-right font-mono tabular-nums text-muted-foreground")}>{s.judge.rules.length}</TableCell>
              <TableCell className={cn(CELL, "text-right font-mono tabular-nums text-muted-foreground")}>{s.traps.length}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </main>
  );
}
```

- [ ] **Step 4: Verify build + render**

Run: `cd sims/dashboard && pnpm build`
Expected: compiled successfully, TypeScript no errors, route `/scenarios` listed.
Then with the dev server running, screenshot `http://localhost:3000/scenarios` via playwriter and confirm the nav bar + scenarios table render with no console errors.

- [ ] **Step 5: Commit**

```bash
git add sims/dashboard/components/Nav.tsx sims/dashboard/app/layout.tsx sims/dashboard/app/scenarios/page.tsx
git commit -m "dashboard: global nav + scenarios list page"
```

---

### Task 3: Run menu + runScenario action (replace trigger)

**Files:**
- Modify: `sims/dashboard/app/actions.ts` (replace `triggerRun` with `runScenario`)
- Create: `sims/dashboard/components/RunMenu.tsx`
- Delete: `sims/dashboard/components/TriggerButton.tsx`
- Modify: `sims/dashboard/app/page.tsx` (use `RunMenu`)

**Interfaces:**
- Consumes: `isKnownScenario`, `listScenarios` (Task 1).
- Produces: `runScenario(scenarioId: string): Promise<void>`; `<RunMenu scenarios={...} />`.

- [ ] **Step 1: Replace the action in `app/actions.ts`**

Change the file to (keeping the spawn behavior, adding validation):
```ts
"use server";

import { spawn } from "node:child_process";
import { runnerDir } from "@/lib/paths";
import { isKnownScenario } from "@/lib/scenarios";

// Spawn the eval detached so it outlives this request; the new run dir shows up
// on the next list render.
export async function runScenario(scenarioId: string): Promise<void> {
  if (!isKnownScenario(scenarioId)) throw new Error(`unknown scenario: ${scenarioId}`);
  const child = spawn("go", ["run", ".", "run", scenarioId], {
    cwd: runnerDir(),
    detached: true,
    stdio: "ignore",
  });
  child.unref();
}
```
(`cancelRun` and `saveScenario` are added in later tasks.)

- [ ] **Step 2: Ground the base-ui Menu API**

Use Context7 (`/websites/base-ui` or the installed `sims/dashboard/node_modules/@base-ui/react/menu`) to confirm the Menu parts and props: `Menu.Root`, `Menu.Trigger` (and its `render` prop for composing with `Button`), `Menu.Portal`, `Menu.Positioner` (offset/align props), `Menu.Popup`, `Menu.Item` (and its click/select handler prop name). Do not guess prop names.

- [ ] **Step 3: Create `components/RunMenu.tsx`**

Adapt the structure below to the grounded API (the `render`-prop pattern matches the repo's `components/ui/button.tsx`):
```tsx
"use client";

import { Menu } from "@base-ui/react/menu";
import { ChevronDown, Play } from "lucide-react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { runScenario } from "@/app/actions";
import type { ScenarioConfig } from "@/lib/types";

export function RunMenu({ scenarios }: { scenarios: ScenarioConfig[] }) {
  const router = useRouter();
  return (
    <Menu.Root>
      <Menu.Trigger render={<Button variant="outline" size="sm" />}>
        <Play className="size-3.5" />
        run
        <ChevronDown className="size-3.5" />
      </Menu.Trigger>
      <Menu.Portal>
        <Menu.Positioner sideOffset={6} align="end">
          <Menu.Popup className="z-50 max-h-80 min-w-64 overflow-auto rounded-lg border border-border bg-popover p-1 text-popover-foreground shadow-md">
            {scenarios.map((s) => (
              <Menu.Item
                key={s.id}
                className="flex cursor-pointer items-center gap-3 rounded-md px-2.5 py-2 text-sm outline-none data-[highlighted]:bg-muted"
                onClick={async () => {
                  await runScenario(s.id);
                  router.refresh();
                }}
              >
                <span className="font-mono text-xs text-muted-foreground">{s.id}</span>
                <span className="truncate">{s.title}</span>
              </Menu.Item>
            ))}
          </Menu.Popup>
        </Menu.Positioner>
      </Menu.Portal>
    </Menu.Root>
  );
}
```
Note: confirm `--popover`/`--popover-foreground` tokens exist in `globals.css` (they do, in `@theme inline`).

- [ ] **Step 4: Use `RunMenu` in `app/page.tsx`**

Replace the `TriggerButton` import/usage:
```tsx
import { listScenarios } from "@/lib/scenarios";
import { RunMenu } from "@/components/RunMenu";
```
In `Home()`: `const scenarios = listScenarios();` and in the header replace `<TriggerButton />` with `<RunMenu scenarios={scenarios} />`.

- [ ] **Step 5: Delete `components/TriggerButton.tsx`**

```bash
git rm sims/dashboard/components/TriggerButton.tsx
```

- [ ] **Step 6: Verify build + run a scenario via the UI**

Run: `cd sims/dashboard && pnpm build` → expect clean.
With the dev server + Docker up, open the home page via playwriter, click `run ▾`, pick a scenario, and confirm a new `running…` row appears for that scenario id.

- [ ] **Step 7: Commit**

```bash
git add sims/dashboard/app/actions.ts sims/dashboard/components/RunMenu.tsx sims/dashboard/app/page.tsx
git commit -m "dashboard: run menu with scenario picker (replaces hardcoded trigger)"
```

---

### Task 4: Scenario editor + saveScenario action

**Files:**
- Modify: `sims/dashboard/app/actions.ts` (add `saveScenario`)
- Create: `sims/dashboard/components/ScenarioEditor.tsx`
- Create: `sims/dashboard/app/scenarios/[id]/page.tsx`

**Interfaces:**
- Consumes: `loadScenario`, `isKnownScenario`, `validateConfig` (Task 1); `ScenarioConfig`, `ScenarioDetail` (Task 1).
- Produces: `saveScenario(id, { config, task, solution }): Promise<void>`; `<ScenarioEditor detail={ScenarioDetail} />`; route `/scenarios/[id]`.

- [ ] **Step 1: Add `saveScenario` to `app/actions.ts`**

Add imports `import fs from "node:fs"; import path from "node:path"; import { scenariosDir } from "@/lib/paths"; import { isKnownScenario, validateConfig } from "@/lib/scenarios"; import type { ScenarioConfig } from "@/lib/types";` and append:
```ts
export async function saveScenario(
  id: string,
  data: { config: ScenarioConfig; task: string; solution: string },
): Promise<void> {
  if (!isKnownScenario(id)) throw new Error(`unknown scenario: ${id}`);
  const err = validateConfig(data.config);
  if (err) throw new Error(`invalid scenario config: ${err}`);
  const dir = path.join(scenariosDir(), id);
  fs.writeFileSync(path.join(dir, "scenario.json"), JSON.stringify(data.config, null, 2) + "\n");
  fs.writeFileSync(path.join(dir, "task.md"), data.task);
  fs.writeFileSync(path.join(dir, "SOLUTION.md"), data.solution);
}
```

- [ ] **Step 2: Create `components/ScenarioEditor.tsx`**

```tsx
"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Plus, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { saveScenario } from "@/app/actions";
import type { ScenarioDetail, Verdicts } from "@/lib/types";

const LABEL = "text-[0.7rem] font-medium uppercase tracking-[0.08em] text-muted-foreground";
const INPUT =
  "w-full rounded-md border border-border bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50";
const BUILD_TESTS = ["PASS", "FAIL"];
const JUDGE = ["conformant", "NON-COMPLIANT"];

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block space-y-1.5">
      <span className={LABEL}>{label}</span>
      {children}
    </label>
  );
}

function StringList({ label, items, onChange }: { label: string; items: string[]; onChange: (v: string[]) => void }) {
  const [draft, setDraft] = useState("");
  return (
    <div className="space-y-1.5">
      <span className={LABEL}>{label}</span>
      <div className="flex flex-wrap gap-1.5">
        {items.map((item, i) => (
          <span key={i} className="inline-flex items-center gap-1.5 rounded-md border border-border px-2 py-1 font-mono text-xs">
            {item}
            <button type="button" onClick={() => onChange(items.filter((_, j) => j !== i))} className="text-muted-foreground hover:text-danger">
              <X className="size-3" />
            </button>
          </span>
        ))}
      </div>
      <div className="flex gap-2">
        <input className={INPUT} value={draft} onChange={(e) => setDraft(e.target.value)} placeholder={`add ${label}…`} />
        <Button type="button" variant="outline" size="sm" onClick={() => { if (draft.trim()) { onChange([...items, draft.trim()]); setDraft(""); } }}>
          <Plus className="size-3.5" />
        </Button>
      </div>
    </div>
  );
}

function VerdictRow({ label, value, onChange }: { label: string; value: Verdicts; onChange: (v: Verdicts) => void }) {
  const sel = (k: keyof Verdicts, opts: string[]) => (
    <select className={INPUT} value={value[k]} onChange={(e) => onChange({ ...value, [k]: e.target.value })}>
      {opts.map((o) => <option key={o} value={o}>{o}</option>)}
    </select>
  );
  return (
    <div className="space-y-1.5">
      <span className={LABEL}>{label}</span>
      <div className="grid grid-cols-3 gap-2">
        {sel("build", BUILD_TESTS)}
        {sel("tests", BUILD_TESTS)}
        {sel("judge", JUDGE)}
      </div>
    </div>
  );
}

export function ScenarioEditor({ detail }: { detail: ScenarioDetail }) {
  const router = useRouter();
  const [config, setConfig] = useState(detail.config);
  const [task, setTask] = useState(detail.task);
  const [solution, setSolution] = useState(detail.solution);
  const [state, setState] = useState<{ kind: "idle" | "saving" | "saved" | "error"; msg?: string }>({ kind: "idle" });

  async function save() {
    setState({ kind: "saving" });
    try {
      await saveScenario(config.id, { config, task, solution });
      setState({ kind: "saved" });
      router.refresh();
    } catch (e) {
      setState({ kind: "error", msg: e instanceof Error ? e.message : String(e) });
    }
  }

  return (
    <div className="space-y-8">
      <div className="grid grid-cols-1 gap-5 sm:grid-cols-2">
        <Field label="title">
          <input className={INPUT} value={config.title} onChange={(e) => setConfig({ ...config, title: e.target.value })} />
        </Field>
        <Field label="tier">
          <input className={INPUT} type="number" value={config.tier} onChange={(e) => setConfig({ ...config, tier: Number(e.target.value) })} />
        </Field>
        <Field label="capability">
          <input className={INPUT} value={config.capability} onChange={(e) => setConfig({ ...config, capability: e.target.value })} />
        </Field>
        <Field label="persona_ref">
          <input className={INPUT} value={config.persona_ref} onChange={(e) => setConfig({ ...config, persona_ref: e.target.value })} />
        </Field>
      </div>

      <StringList label="judge rules" items={config.judge.rules} onChange={(rules) => setConfig({ ...config, judge: { rules } })} />
      <StringList label="traps" items={config.traps} onChange={(traps) => setConfig({ ...config, traps })} />

      <div className="grid grid-cols-1 gap-5 sm:grid-cols-2">
        <VerdictRow label="baseline" value={config.baseline} onChange={(baseline) => setConfig({ ...config, baseline })} />
        <VerdictRow label="target" value={config.target} onChange={(target) => setConfig({ ...config, target })} />
      </div>

      <Field label="task.md">
        <textarea className={cn(INPUT, "min-h-40 font-mono text-xs leading-relaxed")} value={task} onChange={(e) => setTask(e.target.value)} />
      </Field>
      <Field label="SOLUTION.md">
        <textarea className={cn(INPUT, "min-h-40 font-mono text-xs leading-relaxed")} value={solution} onChange={(e) => setSolution(e.target.value)} />
      </Field>

      <div className="flex items-center gap-4 border-t border-border pt-5">
        <Button onClick={save} disabled={state.kind === "saving"}>
          {state.kind === "saving" ? "saving…" : "save"}
        </Button>
        {state.kind === "saved" && <span className="text-xs text-success">saved</span>}
        {state.kind === "error" && <span className="text-xs text-danger">{state.msg}</span>}
      </div>
    </div>
  );
}
```

- [ ] **Step 3: Create `app/scenarios/[id]/page.tsx`**

```tsx
import { notFound } from "next/navigation";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { loadScenario } from "@/lib/scenarios";
import { ScenarioEditor } from "@/components/ScenarioEditor";

export const dynamic = "force-dynamic";

export default async function ScenarioEditPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const detail = loadScenario(id);
  if (!detail) notFound();
  return (
    <main className="mx-auto w-full max-w-3xl px-8 py-12">
      <Link href="/scenarios" className="inline-flex items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground">
        <ArrowLeft className="size-3.5" />
        scenarios
      </Link>
      <h1 className="mt-3 mb-8 font-mono text-2xl font-semibold tracking-tight">{detail.config.id}</h1>
      <ScenarioEditor detail={detail} />
    </main>
  );
}
```

- [ ] **Step 4: Verify build + edit round-trip**

Run: `cd sims/dashboard && pnpm build` → expect clean.
Pick a throwaway field: via playwriter open `/scenarios/<id>`, change the title, click save, confirm "saved". Then verify on disk: `git diff sims/scenarios/<id>/scenario.json` shows the title change. Revert with `git checkout sims/scenarios/<id>/scenario.json` after confirming (so the eval suite is untouched).

- [ ] **Step 5: Commit**

```bash
git add sims/dashboard/app/actions.ts sims/dashboard/components/ScenarioEditor.tsx "sims/dashboard/app/scenarios/[id]/page.tsx"
git commit -m "dashboard: scenario editor (config + task + solution) with saveScenario"
```

---

### Task 5: Runner records run.json + names the container

**Files:**
- Modify: `sims/runner/docker.go` (add `containerName`; `--name` on `docker run`)
- Modify: `sims/runner/artifacts.go` (add `runHandle` + `writeRunHandle`)
- Modify: `sims/runner/run.go` (call `writeRunHandle` after `prepareRun`)
- Test: `sims/runner/run_test.go` (add cases)

**Interfaces:**
- Produces: `containerName(runPath string) string` → `"fiskaly-eval-" + base(runPath)`; `writeRunHandle(runPath string) error` writing `run.json` `{pid, pgid, container}`.

- [ ] **Step 1: Write failing Go tests**

Append to `sims/runner/run_test.go`:
```go
func TestContainerName(t *testing.T) {
	if got := containerName("/x/y/run.AbC.123"); got != "fiskaly-eval-run.AbC.123" {
		t.Errorf("containerName = %q", got)
	}
}

func TestWriteRunHandle(t *testing.T) {
	rp := filepath.Join(t.TempDir(), "run.ZZZ")
	if err := os.MkdirAll(rp, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeRunHandle(rp); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(rp, "run.json"))
	if err != nil {
		t.Fatal(err)
	}
	var h runHandle
	if err := json.Unmarshal(data, &h); err != nil {
		t.Fatal(err)
	}
	if h.Container != "fiskaly-eval-run.ZZZ" {
		t.Errorf("container = %q", h.Container)
	}
	if h.PID == 0 || h.PGID == 0 {
		t.Errorf("pid/pgid not set: %+v", h)
	}
}
```
Ensure `run_test.go` imports `encoding/json`, `os`, `path/filepath`, `testing` (add any missing).

- [ ] **Step 2: Run tests, verify fail**

Run: `cd sims/runner && go test ./... -run 'ContainerName|WriteRunHandle'`
Expected: FAIL (undefined: containerName / writeRunHandle / runHandle).

- [ ] **Step 3: Add `containerName` to `docker.go`**

Add (top-level):
```go
// containerName derives a deterministic, per-run container name so a run can be
// cancelled with `docker kill` even though it was spawned detached.
func containerName(runPath string) string {
	return "fiskaly-eval-" + filepath.Base(runPath)
}
```
In `dockerAgent.run`, add `--name` to the `docker run` invocation, right after `"--rm",`:
```go
	run := exec.Command("docker", "run", "--rm",
		"--name", containerName(rd.path),
		"-e", "CLAUDE_CODE_OAUTH_TOKEN="+cfg.token,
```
(`path/filepath` is already imported in docker.go.)

- [ ] **Step 4: Add `runHandle` + `writeRunHandle` to `artifacts.go`**

Add `"syscall"` to the imports, then:
```go
// runHandle is the cancellation handle the dashboard reads: the process group to
// signal and the container to docker-kill. Written before the long docker work.
type runHandle struct {
	PID       int    `json:"pid"`
	PGID      int    `json:"pgid"`
	Container string `json:"container"`
}

func writeRunHandle(runPath string) error {
	pgid, err := syscall.Getpgid(0)
	if err != nil {
		return fmt.Errorf("getpgid: %w", err)
	}
	data, err := json.Marshal(runHandle{PID: os.Getpid(), PGID: pgid, Container: containerName(runPath)})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(runPath, "run.json"), append(data, '\n'), 0o644)
}
```

- [ ] **Step 5: Call `writeRunHandle` in `run.go`**

In `runScenario`, immediately after the successful `prepareRun`:
```go
	rd, err := prepareRun(runsBase, s, cfg)
	if err != nil {
		return scenarioResult{}, fmt.Errorf("prepareRun: %w", err)
	}
	if err := writeRunHandle(rd.path); err != nil {
		return scenarioResult{}, fmt.Errorf("writeRunHandle: %w", err)
	}
```

- [ ] **Step 6: Run tests + full suite**

Run: `cd sims/runner && go test ./...`
Expected: PASS (new cases + existing suite green).

- [ ] **Step 7: Commit**

```bash
git add sims/runner/docker.go sims/runner/artifacts.go sims/runner/run.go sims/runner/run_test.go
git commit -m "runner: record run.json (pgid + container) and name the run container"
```

---

### Task 6: cancelRun action + cancelled status + cancel UI

**Files:**
- Modify: `sims/dashboard/app/actions.ts` (add `cancelRun`)
- Modify: `sims/dashboard/lib/runs.ts` (cancelled status)
- Test: `sims/dashboard/lib/runs.test.ts` (cancelled case)
- Create: `sims/dashboard/components/CancelButton.tsx`
- Modify: `sims/dashboard/components/RunTable.tsx` (cancel control + cancelled dot)

**Interfaces:**
- Consumes: `runsDir()` (existing); `run.json` from Task 5.
- Produces: `cancelRun(runId: string): Promise<void>`; `<CancelButton runId={string} />`.

- [ ] **Step 1: Write failing test for cancelled status**

Add to `sims/dashboard/lib/runs.test.ts` (follow the file's existing temp-dir helper style):
```ts
import { describe, it, expect } from "vitest";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { summarizeRun } from "./runs";

describe("summarizeRun cancelled", () => {
  it("reports cancelled when the marker exists, even with judge.txt", () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "run."));
    fs.writeFileSync(path.join(dir, "meta.json"), JSON.stringify({ scenario: "01-demo", harness: "docker" }));
    fs.writeFileSync(path.join(dir, "judge.txt"), "VERDICT: conformant\n");
    fs.writeFileSync(path.join(dir, "cancelled"), "2026-06-18T00:00:00Z\n");
    expect(summarizeRun(dir).status).toBe("cancelled");
  });
});
```

- [ ] **Step 2: Run, verify fail**

Run: `cd sims/dashboard && pnpm test runs`
Expected: FAIL (status is "done", not "cancelled").

- [ ] **Step 3: Add cancelled check in `lib/runs.ts`**

In `summarizeRun`, immediately before `const judge = readFile(path.join(dir, "judge.txt"));`:
```ts
  if (fs.existsSync(path.join(dir, "cancelled"))) {
    s.status = "cancelled";
    return s;
  }
```
(`fs` is already imported in `runs.ts`.)

- [ ] **Step 4: Run, verify pass**

Run: `cd sims/dashboard && pnpm test runs`
Expected: PASS.

- [ ] **Step 5: Add `cancelRun` to `app/actions.ts`**

Add `import { execFile } from "node:child_process";` (extend the existing `node:child_process` import) and `runsDir` to the `@/lib/paths` import, then append:
```ts
const RUN_ID = /^run\.[A-Za-z0-9.]+$/;

export async function cancelRun(runId: string): Promise<void> {
  if (!RUN_ID.test(runId)) throw new Error(`invalid run id: ${runId}`);
  const dir = path.join(runsDir(), runId);
  if (!fs.existsSync(dir)) throw new Error(`no such run: ${runId}`);
  // Marker first: the UI flips to cancelled even if a kill below fails.
  fs.writeFileSync(path.join(dir, "cancelled"), new Date().toISOString() + "\n");
  let handle: { pgid?: number; container?: string };
  try {
    handle = JSON.parse(fs.readFileSync(path.join(dir, "run.json"), "utf8"));
  } catch {
    return; // pre-existing/zombie run with no handle: marker only.
  }
  if (typeof handle.pgid === "number" && handle.pgid > 1) {
    for (const sig of ["SIGTERM", "SIGKILL"] as const) {
      try {
        process.kill(-handle.pgid, sig); // negative pid = process group
      } catch {
        // already gone
      }
    }
  }
  if (handle.container) {
    execFile("docker", ["kill", handle.container], () => {}); // best-effort
  }
}
```

- [ ] **Step 6: Create `components/CancelButton.tsx`**

```tsx
"use client";

import { X } from "lucide-react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { cancelRun } from "@/app/actions";

export function CancelButton({ runId }: { runId: string }) {
  const router = useRouter();
  return (
    <Button
      variant="ghost"
      size="xs"
      className="text-muted-foreground hover:text-danger"
      onClick={async () => {
        await cancelRun(runId);
        router.refresh();
      }}
    >
      <X className="size-3" />
      cancel
    </Button>
  );
}
```

- [ ] **Step 7: Wire into `components/RunTable.tsx`**

Add `import { CancelButton } from "@/components/CancelButton";`. Replace the single `r.status === "running" ? (...) : (...)` branch with a three-way branch:
```tsx
            {r.status === "running" ? (
              <TableCell colSpan={5} className={CELL}>
                <div className="flex items-center justify-between gap-2">
                  <span className="inline-flex items-center gap-1.5 text-xs font-medium text-warning">
                    <span className="size-1.5 animate-pulse rounded-full bg-warning" aria-hidden />
                    running
                  </span>
                  <CancelButton runId={r.id} />
                </div>
              </TableCell>
            ) : r.status === "cancelled" ? (
              <TableCell colSpan={5} className={CELL}>
                <span className="inline-flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
                  <span className="size-1.5 rounded-full bg-muted-foreground/50" aria-hidden />
                  cancelled
                </span>
              </TableCell>
            ) : (
              <>
                <TableCell className={CELL}><JudgeBadge value={r.build} /></TableCell>
                <TableCell className={CELL}><JudgeBadge value={r.tests} /></TableCell>
                <TableCell className={CELL}><JudgeBadge value={r.judge} /></TableCell>
                <TableCell className={cn(CELL, "text-right font-mono tabular-nums text-muted-foreground")}>{r.turns}</TableCell>
                <TableCell className={cn(CELL, "text-right font-mono tabular-nums")}>{r.cost}</TableCell>
              </>
            )}
```

- [ ] **Step 8: Verify build + unit tests**

Run: `cd sims/dashboard && pnpm build && pnpm test`
Expected: clean build, all vitest suites pass.

- [ ] **Step 9: Commit**

```bash
git add sims/dashboard/app/actions.ts sims/dashboard/lib/runs.ts sims/dashboard/lib/runs.test.ts sims/dashboard/components/CancelButton.tsx sims/dashboard/components/RunTable.tsx
git commit -m "dashboard: cancel in-progress runs (kill group + container, mark cancelled)"
```

---

### Task 7: Full end-to-end verification

**Files:** none (verification only).

- [ ] **Step 1: Build + all unit tests**

Run: `cd sims/dashboard && pnpm build && pnpm test` → clean.
Run: `cd sims/runner && go test ./...` → clean.

- [ ] **Step 2: Browser e2e (Docker up, dev server running) via playwriter**

- Home: `run ▾` lists scenarios; pick `01-zero-to-receipt`; a new `running…` row appears with a `cancel` control.
- Click `cancel`; within a refresh the row shows `cancelled`; confirm on disk the run dir has a `cancelled` marker, and `docker ps` no longer lists `fiskaly-eval-<runId>`.
- Scenarios: `/scenarios` lists scenarios; open one; change a field; save; confirm "saved" and `git diff sims/scenarios/<id>/scenario.json`; then `git checkout` that file to leave the eval suite pristine.
- Confirm light + dark both render cleanly with no console errors.

- [ ] **Step 3: Report results** (commands + actual output) per the verification rule. No commit (verification task).

## Notes for the implementer
- `process.kill(-pgid, ...)` signals the whole detached group (runner + go-run + docker client); the container is separate on the daemon, hence the explicit `docker kill`. Never signal `pgid <= 1`.
- Cancel is idempotent and best-effort: a missing `run.json` (older runs) means marker-only — this is also how you clear the two pre-existing zombie `running…` rows.
- Leave `components/ui/badge.tsx` in place even though unused (shadcn kit primitive).
