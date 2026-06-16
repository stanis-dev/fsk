import os from "node:os";
import path from "node:path";

// The dashboard runs from dashboard/, so the repo root is its parent and the
// eval script lives at <repo>/evals/run-eval-docker.sh. Both are env-overridable.
export function runsDir(): string {
  return process.env.FISKALY_RUNS_DIR ?? path.join(os.homedir(), ".cache", "fiskaly-eval");
}

export function evalScript(): string {
  return process.env.FISKALY_EVAL_SCRIPT ?? path.resolve(process.cwd(), "..", "evals", "run-eval-docker.sh");
}
