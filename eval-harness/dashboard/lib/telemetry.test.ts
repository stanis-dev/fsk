import { expect, test } from "vitest";
import { parseTelemetry, summarizeTelemetry } from "./telemetry";

const jsonl = [
  JSON.stringify({ ts: "t1", tool: "search_fiskaly_docs", args: { query: "idempotency" }, result_count: 3, is_error: false, latency_ms: 10 }),
  JSON.stringify({ ts: "t2", tool: "fetch_fiskaly_doc", args: { id: "probe:records-flow" }, result_count: 1, is_error: false, latency_ms: 20 }),
  JSON.stringify({ ts: "t3", tool: "fetch_fiskaly_doc", args: { id: "missing" }, result_count: 0, is_error: true, error: "no doc", latency_ms: 30 }),
].join("\n");

test("parseTelemetry maps snake_case to typed events", () => {
  const evs = parseTelemetry(jsonl);
  expect(evs).toHaveLength(3);
  expect(evs[0].tool).toBe("search_fiskaly_docs");
  expect(evs[0].latencyMs).toBe(10);
  expect(evs[2].isError).toBe(true);
});

test("parseTelemetry skips blank and malformed lines", () => {
  expect(parseTelemetry("\n{bad}\n")).toHaveLength(0);
});

test("summarizeTelemetry aggregates counts, latency, queries, docs", () => {
  const s = summarizeTelemetry(parseTelemetry(jsonl));
  expect(s.total).toBe(3);
  expect(s.errors).toBe(1);
  expect(s.byTool.find((t) => t.tool === "fetch_fiskaly_doc")?.calls).toBe(2);
  expect(s.p50LatencyMs).toBe(20);
  expect(s.p95LatencyMs).toBe(30);
  expect(s.queries).toEqual(["idempotency"]);
  expect(s.docsFetched).toEqual(["probe:records-flow", "missing"]);
});
