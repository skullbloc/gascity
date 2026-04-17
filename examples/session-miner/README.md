# session-miner

Gas City pack for mining a filtered Claude Code session JSONL corpus
into draft blog posts.

## Shape

- **`perspective_planner`** — singleton agent. Reads a sample of the
  corpus, writes 3–5 perspective files describing angles worth
  examining. No hardcoded perspectives — everything is corpus-grounded.
- **`lens`** — pool of 4. Generic observation gatherer. Reads one
  perspective file per bead, emits structured observations with
  citations back to session + line.
- **`miner_coordinator`** — singleton. Drives the orchestration
  formula and drafts blog-post-style markdown from the observations
  at the end.

## Flow

1. `resolve-corpus.sh` filters `~/.claude/projects/<slug>/*.jsonl` by
   slug, date, or session UUID into a manifest.
2. The planner samples the manifest and writes perspective files.
   The step also writes `REVIEW.md` into the perspectives dir with
   release instructions.
3. The `dispatch-lenses` step carries a `[steps.gate] type = "human"`
   attribute. The formula compiler auto-synthesizes a sibling
   `Gate: human review-perspectives` bead at cook time; `bd close`
   on that bead releases the step. (`--var skip_gate=true` has the
   plan step auto-close the gate bead so nothing blocks.)
4. One lens bead per perspective is dispatched in parallel, up to
   pool size 4.
5. The coordinator reads all observations and drafts posts,
   applying a voice guide (pack default or operator-supplied).

## Minimal invocation

```bash
gc sling mining/miner_coordinator mol-session-mining --formula \
  --var project_slug=-home-admin-workspace-ck3proj
```

Optional vars:

| Var                           | Meaning |
|-------------------------------|---------|
| `project_slugs`               | Comma list (overrides `project_slug`) |
| `since` / `until`             | ISO date bounds |
| `session_ids`                 | Comma list of UUIDs |
| `focus`                       | Operator hint to the planner |
| `output_dir`                  | Defaults to `.gc/mined/<slug>/<yyyy-mm-dd>/` |
| `drafting_instructions_path`  | Path to a voice guide; falls back to pack default |
| `skip_planning=true` + `perspectives_override=<paths>` | Bypass planner |
| `skip_gate=true`              | Autonomous run (no human review) |
| `min_perspectives` / `max_perspectives` | Default 3 / 5 |

## Output layout

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
    └── <slug>.md   (zero or more — coordinator's call)
```

## Running the demo

Two-step setup: start the shared test dolt, then the test city.

```bash
# 1. Start the shared test dolt server (once per session)
./scripts/test-dolt.sh start         # listens on 127.0.0.1:13307 by default

# 2. Stand up the test city
mkdir ~/session-miner-demo && cd ~/session-miner-demo
git init
cp <repo>/examples/session-miner/city.toml .
# Edit city.toml: set [[rigs]] path and includes path as absolute paths
# pointing into the session-miner pack. Keep the [dolt] section so gc
# treats the shared dolt as external.
gc init .                            # creates .beads/, hooks, etc.
gc start .                           # supervisor + agents
bd config set issue_prefix mi        # match the rig-derived prefix to
                                     # avoid prefix-mismatch on formula cook

# 3. Dispatch
gc sling mining/miner_coordinator mol-session-mining --formula \
  --var project_slug=-home-admin-workspace-ck3proj
```

When done, tear down in reverse: `gc stop .`, `./scripts/test-dolt.sh stop`.

After the planner closes its bead, open a second terminal and review
the perspective files at `<rig>/.gc/mined/<slug>/<date>/perspectives/`
— a `REVIEW.md` in that dir has the specific release command. In
short:

```bash
bd list --type gate --status open | grep 'Gate: human review-perspectives'
bd close <gate-bead-id>
```

The lens pool spawns automatically and the coordinator picks up
when they finish.

## Extending

**New perspective for a project**: drop a markdown file at a path of
your choosing, then invoke with
`--var skip_planning=true --var perspectives_override=<paths>`.

**Different drafting voice**: write a markdown file describing voice
conventions, then invoke with `--var drafting_instructions_path=...`.
The pack's default style guide at `packs/session-miner/prompts/shared/default-drafting-style.md`
is the reference shape.

**Custom lens behavior**: edit `packs/session-miner/prompts/lens.md.tmpl`.
The observation block contract lives there.

## Design doc

See `plans/ga-nci-session-miner-pack.md` in the gascity repo for the
role topology, open-question resolutions, and alternatives considered.
