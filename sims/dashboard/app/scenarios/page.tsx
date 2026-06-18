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
