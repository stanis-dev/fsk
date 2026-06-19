import os from "node:os";
import path from "node:path";

export function runsDir(): string {
  return process.env.FISKALY_RUNS_DIR ?? path.join(os.homedir(), ".cache", "fiskaly-eval");
}

export function runnerDir(): string {
  return process.env.FISKALY_RUNNER_DIR ?? path.resolve(process.cwd(), "..", "runner");
}

export function scenariosDir(): string {
  return process.env.FISKALY_SCENARIOS_DIR ?? path.resolve(process.cwd(), "..", "scenarios");
}
