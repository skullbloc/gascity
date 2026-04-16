#!/usr/bin/env bash
# jsonl-tool-uses-only.sh — emit only the JSONL lines whose message.content
# contains a tool_use block.
#
# Each emitted line is prefixed with "L<num>\t" so the caller can cite
# back to the original file.
#
# Useful for methodology / tool-churn perspectives that only care about
# what the agent ran, not what it thought or said.
#
# Usage:
#   jsonl-tool-uses-only.sh <file.jsonl>

set -euo pipefail
FILE="${1:-}"
[ -n "$FILE" ] && [ -f "$FILE" ] || {
  echo "jsonl-tool-uses-only: usage: $0 <file.jsonl>" >&2
  exit 1
}
command -v jq >/dev/null || { echo "jq required" >&2; exit 1; }

awk 'NR { print NR "\t" $0 }' "$FILE" |
while IFS=$'\t' read -r LNUM LINE; do
  # message.content may be a string (user messages) or an array of blocks.
  # We want array-valued content whose blocks include {"type": "tool_use"}.
  echo "$LINE" | jq -e '
    (.message.content | type == "array") and
    (any(.message.content[]; .type == "tool_use"))
  ' >/dev/null 2>&1 && printf "L%s\t%s\n" "$LNUM" "$LINE" || true
done
