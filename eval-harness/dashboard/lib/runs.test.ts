import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { describe, it, expect, test } from "vitest";
import { summarizeRun, loadRun, listRuns, parseJudgeReport } from "./runs";

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

test("parseJudgeReport rejects a truthy expectations without a criteria array", () => {
  expect(parseJudgeReport('{"verdict":"conformant","expectations":{"model":"m"}}')).toBeNull();
  expect(parseJudgeReport('{"verdict":"conformant","expectations":{"model":"m","criteria":null}}')).toBeNull();
  expect(
    parseJudgeReport('{"verdict":"conformant","expectations":null,"checks":{"passed":true,"results":[]},"scenario":"x","note":""}'),
  ).not.toBeNull();
});

test("parseJudgeReport reads checks.passed and expectations.criteria and returns the verdict", () => {
  const json = JSON.stringify({
    scenario: "07-wrong-vat",
    verdict: "conformant",
    checks: { passed: true, results: [{ id: "r1", pass: true, detail: "ok" }] },
    expectations: {
      model: "claude-opus-4-8",
      criteria: [{ id: "vat-derived-from-line", verdict: "MET", evidence_quote: "pct := line.VATRate", reasoning: "ok" }],
    },
    note: "",
  });
  const r = parseJudgeReport(json);
  expect(r).not.toBeNull();
  expect(r!.verdict).toBe("conformant");
  expect(r!.checks.passed).toBe(true);
  expect(r!.expectations).not.toBeNull();
  expect(r!.expectations!.criteria[0].id).toBe("vat-derived-from-line");
});

test("loadRun parses the structured judge.json report", () => {
  const r = loadRun(fixtures, "run.sample");
  expect(r).not.toBeNull();
  expect(r!.judgeReport).not.toBeNull();
  expect(r!.judgeReport!.verdict).toBe("conformant");
  expect(r!.judgeReport!.expectations).not.toBeNull();
  expect(r!.judgeReport!.expectations!.criteria[0].id).toBe("vat-derived-from-line");
  expect(r!.judgeReport!.expectations!.criteria[0].verdict).toBe("MET");
});

test("listRuns finds the fixture run", () => {
  const ids = listRuns(fixtures).map((s) => s.id);
  expect(ids).toContain("run.sample");
});

describe("summarizeRun cancelled", () => {
  it("reports cancelled when the marker exists, even with judge.txt", () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "run."));
    fs.writeFileSync(path.join(dir, "meta.json"), JSON.stringify({ scenario: "01-demo", harness: "docker" }));
    fs.writeFileSync(path.join(dir, "judge.txt"), "VERDICT: conformant\n");
    fs.writeFileSync(path.join(dir, "cancelled"), "2026-06-18T00:00:00Z\n");
    expect(summarizeRun(dir).status).toBe("cancelled");
  });
});
