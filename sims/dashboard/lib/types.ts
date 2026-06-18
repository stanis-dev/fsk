export type Check = "PASS" | "FAIL" | "";
export type RunStatus = "running" | "done";

export interface Summary {
  id: string;
  created: string; // ISO timestamp
  status: RunStatus;
  scenario: string; // scenario id from meta.json (e.g. 06-fire-and-forget)
  coder: string;
  harness: string;
  model: string;
  effort: string;
  build: Check;
  tests: Check;
  judge: Check;
  turns: string;
  cost: string;
}

export type EventKind = "thinking" | "assistant" | "tool" | "result" | "final";
export interface TranscriptEvent {
  kind: EventKind;
  text: string;
}

export type DiffClass = "meta" | "hunk" | "add" | "del" | "ctx";
export interface DiffLine {
  cls: DiffClass;
  text: string;
}

export interface TelemetryEvent {
  ts: string;
  sessionId: string;
  tool: string;
  args: Record<string, unknown>;
  resultCount: number;
  isError: boolean;
  error: string;
  latencyMs: number;
}

export interface TelemetryToolStat {
  tool: string;
  calls: number;
  errors: number;
}

export interface TelemetrySummary {
  total: number;
  errors: number;
  byTool: TelemetryToolStat[];
  p50LatencyMs: number;
  p95LatencyMs: number;
  queries: string[];
  docsFetched: string[];
}
