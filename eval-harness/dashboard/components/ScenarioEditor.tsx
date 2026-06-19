"use client";

import { useState } from "react";
import { Plus, X } from "lucide-react";
import { cn } from "@/lib/utils";
import { saveScenario } from "@/lib/api";
import type { ScenarioDetail, Expectation, ToolReq } from "@/lib/types";

const LABEL = "text-[0.7rem] font-medium uppercase tracking-[0.08em] text-muted-foreground";
const INPUT =
  "w-full rounded-md border border-border bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50";
const BUTTON =
  "inline-flex shrink-0 items-center justify-center gap-1.5 rounded-md text-sm font-medium transition-colors outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-50";
const PRIMARY_BUTTON = cn(BUTTON, "h-8 bg-primary px-2.5 text-primary-foreground hover:bg-primary/80");
const OUTLINE_BUTTON = cn(BUTTON, "h-7 border border-border px-2.5 text-[0.8rem] hover:bg-muted");

const DEFAULT_TOOL_OPTIONS = [
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

const DEFAULT_DOC_OPTIONS = [
  "probe:auth-and-headers",
  "probe:scoped-subject",
  "probe:provisioning",
  "probe:records-flow",
  "probe:money-model",
  "research:connection-loss",
  "research:credential-lifecycle",
];

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block space-y-1.5">
      <span className={LABEL}>{label}</span>
      {children}
    </label>
  );
}

export function ScenarioEditor({ detail }: { detail: ScenarioDetail }) {
  const [config, setConfig] = useState(detail.config);
  const [task, setTask] = useState(detail.task);
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
    setChecks({ toolsCalled: [...(checks.toolsCalled ?? []), { name: DEFAULT_TOOL_OPTIONS[0], min: 1 }] });
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
      await saveScenario(config.id, { config, task });
      setState({ kind: "saved" });
    } catch (e) {
      setState({ kind: "error", msg: e instanceof Error ? e.message : String(e) });
    }
  }

  return (
    <div className="space-y-8">
      <Field label="title">
        <input className={INPUT} value={config.title} onChange={(e) => setConfig({ ...config, title: e.target.value })} />
      </Field>

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
                {(DEFAULT_TOOL_OPTIONS.includes(req.name) ? DEFAULT_TOOL_OPTIONS : [req.name, ...DEFAULT_TOOL_OPTIONS]).map((n) => (
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
          <button type="button" className={OUTLINE_BUTTON} onClick={addToolReq}>
            <Plus className="size-3.5 mr-1" /> add tool
          </button>
        </div>

        <div className="space-y-1.5">
          <span className={LABEL}>docs fetched</span>
          <div className="flex flex-col gap-1.5">
            {[...DEFAULT_DOC_OPTIONS, ...(checks.docsFetched ?? []).filter((d) => !DEFAULT_DOC_OPTIONS.includes(d))].map((id) => {
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
        <button type="button" className={OUTLINE_BUTTON} onClick={addExpectation}>
          <Plus className="size-3.5 mr-1" /> add expectation
        </button>
      </div>

      <Field label="task.md">
        <textarea className={cn(INPUT, "min-h-40 font-mono text-xs leading-relaxed")} value={task} onChange={(e) => setTask(e.target.value)} />
      </Field>

      <div className="flex items-center gap-4 border-t border-border pt-5">
        <button type="button" className={PRIMARY_BUTTON} onClick={save} disabled={state.kind === "saving"}>
          {state.kind === "saving" ? "saving…" : "save"}
        </button>
        {state.kind === "saved" && <span className="text-xs text-success">saved</span>}
        {state.kind === "error" && <span className="text-xs text-danger">{state.msg}</span>}
      </div>
    </div>
  );
}
