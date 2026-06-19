import fs from "node:fs";
import path from "node:path";
import { scenariosDir } from "./paths";
import type { ScenarioConfig, ScenarioDetail } from "./types";

const NUMERIC_PREFIX = /^[0-9]/;

function readConfig(dir: string): ScenarioConfig {
  return JSON.parse(fs.readFileSync(path.join(dir, "scenario.json"), "utf8")) as ScenarioConfig;
}

export function listScenarios(dir = scenariosDir()): ScenarioConfig[] {
  const entries = fs.readdirSync(dir);
  const out: ScenarioConfig[] = [];
  for (const name of entries) {
    if (!NUMERIC_PREFIX.test(name)) continue;
    const d = path.join(dir, name);
    if (!fs.statSync(d).isDirectory()) continue;
    if (!fs.existsSync(path.join(d, "fixture")) || !fs.existsSync(path.join(d, "scenario.json"))) continue;
    out.push(readConfig(d));
  }
  out.sort((a, b) => (a.id < b.id ? -1 : 1));
  return out;
}

export function isKnownScenario(id: string, dir = scenariosDir()): boolean {
  return listScenarios(dir).some((s) => s.id === id);
}

export function loadScenario(id: string, dir = scenariosDir()): ScenarioDetail | null {
  if (!isKnownScenario(id, dir)) return null;
  const d = path.join(dir, id);
  const config = readConfig(d);
  return { config, task: fs.readFileSync(path.join(d, "task.md"), "utf8") };
}

function isExpectationArray(v: unknown): boolean {
  return Array.isArray(v) && (v as unknown[]).every(
    (x) => typeof x === "object" && x !== null && typeof (x as Record<string, unknown>).id === "string" && typeof (x as Record<string, unknown>).expectation === "string",
  );
}

function hasNonEmptyChecks(checks: Record<string, unknown>): boolean {
  return (
    checks.groundedBeforeWrite === true ||
    (Array.isArray(checks.toolsCalled) && (checks.toolsCalled as unknown[]).length > 0) ||
    (Array.isArray(checks.docsFetched) && (checks.docsFetched as unknown[]).length > 0) ||
    typeof checks.maxMcpErrors === "number"
  );
}

export function assignExpectationIds(config: ScenarioConfig): ScenarioConfig {
  const used = new Set(config.judge.expectations.map((e) => e.id).filter(Boolean));
  let n = 1;
  const nextId = (): string => {
    let id = `e${n++}`;
    while (used.has(id)) id = `e${n++}`;
    used.add(id);
    return id;
  };
  const expectations = config.judge.expectations.map((e) => (e.id ? e : { ...e, id: nextId() }));
  return { ...config, judge: { ...config.judge, expectations } };
}

export function validateConfig(obj: unknown): string | null {
  if (typeof obj !== "object" || obj === null) return "config must be an object";
  const c = obj as Record<string, unknown>;
  if (typeof c.id !== "string") return "id must be a string";
  if (typeof c.title !== "string") return "title must be a string";
  if (!Array.isArray(c.traps)) return "traps must be an array";
  const judge = c.judge as Record<string, unknown> | undefined;
  if (typeof judge !== "object" || judge === null) return "judge must be an object";
  if (typeof judge.checks !== "object" || judge.checks === null) return "judge.checks must be an object";
  if (!Array.isArray(judge.expectations)) return "judge.expectations must be an array";
  if (!isExpectationArray(judge.expectations)) return "judge.expectations must be an array of {id, expectation}";
  const hasChecks = hasNonEmptyChecks(judge.checks as Record<string, unknown>);
  const hasExpectations = (judge.expectations as unknown[]).length > 0;
  if (!hasChecks && !hasExpectations) return "judge must have at least one non-empty checks field or a non-empty expectations array";
  return null;
}
