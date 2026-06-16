#!/usr/bin/env bash
# Hermetic eval: the coder runs inside Docker Desktop with ONLY the fixture mounted,
# so it cannot reach the repo, the MCP/judge source, or research/. Auth via the OAuth
# token in .env. Observe (build/test/judge/diff) runs on the host afterward.
#
# Engine: Docker Desktop. On macOS it runs the container in its own Linux VM, so the
# isolation is VM + container (same boundary colima gave us). We pin the context so
# the run can't silently land on another engine if one is also configured.
#
# Needs: Docker Desktop running, go, git. jq optional (nicer summary).

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fixture="$repo_root/pos"
image="fiskaly-eval"
model="${RUN_MODEL:-claude-sonnet-4-6}"
effort="${RUN_EFFORT:-medium}"

# Pin to Docker Desktop. Another context (e.g. colima) may also exist on this machine;
# being explicit keeps the eval on the intended engine. Override with DOCKER_CONTEXT.
export DOCKER_CONTEXT="${DOCKER_CONTEXT:-desktop-linux}"

command -v docker >/dev/null || { echo "docker not found" >&2; exit 1; }
command -v go >/dev/null || { echo "go not found" >&2; exit 1; }
command -v git >/dev/null || { echo "git not found" >&2; exit 1; }

docker context inspect "$DOCKER_CONTEXT" >/dev/null 2>&1 || {
  echo "docker context '$DOCKER_CONTEXT' not found. Is Docker Desktop installed?" >&2
  exit 1
}
docker info >/dev/null 2>&1 || {
  echo "docker daemon not reachable on context '$DOCKER_CONTEXT'. Is Docker Desktop running?" >&2
  exit 1
}

oauth_token="$(sed -nE 's/^CLAUDE_CODE_OAUTH_TOKEN=//p' "$repo_root/.env" | head -1 | sed -E 's/^["'"'"']//; s/["'"'"']$//')"
[ -n "$oauth_token" ] || {
  echo "CLAUDE_CODE_OAUTH_TOKEN not found in $repo_root/.env" >&2
  exit 1
}

read -r -d '' task <<'EOF' || true
This is the checkout backend for an Italian point-of-sale system. By law an order
cannot be completed until it has been fiscalized, and fiscalization is not
implemented yet. Implement it using fiskaly as the fiscalization provider: make
CompleteOrder fiscalize a paid order through fiskaly and only let the order reach
the completed state once that has succeeded. Keep the existing tests green and add
tests for the new behavior.
EOF

echo "building image (cached after first build)..."
docker build -f "$repo_root/evals/Dockerfile" -t "$image" "$repo_root"

# Runs live under ~/.cache/fiskaly-eval so the dashboard finds them (same path as
# run-eval.sh). It's under /Users, which Docker Desktop shares into its VM by default,
# so the $work bind mount below propagates the coder's changes back to the host.
runs_base="${HOME}/.cache/fiskaly-eval"
mkdir -p "$runs_base"
run_dir="$(mktemp -d "$runs_base/run.XXXXXX")"
work="$run_dir/pos"
cp -R "$fixture" "$work"
git -C "$work" init -q
git -C "$work" -c user.email=eval@local -c user.name=eval add -A
git -C "$work" -c user.email=eval@local -c user.name=eval commit -qm baseline

printf '{"harness":"docker","coder":"claude-code","model":"%s","effort":"%s"}\n' "$model" "$effort" >"$run_dir/meta.json"

echo "run dir: $run_dir"
echo "model: $model   effort: $effort"
echo "running coder in docker (only the fixture is mounted)..."

# Only $work is mounted. No repo, no research/, no MCP/judge source.
docker run --rm \
  -e CLAUDE_CODE_OAUTH_TOKEN="$oauth_token" \
  -e IS_SANDBOX=1 \
  -e RUN_MODEL="$model" -e RUN_EFFORT="$effort" \
  -v "$work:/work" \
  "$image" "$task" >"$run_dir/transcript.jsonl" 2>"$run_dir/claude.err" || true

# Observe on the host: build, test, judge, diff.
(cd "$work" && go build ./...) >"$run_dir/build.txt" 2>&1 && build=PASS || build=FAIL
(cd "$work" && go test ./...) >"$run_dir/test.txt" 2>&1 && tests=PASS || tests=FAIL
git -C "$work" add -A
git -C "$work" diff --cached >"$run_dir/changes.diff"
(cd "$repo_root/judge" && go run . "$work") >"$run_dir/judge.txt" 2>&1 && judge=PASS || judge=FAIL

summary=""
if command -v jq >/dev/null; then
  summary="$(jq -rs 'map(select(.type=="result")) | last // empty
    | "turns=\(.num_turns)  cost=$\(.total_cost_usd)  agent_error=\(.is_error)"' \
    "$run_dir/transcript.jsonl" 2>/dev/null || true)"
fi

echo
echo "==== eval result (docker) ===="
echo "build: $build    tests: $tests    judge: $judge"
[ -n "$summary" ] && echo "$summary"
echo "judge:      $run_dir/judge.txt"
echo "diff:       $run_dir/changes.diff"
echo "transcript: $run_dir/transcript.jsonl"
echo "workdir:    $work"
