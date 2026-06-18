import path from "node:path";
import { expect, test } from "vitest";
import { summarizeRun, loadRun, listRuns, parseJudgeReport, verdictFromLog } from "./runs";

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

test("parseJudgeReport returns null for absent or garbage input", () => {
  expect(parseJudgeReport("")).toBeNull();
  expect(parseJudgeReport("not json")).toBeNull();
});

test("parseJudgeReport rejects a truthy rubric without a criteria array", () => {
  expect(parseJudgeReport('{"verdict":"conformant","rubric":{"model":"m"}}')).toBeNull();
  expect(parseJudgeReport('{"verdict":"conformant","rubric":{"model":"m","criteria":null}}')).toBeNull();
  expect(
    parseJudgeReport('{"verdict":"conformant","rubric":null,"gate":{"passed":true,"rules":[]},"scenario":"x","note":""}'),
  ).not.toBeNull();
});

test("verdictFromLog reads the VERDICT line, not model reasoning", () => {
  expect(verdictFromLog("RUBRIC\nUNMET c1\n  the code is not conformant\nVERDICT: NON-COMPLIANT (rubric). exit 1")).toBe("FAIL");
  expect(verdictFromLog("VERDICT: conformant. exit 0")).toBe("PASS");
  expect(verdictFromLog("no verdict line here")).toBe("");
});

test("loadRun parses the structured judge.json report", () => {
  const r = loadRun(fixtures, "run.sample");
  expect(r).not.toBeNull();
  expect(r!.judgeReport).not.toBeNull();
  expect(r!.judgeReport!.verdict).toBe("conformant");
  expect(r!.judgeReport!.rubric).not.toBeNull();
  expect(r!.judgeReport!.rubric!.criteria[0].id).toBe("vat-derived-from-line");
  expect(r!.judgeReport!.rubric!.criteria[0].verdict).toBe("MET");
});

test("listRuns finds the fixture run", () => {
  const ids = listRuns(fixtures).map((s) => s.id);
  expect(ids).toContain("run.sample");
});
