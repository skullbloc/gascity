---
kind: perspective
perspective: methodology-walkthrough
corpus_manifest_sha256: EXAMPLE
generator: session-miner/perspective_planner
generated_at: EXAMPLE
pack: session-miner
pack_version: 0.1.0
schema_version: 1
example: true
---

# Perspective: methodology-walkthrough

**This is an EXAMPLE file** shown to the planner when it's struggling
to find 3 distinct angles in a corpus. Do not copy verbatim. Adapt
the shape to whatever the corpus at hand actually supports.

## What to look for

How the work actually unfolded over time, not the tidy version. The
order in which tools were tried and dropped. The moments where the
approach shifted. The workarounds that got adopted quietly and
never documented. The places where a session ended without closure
and the next session had to rediscover context.

## Why this angle matters for THIS corpus

<Replace with specifics from the corpus sample. Without those
specifics, a "methodology" perspective is just the generic one
that fits every corpus — too abstract to be useful.>

## Citation focus

Process sessions in ascending timestamp order — `scripts/jsonl-by-timestamp.sh`
helps. Within a session, preserve JSONL order. Focus citations on:
- Bash tool_uses that install, configure, or abandon tools.
- thinking blocks where the agent says "actually, let me..." or
  similar reframe language.
- tool_uses that fail and aren't retried.
- Session-boundary points where cwd or gitBranch changes.

If a single session exceeds the lens agent's context, partition by
line offset but preserve the global order in observations.

## Observation contract notes

Emphasize tags: `pivot`, `dead-end`, `workaround`, `methodology`,
`tool-churn`.
