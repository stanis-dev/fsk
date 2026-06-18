"use server";

import { spawn } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { runnerDir, scenariosDir } from "@/lib/paths";
import { isKnownScenario, validateConfig } from "@/lib/scenarios";
import type { ScenarioConfig } from "@/lib/types";

// Spawn the eval detached so it outlives this request; the new run dir shows up
// on the next list render.
export async function runScenario(scenarioId: string): Promise<void> {
  if (!isKnownScenario(scenarioId)) throw new Error(`unknown scenario: ${scenarioId}`);
  const child = spawn("go", ["run", ".", "run", scenarioId], {
    cwd: runnerDir(),
    detached: true,
    stdio: "ignore",
  });
  child.unref();
}

export async function saveScenario(
  id: string,
  data: { config: ScenarioConfig; task: string; solution: string },
): Promise<void> {
  if (!isKnownScenario(id)) throw new Error(`unknown scenario: ${id}`);
  const err = validateConfig(data.config);
  if (err) throw new Error(`invalid scenario config: ${err}`);
  const dir = path.join(scenariosDir(), id);
  fs.writeFileSync(path.join(dir, "scenario.json"), JSON.stringify(data.config, null, 2) + "\n");
  fs.writeFileSync(path.join(dir, "task.md"), data.task);
  fs.writeFileSync(path.join(dir, "SOLUTION.md"), data.solution);
}
