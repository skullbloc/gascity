#!/usr/bin/env bash
# jsonl-by-timestamp.sh — print session JSONL file paths in ascending
# first-timestamp order.
#
# Reads a corpus manifest and emits one absolute path per line.
#
# Usage:
#   jsonl-by-timestamp.sh <corpus-manifest.json>

set -euo pipefail
MANIFEST="${1:-}"
[ -n "$MANIFEST" ] && [ -f "$MANIFEST" ] || {
  echo "jsonl-by-timestamp: usage: $0 <corpus-manifest.json>" >&2
  exit 1
}

command -v jq >/dev/null || { echo "jq required" >&2; exit 1; }

# The manifest is already sorted by first_timestamp; echo paths out.
jq -r '.sessions[].path' "$MANIFEST"
