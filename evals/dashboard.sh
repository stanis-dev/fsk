#!/usr/bin/env bash
# Launch the eval dashboard:  ./evals/dashboard.sh   then open http://localhost:8080
#
# The dashboard is a Next.js app in dashboard/. It reads ~/.cache/fiskaly-eval and
# triggers runs via evals/run-eval-docker.sh (override with FISKALY_EVAL_SCRIPT).
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root/dashboard"
pnpm install --frozen-lockfile
exec pnpm dev -p 8080 "$@"
