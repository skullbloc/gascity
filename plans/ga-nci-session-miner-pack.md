# ga-nci — session-miner pack

Design doc. Implementation blocked on operator review.

## TL;DR

Ship a Gas City pack at `examples/session-miner/` with three
rig-scoped agents:

- **`perspective_planner`** (singleton) — reads a sample of the
  filtered Claude Code session JSONL corpus and writes 3–5
  perspective files, each a corpus-grounded "here's an angle worth
  examining and why". Nothing about perspectives is hardcoded in the
  pack; example perspective files ship alongside only as fallback
  inspiration surfaced in the planner's prompt.
- **`lens`** (pool, `max=4`) — generic observation gatherer. Reads
  one perspective file per bead, writes structured
  observations-with-citations to disk. Homogeneous workers.
- **`miner_coordinator`** (singleton) — reads all observations and
  drafts blog-post-style markdown files, one per theme it judges
  post-worthy. Count is coordinator-decided. Drafting-style guidance
  is supplied per-invocation by the operator via a file reference,
  falling back to a pack default.

A single `session-mining.formula.toml` orchestrates: resolve-corpus
→ plan-perspectives → **human gate** (operator reviews/edits the
planner's perspective files before releasing) → dispatch-lenses
(parallel) → await-lenses → dispatch-coordinator → await-coordinator
→ report. No orders. No mail. MVP re-runs from scratch and overwrites
outputs.

## What the pack does

1. Operator invokes the pack with a corpus spec and optionally a
   focus string, a drafting-instructions file path, and an explicit
   perspective list (override hatch).
2. A shell helper resolves the corpus spec into a concrete JSONL file
   list and a manifest (session count, date range, byte total).
3. Unless `--var skip_planning=true` is set, the orchestrator dispatches
   the `perspective_planner` to a bead with metadata pointing at the
   manifest, focus, and output paths. Planner samples the corpus and
   writes 3–5 perspective files. It then closes its bead.
4. **Human gate**: the formula creates a review bead. Operator inspects
   the generated perspective files, edits or removes any they don't
   want, and closes the review bead to release. (Or passes `--var
   skip_gate=true` for autonomous runs.)
5. The orchestrator reads the final set of perspective files and
   creates one child bead per file, each with metadata pointing at the
   perspective file, the corpus manifest, and an observations output
   path. Slings all to the `lens` pool; up to 4 workers run in
   parallel.
6. `lens` workers each read their perspective file + corpus manifest
   and emit a markdown file of structured observation blocks (title,
   finding, citations, tags). Each agent decides how to handle context
   limits (chunk-read, subagent dispatch).
7. After all lens beads close, the orchestrator creates a coordinator
   bead with metadata pointing at the observations directory, the
   drafting-instructions path, and the posts output directory. Slings
   to `miner_coordinator`.
8. Coordinator reads all observations, identifies themes it judges
   post-worthy, and drafts one blog-post-style markdown file per
   theme. Also writes an `index.md` listing drafted posts with a
   one-line rationale each.
9. Formula reports the posts directory path.

## Non-goals (from the operator brief, restated)

- Not a generic session analysis framework — shape to Harrison's corpus.
- Don't replicate what Gas City already does (dispatch, pools, pack
  composition, formulas).
- No multi-user / team workflows.
- Don't commit to an LLMWiki schema here — outputs must be consumable,
  not prescribed.
- No UI.

## Role topology

**Three agents ship with the pack**, all `scope = "rig"`.

| Agent | Count | Role | Output |
|-------|-------|------|--------|
| `perspective_planner` | Singleton (`min=0, max=1`) | Reads a corpus sample and writes 3–5 perspective files describing what's worth looking at here | `perspectives/<slug>.md` + `perspectives/index.json` |
| `lens` | Pool (`min=0, max=4`) | Generic gatherer — reads one perspective file per bead and emits structured observations with citations | `observations/<slug>.md` per bead |
| `miner_coordinator` | Singleton (`min=0, max=1`) | Author — drafts blog-post-style writeups from all observations | `posts/<slug>.md` (N, coordinator-chosen) + `posts/index.md` |

