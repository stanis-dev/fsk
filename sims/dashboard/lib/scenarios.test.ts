import { describe, it, expect } from "vitest";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { listScenarios, isKnownScenario, loadScenario, validateConfig } from "./scenarios";

function fixtureDir(): string {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "scenarios-"));
  const s = path.join(root, "01-demo");
  fs.mkdirSync(path.join(s, "fixture"), { recursive: true });
  fs.writeFileSync(
    path.join(s, "scenario.json"),
    JSON.stringify({
      id: "01-demo", title: "Demo", tier: 1, capability: "do x", persona_ref: "P",
      traps: [], judge: { rules: ["r1"] },
      baseline: { build: "PASS", tests: "PASS", judge: "NON-COMPLIANT" },
      target: { build: "PASS", tests: "PASS", judge: "conformant" },
    }),
  );
  fs.writeFileSync(path.join(s, "task.md"), "do the task");
  fs.writeFileSync(path.join(s, "SOLUTION.md"), "the solution");
  // a non-scenario dir that must be ignored (no numeric prefix)
  fs.mkdirSync(path.join(root, "notes"));
  return root;
}

describe("scenarios", () => {
  it("lists numeric-prefixed scenarios with fixture + scenario.json", () => {
    const dir = fixtureDir();
    const list = listScenarios(dir);
    expect(list.map((s) => s.id)).toEqual(["01-demo"]);
    expect(list[0].judge.rules).toEqual(["r1"]);
  });

  it("isKnownScenario gates ids", () => {
    const dir = fixtureDir();
    expect(isKnownScenario("01-demo", dir)).toBe(true);
    expect(isKnownScenario("99-nope", dir)).toBe(false);
    expect(isKnownScenario("../etc", dir)).toBe(false);
  });

  it("loadScenario returns config + task + solution, null for unknown", () => {
    const dir = fixtureDir();
    const d = loadScenario("01-demo", dir);
    expect(d?.task).toBe("do the task");
    expect(d?.solution).toBe("the solution");
    expect(d?.config.title).toBe("Demo");
    expect(loadScenario("99-nope", dir)).toBeNull();
  });

  it("listScenarios throws on malformed scenario.json", () => {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), "scenarios-bad-"));
    const s = path.join(root, "02-broken");
    fs.mkdirSync(path.join(s, "fixture"), { recursive: true });
    fs.writeFileSync(path.join(s, "scenario.json"), "{ not json");
    expect(() => listScenarios(root)).toThrow();
  });

  it("validateConfig accepts a good config and rejects bad shapes", () => {
    const good = {
      id: "01-demo", title: "Demo", tier: 1, capability: "x", persona_ref: "P",
      traps: [], judge: { rules: ["r1"] },
      baseline: { build: "PASS", tests: "PASS", judge: "NON-COMPLIANT" },
      target: { build: "PASS", tests: "PASS", judge: "conformant" },
    };
    expect(validateConfig(good)).toBeNull();
    expect(validateConfig({ ...good, tier: "1" })).toMatch(/tier/);
    expect(validateConfig({ ...good, judge: {} })).toMatch(/judge.rules/);
    expect(validateConfig({ ...good, traps: "none" })).toMatch(/traps/);
    expect(validateConfig(null)).toMatch(/object/);
  });
});
