import fs from "node:fs";
import path from "node:path";
import type { Summary, TranscriptEvent, DiffLine } from "./types";
import { runsDir } from "./paths";
import { parseTranscript } from "./transcript";
import { classifyDiff } from "./diff";

export interface RunDetail {
  summary: Summary;
  judgeLog: string;
  buildLog: string;
  testLog: string;
  err: string;
  transcript: TranscriptEvent[];
  diff: DiffLine[];
}

export function listRuns(dir = runsDir()): Summary[] {
  let entries: string[];
  try {
    entries = fs.readdirSync(dir);
  } catch {
    return [];
  }
  const out: Summary[] = [];
  for (const name of entries) {
    if (!name.startsWith("run.")) continue;
    const d = path.join(dir, name);
    let st: fs.Stats;
    try {
      st = fs.statSync(d);
    } catch {
      continue;
    }
    if (st.isDirectory()) out.push(summarizeRun(d, st.mtime));
  }
  out.sort((a, b) => (a.created < b.created ? 1 : -1)); // newest first
  return out;
}

export function summarizeRun(dir: string, created = safeMtime(dir)): Summary {
  const s: Summary = {
    id: path.basename(dir), created: created.toISOString(), status: "running",
    scenario: "", coder: "", harness: "", model: "", effort: "", build: "", tests: "", judge: "", turns: "", cost: "",
  };

  const log = logInfo(path.join(dir, "transcript.jsonl"));
  const meta = readMeta(dir);
  s.scenario = meta.scenario || "—";
  s.effort = meta.effort || "—";
  s.model = log.model || meta.model;
  s.coder = (log.ccver ? "claude-code" : "") || meta.coder || "?";
  s.harness = log.cwd === "/work" ? "docker" : log.cwd ? "local" : meta.harness || "?";

  const judge = readFile(path.join(dir, "judge.txt"));
  if (!judge) return s; // judge.txt is the last step; absent means still running
  s.status = "done";
  if (judge.includes("conformant")) s.judge = "PASS";
  else if (judge.includes("NON-COMPLIANT")) s.judge = "FAIL";

  s.build = readFile(path.join(dir, "build.txt")).trim() === "" ? "PASS" : "FAIL";
  const tt = readFile(path.join(dir, "test.txt"));
  s.tests = tt && !tt.includes("FAIL") && tt.includes("ok") ? "PASS" : "FAIL";

  const r = parseResult(path.join(dir, "transcript.jsonl"));
  s.turns = r.turns;
  s.cost = r.cost;
  return s;
}

export function loadRun(dir: string, id: string): RunDetail | null {
  if (!id.startsWith("run.") || id.includes("/") || id.includes("..")) return null;
  const d = path.join(dir, id);
  try {
    if (!fs.statSync(d).isDirectory()) return null;
  } catch {
    return null;
  }
  return {
    summary: summarizeRun(d),
    judgeLog: readFile(path.join(d, "judge.txt")),
    buildLog: readFile(path.join(d, "build.txt")),
    testLog: readFile(path.join(d, "test.txt")),
    err: readFile(path.join(d, "claude.err")),
    transcript: parseTranscript(readFile(path.join(d, "transcript.jsonl"))),
    diff: classifyDiff(readFile(path.join(d, "changes.diff"))),
  };
}

function parseResult(file: string): { turns: string; cost: string } {
  let turns = "", cost = "";
  for (const line of readLines(file)) {
    let m: any;
    try {
      m = JSON.parse(line);
    } catch {
      continue;
    }
    if (m.type !== "result") continue;
    if (typeof m.num_turns === "number") turns = String(Math.round(m.num_turns));
    if (typeof m.total_cost_usd === "number") cost = "$" + m.total_cost_usd.toFixed(2);
  }
  return { turns, cost };
}

function logInfo(file: string): { model: string; cwd: string; ccver: string } {
  for (const line of readLines(file)) {
    let m: any;
    try {
      m = JSON.parse(line);
    } catch {
      continue;
    }
    if (m.type === "system") return { model: str(m.model), cwd: str(m.cwd), ccver: str(m.claude_code_version) };
  }
  return { model: "", cwd: "", ccver: "" };
}

function readMeta(dir: string): { harness: string; coder: string; model: string; effort: string; scenario: string } {
  try {
    const m = JSON.parse(fs.readFileSync(path.join(dir, "meta.json"), "utf8"));
    return { harness: str(m.harness), coder: str(m.coder), model: str(m.model), effort: str(m.effort), scenario: str(m.scenario) };
  } catch {
    return { harness: "", coder: "", model: "", effort: "", scenario: "" };
  }
}

function readFile(p: string): string {
  try {
    return fs.readFileSync(p, "utf8");
  } catch {
    return "";
  }
}
function readLines(p: string): string[] {
  return readFile(p).split("\n").filter((l) => l.trim());
}
function str(v: any): string {
  return typeof v === "string" ? v : "";
}
function safeMtime(dir: string): Date {
  try {
    return fs.statSync(dir).mtime;
  } catch {
    return new Date(0);
  }
}
