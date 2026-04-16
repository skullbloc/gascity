#!/usr/bin/env bash
# prepare-explicit-perspectives.sh — copy operator-supplied perspective
# files into the run's perspectives directory, rewriting frontmatter to
# reflect this run, and emit an index.json.
#
# Used by the skip_planning branch of the session-miner formula.
#
# Usage:
#   prepare-explicit-perspectives.sh \
#     --paths <path1,path2,...> \
#     --dest <output dir> \
#     --manifest <corpus-manifest.json path>

set -euo pipefail
die() { echo "prepare-explicit-perspectives: $*" >&2; exit 1; }
need() { command -v "$1" >/dev/null || die "missing dependency: $1"; }
need jq
need python3
need sha256sum

PATHS=""
DEST=""
MANIFEST=""

while [ $# -gt 0 ]; do
  case "$1" in
    --paths)    PATHS="$2"; shift 2 ;;
    --dest)     DEST="$2"; shift 2 ;;
    --manifest) MANIFEST="$2"; shift 2 ;;
    *) die "unknown arg: $1" ;;
  esac
done

[ -n "$PATHS" ]    || die "--paths required"
[ -n "$DEST" ]     || die "--dest required"
[ -n "$MANIFEST" ] || die "--manifest required"
[ -f "$MANIFEST" ] || die "manifest not found: $MANIFEST"

mkdir -p "$DEST"
SHA=$(sha256sum "$MANIFEST" | awk '{print $1}')
NOW=$(date -u +%Y-%m-%dT%H:%M:%SZ)

IFS=',' read -r -a PATH_ARR <<< "$PATHS"

python3 - "$DEST" "$SHA" "$NOW" "${PATH_ARR[@]}" <<'PY'
import json
import os
import re
import sys
from pathlib import Path

dest = Path(sys.argv[1])
sha = sys.argv[2]
now = sys.argv[3]
paths = [Path(p) for p in sys.argv[4:]]

entries = []
for p in paths:
    if not p.is_file():
        print(f"prepare-explicit-perspectives: missing file: {p}", file=sys.stderr)
        sys.exit(1)
    body = p.read_text(encoding="utf-8")

    # Split frontmatter. Allow files that don't have any.
    fm = {}
    rest = body
    m = re.match(r"^---\n(.*?\n)---\n(.*)$", body, re.DOTALL)
    if m:
        fm_text = m.group(1)
        rest = m.group(2)
        for line in fm_text.splitlines():
            if not line.strip() or line.lstrip().startswith("#"):
                continue
            if ":" in line:
                k, _, v = line.partition(":")
                fm[k.strip()] = v.strip()

    slug = fm.get("perspective") or p.stem
    fm["perspective"] = slug
    fm["corpus_manifest_sha256"] = sha
    fm["generated_at"] = now
    fm["generator"] = "session-miner/operator-supplied"
    fm.setdefault("kind", "perspective")
    fm.setdefault("pack", "session-miner")
    fm.setdefault("pack_version", "0.1.0")
    fm.setdefault("schema_version", "1")

    # Drop example marker if present — these are real perspectives now.
    fm.pop("example", None)

    new_fm = "\n".join(f"{k}: {v}" for k, v in fm.items())
    new_body = f"---\n{new_fm}\n---\n{rest.lstrip()}"

    out_path = dest / f"{slug}.md"
    out_path.write_text(new_body, encoding="utf-8")
    entries.append({
        "slug": slug,
        "title": f"Operator-supplied: {slug}",
        "rationale": f"Supplied via --var perspectives_override from {p.name}",
    })

index = {"perspectives": entries}
(dest / "index.json").write_text(json.dumps(index, indent=2), encoding="utf-8")
print(f"prepare-explicit-perspectives: wrote {len(entries)} perspective(s) to {dest}",
      file=sys.stderr)
PY
