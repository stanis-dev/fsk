import type { Check } from "@/lib/types";
import { cn } from "@/lib/utils";

const STYLES = {
  PASS: { dot: "bg-success", text: "text-success", label: "pass" },
  FAIL: { dot: "bg-danger", text: "text-danger", label: "fail" },
} as const;

export function JudgeBadge({ value }: { value: Check }) {
  if (value === "") return <span className="text-muted-foreground/50">—</span>;
  const s = STYLES[value];
  return (
    <span className="inline-flex items-center gap-1.5 text-xs font-medium">
      <span className={cn("size-1.5 rounded-full", s.dot)} aria-hidden />
      <span className={s.text}>{s.label}</span>
    </span>
  );
}
