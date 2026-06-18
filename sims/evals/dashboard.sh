#!/usr/bin/env bash
# Launch the eval dashboard:  ./sims/evals/dashboard.sh   then open http://localhost:8080
#
# The dashboard is a Next.js app in sims/dashboard/. It reads ~/.cache/fiskaly-eval
# and triggers runs via the runner (cd sims/runner && go run . run <id>).
set -euo pipefail
sims_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$sims_root/dashboard"
pnpm install --frozen-lockfile
exec pnpm dev -p 8080 "$@"
