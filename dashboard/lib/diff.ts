import type { DiffLine } from "./types";

// Marker order matters: +++/--- must be classified as meta before the +/- checks.
export function classifyDiff(raw: string): DiffLine[] {
  if (!raw.trim()) return [];
  return raw.split("\n").map((text) => {
    let cls: DiffLine["cls"] = "ctx";
    if (text.startsWith("diff ") || text.startsWith("index ") || text.startsWith("+++") || text.startsWith("---")) cls = "meta";
    else if (text.startsWith("@@")) cls = "hunk";
    else if (text.startsWith("+")) cls = "add";
    else if (text.startsWith("-")) cls = "del";
    return { cls, text };
  });
}
