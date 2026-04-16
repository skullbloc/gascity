# session-miner work directory

This file is dropped into every session-miner agent's work directory
at session start as a stable touchpoint you can read if you need to
re-orient after a context reset.

## What runs here

One of three agents, depending on who owns this session:

- **perspective_planner** — reads a sample of a JSONL corpus, writes
  3–5 perspective files + an `index.json`.
- **lens** — reads one perspective file and a corpus manifest, emits
  a structured observations markdown file with citations.
- **miner_coordinator** — drives the `mol-session-mining` formula
  (orchestration) and drafts the posts (authoring) at the end.

Your specific role is determined by:
1. Your session's agent name (`$GC_AGENT`).
2. The metadata on the bead you've been handed via `gc hook`.

## Where things live

Output paths are on the root bead's metadata. Typical layout:

```
<output_dir>/
├── corpus-manifest.json
├── perspectives/
│   ├── index.json
│   └── <slug>.md
├── observations/
│   └── <slug>.md
└── posts/
    ├── index.md
    └── <slug>.md
```

## Recovery after context reset

```bash
gc prime
bd prime
bd show $(gc hook | grep -oE '[a-z]+-[a-z0-9]+$') --json
```

That prints the hook bead with its metadata — includes every path
your role needs. Read your prompt template again (via `gc session
prompt $GC_AGENT`) if you've lost track of the contract.
