import type { TelemetrySummary } from "@/lib/types";

export function TelemetryView({ summary }: { summary: TelemetrySummary }) {
  if (summary.total === 0) {
    return <div className="text-xs text-muted-foreground">no MCP telemetry for this run</div>;
  }
  return (
    <div className="space-y-2 font-mono text-xs">
      <div className="flex flex-wrap gap-4">
        <span>calls {summary.total}</span>
        <span>errors {summary.errors}</span>
        <span>p50 {summary.p50LatencyMs}ms</span>
        <span>p95 {summary.p95LatencyMs}ms</span>
      </div>
      <div className="space-y-1">
        {summary.byTool.map((t) => (
          <div key={t.tool} className="text-blue-600">
            {t.tool}: {t.calls} calls{t.errors ? `, ${t.errors} err` : ""}
          </div>
        ))}
      </div>
      {summary.queries.length > 0 && (
        <div className="break-words">queries: {summary.queries.join(" · ")}</div>
      )}
      {summary.docsFetched.length > 0 && (
        <div className="break-words">docs fetched: {summary.docsFetched.join(" · ")}</div>
      )}
    </div>
  );
}
