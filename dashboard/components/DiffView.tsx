import type { DiffLine } from "@/lib/types";

const bg: Record<DiffLine["cls"], string> = {
  add: "bg-green-100",
  del: "bg-red-100",
  hunk: "bg-blue-100 text-blue-800",
  meta: "text-muted-foreground font-bold",
  ctx: "",
};

export function DiffView({ lines }: { lines: DiffLine[] }) {
  if (lines.length === 0) return <span className="text-muted-foreground">—</span>;
  return (
    <pre className="overflow-auto rounded bg-muted p-2 font-mono text-xs">
      {lines.map((l, i) => (
        <span key={i} className={`block px-2 ${bg[l.cls]}`}>{l.text}</span>
      ))}
    </pre>
  );
}
