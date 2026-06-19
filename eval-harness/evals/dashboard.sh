#!/usr/bin/env bash
# Launch the eval dashboard:  ./eval-harness/evals/dashboard.sh   then open http://localhost:8080
#
# The dashboard is a Next.js app in eval-harness/dashboard/. It reads ~/.cache/fiskaly-eval
# and triggers runs via the runner (cd eval-harness/backend && go run ./cmd/eval-harness run <id>).
set -euo pipefail
sims_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$sims_root/dashboard"
pnpm install --frozen-lockfile
exec pnpm dev -p 8080 "$@"
