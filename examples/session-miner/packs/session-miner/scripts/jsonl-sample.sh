#!/usr/bin/env bash
# jsonl-sample.sh — emit a deterministic "first + middle + last" sample
# of N lines from a JSONL file, preserving line numbers.
#
# Output format: each sampled line is prefixed with "L<num>\t" so the
# caller can cite back to the original file.
#
# Usage:
#   jsonl-sample.sh <file.jsonl> <n>
#
# Sampling strategy: first N/3, middle N/3 (around file midpoint),
# last N/3. Rounding distributes remainders toward the first section.
# Preserves order.

set -euo pipefail
FILE="${1:-}"
N="${2:-60}"

[ -n "$FILE" ] && [ -f "$FILE" ] || {
  echo "jsonl-sample: usage: $0 <file.jsonl> <n>" >&2
  exit 1
}
[[ "$N" =~ ^[0-9]+$ ]] || {
  echo "jsonl-sample: n must be a positive integer" >&2
  exit 1
}

TOTAL=$(wc -l < "$FILE")

if [ "$TOTAL" -le "$N" ]; then
  # File shorter than sample — just emit all lines prefixed.
  awk '{printf "L%d\t%s\n", NR, $0}' "$FILE"
  exit 0
fi

THIRD=$((N / 3))
REMAINDER=$((N - THIRD * 3))
FIRST=$((THIRD + REMAINDER))
MIDDLE=$THIRD
LAST=$THIRD

MID_START=$(( (TOTAL - MIDDLE) / 2 + 1 ))
MID_END=$(( MID_START + MIDDLE - 1 ))

LAST_START=$(( TOTAL - LAST + 1 ))

awk -v first="$FIRST" \
    -v mid_start="$MID_START" -v mid_end="$MID_END" \
    -v last_start="$LAST_START" '
  NR <= first                        { printf "L%d\t%s\n", NR, $0; next }
  NR >= mid_start && NR <= mid_end   { printf "L%d\t%s\n", NR, $0; next }
  NR >= last_start                   { printf "L%d\t%s\n", NR, $0; next }
' "$FILE"
