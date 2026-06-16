import { afterEach, expect, test } from "vitest";
import { runsDir, evalScript } from "./paths";

afterEach(() => {
  delete process.env.FISKALY_RUNS_DIR;
  delete process.env.FISKALY_EVAL_SCRIPT;
});

test("runsDir defaults under the home cache dir", () => {
  expect(runsDir().endsWith("/.cache/fiskaly-eval")).toBe(true);
});

test("runsDir honors the env override", () => {
  process.env.FISKALY_RUNS_DIR = "/tmp/runs";
  expect(runsDir()).toBe("/tmp/runs");
});

test("evalScript honors the env override", () => {
  process.env.FISKALY_EVAL_SCRIPT = "/tmp/run.sh";
  expect(evalScript()).toBe("/tmp/run.sh");
});
