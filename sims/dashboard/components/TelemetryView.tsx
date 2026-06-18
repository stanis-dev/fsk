import type { TelemetrySummary } from "@/lib/types";

function Stat({ label, value }: { label: string; value: string | number }) {
  return (
    <div>
      <div className="text-[0.65rem] font-medium uppercase tracking-wider text-muted-foreground">
        {label}
      </div>
      <div className="mt-0.5 font-mono text-sm tabular-nums">{value}</div>
    </div>
  );
}

export function TelemetryView({ summary }: { summary: TelemetrySummary }) {
  if (summary.total === 0) {
    return <div className="text-xs text-muted-foreground">no MCP telemetry for this run</div>;
  }
  return (
    <div className="space-y-5 text-xs">
      <div className="flex flex-wrap gap-x-10 gap-y-3">
        <Stat label="calls" value={summary.total} />
        <Stat label="errors" value={summary.errors} />
        <Stat label="p50" value={`${summary.p50LatencyMs}ms`} />
        <Stat label="p95" value={`${summary.p95LatencyMs}ms`} />
      </div>
      <div className="space-y-1.5 font-mono">
        {summary.byTool.map((t) => (
          <div key={t.tool} className="flex items-baseline gap-2">
            <span className="text-foreground">{t.tool}</span>
            <span className="text-muted-foreground">
              {t.calls} calls{t.errors ? ` · ${t.errors} err` : ""}
            </span>
          </div>
        ))}
      </div>
      {summary.queries.length > 0 && (
        <div>
          <div className="mb-1 text-[0.65rem] font-medium uppercase tracking-wider text-muted-foreground">
            queries
          </div>
          <div className="break-words font-mono text-muted-foreground">
            {summary.queries.join(" · ")}
          </div>
        </div>
      )}
      {summary.docsFetched.length > 0 && (
        <div>
          <div className="mb-1 text-[0.65rem] font-medium uppercase tracking-wider text-muted-foreground">
            docs fetched
          </div>
          <div className="break-words font-mono text-muted-foreground">
            {summary.docsFetched.join(" · ")}
          </div>
        </div>
      )}
    </div>
  );
}
