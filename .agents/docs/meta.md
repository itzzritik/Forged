---
title: KB Meta
applies_to:
  - .agents/docs/**
last_verified: 2026-04-21
stable: yes
---

# Knowledge Base Meta

Rules for reading, writing, and maintaining `.agents/docs/`.

## Purpose

`.agents/docs/` holds conditional product knowledge for AI agents. Each
shard is the *minimum* an agent needs to work on a subsystem safely
without grepping the code blindly. Anything an agent can trivially derive
from the code does NOT belong here.

Agents load only the shard(s) matched by the routing table in root
`AGENTS.md`. Everything else, including per-file detail, is read directly
from the code.

This KB is **not** a plan, a roadmap, a changelog, or historical record.
Those live in `.agents/plan/` (plans) or in git log (history).

## Writing Style

- Describe **what the product does conceptually**, not what the code does.
- No `file:line` citations. No code-location pointers. No line numbers.
- Only include what is **non-obvious from reading the code**:
  - Invariants and gotchas an agent would regret missing.
  - Security gaps, tech debt, and known broken behavior worth a warning.
  - Decisions an agent might unwittingly undo.
- Skip: verbose "what it does" prose, exhaustive boundary lists,
  bullet-listing things the agent will see as soon as they open a file.
- Link to `proto/*.md` and other shards; never duplicate them.
- Present tense. `IF X THEN Y` for conditional rules.

## Shard Format

```markdown
---
title: <short name>
applies_to:
  - <glob>
depends_on:
  - <other shard, optional>
last_verified: YYYY-MM-DD
stable: yes | partial | in-flux
---

# <Title>

<One or two sentences: what the subsystem is, at the product level.>

## Must know

- <Non-obvious invariant, gap, or surprise>
- <Non-obvious invariant, gap, or surprise>

## Decisions

- <Current decision> — <short reason>. <Reversal warning if tempting>.
```

Sections are collapsible. Omit `## Decisions` if the subsystem has no
reversal-sensitive choices. Do not invent `## Boundaries` / `## What it does`
sections to fill space — if there's nothing non-obvious to say, the shard
is short.

## Size Target

- Typical: 20–50 lines including frontmatter.
- Hard ceiling: 80 lines. Over the ceiling → the shard is probably
  describing too much code and should be cut, not split.

## What NOT to Put in a Shard

- `[path:line]` citations or code-location pointers.
- Phase numbers, roadmap items, completion checklists.
- References to active `.agents/plan/` content except when a subsystem is
  *actively being reworked* and the `stable` field is `partial`.
- Tutorial prose or explanations a reader could get from the code itself.
- Exhaustive command lists, field lists, route lists. Link to specs.
- Duplication of `proto/*.md`.

## Update Triggers

Shards are updated at three moments (also in root `AGENTS.md`):

1. **Plan completion** — a plan cannot flip to `done` until its shards
   reflect the shipped product behavior.
2. **Mid-work reversal** — if work-in-progress changes a design decision
   a shard describes, update the shard in the same session.
3. **Orphan edits** — ad-hoc changes outside any plan. If the routing
   table loads a shard and the edit changes what it says, update the
   shard in the same change set.

Always bump `last_verified` when content is touched.

## Accuracy Posture

Human- and agent-verified at update time against observed product
behavior. No mechanical checks. If a shard drifts, the next agent
loading it catches it on the next real change.

## Creating a New Shard

Required when:

- A new subsystem appears with no home.
- An existing shard exceeds 80 lines and can't be shortened.

When creating:

1. Add a routing-table row in root `AGENTS.md`.
2. Add an entry to `.agents/docs/INDEX.md`.
3. Author per this format.
