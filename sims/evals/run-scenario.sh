#!/usr/bin/env bash
# run-scenario.sh — run one eval scenario from the scenario library.
#
# A consumer agent does a fiskaly integration task against an isolated copy of a
# scenario fixture. We capture the transcript, the exact diff, and whether the
# result still builds and tests green, then run the deterministic conformance
# judge with the rule set that scenario declares. Several fixtures carry planted
# traps (red herrings, false info, dormant silent bugs); see the scenario's
# SOLUTION.md for the answer key.
#
# Usage:  evals/run-scenario.sh <scenario-id>      # e.g. 06-fire-and-forget
#         SCENARIO=<id> evals/run-scenario.sh
# Env:    RUN_MODEL  (default: claude-sonnet-4-6)   RUN_EFFORT (default: medium)
# Needs:  claude, go, git.  jq optional (nicer summary line).
#
# This is the local, context-isolated runner (clean HOME, only the fiskaly MCP).
# run-eval-docker.sh is the hermetic Docker variant. run-eval.sh delegates here.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
sims_root="$repo_root/sims"
scenarios_dir="$sims_root/scenarios"
model="${RUN_MODEL:-claude-sonnet-4-6}"
effort="${RUN_EFFORT:-medium}"

scenario="${1:-${SCENARIO:-}}"
[ -n "$scenario" ] || {
  echo "usage: run-scenario.sh <scenario-id>   (available below)" >&2
  ls -1 "$scenarios_dir" 2>/dev/null | grep -E '^[0-9]' >&2 || true
  exit 2
}

scenario_dir="$scenarios_dir/$scenario"
fixture="$scenario_dir/fixture"
task_file="$scenario_dir/task.md"
scenario_json="$scenario_dir/scenario.json"

[ -d "$fixture" ] || { echo "fixture not found: $fixture" >&2; exit 1; }
[ -f "$task_file" ] || { echo "task not found: $task_file" >&2; exit 1; }
[ -f "$scenario_json" ] || { echo "scenario.json not found: $scenario_json" >&2; exit 1; }
command -v claude >/dev/null || { echo "claude CLI not found" >&2; exit 1; }
command -v go >/dev/null || { echo "go not found" >&2; exit 1; }
command -v git >/dev/null || { echo "git not found" >&2; exit 1; }

# The task is business-framed: what the business needs, never how fiscalization
# works. Discovering the "how" — and seeing through any planted trap — is the point.
task="$(cat "$task_file")"

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

printf '{"harness":"local","coder":"claude-code","model":"%s","effort":"%s","scenario":"%s"}\n' \
  "$model" "$effort" "$scenario" >"$run_dir/meta.json"

# Build the fiskaly MCP and hand it to the coder.
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

echo "scenario: $scenario"
echo "run dir: $run_dir"
echo "model: $model   effort: $effort"
echo "running agent (headless)..."

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

# Grounded-in-docs check: did the agent search before writing integration code?
"$sims_root/evals/assert-grounded.sh" "$run_dir/transcript.jsonl" >"$run_dir/grounded.txt" 2>&1 \
  && grounded=GROUNDED || grounded="NOT-GROUNDED"

# Judge: deterministic fiskaly-contract conformance with this scenario's rule set.
(cd "$sims_root/judge" && go run . -scenario "$scenario_json" "$work") >"$run_dir/judge.txt" 2>&1 \
  && judge=PASS || judge=FAIL

# Summary from the final result event, if jq is available.
summary=""
if command -v jq >/dev/null; then
  summary="$(jq -rs 'map(select(.type=="result")) | last // empty
    | "turns=\(.num_turns)  cost=$\(.total_cost_usd)  agent_error=\(.is_error)"' \
    "$run_dir/transcript.jsonl" 2>/dev/null || true)"
fi

echo
echo "==== eval result: $scenario ===="
echo "build: $build    tests: $tests    judge: $judge    grounded: $grounded"
[ -n "$summary" ] && echo "$summary"
echo "judge:      $run_dir/judge.txt"
echo "diff:       $run_dir/changes.diff"
echo "transcript: $run_dir/transcript.jsonl"
echo "rubric:     $scenario_dir/SOLUTION.md"
echo "logs:       $run_dir/build.txt  $run_dir/test.txt  $run_dir/claude.err"
echo "workdir:    $work"
