#!/usr/bin/env bash
# await-children.sh — poll a list of bead IDs until all are closed.
#
# Reads bead IDs from:
#   - positional args
#   - OR whitespace-separated content of the file passed as $1 (if it's a
#     readable file rather than a bead ID)
#
# Logs status transitions. Exits 0 when every bead is closed.
#
# Usage:
#   await-children.sh gt-abc gt-def ...
#   await-children.sh /path/to/file-with-bead-ids
#
# Environment:
#   POLL_INTERVAL — seconds between polls (default: 30)
#   MAX_WAIT      — max seconds to wait total; 0 = unbounded (default: 0)

set -euo pipefail

die() { echo "await-children: $*" >&2; exit 1; }
command -v bd >/dev/null 2>&1 || die "bd not found on PATH"
command -v jq >/dev/null 2>&1 || die "jq not found on PATH"

POLL_INTERVAL="${POLL_INTERVAL:-30}"
MAX_WAIT="${MAX_WAIT:-0}"

# Collect IDs.
IDS=()
if [ $# -eq 1 ] && [ -r "$1" ] && ! [[ "$1" =~ ^[a-z]+-[a-z0-9]+$ ]]; then
  # Treat sole arg as a file to read IDs from.
  # shellcheck disable=SC2207
  IDS=( $(tr '[:space:]' '\n' < "$1" | grep -v '^$' || true) )
else
  IDS=("$@")
fi

[ ${#IDS[@]} -gt 0 ] || die "no bead IDs supplied"

declare -A LAST_STATUS=()
for id in "${IDS[@]}"; do
  LAST_STATUS["$id"]="?"
done

START=$(date +%s)
while true; do
  ALL_CLOSED=1
  OPEN_COUNT=0
  for id in "${IDS[@]}"; do
    STATUS=$(bd show "$id" --json 2>/dev/null | jq -r '.[0].status // "unknown"')
    if [ "${LAST_STATUS[$id]}" != "$STATUS" ]; then
      echo "await-children: $id: ${LAST_STATUS[$id]} -> $STATUS"
      LAST_STATUS["$id"]="$STATUS"
    fi
    if [ "$STATUS" != "closed" ]; then
      ALL_CLOSED=0
      OPEN_COUNT=$((OPEN_COUNT + 1))
    fi
  done

  if [ "$ALL_CLOSED" -eq 1 ]; then
    echo "await-children: all ${#IDS[@]} bead(s) closed."
    exit 0
  fi

  if [ "$MAX_WAIT" -gt 0 ]; then
    ELAPSED=$(($(date +%s) - START))
    if [ "$ELAPSED" -ge "$MAX_WAIT" ]; then
      echo "await-children: MAX_WAIT ($MAX_WAIT s) exceeded; $OPEN_COUNT still open."
      exit 2
    fi
  fi

  sleep "$POLL_INTERVAL"
done
