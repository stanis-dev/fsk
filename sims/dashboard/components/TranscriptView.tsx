import type { TranscriptEvent } from "@/lib/types";

const color: Record<TranscriptEvent["kind"], string> = {
  thinking: "text-purple-600 italic",
  assistant: "text-foreground",
  tool: "text-blue-600",
  result: "text-green-700",
  final: "font-bold",
};

export function TranscriptView({ events }: { events: TranscriptEvent[] }) {
  return (
    <div className="space-y-2 font-mono text-xs">
      {events.map((e, i) => (
        <div key={i} className="flex gap-3 border-l-2 border-muted pl-2">
          <span className="w-20 shrink-0 text-muted-foreground">{e.kind}</span>
          <span className={`whitespace-pre-wrap break-words ${color[e.kind]}`}>{e.text}</span>
        </div>
      ))}
    </div>
  );
}