**Perspectives are data, not agent identity.** A lens worker is
homogeneous; its behavior is determined by the perspective file handed
to it via bead metadata. This is how the pack stays extensible
without requiring new agent declarations for new analytical angles.

**Lenses gather, they don't author.** Observations are block-structured
raw material with citations, tuned for the coordinator to pull quotes
from — not prose for re-reading.

**The coordinator decides what gets drafted.** It reads all observation
files and judges which themes have enough substance and surprise to
merit a blog-post-style draft. It decides how many posts to produce,
their titles, and their scope.

**Why rig-scoped?** (1) Mining jobs are corpus-bound, and corpora are
project-bound. (2) Multiple rigs can run their own mining jobs
concurrently without fighting over pool slots.

**Why `max=4` on the lens pool?** The planner caps at 5 perspectives.
A pool of 4 means up to 4 perspectives run concurrently while a 5th
waits briefly — a reasonable balance between parallelism and not
pinning the machine. Easy to retune later.

**Why singletons for planner and coordinator?** One planning pass per
job; one drafting pass per job. No parallelism gain from a pool.

## Agent declaration & perspective extensibility

Only one lens-related agent in the pack manifest:

```toml
[pack]
name = "session-miner"
schema = 1

[[agent]]
name = "perspective_planner"
scope = "rig"
prompt_template = "prompts/perspective-planner.md.tmpl"
nudge = "Check your hook for a planning job."
overlay_dir = "overlays/default"
idle_timeout = "30m"
min_active_sessions = 0
max_active_sessions = 1

[[agent]]
name = "lens"
scope = "rig"
prompt_template = "prompts/lens.md.tmpl"
nudge = "Check your hook for a lens job."
overlay_dir = "overlays/default"
idle_timeout = "30m"
min_active_sessions = 0
max_active_sessions = 4

[[agent]]
name = "miner_coordinator"
scope = "rig"
prompt_template = "prompts/coordinator.md.tmpl"
nudge = "Check your hook for a coordination job."
overlay_dir = "overlays/default"
idle_timeout = "1h"
min_active_sessions = 0
max_active_sessions = 1
```

**Perspectives are not agents.** They're markdown files the planner
writes (or the operator provides). To shape what gets observed, you
work with perspectives, not with agent declarations.

Three ways to shape what gets observed:

1. **Let the planner decide** — default. Planner reads the corpus and
   emits 3–5 perspective files. Operator reviews at the human gate.
2. **Seed the planner** — pass `--var focus="developer experience,
   onboarding friction"` and the planner uses that as guidance.
3. **Override the planner entirely** — pass `--var perspectives=file1.md,file2.md`
   and set `--var skip_planning=true`. Formula jumps past planning and
   uses your files as-is.

Editing at the human gate is a fourth, finer-grained route: planner
runs, you tweak or delete files, you release.

### Alternatives considered

- **Lens-per-perspective agents** (chronological_lens / taxonomic_lens /
  relational_lens each as `[[agent]]`). Rejected by operator direction:
  hardcoded perspectives aren't data, they're config. The pool model
  makes perspective selection corpus-adaptive.
- **Coordinator does planning too.** Rejected: the planning and drafting
  passes want different context windows and different success criteria.
  Keeping them separate lets us iterate on the planner prompt without
  disturbing the drafter.

## Corpus filtering

MVP filters, passed as formula vars:

| Var                | Meaning |
|--------------------|---------|
| `project_slug`     | One slug from `~/.claude/projects/` (e.g., `-home-admin-workspace-ck3proj`) |
| `project_slugs`    | Comma-separated list (overrides `project_slug` if set) |
| `since`            | ISO date; filter session files by first-line timestamp ≥ this |
| `until`            | ISO date; filter by first-line timestamp ≤ this |
| `session_ids`      | Comma-separated list of session UUIDs; overrides slug-based selection |
| `output_dir`       | Where the pack writes. Defaults to `.gc/mined/<project-slug>/<yyyy-mm-dd>/` when unset. Observations go to `<output_dir>/observations/`, drafts go to `<output_dir>/posts/`, perspectives to `<output_dir>/perspectives/` |
| `drafting_instructions_path` | Optional path to a markdown file of operator-supplied drafting-style guidance (voice, length, scope, structure). Coordinator reads this if set; otherwise uses `prompts/shared/default-drafting-style.md` from the pack |

