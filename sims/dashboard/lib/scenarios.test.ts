import { describe, it, expect } from "vitest";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { listScenarios, isKnownScenario, loadScenario, validateConfig, assignExpectationIds } from "./scenarios";
import type { ScenarioConfig } from "./types";

function fixtureDir(): string {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "scenarios-"));
  const s = path.join(root, "01-demo");
  fs.mkdirSync(path.join(s, "fixture"), { recursive: true });
  fs.writeFileSync(
    path.join(s, "scenario.json"),
    JSON.stringify({
      id: "01-demo", title: "Demo", tier: 1, capability: "do x", persona_ref: "P",
      traps: [], judge: { checks: { groundedBeforeWrite: true }, expectations: [{ id: "x", expectation: "y" }] },
    }),
  );
  fs.writeFileSync(path.join(s, "task.md"), "do the task");
  // a non-scenario dir that must be ignored (no numeric prefix)
  fs.mkdirSync(path.join(root, "notes"));
  return root;
}

describe("scenarios", () => {
  it("lists numeric-prefixed scenarios with fixture + scenario.json", () => {
    const dir = fixtureDir();
    const list = listScenarios(dir);
    expect(list.map((s) => s.id)).toEqual(["01-demo"]);
    expect(list[0].judge.expectations[0].id).toBe("x");
  });

  it("isKnownScenario gates ids", () => {
    const dir = fixtureDir();
    expect(isKnownScenario("01-demo", dir)).toBe(true);
    expect(isKnownScenario("99-nope", dir)).toBe(false);
    expect(isKnownScenario("../etc", dir)).toBe(false);
  });

  it("loadScenario returns config + task, null for unknown", () => {
    const dir = fixtureDir();
    const d = loadScenario("01-demo", dir);
    expect(d?.task).toBe("do the task");
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
      traps: [], judge: { checks: { groundedBeforeWrite: true }, expectations: [{ id: "x", expectation: "y" }] },
    };
    expect(validateConfig(good)).toBeNull();
    expect(validateConfig({ ...good, tier: "1" })).toMatch(/tier/);
    expect(validateConfig({ ...good, judge: {} })).toMatch(/judge/);
    expect(validateConfig({ ...good, traps: "none" })).toMatch(/traps/);
    expect(validateConfig(null)).toMatch(/object/);
  });

  it("validateConfig rejects non-array expectations", () => {
    const good = {
      id: "01-demo", title: "Demo", tier: 1, capability: "x", persona_ref: "P",
      traps: [], judge: { checks: { groundedBeforeWrite: true }, expectations: [{ id: "x", expectation: "y" }] },
    };
    expect(validateConfig({ ...good, judge: { checks: { groundedBeforeWrite: true }, expectations: "not-array" } })).toMatch(/expectations/);
  });

  it("validateConfig rejects judge with empty checks and empty expectations", () => {
    const good = {
      id: "01-demo", title: "Demo", tier: 1, capability: "x", persona_ref: "P",
      traps: [], judge: { checks: {}, expectations: [] },
    };
    expect(validateConfig(good)).toMatch(/judge/);
  });

  it("validateConfig accepts judge with only checks (no expectations)", () => {
    const good = {
      id: "01-demo", title: "Demo", tier: 1, capability: "x", persona_ref: "P",
      traps: [], judge: { checks: { groundedBeforeWrite: true }, expectations: [] },
    };
    expect(validateConfig(good)).toBeNull();
  });

  it("validateConfig accepts judge with only expectations (empty checks)", () => {
    const good = {
      id: "01-demo", title: "Demo", tier: 1, capability: "x", persona_ref: "P",
      traps: [], judge: { checks: {}, expectations: [{ id: "x", expectation: "y" }] },
    };
    expect(validateConfig(good)).toBeNull();
  });

  it("assignExpectationIds preserves existing ids and fills empty ones without collision", () => {
    const config = {
      id: "01-demo", title: "Demo", tier: 1, capability: "x", persona_ref: "P",
      traps: [],
      judge: {
        checks: {},
        expectations: [
          { id: "e1", expectation: "a" },
          { id: "", expectation: "b" },
          { id: "kept", expectation: "c" },
          { id: "", expectation: "d" },
        ],
      },
    } as ScenarioConfig;
    const out = assignExpectationIds(config);
    const ids = out.judge.expectations.map((e) => e.id);
    expect(ids[0]).toBe("e1");
    expect(ids[2]).toBe("kept");
    expect(ids[1]).not.toBe("e1"); // must skip the already-used e1
    expect(new Set(ids).size).toBe(4); // all unique
    expect(ids.every(Boolean)).toBe(true); // none empty
    expect(config.judge.expectations[1].id).toBe(""); // input not mutated
  });
});
