#!/usr/bin/env bash
# run-eval.sh — simplest one-shot eval.
#
# A consumer agent integrates fiscalization into an isolated copy of the `pos`
# fixture. We capture the transcript, the exact diff, and whether the result
# still builds and tests green, then runs the deterministic conformance judge.
# No MCP and no sandbox yet.
#
# Usage:  evals/run-eval.sh
# Env:    RUN_MODEL  (default: sonnet)   RUN_EFFORT (default: high)
# Needs:  claude, go, git.  jq optional (nicer summary line).

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fixture="$repo_root/pos"
model="${RUN_MODEL:-claude-sonnet-4-6}"
effort="${RUN_EFFORT:-medium}"

[ -d "$fixture" ] || {
  echo "fixture not found: $fixture" >&2
  exit 1
}
command -v claude >/dev/null || {
  echo "claude CLI not found" >&2
  exit 1
}
command -v go >/dev/null || {
  echo "go not found" >&2
  exit 1
}
command -v git >/dev/null || {
  echo "git not found" >&2
  exit 1
}

# The task is business-framed: what the business needs, never how fiscalization
# works. Discovering the "how" is the point of the eval.
read -r -d '' task <<'EOF' || true
This is the checkout backend for an Italian point-of-sale system. By law an order
cannot be completed until it has been fiscalized, and fiscalization is not
implemented yet. Implement it using fiskaly as the fiscalization provider: make
CompleteOrder fiscalize a paid order through fiskaly and only let the order reach
the completed state once that has succeeded. Keep the existing tests green and add tests for
the new behavior.
EOF

# Isolated throwaway copy under a shared runs dir (so the dashboard sees it too).
runs_base="${HOME}/.cache/fiskaly-eval"
mkdir -p "$runs_base"
run_dir="$(mktemp -d "$runs_base/run.XXXXXX")"
work="$run_dir/pos"
cp -R "$fixture" "$work"

# Baseline commit so we can diff exactly what the agent changed.
git -C "$work" init -q
git -C "$work" -c user.email=eval@local -c user.name=eval add -A
git -C "$work" -c user.email=eval@local -c user.name=eval commit -qm baseline

printf '{"harness":"local","coder":"claude-code","model":"%s","effort":"%s"}\n' "$model" "$effort" >"$run_dir/meta.json"

# Build the (currently empty) fiskaly MCP and hand it to the coder.
mcp_bin="$run_dir/fiskaly-mcp"
(cd "$repo_root/mcp" && go build -o "$mcp_bin" .)
printf '{ "mcpServers": { "fiskaly": { "command": "%s" } } }\n' "$mcp_bin" >"$run_dir/mcp.json"

# Clean room: an empty HOME so no global ~/.claude (CLAUDE.md, skills, settings)
# loads, with subscription auth from the OAuth token. Only CLAUDE_CODE_OAUTH_TOKEN
# is read from .env; the fiskaly credentials are deliberately NOT exported to the
# coder. Context-isolated, not filesystem-hermetic (Docker adds that later).
clean_home="$run_dir/home"
mkdir -p "$clean_home"
oauth_token="$(sed -nE 's/^CLAUDE_CODE_OAUTH_TOKEN=//p' "$repo_root/.env" | head -1 | sed -E 's/^["'"'"']//; s/["'"'"']$//')"
[ -n "$oauth_token" ] || {
  echo "CLAUDE_CODE_OAUTH_TOKEN not found in $repo_root/.env" >&2
  exit 1
}

echo "run dir: $run_dir"
echo "model: $model   effort: $effort"
echo "running agent (headless)..."

# HOME=$clean_home + the OAuth token: a vanilla consumer with no operator CLAUDE.md
#   or skills, authenticated by the subscription. --strict-mcp-config gives it ONLY
#   the fiskaly MCP. --permission-mode bypassPermissions: headless cannot answer
#   prompts; the copy is a throwaway sandbox.
(
  cd "$work" && HOME="$clean_home" CLAUDE_CODE_OAUTH_TOKEN="$oauth_token" \
    claude -p "$task" \
    --model "$model" --effort "$effort" \
    --mcp-config "$run_dir/mcp.json" --strict-mcp-config \
    --permission-mode bypassPermissions \
    --output-format stream-json --verbose
) >"$run_dir/transcript.jsonl" 2>"$run_dir/claude.err" || true

# Observe: does it still build and test green, and what changed.
(cd "$work" && go build ./...) >"$run_dir/build.txt" 2>&1 && build=PASS || build=FAIL
(cd "$work" && go test ./...) >"$run_dir/test.txt" 2>&1 && tests=PASS || tests=FAIL
git -C "$work" add -A
git -C "$work" diff --cached >"$run_dir/changes.diff"

# Judge: deterministic fiskaly-contract conformance — the compliance check that
# go build / go test structurally cannot perform.
(cd "$repo_root/judge" && go run . "$work") >"$run_dir/judge.txt" 2>&1 && judge=PASS || judge=FAIL

# Summary from the final result event, if jq is available.
summary=""
if command -v jq >/dev/null; then
  summary="$(jq -rs 'map(select(.type=="result")) | last // empty
    | "turns=\(.num_turns)  cost=$\(.total_cost_usd)  agent_error=\(.is_error)"' \
    "$run_dir/transcript.jsonl" 2>/dev/null || true)"
fi

echo
echo "==== eval result ===="
echo "build: $build    tests: $tests    judge: $judge"
[ -n "$summary" ] && echo "$summary"
echo "judge:      $run_dir/judge.txt"
echo "diff:       $run_dir/changes.diff"
echo "transcript: $run_dir/transcript.jsonl"
echo "logs:       $run_dir/build.txt  $run_dir/test.txt  $run_dir/claude.err"
echo "workdir:    $work"
