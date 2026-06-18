"use server";

import { spawn } from "node:child_process";
import { runnerDir } from "@/lib/paths";

// Spawn the eval detached so it outlives this request; the new run dir shows up
// on the next list render. Runs the zero-to-receipt scenario via the Go runner.
export async function triggerRun(): Promise<void> {
  const child = spawn("go", ["run", ".", "run", "01-zero-to-receipt"], {
    cwd: runnerDir(),
    detached: true,
    stdio: "ignore",
  });
  child.unref();
}
