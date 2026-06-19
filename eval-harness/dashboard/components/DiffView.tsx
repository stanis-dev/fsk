import { cn } from "@/lib/utils";
import type { DiffLine } from "@/lib/types";

const cls: Record<DiffLine["cls"], string> = {
  add: "bg-success/10",
  del: "bg-danger/10",
  hunk: "text-muted-foreground",
  meta: "font-medium text-muted-foreground",
  ctx: "",
};

export function DiffView({ lines }: { lines: DiffLine[] }) {
  if (lines.length === 0) return <span className="text-muted-foreground">-</span>;
  return (
    <pre className="overflow-auto font-mono text-xs leading-relaxed">
      {lines.map((l, i) => (
        <span key={i} className={cn("block px-2", cls[l.cls])}>
          {l.text || " "}
        </span>
      ))}
    </pre>
  );
}
