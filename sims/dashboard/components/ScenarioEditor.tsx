"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Plus, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { saveScenario } from "@/app/actions";
import type { ScenarioDetail, Expectation, ToolReq } from "@/lib/types";

const LABEL = "text-[0.7rem] font-medium uppercase tracking-[0.08em] text-muted-foreground";
const INPUT =
  "w-full rounded-md border border-border bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50";

// The tools a trajectory check can assert: the fiskaly MCP docs tools plus the
// built-in code tools that appear in agent transcripts. The judge matches these
// bare names against the MCP-prefixed transcript names.
const TOOL_NAMES = [
  "search_fiskaly_docs",
  "fetch_fiskaly_doc",
  "Read",
  "Edit",
  "Write",
  "MultiEdit",
  "Bash",
  "Glob",
  "Grep",
  "Task",
];

// The corpus doc IDs (mcp/corpus/index.json) a docsFetched check can require.
const DOC_IDS = [
  "probe:auth-and-headers",
  "probe:scoped-subject",
  "probe:provisioning",
  "probe:records-flow",
  "probe:money-model",
];

// The scenario tiers in use.
const TIERS = [1, 2, 3];

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block space-y-1.5">
      <span className={LABEL}>{label}</span>
      {children}
    </label>
  );
}

export function ScenarioEditor({ detail }: { detail: ScenarioDetail }) {
  const router = useRouter();
  const [config, setConfig] = useState(detail.config);
  const [task, setTask] = useState(detail.task);
  const [solution, setSolution] = useState(detail.solution);
  const [state, setState] = useState<{ kind: "idle" | "saving" | "saved" | "error"; msg?: string }>({ kind: "idle" });

  const checks = config.judge.checks;
  const expectations = config.judge.expectations;

  function setChecks(patch: Partial<typeof checks>) {
    setConfig({ ...config, judge: { ...config.judge, checks: { ...checks, ...patch } } });
  }

  function setExpectations(next: Expectation[]) {
    setConfig({ ...config, judge: { ...config.judge, expectations: next } });
  }

  function addExpectation() {
    setExpectations([...expectations, { id: "", expectation: "" }]);
  }

  function removeExpectation(i: number) {
    setExpectations(expectations.filter((_, j) => j !== i));
  }

  function patchExpectation(i: number, patch: Partial<Expectation>) {
    setExpectations(expectations.map((e, j) => (j === i ? { ...e, ...patch } : e)));
  }

  function addToolReq() {
    setChecks({ toolsCalled: [...(checks.toolsCalled ?? []), { name: TOOL_NAMES[0], min: 1 }] });
  }

  function removeToolReq(i: number) {
    setChecks({ toolsCalled: (checks.toolsCalled ?? []).filter((_, j) => j !== i) });
  }

  function patchToolReq(i: number, patch: Partial<ToolReq>) {
    setChecks({ toolsCalled: (checks.toolsCalled ?? []).map((r, j) => (j === i ? { ...r, ...patch } : r)) });
  }

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
          <select
            className={INPUT}
            value={config.tier}
            onChange={(e) => setConfig({ ...config, tier: Number(e.target.value) })}
          >
            {(TIERS.includes(config.tier) ? TIERS : [config.tier, ...TIERS]).map((t) => (
              <option key={t} value={t}>
                {t}
              </option>
            ))}
          </select>
        </Field>
        <Field label="capability">
          <input className={INPUT} value={config.capability} onChange={(e) => setConfig({ ...config, capability: e.target.value })} />
        </Field>
        <Field label="persona_ref">
          <input className={INPUT} value={config.persona_ref} onChange={(e) => setConfig({ ...config, persona_ref: e.target.value })} />
        </Field>
      </div>

      {/* checks */}
      <div className="space-y-4 rounded-lg border border-border p-4">
        <span className={LABEL}>checks</span>

        <label className="flex items-center gap-2.5 text-sm">
          <input
            type="checkbox"
            checked={checks.groundedBeforeWrite ?? false}
            onChange={(e) => setChecks({ groundedBeforeWrite: e.target.checked || undefined })}
            className="rounded border-border"
          />
          <span>grounded before write</span>
        </label>

        <div className="space-y-1.5">
          <span className={LABEL}>tools called</span>
          {(checks.toolsCalled ?? []).map((req, i) => (
            <div key={i} className="flex items-center gap-2">
              <select
                className={cn(INPUT, "flex-1")}
                value={req.name}
                onChange={(e) => patchToolReq(i, { name: e.target.value })}
              >
                {(TOOL_NAMES.includes(req.name) ? TOOL_NAMES : [req.name, ...TOOL_NAMES]).map((n) => (
                  <option key={n} value={n}>
                    {n}
                  </option>
                ))}
              </select>
              <input
                className={cn(INPUT, "w-20")}
                type="number"
                min={1}
                placeholder="min"
                value={req.min}
                onChange={(e) => patchToolReq(i, { min: Number(e.target.value) })}
              />
              <button type="button" onClick={() => removeToolReq(i)} className="text-muted-foreground hover:text-danger">
                <X className="size-4" />
              </button>
            </div>
          ))}
          <Button type="button" variant="outline" size="sm" onClick={addToolReq}>
            <Plus className="size-3.5 mr-1" /> add tool
          </Button>
        </div>

        <div className="space-y-1.5">
          <span className={LABEL}>docs fetched</span>
          <div className="flex flex-col gap-1.5">
            {[...DOC_IDS, ...(checks.docsFetched ?? []).filter((d) => !DOC_IDS.includes(d))].map((id) => {
              const list = checks.docsFetched ?? [];
              return (
                <label key={id} className="flex items-center gap-2.5 text-sm">
                  <input
                    type="checkbox"
                    checked={list.includes(id)}
                    onChange={(e) => {
                      const next = e.target.checked ? [...list, id] : list.filter((x) => x !== id);
                      setChecks({ docsFetched: next.length ? next : undefined });
                    }}
                    className="rounded border-border"
                  />
                  <span className="font-mono text-xs">{id}</span>
                </label>
              );
            })}
          </div>
        </div>

        <Field label="max mcp errors">
          <input
            className={cn(INPUT, "w-32")}
            type="number"
            min={0}
            placeholder="unset"
            value={checks.maxMcpErrors ?? ""}
            onChange={(e) => setChecks({ maxMcpErrors: e.target.value === "" ? undefined : Number(e.target.value) })}
          />
        </Field>
      </div>

      {/* expectations */}
      <div className="space-y-4 rounded-lg border border-border p-4">
        <span className={LABEL}>expectations</span>
        {expectations.map((exp, i) => (
          <div key={i} className="flex items-start gap-2">
            <textarea
              className={cn(INPUT, "min-h-16 flex-1 font-mono text-xs leading-relaxed")}
              value={exp.expectation}
              onChange={(e) => patchExpectation(i, { expectation: e.target.value })}
            />
            <button
              type="button"
              onClick={() => removeExpectation(i)}
              className="mt-2 text-muted-foreground hover:text-danger"
            >
              <X className="size-4" />
            </button>
          </div>
        ))}
        <Button type="button" variant="outline" size="sm" onClick={addExpectation}>
          <Plus className="size-3.5 mr-1" /> add expectation
        </Button>
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
