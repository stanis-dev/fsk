"use server";

import { spawn } from "node:child_process";
import { evalScript } from "@/lib/paths";

// Spawn the eval detached so it outlives this request; the new run dir shows up
// on the next list render. Mirrors the Go dashboard's async POST /trigger.
export async function triggerRun(): Promise<void> {
  const script = evalScript();
  const child = spawn("bash", [script], { detached: true, stdio: "ignore" });
  child.unref();
}
