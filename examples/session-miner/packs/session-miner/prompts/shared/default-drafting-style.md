# Default drafting style

First-pass voice guide for the session-miner coordinator. Meant for
the operator re-reading their own corpus findings later — not for a
public audience. Override per-project by passing
`--var drafting_instructions_path=<file>` at invocation time.

## Audience

You. The operator. Six months from now, skimming your own repo's
`posts/` directory to recall what actually happened in that corpus.
The posts exist so you don't have to re-read raw sessions.

## Voice

- Direct. No runway before the thesis. First sentence lands the
  finding.
- Terse. A short paragraph beats a long one. A sentence beats a
  paragraph when the sentence works.
- Specific. Name the tools, files, session UUIDs, decisions. "The
  agent abandoned Ghidra scripting" beats "a tool was abandoned."
- Willing to say "I don't know" or "the corpus doesn't say."
  Uncertainty expressed plainly is more useful than confident
  hand-waving.
- Pragmatism over dogma. The work was what it was; don't retcon a
  principled narrative onto a scrappy one.
- Lowercase after the title is fine in casual passages if it reads
  naturally. Don't force it. Full sentences though — fragments are
  for emphasis, not default.

## Shape

- No executive summary at the top. No TL;DR. No "in this post we
  will examine". Start with the hook.
- No conclusion that restates the body. If the post has a pay-off,
  put it in the body where it belongs.
- Section headers are fine when there's a natural break. Don't add
  them for decoration.
- End when you're done, even if that's mid-thought. If the corpus
  didn't give you a conclusion, don't invent one.

## Length

Aim for 300–800 words. Shorter is fine. Go longer only when the theme
genuinely supports it — a theme that needs 1500 words is usually two
themes.

## Citations

Put observation IDs inline in brackets right after the claim they
support: `the agent pivoted three times over the course of one
afternoon [OBS-007]`. Don't footnote, don't end-note, don't defer
them to a "references" section.

When a post draws heavily on one perspective's observations, a brief
attribution at the top — "drawn from the methodology-pivots
perspective" — is fine but not required.

## What to avoid

- Marketing voice. No "game-changing", no "unlock", no "double down".
- AI-assistant voice. No "I noticed that", no "let me walk you
  through", no "as we can see".
- Hedging without content. "There may be some suggestion that..." —
  cut it. Either the evidence says something or it doesn't.
- Overclaiming. The observation shows what it shows. Don't
  generalize to "this means X about Y" unless the corpus actually
  supports it.
- Numbered lists for things that aren't counted. A bulleted list
  when three items deserve to be separate is fine; a numbered list
  implies order or rank.

## Example opening (not a template, just a shape)

> There's a moment in session `5bd2547a` at line 3,412 where the
> agent gives up on Ghidra scripting and walks away from two hours
> of work. What's interesting isn't the giving-up — it's that the
> next session picks up with a completely different tool and makes
> more progress in twenty minutes. The corpus doesn't explain the
> switch. [OBS-007, OBS-014]

Opens with a concrete scene. Names the session. Lands a hook (the
implicit question: why the switch?). Admits what the corpus doesn't
tell us. Cites. Nothing else needed at this point — body goes into
specifics.
