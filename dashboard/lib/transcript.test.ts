import { expect, test } from "vitest";
import { parseTranscript, summarizeTool } from "./transcript";

const jsonl = [
  JSON.stringify({ type: "system", model: "claude-sonnet-4-6", cwd: "/work" }),
  JSON.stringify({ type: "assistant", message: { content: [
    { type: "thinking", thinking: "let me look" },
    { type: "text", text: "Reading the file" },
    { type: "tool_use", name: "Read", input: { file_path: "pos/checkout.go" } },
  ] } }),
  JSON.stringify({ type: "user", message: { content: [
    { type: "tool_result", content: "file contents here", is_error: false },
  ] } }),
  JSON.stringify({ type: "result", result: "done", num_turns: 12, total_cost_usd: 1.5 }),
].join("\n");

test("parseTranscript yields ordered typed events", () => {
  const evs = parseTranscript(jsonl);
  expect(evs.map((e) => e.kind)).toEqual(["thinking", "assistant", "tool", "result", "final"]);
  expect(evs[2].text).toBe("Read  pos/checkout.go");
});

test("tool_result errors are prefixed", () => {
  const evs = parseTranscript(JSON.stringify({ type: "user", message: { content: [
    { type: "tool_result", content: "boom", is_error: true },
  ] } }));
  expect(evs[0].text).toBe("error: boom");
});

test("summarizeTool formats a Bash command on one line", () => {
  expect(summarizeTool("Bash", { command: "go test ./..." })).toBe("Bash  $ go test ./...");
});
