---
kind: perspective
perspective: dataset-taxonomy
corpus_manifest_sha256: EXAMPLE
generator: session-miner/perspective_planner
generated_at: EXAMPLE
pack: session-miner
pack_version: 0.1.0
schema_version: 1
example: true
---

# Perspective: dataset-taxonomy

**This is an EXAMPLE file** shown to the planner when it's struggling
to find 3 distinct angles in a corpus. Do not copy verbatim.

## What to look for

Raw structured outputs produced during the work — class lists, token
tables, enumerations, glossaries, generated configs, or anything that
looks dataset-shaped. Then: the distribution shape, the outliers,
the things that don't fit the obvious categories, and the
not-yet-classified cases.

A "taxonomy" perspective only works when the corpus actually produced
data to taxonomize. If the corpus is conversational without
structured outputs, this angle doesn't fit — don't force it.

## Why this angle matters for THIS corpus

<Replace with specifics: what structured outputs exist, where they
live, rough size, and why their structure is worth examining.>

## Citation focus

Order-insensitive — arbitrary chunking is fine. Focus citations on:
- tool_uses that write to files ending in `.json`, `.csv`, `.tsv`,
  `.yaml`, or big `.md` tables.
- Assistant messages that render tables or lists with ≥ 20 entries.
- Moments where the agent categorizes or groups things explicitly.

Look for outliers explicitly: the largest class, the smallest, the
weird-shaped ones, the duplicates, the things the corpus couldn't
quite fit.

## Observation contract notes

Emphasize tags: `dataset`, `outlier`, `distribution`, `gap`,
`misfit`.
