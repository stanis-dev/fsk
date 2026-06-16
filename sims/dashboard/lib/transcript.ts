import type { TranscriptEvent } from "./types";

export function parseTranscript(jsonl: string): TranscriptEvent[] {
  const events: TranscriptEvent[] = [];
  for (const line of jsonl.split("\n")) {
    const s = line.trim();
    if (!s) continue;
    let m: any;
    try {
      m = JSON.parse(s);
    } catch {
      continue;
    }
    switch (m.type) {
      case "assistant":
        for (const c of content(m)) {
          if (c?.type === "thinking" && c.thinking?.trim()) events.push({ kind: "thinking", text: c.thinking });
          else if (c?.type === "text" && c.text?.trim()) events.push({ kind: "assistant", text: c.text });
          else if (c?.type === "tool_use") events.push({ kind: "tool", text: summarizeTool(c.name, c.input ?? {}) });
        }
        break;
      case "user":
        for (const c of content(m)) {
          if (c?.type === "tool_result") {
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

export function summarizeTool(name: string, input: Record<string, any>): string {
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

function content(m: any): any[] {
  return m?.message?.content ?? [];
}

function flatten(v: any): string {
  if (typeof v === "string") return v;
  if (Array.isArray(v)) return v.map((e) => (typeof e?.text === "string" ? e.text : "")).join("");
  return "";
}

function truncate(s: string, n: number): string {
  s = s.trim();
  return s.length > n ? s.slice(0, n) + " …" : s;
}

function oneLine(s: string): string {
  return truncate(s.split(/\s+/).filter(Boolean).join(" "), 300);
}
