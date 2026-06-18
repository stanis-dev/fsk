"use server";

import { spawn } from "node:child_process";
import { runnerDir } from "@/lib/paths";
import { isKnownScenario } from "@/lib/scenarios";

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
