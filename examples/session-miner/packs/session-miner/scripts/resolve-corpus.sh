#!/usr/bin/env bash
# resolve-corpus.sh — enumerate and filter Claude Code session JSONL files
# into a manifest JSON for the session-miner pack.
#
# Reads sessions from $CLAUDE_PROJECTS_DIR (default: ~/.claude/projects),
# applies slug / date / session-id filters, and writes a manifest at the
# --output path.
#
# Manifest shape:
# {
#   "generated_at": "<ISO-8601 UTC>",
#   "projects_dir": "<path>",
#   "project_slugs": ["-home-admin-..."],
#   "date_range": "YYYY-MM-DD..YYYY-MM-DD" | null,
#   "total_bytes": <int>,
#   "total_lines": <int>,
#   "sessions": [
#     {
#       "session_id": "<uuid>",
#       "project_slug": "-home-admin-...",
#       "path": "<abs path>",
#       "bytes": <int>,
#       "line_count": <int>,
#       "first_timestamp": "<ISO-8601>",
#       "last_timestamp": "<ISO-8601>"
#     }
#   ]
# }
#
# Usage:
#   resolve-corpus.sh \
#     --project-slug <slug> \
#     [--project-slugs <a,b,c>] \
#     [--since YYYY-MM-DD] \
#     [--until YYYY-MM-DD] \
#     [--session-ids <uuid1,uuid2>] \
#     [--projects-dir <path>] \
#     --output <manifest.json>

set -euo pipefail

die() { echo "resolve-corpus: $*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || die "missing dependency: $1"; }
need jq
need python3

PROJECT_SLUG=""
PROJECT_SLUGS=""
SINCE=""
UNTIL=""
SESSION_IDS=""
PROJECTS_DIR="${CLAUDE_PROJECTS_DIR:-$HOME/.claude/projects}"
OUTPUT=""

while [ $# -gt 0 ]; do
  case "$1" in
    --project-slug)   PROJECT_SLUG="$2"; shift 2 ;;
    --project-slugs)  PROJECT_SLUGS="$2"; shift 2 ;;
    --since)          SINCE="$2"; shift 2 ;;
    --until)          UNTIL="$2"; shift 2 ;;
    --session-ids)    SESSION_IDS="$2"; shift 2 ;;
    --projects-dir)   PROJECTS_DIR="$2"; shift 2 ;;
    --output)         OUTPUT="$2"; shift 2 ;;
    *) die "unknown arg: $1" ;;
  esac
done

[ -n "$OUTPUT" ] || die "--output is required"
[ -d "$PROJECTS_DIR" ] || die "projects dir not found: $PROJECTS_DIR"

# Build the slug list.
SLUGS=()
if [ -n "$PROJECT_SLUGS" ]; then
  IFS=',' read -r -a SLUGS <<< "$PROJECT_SLUGS"
elif [ -n "$PROJECT_SLUG" ]; then
  SLUGS=("$PROJECT_SLUG")
else
  die "need --project-slug or --project-slugs"
fi

# Build the session-ids whitelist, if any.
declare -A SESSION_WHITELIST=()
if [ -n "$SESSION_IDS" ]; then
  IFS=',' read -r -a ids <<< "$SESSION_IDS"
  for id in "${ids[@]}"; do
    SESSION_WHITELIST["$id"]=1
  done
fi

mkdir -p "$(dirname "$OUTPUT")"

# Delegate the heavy lifting (JSONL scanning, frontmatter reading, JSON
# assembly) to a small Python helper — shell is the wrong tool once we
# start parsing JSON per line.
python3 - "$PROJECTS_DIR" "$OUTPUT" "$SINCE" "$UNTIL" \
  <<'PY' "${SLUGS[@]}" "--sessions" "${!SESSION_WHITELIST[@]}"
import json
import os
import sys
from datetime import datetime, timezone

projects_dir = sys.argv[1]
output_path = sys.argv[2]
since = sys.argv[3]
until = sys.argv[4]

# Remaining args: slugs..., --sessions, session_ids...
rest = sys.argv[5:]
if "--sessions" in rest:
    split = rest.index("--sessions")
    slugs = rest[:split]
    session_whitelist = set(rest[split + 1:])