Resolved by `scripts/resolve-corpus.sh`:
- Enumerates candidate JSONL files.
- Applies filters.
- Emits `corpus-manifest.json` with absolute paths, per-session line counts, byte totals, earliest/latest timestamps.
- The orchestrator stashes this manifest at a path referenced by each
  child bead's metadata.

Filters expressed as CLI vars rather than a separate config file: a
config file adds a layer of indirection for a single-use invocation, and
this is a one-shot pack, not a long-lived daemon.

## Partitioning strategy

**Perspectives decide, the `lens` prompt executes.** The generic `lens`
prompt establishes a surprise-over-coverage bias and a citation
requirement, then defers to the perspective file for what to look at
and how to approach the corpus. A perspective file's "Citation focus"
section is where partitioning guidance lives — e.g., "process sessions
in timestamp order"; "one-pass arbitrary chunks are fine"; "two-pass:
per-session then merge".

The planner's prompt includes guidance like: "If the corpus is
timeline-heavy, recommend a time-ordered approach in the perspective.
If it's dataset-heavy, recommend chunked sampling. If it's
multi-session entity-linked, recommend two-pass."

Pack ships helper scripts (`scripts/jsonl-by-timestamp.sh`,
`scripts/jsonl-sample.sh`, `scripts/jsonl-tool-uses-only.sh`) that the
agents can invoke from within their session. Pack does not mandate how
any agent uses them — Bitter Lesson.

## Perspective planner

**Prompt** (`prompts/perspective-planner.md.tmpl`) establishes:
- Read the corpus manifest at `$CORPUS_MANIFEST`.
- Sample the corpus (choose strategy based on size and shape:
  first/last N lines per session, smallest session full-read, random
  line sampling, tool_use-only pre-filter).
- Honor `$FOCUS` if non-empty as operator-supplied guidance, but do
  not let it constrain you to angles that don't fit the corpus.
- Produce **3–5 perspective files** at `$PERSPECTIVES_DIR/<slug>.md`.
  Target 3; go up to 5 only if the corpus genuinely supports
  non-overlapping distinct angles. More is not better — coordinator
  will be reading all of it.
- Also write `$PERSPECTIVES_DIR/index.json` listing each generated
  perspective (slug, title, one-line rationale).
- When struggling to find 3 distinct angles, consult the example
  perspective files at `$PACK_DIR/perspectives/examples/` for
  inspiration — but adapt them to this corpus, don't copy them.

**Perspective file shape:**

```markdown
---
perspective: methodology-shifts
corpus_manifest_sha256: <hash>
generator: session-miner/perspective_planner
generated_at: 2026-04-16T12:45:00Z
pack: session-miner
pack_version: 0.1.0
schema_version: 1
---

# Perspective: methodology-shifts

## What to look for

<2–6 sentences describing the angle.>

## Why this angle matters for THIS corpus

<Planner's justification grounded in what it saw in the sample. This
section is what distinguishes a corpus-grounded perspective from a
generic one.>

## Citation focus

<Hints about which JSONL blocks, fields, or patterns to lean on.
Partitioning guidance goes here if order or passes matter.>

## Observation contract notes

<Any deviations from the standard observation block format, or tags
this perspective should emphasize.>
```

**Example perspective files** (ship in pack at
`perspectives/examples/`): three general-purpose starters — one
methodology-oriented, one taxonomy-oriented, one relational — exist as
reference material the planner can draw from when struggling. They are
NOT default output; the planner writes its own files, grounded in
this corpus. They are read only under the "struggling" fallback branch
of the planner's prompt.

## Human gate

