import { expect, test } from "vitest";
import { classifyDiff } from "./diff";

test("classifies diff lines by leading marker", () => {
  const raw = ["diff --git a/x b/x", "@@ -1 +1 @@", "+added", "-removed", " context"].join("\n");
  expect(classifyDiff(raw).map((l) => l.cls)).toEqual(["meta", "hunk", "add", "del", "ctx"]);
});

test("--- and +++ are meta, not del/add", () => {
  expect(classifyDiff("--- a/x").at(0)?.cls).toBe("meta");
  expect(classifyDiff("+++ b/x").at(0)?.cls).toBe("meta");
});

test("empty diff yields no lines", () => {
  expect(classifyDiff("   ")).toEqual([]);
});
