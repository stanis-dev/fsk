#!/usr/bin/env bash
# assert-grounded.sh — the consumer agent must ground itself in the docs before
# writing integration code: the transcript must contain a search_fiskaly_docs
# tool call BEFORE the first file mutation (Write/Edit/MultiEdit).
#
# Usage: evals/assert-grounded.sh <transcript.jsonl>
# Exit:  0 grounded · 1 not grounded · 2 usage/error
set -euo pipefail
transcript="${1:?usage: assert-grounded.sh <transcript.jsonl>}"
[ -f "$transcript" ] || { echo "transcript not found: $transcript" >&2; exit 2; }

# stream-json writes one event per line, in order, so line numbers are a faithful
# ordering. We compare the first search call against the first code mutation.
search_line=$(grep -n '"name":"search_fiskaly_docs"' "$transcript" | head -1 | cut -d: -f1 || true)
mutate_line=$(grep -nE '"name":"(Write|Edit|MultiEdit)"' "$transcript" | head -1 | cut -d: -f1 || true)

if [ -z "$search_line" ]; then
  echo "NOT GROUNDED: agent never called search_fiskaly_docs"
  exit 1
fi
if [ -z "$mutate_line" ]; then
  echo "INCONCLUSIVE: agent searched but never wrote integration code"
  exit 1
fi
if [ "$search_line" -lt "$mutate_line" ]; then
  echo "GROUNDED: searched (line $search_line) before first code change (line $mutate_line)"
  exit 0
fi
echo "NOT GROUNDED: first code change (line $mutate_line) precedes first search (line $search_line)"
exit 1
