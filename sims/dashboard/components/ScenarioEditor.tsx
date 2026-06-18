"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Plus, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { saveScenario } from "@/app/actions";
import type { ScenarioDetail } from "@/lib/types";

const LABEL = "text-[0.7rem] font-medium uppercase tracking-[0.08em] text-muted-foreground";
const INPUT =
  "w-full rounded-md border border-border bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50";

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
          <span key={item} className="inline-flex items-center gap-1.5 rounded-md border border-border px-2 py-1 font-mono text-xs">
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

      <StringList
        label="judge expectations"
        items={config.judge.expectations.map((e) => e.id)}
        onChange={(ids) => setConfig({ ...config, judge: { ...config.judge, expectations: ids.map((id) => ({ id, expectation: id })) } })}
      />
      <StringList label="traps" items={config.traps} onChange={(traps) => setConfig({ ...config, traps })} />

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
