#!/usr/bin/env bash
set -euo pipefail
sims_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$sims_root/dashboard"
pnpm install --frozen-lockfile
exec pnpm dev -p 8080 "$@"
