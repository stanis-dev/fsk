#!/usr/bin/env bash
# Launch the eval dashboard from anywhere:  ./evals/dashboard.sh
#
# dashboard/ is its own Go module, so we run from inside it (the repo root has no
# go.mod) and pass the absolute path to the run script. Open http://localhost:8080.
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root/dashboard"
exec go run . -script "$repo_root/evals/run-eval-docker.sh" "$@"
