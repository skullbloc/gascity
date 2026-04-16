---
kind: perspective
perspective: cross-session-linkage
corpus_manifest_sha256: EXAMPLE
generator: session-miner/perspective_planner
generated_at: EXAMPLE
pack: session-miner
pack_version: 0.1.0
schema_version: 1
example: true
---

# Perspective: cross-session-linkage

**This is an EXAMPLE file** shown to the planner when it's struggling
to find 3 distinct angles in a corpus. Do not copy verbatim.

## What to look for

Entities, decisions, or bodies of work that span multiple sessions
but are never explicitly named across them. The same thing called
different names. Implicit dependencies — session B couldn't have
worked without something session A produced, but neither session
says so. Work started in one session and quietly abandoned or
picked up in another.

Particularly useful in Gas Town-style multi-agent corpora where the
same rig has multiple slugs (`crew/*`, `polecats/*`, `witness`,
`refinery`) — decisions and artifacts hop between them without
explicit handoffs.

## Why this angle matters for THIS corpus

<Replace with specifics: which sessions seem to be "talking about
the same thing" under different names, or which slugs look like
they're coordinating implicitly.>

## Citation focus

Two-pass processing:
- Pass 1, per-session: extract entity mentions (names, file paths,
  identifiers, decision statements) with session attribution.
- Pass 2: merge across sessions. Look for same entity, different
  name. Look for session-B-references-without-source. Look for
  decisions in one session that constrain another.

Use `scripts/jsonl-by-timestamp.sh` to order; cross-references make
more sense when you know which session came first.

## Observation contract notes

Emphasize tags: `cross-session`, `implicit-dependency`, `alias`,
`handoff`, `orphan`. Each observation MUST cite at least two
sessions — that's what makes it a relational finding.
