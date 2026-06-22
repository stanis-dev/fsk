import Link from "next/link";
import { CheckBadge } from "@/components/CheckBadge";
import { CancelButton } from "@/components/CancelButton";
import { cn, HEAD, CELL } from "@/lib/utils";
import type { Summary } from "@/lib/types";

// Two formatters (not one) because the "·" separator is intentional and Intl has
// no option to set a custom date/time separator.
const DATE_FMT = new Intl.DateTimeFormat(undefined, { month: "short", day: "numeric" });
const TIME_FMT = new Intl.DateTimeFormat(undefined, { hour: "2-digit", minute: "2-digit" });

function formatWhen(iso: string): string {
  const d = new Date(iso);
  return `${DATE_FMT.format(d)} · ${TIME_FMT.format(d)}`;
}

export function RunTable({ runs }: { runs: Summary[] }) {
  return (
    <div className="relative w-full overflow-x-auto">
      <table className="w-full caption-bottom text-sm">
        <thead className="[&_tr]:border-b">
          <tr className="border-b border-border">
            <th className={HEAD}>run</th>
            <th className={HEAD}>scenario</th>
            <th className={HEAD}>when</th>
            <th className={HEAD}>coder</th>
            <th className={HEAD}>harness</th>
            <th className={HEAD}>model</th>
            <th className={HEAD}>build</th>
            <th className={HEAD}>tests</th>
            <th className={HEAD}>judge</th>
            <th className={cn(HEAD, "text-right")}>turns</th>
            <th className={cn(HEAD, "text-right")}>cost</th>
          </tr>
        </thead>
        <tbody className="[&_tr:last-child]:border-0">
          {runs.length === 0 && (
            <tr className="border-b border-border">
              <td colSpan={11} className="h-24 whitespace-nowrap px-3 py-2.5 text-center text-muted-foreground">
                no runs yet
              </td>
            </tr>
          )}
          {runs.map((r) => (
            <tr key={r.id} className="group border-b border-border transition-colors hover:bg-muted/50">
              <td className={CELL}>
                <Link
                  href={`/run/${r.id}`}
                  className="font-mono text-foreground underline-offset-4 decoration-muted-foreground/30 group-hover:underline"
                >
                  {r.id}
                </Link>
              </td>
              <td className={cn(CELL, "font-medium")}>{r.scenario}</td>
              <td className={cn(CELL, "font-mono text-xs tabular-nums text-muted-foreground")}>
                {formatWhen(r.updatedIso)}
              </td>
              <td className={cn(CELL, "text-muted-foreground")}>{r.coder}</td>
              <td className={cn(CELL, "text-muted-foreground")}>{r.harness}</td>
              <td className={cn(CELL, "font-mono text-xs text-muted-foreground")}>{r.model}</td>
              {r.status === "running" ? (
                <td colSpan={5} className={CELL}>
                  <div className="flex items-center justify-between gap-2">
                    <span className="inline-flex items-center gap-1.5 text-xs font-medium text-warning">
                      <span className="size-1.5 animate-pulse rounded-full bg-warning" aria-hidden />
                      running
                    </span>
                    <CancelButton runId={r.id} />
                  </div>
                </td>
              ) : r.status === "cancelled" ? (
                <td colSpan={5} className={CELL}>
                  <span className="inline-flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
                    <span className="size-1.5 rounded-full bg-muted-foreground/50" aria-hidden />
                    cancelled
                  </span>
                </td>
              ) : (
                <>
                  <td className={CELL}><CheckBadge value={r.build} /></td>
                  <td className={CELL}><CheckBadge value={r.tests} /></td>
                  <td className={CELL}><CheckBadge value={r.judge} /></td>
                  <td className={cn(CELL, "text-right font-mono tabular-nums text-muted-foreground")}>{r.turns}</td>
                  <td className={cn(CELL, "text-right font-mono tabular-nums")}>{r.cost}</td>
                </>
              )}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
