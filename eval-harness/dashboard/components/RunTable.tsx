import Link from "next/link";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { JudgeBadge } from "@/components/JudgeBadge";
import { CancelButton } from "@/components/CancelButton";
import { cn } from "@/lib/utils";
import type { Summary } from "@/lib/types";

const HEAD = "h-9 px-3 text-[0.7rem] font-medium uppercase tracking-[0.08em] text-muted-foreground";
const CELL = "px-3 py-2.5";

function formatWhen(iso: string): string {
  const d = new Date(iso);
  const day = d.toLocaleDateString(undefined, { month: "short", day: "numeric" });
  const time = d.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
  return `${day} · ${time}`;
}

export function RunTable({ runs }: { runs: Summary[] }) {
  return (
    <Table className="text-sm">
      <TableHeader>
        <TableRow className="border-border hover:bg-transparent">
          <TableHead className={HEAD}>run</TableHead>
          <TableHead className={HEAD}>scenario</TableHead>
          <TableHead className={HEAD}>when</TableHead>
          <TableHead className={HEAD}>coder</TableHead>
          <TableHead className={HEAD}>harness</TableHead>
          <TableHead className={HEAD}>model</TableHead>
          <TableHead className={HEAD}>build</TableHead>
          <TableHead className={HEAD}>tests</TableHead>
          <TableHead className={HEAD}>judge</TableHead>
          <TableHead className={cn(HEAD, "text-right")}>turns</TableHead>
          <TableHead className={cn(HEAD, "text-right")}>cost</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {runs.length === 0 && (
          <TableRow>
            <TableCell colSpan={11} className="h-24 text-center text-muted-foreground">
              no runs yet
            </TableCell>
          </TableRow>
        )}
        {runs.map((r) => (
          <TableRow key={r.id} className="group border-border">
            <TableCell className={CELL}>
              <Link
                href={`/run/${r.id}`}
                className="font-mono text-foreground underline-offset-4 decoration-muted-foreground/30 group-hover:underline"
              >
                {r.id}
              </Link>
            </TableCell>
            <TableCell className={cn(CELL, "font-medium")}>{r.scenario}</TableCell>
            <TableCell className={cn(CELL, "font-mono text-xs tabular-nums text-muted-foreground")}>
              {formatWhen(r.createdIso)}
            </TableCell>
            <TableCell className={cn(CELL, "text-muted-foreground")}>{r.coder}</TableCell>
            <TableCell className={cn(CELL, "text-muted-foreground")}>{r.harness}</TableCell>
            <TableCell className={cn(CELL, "font-mono text-xs text-muted-foreground")}>{r.model}</TableCell>
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
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