else:
    slugs = rest
    session_whitelist = set()


def read_first_timestamp(path):
    try:
        with open(path, "rb") as f:
            line = f.readline()
            if not line:
                return None
            obj = json.loads(line.decode("utf-8", "replace"))
            return obj.get("timestamp")
    except Exception:
        return None


def read_last_timestamp(path):
    # Tail-read: seek near the end, find the last newline, parse the
    # trailing line. Robust to large files.
    try:
        size = os.path.getsize(path)
        with open(path, "rb") as f:
            if size < 8192:
                f.seek(0)
                tail = f.read()
            else:
                f.seek(-8192, os.SEEK_END)
                tail = f.read()
            lines = tail.splitlines()
            for line in reversed(lines):
                line = line.strip()
                if not line:
                    continue
                try:
                    obj = json.loads(line.decode("utf-8", "replace"))
                    return obj.get("timestamp")
                except Exception:
                    continue
    except Exception:
        pass
    return None


def count_lines(path):
    count = 0
    with open(path, "rb") as f:
        for _ in f:
            count += 1
    return count


def in_range(ts):
    if ts is None:
        return True  # No timestamp; don't exclude.
    try:
        # Claude Code timestamps are ISO-8601 with Z.
        t = datetime.fromisoformat(ts.replace("Z", "+00:00"))
    except ValueError:
        return True
    if since:
        try:
            s = datetime.fromisoformat(since).replace(tzinfo=timezone.utc)
            if t < s:
                return False
        except ValueError:
            pass
    if until:
        try:
            u = datetime.fromisoformat(until).replace(tzinfo=timezone.utc)
            # Make 'until' inclusive of the whole day.
            if u.hour == 0 and u.minute == 0 and u.second == 0:
                u = u.replace(hour=23, minute=59, second=59)
            if t > u:
                return False
        except ValueError:
            pass
    return True


sessions = []
for slug in slugs:
    slug_dir = os.path.join(projects_dir, slug)
    if not os.path.isdir(slug_dir):
        print(f"resolve-corpus: no such slug dir: {slug_dir}", file=sys.stderr)
        continue
    for name in sorted(os.listdir(slug_dir)):
        if not name.endswith(".jsonl"):
            continue
        session_id = name[:-len(".jsonl")]
        if session_whitelist and session_id not in session_whitelist:
            continue
        path = os.path.join(slug_dir, name)
        if not os.path.isfile(path):
            continue
        bytes_ = os.path.getsize(path)
        if bytes_ == 0:
            continue
        first_ts = read_first_timestamp(path)
        if not in_range(first_ts):
            continue
        last_ts = read_last_timestamp(path)
        line_count = count_lines(path)
        sessions.append({
            "session_id": session_id,
            "project_slug": slug,
            "path": path,
            "bytes": bytes_,
            "line_count": line_count,
            "first_timestamp": first_ts,
            "last_timestamp": last_ts,
        })

# Sort by first_timestamp (ascending), then path for stability.
sessions.sort(key=lambda s: (s["first_timestamp"] or "", s["path"]))

total_bytes = sum(s["bytes"] for s in sessions)
total_lines = sum(s["line_count"] for s in sessions)

# Date range summary.
dates = sorted([s["first_timestamp"][:10] for s in sessions if s.get("first_timestamp")])
date_range = f"{dates[0]}..{dates[-1]}" if dates else None

manifest = {
    "generated_at": datetime.now(timezone.utc).isoformat(timespec="seconds"),
    "projects_dir": projects_dir,
    "project_slugs": slugs,
    "filters": {
        "since": since or None,
        "until": until or None,
        "session_ids": sorted(session_whitelist) if session_whitelist else None,
    },
    "date_range": date_range,
    "total_bytes": total_bytes,
    "total_lines": total_lines,
    "sessions": sessions,
}

with open(output_path, "w", encoding="utf-8") as f:
    json.dump(manifest, f, indent=2)

print(f"resolve-corpus: wrote {len(sessions)} sessions, "
      f"{total_bytes} bytes, to {output_path}", file=sys.stderr)
PY
