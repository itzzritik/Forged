---
title: KB Meta
applies_to:
  - .agents/docs/**
last_verified: 2026-04-23
stable: yes
---

# Knowledge Base Meta

`.agents/docs/` is a small agent KB, not a plan, changelog, or code tour.

## Rules

- Document only what is hard to recover from code quickly:
  - invariants
  - surprising behavior
  - important decisions
  - real risks or traps
- If code already makes something obvious, leave it out.
- Cut stale or obvious text instead of appending more context.
- No `file:line` notes, no history, no route dumps, no exhaustive field lists.
- Link to `proto/*.md` or another shard instead of duplicating specs.

## Shape

- Keep shards short. Target 15–40 lines. Hard ceiling 60.
- Use only these sections when needed:
  - short intro
  - `## Must know`
  - `## Decisions`
- Omit a section if there is nothing important to say.

## Updating

- Update a shard only when its non-obvious behavior changed.
- Bump `last_verified` whenever you touch it.
- While touching a shard, remove stale or low-value text.
