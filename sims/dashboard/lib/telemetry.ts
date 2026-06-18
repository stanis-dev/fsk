import type { TelemetryEvent, TelemetrySummary, TelemetryToolStat } from "./types";

export function parseTelemetry(jsonl: string): TelemetryEvent[] {
  const out: TelemetryEvent[] = [];
  for (const line of jsonl.split("\n")) {
    const s = line.trim();
    if (!s) continue;
    let m: any;
    try {
      m = JSON.parse(s);
    } catch {
      continue;
    }
    out.push({
      ts: str(m.ts),
      sessionId: str(m.session_id),
      tool: str(m.tool),
      args: m.args && typeof m.args === "object" ? m.args : {},
      resultCount: typeof m.result_count === "number" ? m.result_count : 0,
      isError: m.is_error === true,
      error: str(m.error),
      latencyMs: typeof m.latency_ms === "number" ? m.latency_ms : 0,
    });
  }
  return out;
}

export function summarizeTelemetry(events: TelemetryEvent[]): TelemetrySummary {
  const byTool = new Map<string, TelemetryToolStat>();
  const latencies: number[] = [];
  const queries = new Set<string>();
  const docs = new Set<string>();
  let errors = 0;

  for (const e of events) {
    const st = byTool.get(e.tool) ?? { tool: e.tool, calls: 0, errors: 0 };
    st.calls++;
    if (e.isError) {
      st.errors++;
      errors++;
    }
    byTool.set(e.tool, st);
    latencies.push(e.latencyMs);
    if (e.tool === "search_fiskaly_docs" && typeof e.args.query === "string") queries.add(e.args.query);
    if (e.tool === "fetch_fiskaly_doc" && typeof e.args.id === "string") docs.add(e.args.id);
  }

  latencies.sort((a, b) => a - b);
  return {
    total: events.length,
    errors,
    byTool: [...byTool.values()].sort((a, b) => b.calls - a.calls),
    p50LatencyMs: percentile(latencies, 50),
    p95LatencyMs: percentile(latencies, 95),
    queries: [...queries],
    docsFetched: [...docs],
  };
}

function percentile(sorted: number[], p: number): number {
  if (sorted.length === 0) return 0;
  const idx = Math.min(sorted.length - 1, Math.floor((p / 100) * sorted.length));
  return sorted[idx];
}

function str(v: any): string {
  return typeof v === "string" ? v : "";
}
