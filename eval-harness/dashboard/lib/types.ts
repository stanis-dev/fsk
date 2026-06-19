export type Check = "PASS" | "FAIL" | "";
export type RunStatus = "running" | "done" | "cancelled";

export interface Summary {
  id: string;
  createdIso: string;
  status: RunStatus;
  scenario: string;
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

export type CriterionVerdict = "MET" | "UNMET" | "CANNOT_ASSESS";

export interface JudgeCriterion {
  id: string;
  verdict: CriterionVerdict;
  evidence_quote: string;
  reasoning: string;
}

export interface ToolReq { name: string; min: number }
export interface JudgeChecks {
  groundedBeforeWrite?: boolean;
  toolsCalled?: ToolReq[];
  docsFetched?: string[];
  maxMcpErrors?: number;
}
export interface Expectation { id: string; expectation: string }

export interface JudgeReport {
  scenario: string;
  verdict: "conformant" | "NON-COMPLIANT";
  checks: { passed: boolean; results: { id: string; pass: boolean; detail: string }[] };
  expectations: { model: string; criteria: JudgeCriterion[] } | null;
  note: string;
}

export interface ScenarioConfig {
  id: string;
  title: string;
  traps: unknown[];
  judge: { checks: JudgeChecks; expectations: Expectation[] };
}

export interface ScenarioDetail {
  config: ScenarioConfig;
  task: string;
}

export interface RunDetail {
  summary: Summary;
  judgeLog: string;
  judgeReport: JudgeReport | null;
  buildLog: string;
  testLog: string;
  err: string;
  transcript: TranscriptEvent[];
  diff: DiffLine[];
  telemetry: TelemetrySummary;
}
