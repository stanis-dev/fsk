import type { Summary, RunDetail, ScenarioConfig, ScenarioDetail } from "./types";

const BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(BASE + path, init);
  if (!res.ok) throw new Error(`${init?.method ?? "GET"} ${path}: ${res.status}`);
  return (res.status === 204 ? undefined : await res.json()) as T;
}

export const listRuns = () => req<Summary[]>("/runs");
export const getRun = (id: string) => req<RunDetail>(`/runs/${id}`);
export const listScenarios = () => req<ScenarioConfig[]>("/scenarios");
export const getScenario = (id: string) => req<ScenarioDetail>(`/scenarios/${id}`);
export const postRun = (scenarioId: string) =>
  req<{ runId: string }>("/runs", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ scenarioId }) });
export const cancelRun = (id: string) => req<void>(`/runs/${id}/cancel`, { method: "POST" });
export const saveScenario = (id: string, data: { config: ScenarioConfig; task: string }) =>
  req<void>(`/scenarios/${id}`, { method: "PUT", headers: { "content-type": "application/json" }, body: JSON.stringify(data) });
export const runsStreamURL = () => BASE + "/runs/stream";
