"use server";

import { spawn, execFile } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { runnerDir, runsDir, scenariosDir } from "@/lib/paths";
import { assignExpectationIds, isKnownScenario, validateConfig } from "@/lib/scenarios";
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

const RUN_ID = /^run\.[A-Za-z0-9.]+$/;

export async function cancelRun(runId: string): Promise<void> {
  if (!RUN_ID.test(runId)) throw new Error(`invalid run id: ${runId}`);
  const dir = path.join(runsDir(), runId);
  if (!fs.existsSync(dir)) throw new Error(`no such run: ${runId}`);
  // Only a still-running run can be cancelled. If it already finished (judge.txt
  // present) or was already cancelled, do nothing — never signal a pgid the OS may
  // have recycled after the run's process group exited, and never flip a finished
  // run to "cancelled".
  if (fs.existsSync(path.join(dir, "judge.txt")) || fs.existsSync(path.join(dir, "cancelled"))) return;
  // Marker first: the UI flips to cancelled even if a kill below fails.
  fs.writeFileSync(path.join(dir, "cancelled"), new Date().toISOString() + "\n");
  let handle: { pgid?: number; container?: string };
  try {
    handle = JSON.parse(fs.readFileSync(path.join(dir, "run.json"), "utf8"));
  } catch {
    return; // pre-existing/zombie run with no handle: marker only.
  }
  if (typeof handle.pgid === "number" && handle.pgid > 1) {
    for (const sig of ["SIGTERM", "SIGKILL"] as const) {
      try {
        process.kill(-handle.pgid, sig); // negative pid = process group
      } catch {
        // already gone
      }
    }
  }
  if (handle.container) {
    execFile("docker", ["kill", handle.container], () => {}); // best-effort
  }
}

export async function saveScenario(
  id: string,
  data: { config: ScenarioConfig; task: string; solution: string },
): Promise<void> {
  if (!isKnownScenario(id)) throw new Error(`unknown scenario: ${id}`);
  if (data.config.id !== id) throw new Error(`scenario id mismatch: ${data.config.id} !== ${id}`);
  const config = assignExpectationIds(data.config);
  const err = validateConfig(config);
  if (err) throw new Error(`invalid scenario config: ${err}`);
  const dir = path.join(scenariosDir(), id);
  fs.writeFileSync(path.join(dir, "scenario.json"), JSON.stringify(config, null, 2) + "\n");
  fs.writeFileSync(path.join(dir, "task.md"), data.task);
  fs.writeFileSync(path.join(dir, "SOLUTION.md"), data.solution);
}
