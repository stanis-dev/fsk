#!/usr/bin/env bash
# run-eval.sh — the simplest one-shot eval: scenario 01 (Zero to Receipt).
#
# Kept as the original entrypoint. It now delegates to run-scenario.sh, which runs
# any scenario from sims/scenarios/. Override the scenario with SCENARIO=<id> or by
# passing an id as the first argument.
#
# Usage:  evals/run-eval.sh                 # scenario 01-zero-to-receipt
#         evals/run-eval.sh 06-fire-and-forget
#         SCENARIO=03-cancellation evals/run-eval.sh
# Env:    RUN_MODEL  (default: claude-sonnet-4-6)   RUN_EFFORT (default: medium)
set -euo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec "$here/run-scenario.sh" "${1:-${SCENARIO:-01-zero-to-receipt}}"