After the planner closes its bead, the formula creates a review bead
with metadata `perspectives_dir=<path>`, `nudge_text="Review perspective
files at <path>, edit or delete any, then close this bead to release."`.
The formula's next step (dispatch-lenses) `needs = ["human-gate"]` —
so it blocks until that review bead is closed.

Two operator paths:
- Interactive: review the perspective files in the directory, edit or
  delete as needed, then `bd close <gate-bead>`.
- Autonomous: pass `--var skip_gate=true` at invocation. Formula makes
  the gate step a no-op (created-and-closed in one action).

The gate step does not lean on agents — it's a direct bead created
and waited on by the formula's shell step. This avoids any dependency
on a specific agent being available to render "gate" into UI.

## Handling context limits

Any agent (planner, lens, coordinator) is free to:
- Read partitions sequentially from disk and summarize incrementally.
- Dispatch a subagent per partition and stitch results.
- Use the helper scripts to pre-filter JSONL.

Pack does not pre-partition. Pre-partitioning would force a scheme at
pack level that each agent would have to work around. Pushing the
decision into agent prompts (and the perspective file's "Citation
focus" section) keeps the SDK-infrastructure / agent-work boundary
clean.

## Output schema

### Lens observation files

Written to `<output_dir>/observations/<perspective-slug>.md`. Optimized
for the coordinator to pull quotes from — structured repeatable
observation blocks, not prose.

```markdown
---
kind: observations
perspective: methodology-shifts
perspective_file: perspectives/methodology-shifts.md
pack: session-miner
pack_version: 0.1.0
corpus_project_slugs:
  - -home-admin-workspace-ck3proj
corpus_sessions: 8
corpus_date_range: 2026-03-05..2026-04-12
corpus_line_count: 84213
corpus_manifest_sha256: <hash>
generated_at: 2026-04-16T12:34:56Z
generator: session-miner/lens
schema_version: 1
---

# Observations: methodology-shifts

## OBS-001 — <one-line title>

**finding:** <2–5 sentences describing what's surprising/interesting/
unique and why.>

**evidence:**
- `<session_uuid>:L<line_number>` — "<quoted snippet>"
- `<session_uuid>:L<line_number>` — "<quoted snippet>"

**tags:** methodology, dead-end, workaround

---

## OBS-002 — <one-line title>
...
```

Every observation MUST carry at least one citation. Citations use
`<session_uuid>:L<line_number>` form; helper scripts the pack ships
make this cheap to produce.

### Draft post files

Written to `<output_dir>/posts/<slug>.md` by the coordinator. Slug is
coordinator-chosen.

```markdown
---
kind: draft-post
pack: session-miner
pack_version: 0.1.0
generator: session-miner/miner_coordinator
generated_at: 2026-04-16T13:10:00Z
corpus_manifest_sha256: <hash>
sources:
  - observations/methodology-shifts.md#OBS-001
  - observations/methodology-shifts.md#OBS-004
  - observations/cross-session-linkage.md#OBS-007
schema_version: 1
---

# <Post title the coordinator chose>

<blog-post-style prose, applying the operator's drafting instructions.
Quotes or paraphrases from observations, with links back to the
source blocks.>
```

### Index file

Written to `<output_dir>/posts/index.md`. One entry per drafted post:
title, output path, one-line rationale for why the coordinator judged
it post-worthy, source observation block IDs.

Downstream LLMWiki-style consumers can read either kind's frontmatter
for provenance without coupling to the body structure.

## Coordinator & drafting instructions

The coordinator's prompt template declares the general role: read lens
observation files, judge which themes have enough substance and
surprise to merit a draft post, produce zero or more markdown files
(coordinator's call — zero is allowed when nothing crosses the bar),
write an index. It ships as `prompts/coordinator.md.tmpl`.

The *stylistic* guidance — voice, target length, paragraph shape —
ships in the pack as `prompts/shared/default-drafting-style.md`,
written up-front in Harrison's voice based on observed preferences
(terse, direct, specific-over-generic, uncertainty expressed rather
than hidden, pragmatism over dogma, no summary-of-summary fluff).
Operator can override per-invocation with `--var drafting_instructions_path=<file>`
pointing at a project-local voice guide.

Flow:

1. Operator optionally passes `--var drafting_instructions_path=<file>`.
2. Orchestrator stashes the effective path on the coordinator bead —
   the supplied path if set and readable, otherwise the pack default.
3. Coordinator's prompt template includes an instruction: "Before
   drafting, read the file at the `drafting_instructions` metadata
   value on your bead. Treat its contents as authoritative for voice,
   length, structure, and scope."
4. Coordinator reads the file in-session and applies it.

The pack default is a living artifact — we can edit it globally, or
override per-project via the var. Voice guide is a deliverable of the
implementation phase; first draft will need an eyeball-and-adjust
review pass before the Phase-B demo.

## Incremental mining

**MVP: re-run from scratch, overwrite outputs.** Observation filenames
are deterministic (`<perspective-slug>.md`) so a re-run with the same
perspective files overwrites predictably. Operator git-commits between
runs if they want history.

**Extension path (non-breaking):** a `--var incremental=true` mode that:
- Reads existing output's `corpus_manifest_sha256` frontmatter.
- Diffs against the current manifest to produce a "new sessions" delta
  manifest.
- Passes a `previous_output_path` var on the lens and coordinator beads.
- Prompts handle "merge delta into existing output" explicitly.

The schema already has the provenance fields needed to make this work
later, so we don't need to pre-commit the mechanism now.

## Flow (concrete CLI)

```bash
gc formula cook session-mining \
  --title "Mine CK3 RE corpus" \
  --var project_slug=-home-admin-workspace-ck3proj \
  --var since=2026-03-01 \
  --var until=2026-04-16 \
  --var focus="methodology pivots, cross-session entity drift" \
  --var output_dir=/home/admin/mined/ck3 \
  --var drafting_instructions_path=/home/admin/mined/ck3/drafting-style.md
```

Optional overrides:
- `--var skip_planning=true --var perspectives=/path/a.md,/path/b.md`
  to bypass the planner with explicit perspective files.
- `--var skip_gate=true` for autonomous runs.

Formula steps (`session-mining.formula.toml`):

1. `resolve-corpus` — runs `scripts/resolve-corpus.sh`. Produces
   `corpus-manifest.json` at a known path. Records path in root-bead
   metadata.
2. `plan-perspectives` (needs: resolve-corpus) —
   - If `skip_planning=true`: copy the explicit `--var perspectives`
     files into `<output_dir>/perspectives/` and emit `index.json`.
     No bead, no planner dispatch.
   - Otherwise:
     - `bd create "Plan perspectives" --type task --parent <root-bead>`
     - `bd update <planner-bead> --set-metadata corpus_manifest=<path>`
     - `bd update <planner-bead> --set-metadata focus=<focus>`
     - `bd update <planner-bead> --set-metadata perspectives_dir=<output_dir>/perspectives`
     - `bd update <planner-bead> --set-metadata min_perspectives=3 max_perspectives=5`
     - `gc sling <rig>/perspective_planner <planner-bead> --nudge`
3. `await-planner` (needs: plan-perspectives) — poll on planner bead
   closure. Skipped (no-op) when `skip_planning=true`.
4. `human-gate` (needs: await-planner) —
   - If `skip_gate=true`: no-op.
   - Otherwise: `bd create "Review perspective files" --type task
     --parent <root-bead> --description "Review files at <path>, edit
     or delete any, then bd close this bead."`. Formula polls until
     bead closes.
5. `dispatch-lenses` (needs: human-gate) — read `<perspectives_dir>`
   to get the final list of files, then for each:
   - `bd create "Observe from <slug>" --type task --parent <root-bead>`
   - `bd update <child> --set-metadata perspective_path=<file>`
   - `bd update <child> --set-metadata corpus_manifest=<path>`
   - `bd update <child> --set-metadata observations_path=<output_dir>/observations/<slug>.md`
   - `gc sling <rig>/lens <child> --nudge`
6. `await-lenses` (needs: dispatch-lenses) — blocks on all lens child
   beads reaching closed status. Poll via helper script.
7. `dispatch-coordinator` (needs: await-lenses) —
   - `bd create "Draft posts from observations" --type task --parent <root-bead>`
   - `bd update <coord-bead> --set-metadata corpus_manifest=<path>`
   - `bd update <coord-bead> --set-metadata observations_dir=<output_dir>/observations`
   - `bd update <coord-bead> --set-metadata posts_dir=<output_dir>/posts`
   - `bd update <coord-bead> --set-metadata drafting_instructions=<path_or_pack_default>`
   - `gc sling <rig>/miner_coordinator <coord-bead> --nudge`
8. `await-coordinator` (needs: dispatch-coordinator) — poll on coord
   bead closure.
9. `report` (needs: await-coordinator) — writes summary:
   perspectives/ file list, observations/ file list, posts/ file list,
   path to index.md.

Every agent picks up its bead via `gc hook`, reads its metadata, and
executes its prompt against the supplied paths.

## Proposed directory structure

```
examples/session-miner/
├── README.md
├── city.toml                        # Minimal city configuring this pack for demo
└── packs/
    └── session-miner/
        ├── pack.toml
        ├── embed.go                 # //go:embed for bundling (optional for MVP)
        ├── prompts/
        │   ├── perspective-planner.md.tmpl
        │   ├── lens.md.tmpl
        │   ├── coordinator.md.tmpl
        │   └── shared/
        │       ├── observation-contract.md.tmpl   # fragment for lens prompt
        │       └── default-drafting-style.md      # read by coordinator when var unset
        ├── perspectives/
        │   └── examples/                           # fallback inspiration for the planner
        │       ├── methodology-walkthrough.md
        │       ├── dataset-taxonomy.md
        │       └── cross-session-linkage.md
        ├── scripts/
        │   ├── resolve-corpus.sh
        │   ├── jsonl-by-timestamp.sh
        │   ├── jsonl-sample.sh
        │   ├── jsonl-tool-uses-only.sh
        │   ├── await-children.sh
        │   └── prepare-explicit-perspectives.sh    # skip_planning branch
        ├── formulas/
        │   └── session-mining.formula.toml
        └── overlays/
            └── default/
                └── MINER_README.md      # What the agent sees in its work_dir
```

## Idiosyncratic shaping for Harrison's corpus

What I know about the operator's session corpus:
- ~112 project slugs in `~/.claude/projects/`, dominated by multi-agent
  Gas Town-style workloads (polecats, witnesses, mayor, crew, refinery,
  deacon, dogs).
- CK3 reverse engineering corpus at `-home-admin-workspace-ck3proj` —
  8 sessions totalling ~541MB, including a single 109MB session.
- Gas City / gascity sessions, bulletfarm, goose — all Gas-Town-native.

Pack concessions to this reality:
- (Deferred post-MVP: a `role_filter` var that whitelists slugs by
  Gas Town role.)
- The planner's prompt explicitly names the "a single session may
  exceed your context" case and tells it to flag this in the
  perspective's "Citation focus" section when relevant — pushing the
  partition-by-offset instruction downstream into the right
  perspective file rather than hardcoding it.
- The planner's prompt includes a Gas-Town-specific hint for
  multi-rig corpora: "entities often cross `crew/*`, `polecats/*`,
  `witness`, `refinery` slugs for the same rig — if you see this
  shape, a cross-session-linkage perspective is likely warranted".
- Example perspective files lean Gas-Town-aware: one is explicitly
  about the hand-off patterns between crew / polecat / witness roles.

## Open design questions (from operator brief)

1. **Lens declaration format.** → Perspectives are data (markdown
   files), not agent declarations. One generic `lens` pool agent,
   one `perspective_planner`, one `miner_coordinator`. Operator
   shapes output by editing perspective files at the human gate or
   supplying them explicitly via `--var perspectives=...`.
2. **Default vs project-specific lenses.** → No hardcoded defaults.
   Planner generates 3–5 corpus-grounded perspectives per run.
   Pack ships example perspective files as fallback inspiration for
   the planner's prompt, but they aren't the output.
3. **Corpus filtering.** → CLI vars: `project_slug` / `project_slugs` /
   `since` / `until` / `session_ids`. Resolved by a shell helper.
   (Gas-Town `role_filter` deferred post-MVP.)
4. **Partitioning strategy.** → Perspective file's "Citation focus"
   section encodes it, so the planner makes the call corpus-by-corpus.
   Pack ships helper scripts; lens decides how to use them.
5. **Coordinator role.** → `miner_coordinator` agent. Reads all
   observation files, drafts one blog-post-style markdown per theme
   it judges post-worthy. Operator supplies drafting-style guidance
   via `drafting_instructions_path` var.
6. **Output schema.** → Three `kind`s: `perspective` (planner output,
   instructions for a lens), `observations` (lens output,
   block-structured with citations), `draft-post` (coordinator
   output, prose). Shared provenance frontmatter across all three.
7. **Incremental mining.** → MVP re-runs from scratch. Schema has
   provenance fields so delta mode can land later non-breakingly.

## Demo run plan (after implementation)

Two-phase demo:

**Phase A — plumbing test on a small corpus.** Pick a small,
fast-iterating slug — likely a short-lived polecat or crew worktree
(single-digit session count, small total bytes). Goal: verify the
full pipeline end-to-end (resolve → plan → gate → lens pool → draft →
report) without waiting on the 109MB outlier. Do this first; don't
commit the phase-A artifacts.

**Phase B — real run against the whole shebang.** The CK3 RE corpus
(`-home-admin-workspace-ck3proj`, 8 sessions, ~541MB).
- Filters: `since=2026-03-01`, `until=2026-04-16`.
- No explicit focus; let the planner decide what angles the corpus
  warrants.
- Drafting instructions: the pack-shipped Harrison-voice default
  (see "Drafting style" section).
- Commit the produced artifacts to
  `examples/session-miner/example-output/ck3-re/`:
  - `perspectives/*.md` (planner-generated)
  - `observations/*.md` (one per perspective)
  - `posts/<slug>.md` (however many the coordinator produces)
  - `posts/index.md`

## Gas City config level

Lands at **Level 5** (formulas & molecules). Uses:
- Level 3 capabilities: multiple agents, pools.
- Level 5: formula-as-orchestrator.
Does not use orders (Level 7), messaging (Level 4), or health patrol
(Level 6).

## Out of scope for MVP

- No delta / incremental mode.
- No cross-lens / cross-perspective communication during observation.
- No embed.go bundling into the `gc` binary (keep as path-referenced
  pack first; bundle after the shape stabilizes).
- No custom partitioners as pluggable config — partitioning lives in
  perspective files' "Citation focus" sections.
- No output-format validation beyond "the agent produced a file at the
  declared path". Markdown shape is a social contract at MVP.
- No planner re-planning loop — one planning pass, then human gate,
  then straight through. If the operator wants different perspectives,
  they edit at the gate or re-run from scratch.

## Up for review

All structural and polish items settled across reviews 1–4:

- Three agents: `perspective_planner`, `lens` (pool max=4),
  `miner_coordinator` ✓
- Perspectives are data (markdown files), not agents ✓
- Planner generates 3–5 perspectives grounded in the corpus ✓
- Example perspective files exist as fallback inspiration only ✓
- Human gate after planning, with `skip_gate=true` autonomous bypass ✓
- Coordinator decides post count and titles; zero is allowed ✓
- Pack default drafting-style written up-front in Harrison's voice,
  overridable per invocation ✓
- Output directory defaults to `.gc/mined/<slug>/<yyyy-mm-dd>/`,
  `--var output_dir` overrides ✓
- `role_filter` deferred post-MVP ✓
- Demo runs in two phases: small-corpus plumbing test first, then
  full CK3 RE corpus as the committed example ✓

Ready to move to implementation. No more design questions open.
