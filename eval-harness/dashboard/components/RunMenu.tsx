"use client";

import { useState } from "react";
import { Play } from "lucide-react";
import { postRun } from "@/lib/api";
import type { ScenarioConfig } from "@/lib/types";

export function RunMenu({ scenarios }: { scenarios: ScenarioConfig[] }) {
  const [error, setError] = useState<string | null>(null);
  const [scenarioID, setScenarioID] = useState("");
  const selectedID = scenarioID || scenarios[0]?.id || "";

  async function runSelected() {
    if (!selectedID) return;
    setError(null);
    try {
      await postRun(selectedID);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  return (
    <div className="flex flex-col items-end gap-1">
      <div className="flex items-center gap-2">
        <select
          className="h-7 min-w-64 rounded-md border border-border bg-background px-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
          value={selectedID}
          onChange={(e) => setScenarioID(e.target.value)}
          disabled={scenarios.length === 0}
        >
          {scenarios.map((s) => (
            <option key={s.id} value={s.id}>
              {s.id} · {s.title}
            </option>
          ))}
        </select>
        <button
          type="button"
          className="inline-flex h-7 shrink-0 items-center justify-center gap-1 rounded-md border border-border px-2.5 text-[0.8rem] font-medium transition-colors hover:bg-muted disabled:pointer-events-none disabled:opacity-50"
          onClick={runSelected}
          disabled={!selectedID}
        >
          <Play className="size-3.5" />
          run
        </button>
      </div>
      {error && <span className="text-xs text-danger">{error}</span>}
    </div>
  );
}
