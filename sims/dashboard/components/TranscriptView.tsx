import { cn } from "@/lib/utils";
import type { TranscriptEvent } from "@/lib/types";

const styles: Record<TranscriptEvent["kind"], { label: string; text: string }> = {
  thinking: { label: "text-muted-foreground/70", text: "text-muted-foreground italic" },
  assistant: { label: "text-muted-foreground", text: "text-foreground" },
  tool: { label: "text-foreground", text: "text-foreground" },
  result: { label: "text-muted-foreground/70", text: "text-muted-foreground" },
  final: { label: "text-muted-foreground", text: "font-semibold text-foreground" },
};

export function TranscriptView({ events }: { events: TranscriptEvent[] }) {
  return (
    <div className="space-y-3 font-mono text-xs leading-relaxed">
      {events.map((e, i) => (
        <div key={i} className="flex gap-4">
          <span
            className={cn(
              "w-16 shrink-0 select-none text-[0.65rem] uppercase tracking-wider",
              styles[e.kind].label,
            )}
          >
            {e.kind}
          </span>
          <span className={cn("min-w-0 flex-1 break-words whitespace-pre-wrap", styles[e.kind].text)}>
            {e.text}
          </span>
        </div>
      ))}
    </div>
  );
}
