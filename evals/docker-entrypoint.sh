#!/usr/bin/env bash
# Runs inside the container: drives the coder headless in /work (the mounted
# fixture) against the fiskaly MCP. Auth comes from CLAUDE_CODE_OAUTH_TOKEN in the
# environment. The container has no ~/.claude, so no operator CLAUDE.md or skills
# load. Only /work and the MCP binary are visible to the coder.
set -euo pipefail

task="${1:?task required as first argument}"
cd /work

exec claude -p "$task" \
  --model "${RUN_MODEL:-claude-sonnet-4-6}" --effort "${RUN_EFFORT:-medium}" \
  --mcp-config /etc/mcp.json --strict-mcp-config \
  --permission-mode bypassPermissions \
  --output-format stream-json --verbose
