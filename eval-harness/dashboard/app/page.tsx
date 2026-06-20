"use client";

import { useEffect, useState } from "react";
import { listRuns, listScenarios, runsStreamURL } from "@/lib/api";
import { RunTable } from "@/components/RunTable";
import { RunLauncher } from "@/components/RunLauncher";
import type { Summary, ScenarioConfig } from "@/lib/types";

export default function Home() {
  const [runs, setRuns] = useState<Summary[]>([]);
  const [scenarios, setScenarios] = useState<ScenarioConfig[]>([]);
  const [apiError, setApiError] = useState<string | null>(null);

  useEffect(() => {
    let es: EventSource | undefined;
    const fail = (e: unknown) => setApiError(e instanceof Error ? e.message : String(e));

    Promise.all([listRuns(), listScenarios()])
      .then(([r, s]) => {
        setRuns(r);
        setScenarios(s);
        es = new EventSource(runsStreamURL());
        es.onmessage = () => {
          listRuns().then(setRuns).catch(fail);
        };
        es.onerror = () => {
          es?.close();
          setApiError("run event stream disconnected");
        };
      })
      .catch(fail);

    return () => es?.close();
  }, []);

  if (apiError) {
    return (
      <main className="mx-auto w-full max-w-6xl px-8 py-12">
        <p className="text-sm text-muted-foreground">
          Backend unreachable: <code className="font-mono">eval-harness serve</code>
        </p>
        <p className="mt-2 font-mono text-xs text-danger">{apiError}</p>
      </main>
    );
  }

  return (
    <main className="mx-auto w-full max-w-6xl px-8 py-12">
      <header className="mb-8 flex items-end justify-between gap-4 border-b border-border pb-5">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">fiskaly eval runs</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            agentic coding eval workbench · {runs.length} runs
          </p>
        </div>
        <RunLauncher scenarios={scenarios} />
      </header>
      <RunTable runs={runs} />
    </main>
  );
}
