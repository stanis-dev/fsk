import { afterEach, expect, test } from "vitest";
import { runsDir, runnerDir } from "./paths";

afterEach(() => {
  delete process.env.FISKALY_RUNS_DIR;
  delete process.env.FISKALY_RUNNER_DIR;
});

test("runsDir defaults under the home cache dir", () => {
  expect(runsDir().endsWith("/.cache/fiskaly-eval")).toBe(true);
});

test("runsDir honors the env override", () => {
  process.env.FISKALY_RUNS_DIR = "/tmp/runs";
  expect(runsDir()).toBe("/tmp/runs");
});

test("runnerDir defaults to the sibling runner module", () => {
  expect(runnerDir().endsWith("/runner")).toBe(true);
});

test("runnerDir honors the env override", () => {
  process.env.FISKALY_RUNNER_DIR = "/tmp/runner";
  expect(runnerDir()).toBe("/tmp/runner");
});
