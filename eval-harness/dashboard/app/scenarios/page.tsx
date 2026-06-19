"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { cn } from "@/lib/utils";
import { listScenarios } from "@/lib/api";
import type { ScenarioConfig } from "@/lib/types";

const HEAD = "h-9 whitespace-nowrap px-3 text-left text-[0.7rem] font-medium uppercase tracking-[0.08em] text-muted-foreground";
const CELL = "whitespace-nowrap px-3 py-2.5";

export default function ScenariosPage() {
  const [scenarios, setScenarios] = useState<ScenarioConfig[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    listScenarios()
      .then(setScenarios)
      .catch((e: unknown) => setError(e instanceof Error ? e.message : String(e)));
  }, []);

  return (
    <main className="mx-auto w-full max-w-6xl px-8 py-12">
      <header className="mb-8 border-b border-border pb-5">
        <h1 className="text-2xl font-semibold tracking-tight">scenarios</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          {error ? <span className="text-danger">{error}</span> : `${scenarios.length} scenarios`}
        </p>
      </header>
      <div className="relative w-full overflow-x-auto">
        <table className="w-full caption-bottom text-sm">
          <thead className="[&_tr]:border-b">
            <tr className="border-b border-border">
              <th className={HEAD}>id</th>
              <th className={HEAD}>title</th>
              <th className={cn(HEAD, "text-right")}>expectations</th>
              <th className={cn(HEAD, "text-right")}>traps</th>
            </tr>
          </thead>
          <tbody className="[&_tr:last-child]:border-0">
            {scenarios.length === 0 && !error && (
              <tr className="border-b border-border">
                <td colSpan={4} className="h-24 whitespace-nowrap px-3 py-2.5 text-center text-muted-foreground">
                  no scenarios found
                </td>
              </tr>
            )}
            {scenarios.map((s) => (
              <tr key={s.id} className="group border-b border-border transition-colors hover:bg-muted/50">
                <td className={CELL}>
                  <Link
                    href={`/scenarios/${s.id}`}
                    className="font-mono text-foreground underline-offset-4 decoration-muted-foreground/30 group-hover:underline"
                  >
                    {s.id}
                  </Link>
                </td>
                <td className={cn(CELL, "font-medium")}>{s.title}</td>
                <td className={cn(CELL, "text-right font-mono tabular-nums text-muted-foreground")}>{s.judge.expectations.length}</td>
                <td className={cn(CELL, "text-right font-mono tabular-nums text-muted-foreground")}>{s.traps.length}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </main>
  );
}
