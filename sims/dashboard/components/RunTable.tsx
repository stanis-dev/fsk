import Link from "next/link";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { JudgeBadge } from "@/components/JudgeBadge";
import type { Summary } from "@/lib/types";

export function RunTable({ runs }: { runs: Summary[] }) {
  return (
    <Table className="font-mono text-sm">
      <TableHeader>
        <TableRow>
          <TableHead>run</TableHead><TableHead>scenario</TableHead><TableHead>when</TableHead><TableHead>coder</TableHead>
          <TableHead>harness</TableHead><TableHead>model</TableHead><TableHead>build</TableHead>
          <TableHead>tests</TableHead><TableHead>judge</TableHead><TableHead>turns</TableHead><TableHead>cost</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {runs.length === 0 && (
          <TableRow><TableCell colSpan={11}>no runs yet</TableCell></TableRow>
        )}
        {runs.map((r) => (
          <TableRow key={r.id}>
            <TableCell><Link className="text-blue-700 underline" href={`/run/${r.id}`}>{r.id}</Link></TableCell>
            <TableCell>{r.scenario}</TableCell>
            <TableCell>{new Date(r.created).toLocaleString()}</TableCell>
            <TableCell>{r.coder}</TableCell>
            <TableCell>{r.harness}</TableCell>
            <TableCell>{r.model}</TableCell>
            {r.status === "running" ? (
              <TableCell colSpan={5} className="text-amber-600">running…</TableCell>
            ) : (
              <>
                <TableCell><JudgeBadge value={r.build} /></TableCell>
                <TableCell><JudgeBadge value={r.tests} /></TableCell>
                <TableCell><JudgeBadge value={r.judge} /></TableCell>
                <TableCell>{r.turns}</TableCell>
                <TableCell>{r.cost}</TableCell>
              </>
            )}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
