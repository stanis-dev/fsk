import type { TranscriptEvent } from "./types";

type JSONRecord = Record<string, unknown>;

export function parseTranscript(jsonl: string): TranscriptEvent[] {
  const events: TranscriptEvent[] = [];
  for (const line of jsonl.split("\n")) {
    const s = line.trim();
    if (!s) continue;
    let m: unknown;
    try {
      m = JSON.parse(s);
    } catch {
      continue;
    }
    if (!isRecord(m)) continue;
    switch (m.type) {
      case "assistant":
        for (const c of content(m)) {
          if (c.type === "thinking" && isNonEmptyString(c.thinking)) events.push({ kind: "thinking", text: c.thinking });
          else if (c.type === "text" && isNonEmptyString(c.text)) events.push({ kind: "assistant", text: c.text });
          else if (c.type === "tool_use" && typeof c.name === "string") {
            events.push({ kind: "tool", text: summarizeTool(c.name, isRecord(c.input) ? c.input : {}) });
          }
        }
        break;
      case "user":
        for (const c of content(m)) {
          if (c.type === "tool_result") {
            let txt = flatten(c.content);
            if (c.is_error) txt = "error: " + txt;
            events.push({ kind: "result", text: truncate(txt, 600) });
          }
        }
        break;
      case "result":
        if (typeof m.result === "string" && m.result) events.push({ kind: "final", text: m.result });
        break;
    }
  }
  return events;
}

export function summarizeTool(name: string, input: JSONRecord): string {
  const sv = (k: string) => (typeof input[k] === "string" ? input[k] : "");
  switch (name) {
    case "Bash":
      return "Bash  $ " + oneLine(sv("command"));
    case "Read":
      return "Read  " + sv("file_path");
    case "Write":
      return "Write  " + sv("file_path");
    case "Edit":
    case "MultiEdit":
      return name + "  " + sv("file_path");
    case "Grep": {
      let p = sv("pattern");
      const path = sv("path");
      if (path) p += "  in " + path;
      return "Grep  " + p;
    }
    case "Glob":
      return "Glob  " + sv("pattern");
    case "TodoWrite":
      return "TodoWrite  (updated task list)";
    case "WebFetch":
      return "WebFetch  " + sv("url");
    case "WebSearch":
      return "WebSearch  " + sv("query");
    case "Task":
    case "Agent":
      return name + "  " + (sv("description") || sv("subagent_type"));
    case "ToolSearch":
      return "ToolSearch  " + sv("query");
    default:
      return name + "  " + truncate(oneLine(JSON.stringify(input)), 200);
  }
}

function content(m: JSONRecord): JSONRecord[] {
  const message = m.message;
  if (!isRecord(message) || !Array.isArray(message.content)) return [];
  return message.content.filter(isRecord);
}

function flatten(v: unknown): string {
  if (typeof v === "string") return v;
  if (Array.isArray(v)) return v.map((e) => (isRecord(e) && typeof e.text === "string" ? e.text : "")).join("");
  return "";
}

function isRecord(v: unknown): v is JSONRecord {
  return typeof v === "object" && v !== null;
}

function isNonEmptyString(v: unknown): v is string {
  return typeof v === "string" && v.trim() !== "";
}

function truncate(s: string, n: number): string {
  s = s.trim();
  return s.length > n ? s.slice(0, n) + " …" : s;
}

function oneLine(s: string): string {
  return truncate(s.split(/\s+/).filter(Boolean).join(" "), 300);
}
