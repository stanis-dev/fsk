import fs from "node:fs";
import path from "node:path";
import type { Summary, TranscriptEvent, DiffLine, TelemetrySummary, JudgeReport, Check } from "./types";
import { runsDir } from "./paths";
import { parseTranscript } from "./transcript";
import { classifyDiff } from "./diff";
import { parseTelemetry, summarizeTelemetry } from "./telemetry";

export interface RunDetail {
  summary: Summary;
  judgeLog: string;
  judgeReport: JudgeReport | null;
  buildLog: string;
  testLog: string;
  err: string;
  transcript: TranscriptEvent[];
  diff: DiffLine[];
  telemetry: TelemetrySummary;
}

// parseJudgeReport reads judge.json; returns null on absent or malformed input so
// callers fall back to the judge.txt VERDICT line.
export function parseJudgeReport(json: string): JudgeReport | null {
  if (!json.trim()) return null;
  try {
    const report = JSON.parse(json) as JudgeReport;
    if (!report || typeof report !== "object") return null;
    if (report.verdict !== "conformant" && report.verdict !== "NON-COMPLIANT") return null;
    if (report.rubric !== null && !(report.rubric && Array.isArray(report.rubric.criteria))) return null;
    return report;
  } catch {
    return null;
  }
}

// verdictFromLog reads the judge's authoritative final "VERDICT:" line. Used as the
// fallback when judge.json is absent — keying off this line (not a bare substring
// scan) prevents untrusted model reasoning/evidence text that mentions "conformant"
// from flipping a FAIL to PASS.
export function verdictFromLog(judge: string): Check {
  const line = judge.split("\n").reverse().find((l) => l.startsWith("VERDICT:"));
  if (!line) return "";
  if (line.includes("conformant")) return "PASS";
  if (line.includes("NON-COMPLIANT")) return "FAIL";
  return "";
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

  if (fs.existsSync(path.join(dir, "cancelled"))) {
    s.status = "cancelled";
    return s;
  }
  const judge = readFile(path.join(dir, "judge.txt"));
  if (!judge) return s; // judge.txt is the last step; absent means still running
  s.status = "done";
  // Prefer the structured verdict (authoritative, unambiguous); fall back to the
  // judge.txt VERDICT line for runs that predate judge.json.
  const report = parseJudgeReport(readFile(path.join(dir, "judge.json")));
  s.judge = report ? (report.verdict === "conformant" ? "PASS" : "FAIL") : verdictFromLog(judge);

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
    judgeReport: parseJudgeReport(readFile(path.join(d, "judge.json"))),
    buildLog: readFile(path.join(d, "build.txt")),
    testLog: readFile(path.join(d, "test.txt")),
    err: readFile(path.join(d, "claude.err")),
    transcript: parseTranscript(readFile(path.join(d, "transcript.jsonl"))),
    diff: classifyDiff(readFile(path.join(d, "changes.diff"))),
    telemetry: summarizeTelemetry(parseTelemetry(readFile(path.join(d, "mcp-telemetry.jsonl")))),
  };
}

function parseResult(file: string): { turns: string; cost: string } {
  let turns = "", cost = "";
  for (const line of readLines(file)) {
    let m: unknown;
    try {
      m = JSON.parse(line);
    } catch {
      continue;
    }
    if (typeof m !== "object" || m === null) continue;
    const r = m as Record<string, unknown>;
    if (r.type !== "result") continue;
    if (typeof r.num_turns === "number") turns = String(Math.round(r.num_turns));
    if (typeof r.total_cost_usd === "number") cost = "$" + r.total_cost_usd.toFixed(2);
  }
  return { turns, cost };
}

function logInfo(file: string): { model: string; cwd: string; ccver: string } {
  for (const line of readLines(file)) {
    let m: unknown;
    try {
      m = JSON.parse(line);
    } catch {
      continue;
    }
    if (typeof m !== "object" || m === null) continue;
    const r = m as Record<string, unknown>;
    if (r.type === "system") return { model: str(r.model), cwd: str(r.cwd), ccver: str(r.claude_code_version) };
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
function str(v: unknown): string {
  return typeof v === "string" ? v : "";
}
function safeMtime(dir: string): Date {
  try {
    return fs.statSync(dir).mtime;
  } catch {
    return new Date(0);
  }
}
