import path from "node:path";
import { expect, test } from "vitest";
import { summarizeRun, loadRun, listRuns } from "./runs";

const fixtures = path.resolve(__dirname, "../__fixtures__");
const sample = path.join(fixtures, "run.sample");

test("summarizeRun derives status and checks from a done run", () => {
  const s = summarizeRun(sample);
  expect(s.status).toBe("done");
  expect(s.judge).toBe("PASS");
  expect(s.build).toBe("PASS");
  expect(s.tests).toBe("PASS");
  expect(s.harness).toBe("docker"); // cwd == /work
  expect(s.model).toBe("claude-sonnet-4-6");
  expect(s.coder).toBe("claude-code");
  expect(s.turns).toBe("12");
  expect(s.cost).toBe("$1.50");
});

test("loadRun returns parsed transcript and diff", () => {
  const r = loadRun(fixtures, "run.sample");
  expect(r).not.toBeNull();
  expect(r!.transcript.some((e) => e.kind === "tool")).toBe(true);
  expect(r!.diff.some((l) => l.cls === "add")).toBe(true);
  expect(r!.judgeLog).toContain("conformant");
});

test("listRuns finds the fixture run", () => {
  const ids = listRuns(fixtures).map((s) => s.id);
  expect(ids).toContain("run.sample");
});
